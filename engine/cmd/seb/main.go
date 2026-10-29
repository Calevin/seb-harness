package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	dbPath         = ".seb/seb-audit.db"
	defaultTimeout = 120 * time.Second
	maxSnippetLen  = 1500 // Protege la ventana de contexto del agente
)

type CheckItem struct {
	ID      string
	ReqID   string
	Desc    string
	Command string
}

type SebLock struct {
	Tasks map[string]TaskLock `json:"tasks"`
}

type TaskLock struct {
	SpecHash     string `json:"spec_hash"`
	VerifierHash string `json:"verifier_hash"`
	Verified     bool   `json:"verified,omitempty"`
	CommitHash   string `json:"commit_hash,omitempty"`
}

// ModelPricing define las tarifas monetarias por millon de tokens
type ModelPricing struct {
	PromptPricePerMillion     float64 `json:"prompt_price_per_million"`
	CompletionPricePerMillion float64 `json:"completion_price_per_million"`
}

// EngineConfig representa la configuracion local opcional de .seb/config.json
type EngineConfig struct {
	Pricing map[string]ModelPricing `json:"pricing"`
}

// UsageRecord representa la linea estructurada de un archivo usage.jsonl
type UsageRecord struct {
	Timestamp        string `json:"timestamp"`
	Turns            int    `json:"turns"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	ApprovalRequired bool   `json:"approval_required"`
	Outcome          string `json:"outcome"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	// Init puede crear el directorio .seb antes de abrir la DB
	if cmd == "init" {
		if err := ensureDirectories(); err != nil {
			fatal("Error creando directorios: %v", err)
		}
	}

	db, err := openDB()
	if err != nil {
		fatal("Error abriendo SQLite (%s): %v", dbPath, err)
	}
	defer db.Close()

	switch cmd {
	case "init":
		_ = clearActiveRole()
		fmt.Println("[seb-harness] Estructura y base de datos SQLite inicializadas correctamente.")
	case "spec-next":
		cmdSpecNext(db, args)
	case "status":
		cmdStatus(db, args)
	case "plan-lock":
		cmdPlanLock(db, args)
	case "verify":
		cmdVerify(db, args)
	case "memory-add":
		cmdMemoryAdd(db, args)
	case "guard":
		cmdGuard(db, args)
	case "audit":
		cmdAudit(db, args)
	case "hook":
		cmdHook(db, args)
	case "role":
		if err := cmdRole(db, args); err != nil {
			fatal("%v", err)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}

func ensureDirectories() error {
	dirs := []string{
		".seb/bin",
		".agents/rules",
		".agents/skills/seb-iniciador",
		".agents/skills/seb-especificador",
		".agents/skills/seb-clarificador",
		".agents/skills/seb-planificador",
		".agents/skills/seb-verificador",
		".agents/hooks",
		"docs",
		"specs",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}

func initDBSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		spec_path TEXT NOT NULL,
		status TEXT NOT NULL,
		spec_sha256 TEXT DEFAULT '',
		verifier_sha256 TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS agent_actions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		task_id TEXT DEFAULT '',
		agent TEXT NOT NULL,
		skill TEXT DEFAULT '',
		action TEXT NOT NULL,
		parameters TEXT DEFAULT '',
		result_summary TEXT DEFAULT '',
		level TEXT NOT NULL DEFAULT 'autonomous',
		duration_ms INTEGER DEFAULT 0,
		tokens_in INTEGER DEFAULT 0,
		cache_read_tokens INTEGER DEFAULT 0,
		tokens_out INTEGER DEFAULT 0,
		cost_usd REAL DEFAULT 0.0,
		exit_code INTEGER DEFAULT 0,
		step_idx INTEGER DEFAULT -1,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_actions_session_ts ON agent_actions(session_id, timestamp);
	CREATE INDEX IF NOT EXISTS idx_actions_agent_ts ON agent_actions(agent, timestamp);
	CREATE INDEX IF NOT EXISTS idx_actions_task ON agent_actions(task_id);

	CREATE TABLE IF NOT EXISTS verifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT NOT NULL,
		check_id TEXT NOT NULL,
		req_id TEXT NOT NULL,
		command TEXT NOT NULL,
		status TEXT NOT NULL,
		exit_code INTEGER NOT NULL,
		duration_ms INTEGER NOT NULL,
		output_snippet TEXT DEFAULT '',
		commit_hash TEXT DEFAULT '',
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_verif_task_ts ON verifications(task_id, timestamp);
	`
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	return migrateDBSchema(db)
}

func migrateDBSchema(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(agent_actions)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasCacheRead := false
	hasStepIdx := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
			if name == "cache_read_tokens" {
				hasCacheRead = true
			}
			if name == "step_idx" {
				hasStepIdx = true
			}
		}
	}

	if !hasCacheRead {
		if _, err := db.Exec(`ALTER TABLE agent_actions ADD COLUMN cache_read_tokens INTEGER DEFAULT 0`); err != nil {
			return err
		}
	}
	if !hasStepIdx {
		if _, err := db.Exec(`ALTER TABLE agent_actions ADD COLUMN step_idx INTEGER DEFAULT -1`); err != nil {
			return err
		}
	}
	return nil
}

func openDB() (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if err := initDBSchema(db); err != nil {
		return nil, err
	}
	return db, nil
}

// TaskInfo representa los metadatos completos y rutas canonicas de una tarea
type TaskInfo struct {
	ID            string
	Name          string
	CanonicalName string
	SpecPath      string
	PlanPath      string
	VerifierPath  string
	Status        string
}

// buildArtifactPath construye de manera determinista la ruta con prefijo para cualquier tipo de artefacto
func buildArtifactPath(id, name, kind string) string {
	return filepath.Join("specs", fmt.Sprintf("%s_%s_%s.md", id, name, kind))
}

// resolveTask resuelve de forma flexible una tarea por su ID, nombre simple o clave canonica
func resolveTask(db *sql.DB, query string) (TaskInfo, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return TaskInfo{}, errors.New("identificador de tarea vacio")
	}

	// Normalizar numero simple a 3 digitos con ceros a la izquierda (ej. "1" -> "001")
	if matched, _ := regexp.MatchString(`^\d+$`, q); matched {
		var num int
		fmt.Sscanf(q, "%d", &num)
		q = fmt.Sprintf("%03d", num)
	}

	tasksMap := make(map[string]TaskInfo)

	// 1. Consultar tareas registradas en la base de datos local SQLite si esta disponible
	if db != nil {
		rows, err := db.Query(`SELECT id, name, status, spec_path FROM tasks`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id, name, status, specPath string
				if err := rows.Scan(&id, &name, &status, &specPath); err == nil {
					canonical := fmt.Sprintf("%s_%s", id, name)
					tasksMap[canonical] = TaskInfo{
						ID:            id,
						Name:          name,
						CanonicalName: canonical,
						SpecPath:      specPath,
						PlanPath:      buildArtifactPath(id, name, "plan"),
						VerifierPath:  buildArtifactPath(id, name, "verificador"),
						Status:        status,
					}
				}
			}
		}
	}

	// 2. Escanear specs fisicos en disco para autonomia multi-maquina sin DB inicializada
	specFiles, _ := filepath.Glob("specs/*_*_spec.md")
	reSpec := regexp.MustCompile(`^specs/(\d{3})_(.+)_spec\.md$`)
	for _, f := range specFiles {
		m := reSpec.FindStringSubmatch(f)
		if len(m) == 3 {
			id := m[1]
			name := m[2]
			canonical := fmt.Sprintf("%s_%s", id, name)
			if _, exists := tasksMap[canonical]; !exists {
				tasksMap[canonical] = TaskInfo{
					ID:            id,
					Name:          name,
					CanonicalName: canonical,
					SpecPath:      f,
					PlanPath:      buildArtifactPath(id, name, "plan"),
					VerifierPath:  buildArtifactPath(id, name, "verificador"),
				}
			}
		}
	}

	// 3. Filtrar coincidencias por ID, Name o CanonicalName
	var matches []TaskInfo
	for _, t := range tasksMap {
		if t.ID == q || t.Name == q || t.CanonicalName == q {
			matches = append(matches, t)
		}
	}

	if len(matches) == 0 {
		return TaskInfo{}, fmt.Errorf("tarea no encontrada: '%s'", query)
	}
	if len(matches) > 1 {
		var candidates []string
		for _, m := range matches {
			candidates = append(candidates, m.CanonicalName)
		}
		return TaskInfo{}, fmt.Errorf("identificador ambiguo: '%s' coincide con multiples tareas (%s)", query, strings.Join(candidates, ", "))
	}

	t := matches[0]
	if t.SpecPath == "" {
		t.SpecPath = buildArtifactPath(t.ID, t.Name, "spec")
	}
	t.PlanPath = buildArtifactPath(t.ID, t.Name, "plan")
	t.VerifierPath = buildArtifactPath(t.ID, t.Name, "verificador")

	return t, nil
}

// 1. seb spec-next <nombre_tarea>
func cmdSpecNext(db *sql.DB, args []string) {
	start := time.Now()
	if len(args) < 1 {
		fatal("Uso: seb spec-next <nombre_tarea>")
	}
	taskName := sanitizeName(args[0])

	// 1. Escanear el directorio specs/ para encontrar el prefijo numérico más alto real
	files, _ := filepath.Glob("specs/*_*_spec.md")
	maxID := 0
	re := regexp.MustCompile(`^specs/(\d{3})_`)

	for _, f := range files {
		matches := re.FindStringSubmatch(f)
		if len(matches) == 2 {
			var id int
			fmt.Sscanf(matches[1], "%d", &id)
			if id > maxID {
				maxID = id
			}
		}
	}

	nextID := fmt.Sprintf("%03d", maxID+1)
	specPath := buildArtifactPath(nextID, taskName, "spec")

	// 2. Sincronizar idempotentemente la base de datos local
	_, err := db.Exec(`
		INSERT INTO tasks (id, name, spec_path, status)
		VALUES (?, ?, ?, 'SPECIFIED')
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, spec_path=excluded.spec_path`,
		nextID, taskName, specPath)
	if err != nil {
		fatal("Error sincronizando tarea en SQLite: %v", err)
	}

	dur := time.Since(start).Milliseconds()
	logInternalAction(db, nextID, "seb-especificador", "spec-next", taskName, specPath, dur, 0)
	fmt.Println(specPath)
}

// 2. seb plan-lock <nombre_tarea> (Sella hashes SHA-256 de spec y verificador)
func cmdPlanLock(db *sql.DB, args []string) {
	start := time.Now()
	if len(args) < 1 {
		fatal("Uso: seb plan-lock <nombre_o_id_tarea>")
	}
	task, err := resolveTask(db, args[0])
	if err != nil {
		fatal("Error resolviendo tarea: %v", err)
	}

	// Verificar que la spec no tenga etiquetas [NECESITA ACLARACIÓN] pendientes
	specBytes, err := os.ReadFile(task.SpecPath)
	if err != nil {
		fatal("No se pudo leer %s: %v", task.SpecPath, err)
	}
	if strings.Contains(string(specBytes), "[NECESITA ACLARACIÓN]") {
		fatal("BLOQUEO SDD: La spec '%s' aún contiene etiquetas [NECESITA ACLARACIÓN]. Ejecuta el skill seb-clarificador antes de planificar.", task.SpecPath)
	}

	checks, err := parseVerifierFile(task.VerifierPath)
	if err != nil {
		fatal("BLOQUEO SDD: Error en '%s': %v", task.VerifierPath, err)
	}
	if len(checks) == 0 {
		fatal("BLOQUEO SDD: '%s' no existe o no contiene comandos de verificación programática válidos.", task.VerifierPath)
	}

	// Guardar hashes en seb.lock versionado por Git con clave canonica
	lock := readLockFile()

	// Guardarrail de Dependencias Directas (DAG)
	deps, err := parseSpecDependencies(task.SpecPath)
	if err != nil {
		fatal("Error analizando dependencias de %s: %v", task.SpecPath, err)
	}
	if len(deps) > 0 {
		if err := validateDirectDependencies(db, task, deps, lock); err != nil {
			fatal("BLOQUEO SDD: %v", err)
		}
	}

	specHash, _ := fileSHA256(task.SpecPath)
	verifHash, _ := fileSHA256(task.VerifierPath)

	existingLock := lock.Tasks[task.CanonicalName]
	lock.Tasks[task.CanonicalName] = TaskLock{
		SpecHash:     specHash,
		VerifierHash: verifHash,
		Verified:     existingLock.Verified,
		CommitHash:   existingLock.CommitHash,
	}
	writeLockFile(lock)

	// Marcar la tarea como activa para el pre-commit hook con clave canonica
	setActiveTask(task.CanonicalName)
	_ = setActiveRole("implementador")

	// Actualizar estado en SQLite (solo para observabilidad local)
	_, _ = db.Exec(`UPDATE tasks SET status = 'PLANNED', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, task.ID)

	dur := time.Since(start).Milliseconds()
	logInternalAction(db, task.ID, "seb-planificador", "plan-lock", task.CanonicalName, fmt.Sprintf("checks=%d", len(checks)), dur, 0)
	fmt.Printf("[seb-harness] Plan sellado en .seb/seb.lock y marcado como activo para '%s'.\n", task.CanonicalName)
}

// 3. seb verify <nombre_tarea>
func cmdVerify(db *sql.DB, args []string) {
	startVerify := time.Now()
	if len(args) < 1 {
		fatal("Uso: seb verify <nombre_o_id_tarea>")
	}
	task, err := resolveTask(db, args[0])
	if err != nil {
		fatal("Error resolviendo tarea: %v", err)
	}

	// Control de Integridad Anti-Reward-Hacking
	// Leer hashes desde seb.lock versionado por Git
	lock := readLockFile()
	taskLock, exists := lock.Tasks[task.CanonicalName]
	if !exists {
		// Fallback para claves previas sin prefijo durante transicion
		taskLock, exists = lock.Tasks[task.Name]
	}

	if exists {
		if taskLock.SpecHash != "" {
			currentSpecHash, _ := fileSHA256(task.SpecPath)
			if currentSpecHash != taskLock.SpecHash {
				fatal("VIOLACIÓN DE INTEGRIDAD (Anti-Reward-Hacking): '%s' modificado.", task.SpecPath)
			}
		}
		if taskLock.VerifierHash != "" {
			currentVerifHash, _ := fileSHA256(task.VerifierPath)
			if currentVerifHash != taskLock.VerifierHash {
				fatal("VIOLACIÓN DE INTEGRIDAD (Anti-Reward-Hacking): '%s' modificado.", task.VerifierPath)
			}
		}
	}

	checks, err := parseVerifierFile(task.VerifierPath)
	if err != nil {
		fatal("Error en verificador '%s': %v", task.VerifierPath, err)
	}
	if len(checks) == 0 {
		fatal("No se encontraron controles ejecutables en %s.", task.VerifierPath)
	}

	commitHash := currentGitCommit()
	passed := 0
	failed := 0

	fmt.Printf("## Reporte de Verificación Determinista: `%s` (Spec `%s`)\n\n", task.CanonicalName, task.ID)
	fmt.Println("| Check ID | Requisito | Estado | Tiempo (ms) | Comando |")
	fmt.Println("| :--- | :--- | :--- | :--- | :--- |")

	var failureDetails strings.Builder

	for _, c := range checks {
		start := time.Now()
		exitCode, output := runCommandWithTimeout(c.Command, defaultTimeout)
		duration := time.Since(start).Milliseconds()

		status := "PASS"
		badge := "[PASA]"
		if exitCode != 0 {
			status = "FAIL"
			badge = "[FALLA]"
			failed++
			failureDetails.WriteString(fmt.Sprintf("\n### ❌ Detalle de Fallo: `%s` (%s - %s)\n", c.ID, c.ReqID, c.Desc))
			failureDetails.WriteString(fmt.Sprintf("- **Comando:** `%s`\n- **Exit Code:** `%d`\n```text\n%s\n```\n", c.Command, exitCode, output))
		} else {
			passed++
		}

		fmt.Printf("| `%s` | `%s` | **%s** | %d ms | `%s` |\n", c.ID, c.ReqID, badge, duration, c.Command)

		_, _ = db.Exec(`INSERT INTO verifications (task_id, check_id, req_id, command, status, exit_code, duration_ms, output_snippet, commit_hash)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			task.ID, c.ID, c.ReqID, c.Command, status, exitCode, duration, output, commitHash)
	}

	fmt.Printf("\n**Resumen de Cobertura:** %d/%d controles en `[PASA]`, %d en `[FALLA]`.\n", passed, len(checks), failed)

	durVerify := time.Since(startVerify).Milliseconds()
	if failed > 0 {
		logInternalAction(db, task.ID, "seb-verificador", "verify", task.CanonicalName, fmt.Sprintf("passed=%d failed=%d", passed, failed), durVerify, 2)
		_, _ = db.Exec(`UPDATE tasks SET status = 'FAILED_VERIFICATION', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, task.ID)
		fmt.Println(failureDetails.String())
		fmt.Println("\n**VEREDICTO:** `PENDIENTE DE IMPLEMENTACIÓN` — Corrige el código de aplicación sin alterar `*_verificador.md`.")
		os.Exit(2)
	}

	// Liberar el hook pre-commit porque la tarea pasó la verificación
	clearActiveTask()
	_ = clearActiveRole()
	_ = updateTaskLockVerification(task.CanonicalName, commitHash)
	logInternalAction(db, task.ID, "seb-verificador", "verify", task.CanonicalName, fmt.Sprintf("passed=%d failed=%d", passed, failed), durVerify, 0)
	_, _ = db.Exec(`UPDATE tasks SET status = 'VERIFIED', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, task.ID)
	fmt.Println("\n**VEREDICTO:** `APROBADO` — Todos los controles deterministas pasaron con éxito.")
}

// Identificadores y analizador de tablas Markdown en specs/<tarea>_verificador.md
var (
	checkCandidateRegex = regexp.MustCompile(`^\s*\|\s*CHK-`)
	checkIDRegex        = regexp.MustCompile(`^CHK-[A-Za-z0-9_-]+$`)
)

func parseVerifierFile(path string) ([]CheckItem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var items []CheckItem
	seenIDs := make(map[string]int)
	inFencedBlock := false
	lineNum := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNum++
		rawLine := scanner.Text()
		trimmedLine := strings.TrimSpace(rawLine)

		// Rastrear bloques de codigo cercados para otorgarles inmunidad contextual
		if strings.HasPrefix(trimmedLine, "```") {
			inFencedBlock = !inFencedBlock
			continue
		}
		if inFencedBlock {
			continue
		}

		// Filtrar lineas que no correspondan a una fila candidata a control
		if !checkCandidateRegex.MatchString(trimmedLine) {
			continue
		}

		// Validacion de fila candidata: rechazo de omisiones silenciosas (Anti-Silent-Drop)
		if !strings.HasSuffix(trimmedLine, "|") {
			return nil, fmt.Errorf("línea %d: fila de control malformada, falta delimitador final '|'", lineNum)
		}

		// Division en celdas respetando backticks para aislar comandos con pipes u operadores de shell
		var cells []string
		var current strings.Builder
		inBacktick := false

		for _, r := range trimmedLine {
			switch r {
			case '`':
				inBacktick = !inBacktick
				current.WriteRune(r)
			case '|':
				if !inBacktick {
					cells = append(cells, current.String())
					current.Reset()
				} else {
					current.WriteRune(r)
				}
			default:
				current.WriteRune(r)
			}
		}
		cells = append(cells, current.String())

		if inBacktick {
			return nil, fmt.Errorf("línea %d: fila de control con comillas invertidas no cerradas", lineNum)
		}

		// Se esperan exactamente 4 columnas entre delimitadores de tabla (total 6 celdas: inicio, 4 columnas, fin)
		if len(cells) != 6 || strings.TrimSpace(cells[0]) != "" || strings.TrimSpace(cells[5]) != "" {
			return nil, fmt.Errorf("línea %d: estructura de columnas inválida en fila de control (se esperaban 4 columnas: Check ID, Requisito, Descripción, Comando)", lineNum)
		}

		// 1. Check ID y control de unicidad
		checkID := strings.TrimSpace(cells[1])
		if !checkIDRegex.MatchString(checkID) {
			return nil, fmt.Errorf("línea %d: formato de identificador de control inválido '%s'", lineNum, checkID)
		}
		if prevLine, exists := seenIDs[checkID]; exists {
			return nil, fmt.Errorf("línea %d: identificador de control duplicado '%s' (declarado previamente en línea %d)", lineNum, checkID, prevLine)
		}
		seenIDs[checkID] = lineNum

		// 2. Requisitos heterogeneos y normalizacion de listas (eliminacion de espacios adyacentes)
		rawReq := strings.TrimSpace(cells[2])
		if rawReq == "" {
			return nil, fmt.Errorf("línea %d: el campo de requisitos no puede estar vacío", lineNum)
		}
		reqParts := strings.Split(rawReq, ",")
		var cleanReqs []string
		for _, p := range reqParts {
			trimmedP := strings.TrimSpace(p)
			if trimmedP == "" {
				return nil, fmt.Errorf("línea %d: requisito vacío en lista de requisitos '%s'", lineNum, rawReq)
			}
			cleanReqs = append(cleanReqs, trimmedP)
		}
		normalizedReq := strings.Join(cleanReqs, ",")

		// 3. Descripcion
		desc := strings.TrimSpace(cells[3])
		if desc == "" {
			return nil, fmt.Errorf("línea %d: la descripción del control no puede estar vacía", lineNum)
		}

		// 4. Comando entre backticks
		rawCmdCell := strings.TrimSpace(cells[4])
		if !strings.HasPrefix(rawCmdCell, "`") || !strings.HasSuffix(rawCmdCell, "`") || len(rawCmdCell) < 2 {
			return nil, fmt.Errorf("línea %d: delimitadores de comando inválidos (el comando debe estar encerrado entre comillas invertidas)", lineNum)
		}
		command := strings.TrimSpace(rawCmdCell[1 : len(rawCmdCell)-1])
		if command == "" {
			return nil, fmt.Errorf("línea %d: el comando de verificación no puede estar vacío", lineNum)
		}

		items = append(items, CheckItem{
			ID:      checkID,
			ReqID:   normalizedReq,
			Desc:    desc,
			Command: command,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("el archivo no contiene controles ejecutables válidos")
	}

	return items, nil
}

func runCommandWithTimeout(cmdStr string, timeout time.Duration) (int, string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	output := strings.TrimSpace(out.String())
	if len(output) > maxSnippetLen {
		output = output[:maxSnippetLen/2] + "\n...[TRUNCADO POR SEB-HARNESS PARA PROTEGER CONTEXTO]...\n" + output[len(output)-(maxSnippetLen/2):]
	}

	if ctx.Err() == context.DeadlineExceeded {
		return 124, fmt.Sprintf("TIMEOUT EXCEDIDO (%v)\n%s", timeout, output)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), output
		}
		return 1, output + "\n" + err.Error()
	}
	return 0, output
}

// 4. seb guard <role> <action> <target_path> (Principio de Menor Privilegio)
func cmdGuard(db *sql.DB, args []string) {
	start := time.Now()
	if len(args) < 3 {
		fatal("Uso: seb guard <rol_o_skill> <write|exec> <ruta_o_comando>")
	}
	role := strings.TrimSpace(args[0])
	action := strings.TrimSpace(args[1])
	target := filepath.Clean(strings.TrimSpace(args[2]))

	allowed := checkPermission(role, action, target)
	if !allowed {
		dur := time.Since(start).Milliseconds()
		logInternalAction(db, "", role, "guard-blocked:"+action, target, "DENIED", dur, 1)
		fatal("GUARDARRAÍL DE MENOR PRIVILEGIO: El rol '%s' NO tiene permiso para '%s' sobre '%s'.", role, action, target)
	}
}

func checkPermission(role, action, target string) bool {
	if action != "write" {
		return true
	}
	cleanRole := normalizeRole(role)
	switch cleanRole {
	case "seb-iniciador":
		return target == "docs/constitution.md" || target == "MEMORY.md" || target == "AGENTS.md"
	case "seb-especificador":
		return strings.HasPrefix(target, "specs/") && strings.HasSuffix(target, "_spec.md")
	case "seb-clarificador":
		return (strings.HasPrefix(target, "specs/") && strings.HasSuffix(target, "_spec.md")) || target == "MEMORY.md"
	case "seb-planificador":
		return (strings.HasPrefix(target, "specs/") && (strings.HasSuffix(target, "_plan.md") || strings.HasSuffix(target, "_verificador.md"))) || target == "MEMORY.md"
	case "seb-verificador":
		return target == "MEMORY.md"
	case "implementador", "default", "":
		// El agente implementador jamás puede tocar la constitución, specs ni verificadores
		if target == "docs/constitution.md" ||
			(strings.HasPrefix(target, "specs/") && (strings.HasSuffix(target, "_spec.md") || strings.HasSuffix(target, "_verificador.md"))) {
			return false
		}
		return true
	default:
		// Habilidades de soporte o dominio desconocidas (RF-07) heredan la política de implementador:
		// Tienen permiso para modificar código de aplicación pero NO contratos protegidos (RF-08).
		if target == "docs/constitution.md" ||
			(strings.HasPrefix(target, "specs/") && (strings.HasSuffix(target, "_spec.md") || strings.HasSuffix(target, "_verificador.md"))) {
			return false
		}
		return true
	}
}

// 5. seb memory-add "<categoria>" "<texto>"
func cmdMemoryAdd(db *sql.DB, args []string) {
	start := time.Now()
	if len(args) < 2 {
		fatal("Uso: seb memory-add \"<categoría>\" \"<descripción>\"")
	}
	category := args[0]
	entry := args[1]
	dateStr := time.Now().Format("2006-01-02")
	line := fmt.Sprintf("- **[%s] [%s]** %s\n", dateStr, category, entry)

	f, err := os.OpenFile("MEMORY.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fatal("Error abriendo MEMORY.md: %v", err)
	}
	defer f.Close()

	if _, err := f.WriteString(line); err != nil {
		fatal("Error escribiendo en MEMORY.md: %v", err)
	}
	dur := time.Since(start).Milliseconds()
	logInternalAction(db, "", "seb-cli", "memory-add", category, entry, dur, 0)
	fmt.Println("[seb-harness] Entrada registrada en MEMORY.md y auditada en SQLite.")
}

// 6. seb status [nombre_tarea]
func cmdStatus(db *sql.DB, args []string) {
	rows, err := db.Query(`SELECT id, name, status, spec_path, updated_at FROM tasks ORDER BY id ASC`)
	if err != nil {
		fatal("Error consultando tareas: %v", err)
	}
	defer rows.Close()

	lock := readLockFile()

	fmt.Println("| ID | Clave Canónica | Estado SDD | Dependencias | Archivo Spec | Última Actualización |")
	fmt.Println("| :--- | :--- | :--- | :--- | :--- | :--- |")
	for rows.Next() {
		var id, name, status, specPath, updated string
		_ = rows.Scan(&id, &name, &status, &specPath, &updated)
		canonical := fmt.Sprintf("%s_%s", id, name)

		deps, _ := parseSpecDependencies(specPath)
		depStr := "-"
		if len(deps) > 0 {
			var depFormatted []string
			for _, d := range deps {
				depTask, err := resolveTask(db, d)
				if err != nil {
					depFormatted = append(depFormatted, fmt.Sprintf("`%s` [HUERFANA]", d))
					continue
				}
				tLock, ok := lock.Tasks[depTask.CanonicalName]
				if !ok {
					tLock = lock.Tasks[depTask.Name]
				}
				if tLock.Verified && tLock.CommitHash != "" {
					depFormatted = append(depFormatted, fmt.Sprintf("`%s` [OK]", depTask.ID))
				} else {
					depFormatted = append(depFormatted, fmt.Sprintf("`%s` [BLOQUEADA]", depTask.ID))
				}
			}
			depStr = strings.Join(depFormatted, ", ")
		}

		fmt.Printf("| `%s` | `%s` | **%s** | %s | `%s` | %s |\n", id, canonical, status, depStr, specPath, updated)
	}
}

// 7. seb audit <log|import-usage|report>
func cmdAudit(db *sql.DB, args []string) {
	if len(args) < 1 {
		fatal("Uso: seb audit <log|import-usage|report> [flags]")
	}
	sub := args[0]
	switch sub {
	case "log":
		fs := flag.NewFlagSet("audit log", flag.ExitOnError)
		sessionID := fs.String("session", os.Getenv("SEB_SESSION_ID"), "ID de sesión")
		taskID := fs.String("task", os.Getenv("SEB_TASK_ID"), "ID de tarea (ej. 001)")
		agent := fs.String("agent", os.Getenv("SEB_AGENT"), "Agente (pi / agy)")
		skill := fs.String("skill", os.Getenv("SEB_SKILL"), "Skill activo")
		action := fs.String("action", "tool_use", "Nombre de tool o acción")
		params := fs.String("params", "", "Argumentos resumidos")
		result := fs.String("result", "", "Resumen del resultado")
		level := fs.String("level", "autonomous", "Nivel (autonomous|supervised|always_human)")
		duration := fs.Int64("duration-ms", 0, "Duración en milisegundos")
		tokensIn := fs.Int64("tokens-in", 0, "Tokens de entrada")
		cacheRead := fs.Int64("cache-read-tokens", 0, "Tokens leídos de caché")
		tokensOut := fs.Int64("tokens-out", 0, "Tokens de salida")
		costUSD := fs.Float64("cost-usd", 0.0, "Costo estimado en USD")
		model := fs.String("model", os.Getenv("SEB_MODEL"), "Modelo de lenguaje")
		exitCode := fs.Int("exit-code", 0, "Código de salida")
		_ = fs.Parse(args[1:])

		if *sessionID == "" {
			*sessionID = "local-" + time.Now().Format("20060102")
		}
		if *agent == "" {
			*agent = "unknown-agent"
		}

		resolvedTask := resolveActiveTask(*taskID, os.Getenv("SEB_TASK_ID"), ".seb/ACTIVE_TASK", db)
		normDuration := normalizeDuration(*duration)
		finalCost := calculateCost(*costUSD, *model, *tokensIn, *tokensOut, ".seb/config.json")

		_, err := db.Exec(`INSERT INTO agent_actions
			(session_id, task_id, agent, skill, action, parameters, result_summary, level, duration_ms, tokens_in, cache_read_tokens, tokens_out, cost_usd, exit_code)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			*sessionID, resolvedTask, *agent, *skill, *action, *params, *result, *level, normDuration, *tokensIn, *cacheRead, *tokensOut, finalCost, *exitCode)
		if err != nil {
			fatal("Error insertando audit log: %v", err)
		}

	case "import-usage":
		fs := flag.NewFlagSet("audit import-usage", flag.ExitOnError)
		taskID := fs.String("task", os.Getenv("SEB_TASK_ID"), "ID de tarea")
		file := fs.String("file", "usage.jsonl", "Ruta al archivo usage.jsonl")
		_ = fs.Parse(args[1:])

		filePath := *file
		if fs.NArg() > 0 {
			filePath = fs.Arg(0)
		}

		count, err := importUsageJsonl(db, *taskID, filePath)
		if err != nil {
			fatal("Error importando usage.jsonl: %v", err)
		}
		fmt.Printf("[seb-harness] %d registros importados exitosamente desde %s.\n", count, filePath)

	case "report":
		fmt.Print(renderAuditReport(db))
	}
}

func renderAuditReport(db *sql.DB) string {
	var b strings.Builder
	b.WriteString("## 📊 Audit Trail & Observabilidad (`seb-harness`)\n\n")
	b.WriteString("### 1. Métricas por Tarea / Caso (Latencia, Tokens y Coste)\n")
	rows, err := db.Query(`
		SELECT COALESCE(NULLIF(task_id, ''), 'global'),
		       COUNT(*),
		       COALESCE(SUM(duration_ms), 0),
		       COALESCE(SUM(tokens_in), 0),
		       COALESCE(SUM(cache_read_tokens), 0),
		       COALESCE(SUM(tokens_out), 0),
		       ROUND(COALESCE(SUM(cost_usd), 0.0), 5)
		FROM agent_actions
		GROUP BY COALESCE(NULLIF(task_id, ''), 'global')
		ORDER BY 1 ASC`)
	if err != nil {
		fatal("Error generando reporte: %v", err)
	}
	defer rows.Close()

	b.WriteString("| Caso / Task ID | Llamadas a Tools | Latencia Total (ms) | Tokens In | Cache Read | Tokens Out | Coste Acumulado (USD) |\n")
	b.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")
	for rows.Next() {
		var task string
		var calls int
		var dur, tIn, tCache, tOut int64
		var cost float64
		_ = rows.Scan(&task, &calls, &dur, &tIn, &tCache, &tOut, &cost)
		b.WriteString(fmt.Sprintf("| `%s` | %d | %d ms | %d | %d | %d | $%.5f |\n", task, calls, dur, tIn, tCache, tOut, cost))
	}

	b.WriteString("\n### 2. Últimas 10 Acciones Registradas\n")
	recent, err := db.Query(`
		SELECT timestamp, agent, COALESCE(NULLIF(skill,''), '-'), action, duration_ms, tokens_in+tokens_out, cache_read_tokens, cost_usd, exit_code
		FROM agent_actions ORDER BY id DESC LIMIT 10`)
	if err == nil {
		defer recent.Close()
		b.WriteString("| Timestamp | Agente | Skill | Acción / Tool | Latencia | Tokens (In+Out) | Cache Read | Costo USD | Exit |\n")
		b.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")
		for recent.Next() {
			var ts, ag, sk, act string
			var dur, tok, cache, code int64
			var cost float64
			_ = recent.Scan(&ts, &ag, &sk, &act, &dur, &tok, &cache, &cost, &code)
			b.WriteString(fmt.Sprintf("| %s | `%s` | `%s` | `%s` | %d ms | %d | %d | $%.5f | %d |\n", ts, ag, sk, act, dur, tok, cache, cost, code))
		}
	}
	return b.String()
}

func logInternalAction(db *sql.DB, taskID, skill, action, params, result string, durationMs int64, exitCode int) {
	session := os.Getenv("SEB_SESSION_ID")
	if session == "" {
		session = "seb-cli-" + time.Now().Format("20060102")
	}
	agent := os.Getenv("SEB_AGENT")
	if agent == "" {
		agent = "seb-bin"
	}
	resolvedTask := resolveActiveTask(taskID, os.Getenv("SEB_TASK_ID"), ".seb/ACTIVE_TASK", db)
	durationMs = normalizeDuration(durationMs)
	_, _ = db.Exec(`INSERT INTO agent_actions (session_id, task_id, agent, skill, action, parameters, result_summary, duration_ms, exit_code)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session, resolvedTask, agent, skill, action, params, result, durationMs, exitCode)
}

func resolveActiveTask(explicitTask, envTask string, activeTaskPath string, db *sql.DB) string {
	if strings.TrimSpace(explicitTask) != "" {
		return strings.TrimSpace(explicitTask)
	}
	if strings.TrimSpace(envTask) != "" {
		return strings.TrimSpace(envTask)
	}
	if activeTaskPath != "" {
		if content, err := os.ReadFile(activeTaskPath); err == nil {
			task := strings.TrimSpace(string(content))
			if task != "" {
				if db != nil {
					var id string
					err := db.QueryRow(`SELECT id FROM tasks WHERE name = ? OR id = ? OR (id || '_' || name) = ?`, task, task, task).Scan(&id)
					if err == nil && id != "" {
						return id
					}
				}
				return task
			}
		}
	}
	if db != nil {
		var id string
		err := db.QueryRow(`SELECT id FROM tasks WHERE status != 'VERIFIED' ORDER BY updated_at DESC LIMIT 1`).Scan(&id)
		if err == nil && strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id)
		}
	}
	return "global"
}

func normalizeDuration(durationMs int64) int64 {
	if durationMs < 0 {
		return 0
	}
	return durationMs
}

func calculateCost(explicitCost float64, model string, tokensIn, tokensOut int64, configPath string) float64 {
	if explicitCost > 0.0 {
		return explicitCost
	}
	if model == "" || (tokensIn == 0 && tokensOut == 0) {
		return 0.0
	}
	if configPath == "" {
		configPath = ".seb/config.json"
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0.0
	}
	var cfg EngineConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return 0.0
	}
	pricing, ok := cfg.Pricing[model]
	if !ok {
		for m, p := range cfg.Pricing {
			if strings.EqualFold(m, model) {
				pricing = p
				ok = true
				break
			}
		}
	}
	if !ok {
		return 0.0
	}
	cost := (float64(tokensIn)*pricing.PromptPricePerMillion + float64(tokensOut)*pricing.CompletionPricePerMillion) / 1000000.0
	return cost
}

func importUsageJsonl(db *sql.DB, taskID, filePath string) (int, error) {
	if filePath == "" {
		filePath = "usage.jsonl"
	}
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	resolvedTask := resolveActiveTask(taskID, os.Getenv("SEB_TASK_ID"), ".seb/ACTIVE_TASK", db)
	sessionID := os.Getenv("SEB_SESSION_ID")
	if sessionID == "" {
		sessionID = "import-" + time.Now().Format("20060102")
	}
	agent := os.Getenv("SEB_AGENT")
	if agent == "" {
		agent = "agent-runner"
	}
	skill := os.Getenv("SEB_SKILL")
	if skill == "" {
		skill = "implementador"
	}

	imported := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec UsageRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}

		// Idempotencia: Verificar si ya existe un registro con el mismo task_id, parametros y timestamp
		var count int
		err := db.QueryRow(`SELECT COUNT(*) FROM agent_actions WHERE task_id = ? AND parameters = ? AND timestamp = ?`,
			resolvedTask, fmt.Sprintf("turns=%d", rec.Turns), rec.Timestamp).Scan(&count)
		if err == nil && count > 0 {
			continue // Ya importado
		}

		cost := calculateCost(0.0, os.Getenv("SEB_MODEL"), rec.InputTokens, rec.OutputTokens, ".seb/config.json")
		exitCode := 0
		if rec.Outcome == "failed" || rec.Outcome == "rejected" {
			exitCode = 1
		}

		_, err = db.Exec(`INSERT INTO agent_actions
			(session_id, task_id, agent, skill, action, parameters, result_summary, duration_ms, tokens_in, tokens_out, cost_usd, exit_code, timestamp)
			VALUES (?, ?, ?, ?, 'agent-session', ?, ?, 0, ?, ?, ?, ?, ?)`,
			sessionID, resolvedTask, agent, skill, fmt.Sprintf("turns=%d", rec.Turns), rec.Outcome, rec.InputTokens, rec.OutputTokens, cost, exitCode, rec.Timestamp)
		if err == nil {
			imported++
		}
	}
	return imported, scanner.Err()
}

func sanitizeName(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	re := regexp.MustCompile(`[^a-z0-9_]`)
	return re.ReplaceAllString(s, "")
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func currentGitCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "uncommitted"
	}
	return strings.TrimSpace(string(out))
}

func printUsage() {
	fmt.Println(`seb — Motor determinista SDD y Auditoría SQLite (seb-harness)

Comandos disponibles:
  seb init                                  Inicializa directorios y .seb/seb-audit.db
  seb spec-next <tarea>                     Reserva el siguiente ID (<001>) y devuelve la ruta de la spec
  seb plan-lock <tarea>                     Verifica testabilidad y sella hashes SHA-256 de spec y verificador
  seb verify <tarea>                        Ejecuta determinísticamente specs/<tarea>_verificador.md y audita
  seb guard <rol> <write|exec> <ruta>       Valida permisos de Menor Privilegio (<1ms)
  seb role <set <rol>|get|clear>            Gestiona el rol de sesión activo
  seb memory-add "<categoría>" "<texto>"    Añade aprendizaje a MEMORY.md y registra en SQLite
  seb status                                Muestra el estado de todas las specs en el proyecto
  seb audit log [flags]                     Registra llamada a tool, latencia, tokens y costo USD
  seb audit import-usage [flags] [archivo]  Importa registros de uso de tokens desde usage.jsonl
  seb audit report                          Imprime informe de observabilidad, latencia y costos por tarea`)
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "[seb-harness ERROR] "+format+"\n", a...)
	os.Exit(1)
}

func readLockFile() SebLock {
	var lock SebLock
	lock.Tasks = make(map[string]TaskLock)
	b, err := os.ReadFile(".seb/seb.lock")
	if err == nil {
		_ = json.Unmarshal(b, &lock)
	}
	return lock
}

func writeLockFile(lock SebLock) {
	b, _ := json.MarshalIndent(lock, "", "  ")
	_ = os.WriteFile(".seb/seb.lock", b, 0644)
}

func setActiveTask(taskName string) {
	_ = os.WriteFile(".seb/ACTIVE_TASK", []byte(taskName), 0644)
}

func clearActiveTask() {
	_ = os.Remove(".seb/ACTIVE_TASK")
}

const defaultActiveRolePath = ".seb/ACTIVE_ROLE"

var canonicalRoles = map[string]bool{
	"seb-iniciador":     true,
	"seb-especificador": true,
	"seb-clarificador":  true,
	"seb-planificador":  true,
	"seb-verificador":   true,
	"implementador":     true,
}

func normalizeRole(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

func isCanonicalRole(role string) bool {
	return canonicalRoles[normalizeRole(role)]
}

func getActiveRolePath() string {
	return defaultActiveRolePath
}

func setActiveRole(role string, rolePaths ...string) error {
	norm := normalizeRole(role)
	if !isCanonicalRole(norm) {
		return fmt.Errorf("rol '%s' no reconocido. Roles canónicos válidos: seb-iniciador, seb-especificador, seb-clarificador, seb-planificador, seb-verificador, implementador", role)
	}
	path := defaultActiveRolePath
	if len(rolePaths) > 0 && rolePaths[0] != "" {
		path = rolePaths[0]
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("no se pudo crear directorio para rol activo: %w", err)
	}
	return os.WriteFile(path, []byte(norm), 0644)
}

func getActiveRole(rolePaths ...string) string {
	path := defaultActiveRolePath
	if len(rolePaths) > 0 && rolePaths[0] != "" {
		path = rolePaths[0]
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "implementador"
	}
	norm := normalizeRole(string(data))
	if !isCanonicalRole(norm) {
		return "implementador"
	}
	return norm
}

func clearActiveRole(rolePaths ...string) error {
	path := defaultActiveRolePath
	if len(rolePaths) > 0 && rolePaths[0] != "" {
		path = rolePaths[0]
	}
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// 7b. seb role <set|get|clear>
func cmdRole(db *sql.DB, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("uso: seb role <set <nombre_rol> | get | clear>")
	}
	sub := strings.ToLower(strings.TrimSpace(args[0]))
	switch sub {
	case "set":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("debe especificar un rol válido. Roles canónicos: seb-iniciador, seb-especificador, seb-clarificador, seb-planificador, seb-verificador, implementador")
		}
		roleArg := args[1]
		if err := setActiveRole(roleArg); err != nil {
			return err
		}
		fmt.Printf("[seb-harness] Rol activo de sesión establecido en '%s'.\n", normalizeRole(roleArg))
		return nil
	case "get":
		role := getActiveRole()
		fmt.Println(role)
		return nil
	case "clear":
		if err := clearActiveRole(); err != nil {
			return fmt.Errorf("error limpiando rol activo: %w", err)
		}
		fmt.Println("[seb-harness] Rol activo de sesión restablecido a 'implementador'.")
		return nil
	default:
		return fmt.Errorf("subcomando de rol '%s' no reconocido. Uso: seb role <set <nombre_rol> | get | clear>", sub)
	}
}

func updateTaskLockVerification(canonicalName, commitHash string) error {
	lock := readLockFile()
	tLock, ok := lock.Tasks[canonicalName]
	if !ok {
		for k, v := range lock.Tasks {
			if k == canonicalName || strings.HasSuffix(k, "_"+canonicalName) || strings.HasSuffix(canonicalName, "_"+k) {
				tLock = v
				canonicalName = k
				ok = true
				break
			}
		}
	}
	tLock.Verified = true
	tLock.CommitHash = commitHash
	lock.Tasks[canonicalName] = tLock
	writeLockFile(lock)
	return nil
}

var depRegex = regexp.MustCompile(`(?i)^\s*[-*]?\s*\**\s*(?:dependencias|depends[-_]on)\s*\**\s*:\s*(.+)$`)

func parseSpecDependencies(specPath string) ([]string, error) {
	f, err := os.Open(specPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var deps []string
	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() && lineCount < 40 {
		lineCount++
		line := strings.TrimSpace(scanner.Text())
		m := depRegex.FindStringSubmatch(line)
		if len(m) == 2 {
			raw := strings.TrimSpace(m[1])
			raw = strings.Trim(raw, "`*")
			parts := strings.Split(raw, ",")
			for _, p := range parts {
				token := strings.TrimSpace(p)
				token = strings.Trim(token, "`*")
				if token == "" {
					continue
				}
				lower := strings.ToLower(token)
				if lower == "ninguna" || lower == "none" || lower == "ninguno" || lower == "n/a" || lower == "-" {
					continue
				}
				deps = append(deps, token)
			}
			break
		}
	}
	return deps, scanner.Err()
}

func validateDirectDependencies(db *sql.DB, targetTask TaskInfo, deps []string, lock SebLock) error {
	for _, dep := range deps {
		depTask, err := resolveTask(db, dep)
		if err != nil {
			return fmt.Errorf("la tarea prerrequisito '%s' no existe en el repositorio o es ambigua: %w", dep, err)
		}

		if err := detectDirectCircularDependency(targetTask, depTask); err != nil {
			return err
		}

		tLock, exists := lock.Tasks[depTask.CanonicalName]
		if !exists {
			tLock, exists = lock.Tasks[depTask.Name]
		}
		if !exists || !tLock.Verified || strings.TrimSpace(tLock.CommitHash) == "" {
			return fmt.Errorf("la tarea prerrequisito '%s' no ha completado su verificación determinista (falta verified: true o commit_hash en .seb/seb.lock)", depTask.CanonicalName)
		}
	}
	return nil
}

func detectDirectCircularDependency(targetTask, depTask TaskInfo) error {
	if targetTask.CanonicalName == depTask.CanonicalName || targetTask.ID == depTask.ID || targetTask.Name == depTask.Name {
		return fmt.Errorf("se detectó una dependencia circular directa mutua consigo misma en '%s'", targetTask.CanonicalName)
	}

	if depTask.SpecPath != "" {
		depSpecDeps, err := parseSpecDependencies(depTask.SpecPath)
		if err == nil {
			for _, d := range depSpecDeps {
				normD := strings.TrimSpace(d)
				if matched, _ := regexp.MatchString(`^\d+$`, normD); matched {
					var num int
					fmt.Sscanf(normD, "%d", &num)
					normD = fmt.Sprintf("%03d", num)
				}
				if normD == targetTask.ID || normD == targetTask.Name || normD == targetTask.CanonicalName ||
					strings.HasPrefix(targetTask.CanonicalName, normD+"_") || strings.HasSuffix(targetTask.CanonicalName, "_"+normD) {
					return fmt.Errorf("se detectó una referencia circular mutua entre '%s' y '%s'", targetTask.CanonicalName, depTask.CanonicalName)
				}
			}
		}
	}
	return nil
}

// HookPayload modela el contrato JSON camelCase enviado por Antigravity en stdin
type HookPayload struct {
	ConversationID        string       `json:"conversationId"`
	WorkspacePaths        []string     `json:"workspacePaths"`
	TranscriptPath        string       `json:"transcriptPath"`
	ArtifactDirectoryPath string       `json:"artifactDirectoryPath"`
	ModelName             string       `json:"modelName"`
	StepIdx               *int         `json:"stepIdx"`
	ToolCall              *HookTool    `json:"toolCall"`
	Error                 string       `json:"error"`
	ExecutionNum          int          `json:"executionNum"`
	TerminationReason     string       `json:"terminationReason"`
}

type HookTool struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type TranscriptStep struct {
	StepIndex       int        `json:"step_index"`
	Type            string     `json:"type"`
	InputTokens     int64      `json:"input_tokens"`
	CacheReadTokens int64      `json:"cache_read_tokens"`
	OutputTokens    int64      `json:"output_tokens"`
	ToolCalls       []HookTool `json:"tool_calls"`
}

// 8. seb hook <pre-tool|post-tool|stop>
func cmdHook(db *sql.DB, args []string) {
	if len(args) < 1 {
		fmt.Println("{}")
		return
	}
	sub := args[0]
	var payload []byte
	if stat, err := os.Stdin.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		payload, _ = io.ReadAll(os.Stdin)
	}
	cwd, _ := os.Getwd()
	envSkill := os.Getenv("SEB_SKILL")

	switch sub {
	case "pre-tool":
		out, _ := processHookPreTool(db, payload, envSkill, ".seb/ACTIVE_TASK", cwd)
		fmt.Println(out)
	case "post-tool":
		out, _ := processHookPostTool(db, payload, envSkill, ".seb/ACTIVE_TASK", cwd, ".seb/config.json")
		fmt.Println(out)
	case "stop":
		out, _ := processHookStop(db, payload, envSkill, ".seb/ACTIVE_TASK", cwd, ".seb/config.json")
		fmt.Println(out)
	default:
		fmt.Println("{}")
	}
}

var absPathTokenRegex = regexp.MustCompile(`(/[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)+)`)

func sanitizeStringValue(val string, workspaceRoot string) string {
	wsClean := ""
	if workspaceRoot != "" {
		wsClean = filepath.Clean(workspaceRoot)
	}

	s := strings.TrimSpace(val)
	// Quitar comillas escapadas o envolventes si provienen de JSON anidado
	sUnquoted := strings.Trim(s, `"`)

	// Si el valor entero parece una ruta absoluta
	if strings.HasPrefix(sUnquoted, "/") && !strings.ContainsAny(sUnquoted, " \t\n") {
		cleaned := filepath.Clean(sUnquoted)
		if wsClean != "" && (cleaned == wsClean || strings.HasPrefix(cleaned, wsClean+"/")) {
			rel, err := filepath.Rel(wsClean, cleaned)
			if err == nil {
				s = rel
			}
		} else {
			s = "<EXTERNAL_PATH>/" + filepath.Base(cleaned)
		}
	} else {
		// Reemplazar rutas absolutas embebidas dentro de comandos o mensajes
		s = absPathTokenRegex.ReplaceAllStringFunc(val, func(match string) string {
			cleaned := filepath.Clean(match)
			if wsClean != "" && (cleaned == wsClean || strings.HasPrefix(cleaned, wsClean+"/")) {
				if rel, err := filepath.Rel(wsClean, cleaned); err == nil {
					return rel
				}
			}
			return "<EXTERNAL_PATH>/" + filepath.Base(cleaned)
		})
	}

	if len(s) > 256 {
		s = s[:256] + "...[TRUNCATED]"
	}
	return s
}

func sanitizeValueRecursive(v interface{}, workspaceRoot string) interface{} {
	switch val := v.(type) {
	case string:
		return sanitizeStringValue(val, workspaceRoot)
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, elem := range val {
			out[k] = sanitizeValueRecursive(elem, workspaceRoot)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, elem := range val {
			out[i] = sanitizeValueRecursive(elem, workspaceRoot)
		}
		return out
	default:
		return v
	}
}

// sanitizeToolParams transforma rutas del workspace en relativas, enmascara externas y trunca > 256 chars
func sanitizeToolParams(args map[string]interface{}, workspaceRoot string) string {
	if len(args) == 0 {
		return "{}"
	}
	sanitized := sanitizeValueRecursive(args, workspaceRoot)
	b, err := json.Marshal(sanitized)
	if err != nil {
		return "{}"
	}
	return string(b)
}

var skillPathRegex = regexp.MustCompile(`\.agents/skills/([A-Za-z0-9_-]+)/SKILL\.md`)

// inferActiveSkill determina la habilidad activa con jerarquía estricta de 4 niveles
func inferActiveSkill(envSkill, transcriptPath, activeTaskPath string, activeRolePaths ...string) string {
	// Nivel 1: Variable de entorno explícita SEB_SKILL
	if strings.TrimSpace(envSkill) != "" {
		return strings.TrimSpace(envSkill)
	}

	// Nivel 2: Rol persistido en .seb/ACTIVE_ROLE si existe y es canónico
	activeRolePath := defaultActiveRolePath
	if len(activeRolePaths) > 0 && activeRolePaths[0] != "" {
		activeRolePath = activeRolePaths[0]
	}
	if data, err := os.ReadFile(activeRolePath); err == nil {
		norm := normalizeRole(string(data))
		if isCanonicalRole(norm) {
			return norm
		}
	}

	hasActiveLockedTask := false
	if activeTaskPath != "" {
		if data, err := os.ReadFile(activeTaskPath); err == nil && strings.TrimSpace(string(data)) != "" {
			hasActiveLockedTask = true
		}
	}

	// Nivel 3: Inferencia restrictiva en transcripción (únicamente roles canónicos seb-*)
	lastSkill := ""
	if transcriptPath != "" {
		if f, err := os.Open(transcriptPath); err == nil {
			defer f.Close()
			scanner := bufio.NewScanner(f)
			// Ampliar buffer por si hay lineas largas en transcript.jsonl
			buf := make([]byte, 0, 64*1024)
			scanner.Buffer(buf, 2*1024*1024)
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.Contains(line, "SKILL.md") {
					continue
				}
				var step TranscriptStep
				if err := json.Unmarshal([]byte(line), &step); err == nil && step.Type == "PLANNER_RESPONSE" {
					for _, tc := range step.ToolCalls {
						b, _ := json.Marshal(tc.Args)
						if m := skillPathRegex.FindStringSubmatch(string(b)); len(m) == 2 {
							detected := normalizeRole(m[1])
							if strings.HasPrefix(detected, "seb-") && isCanonicalRole(detected) {
								lastSkill = detected
							}
						}
					}
				} else if m := skillPathRegex.FindStringSubmatch(line); len(m) == 2 {
					detected := normalizeRole(m[1])
					if strings.HasPrefix(detected, "seb-") && isCanonicalRole(detected) {
						lastSkill = detected
					}
				}
			}
		}
	}

	// Nivel 4: Fallback a implementador
	if lastSkill == "" {
		return "implementador"
	}
	if hasActiveLockedTask && lastSkill == "seb-planificador" {
		return "implementador"
	}
	return lastSkill
}

func extractTargetAndCommand(tc *HookTool, workspaceRoot string) (string, string) {
	if tc == nil || len(tc.Args) == 0 {
		return "", ""
	}
	var targetPath, cmdStr string
	for k, v := range tc.Args {
		s, ok := v.(string)
		if !ok {
			continue
		}
		s = strings.Trim(s, `"`)
		kl := strings.ToLower(k)
		if kl == "path" || kl == "file_path" || kl == "targetfile" || kl == "filepath" {
			targetPath = s
		}
		if kl == "commandline" || kl == "command" || kl == "cmd" {
			cmdStr = s
		}
	}
	if targetPath != "" && workspaceRoot != "" {
		wsClean := filepath.Clean(workspaceRoot)
		tpClean := filepath.Clean(targetPath)
		if strings.HasPrefix(tpClean, wsClean+"/") {
			if rel, err := filepath.Rel(wsClean, tpClean); err == nil {
				targetPath = rel
			}
		}
	}
	return targetPath, cmdStr
}

func timerFilePath(sessionID string, stepIdx int, toolName string) string {
	safeSess := sanitizeName(sessionID)
	safeTool := sanitizeName(toolName)
	return fmt.Sprintf("/tmp/seb_timer_%s_%d_%s", safeSess, stepIdx, safeTool)
}

var dangerousCmdRegex = regexp.MustCompile(`(rm\s+-rf\s+/|git\s+commit.*--no-verify|git\s+push.*--force|chmod\s+777)`)
var shellRedirectContractRegex = regexp.MustCompile(`(>|tee|sed\s+-i).*(constitution\.md|_spec\.md|_verificador\.md)`)

// processHookPreTool ejecuta la logica de PreToolUse y devuelve el contrato JSON
func processHookPreTool(db *sql.DB, payloadBytes []byte, envSkill, activeTaskPath, workspaceRoot string) (string, error) {
	var p HookPayload
	if len(payloadBytes) > 0 {
		_ = json.Unmarshal(payloadBytes, &p)
	}
	if len(p.WorkspacePaths) > 0 && p.WorkspacePaths[0] != "" {
		workspaceRoot = p.WorkspacePaths[0]
	}

	sessionID := p.ConversationID
	if sessionID == "" {
		sessionID = "local-" + time.Now().Format("20060102")
	}
	stepIdx := 0
	if p.StepIdx != nil {
		stepIdx = *p.StepIdx
	}
	toolName := "unknown"
	if p.ToolCall != nil && p.ToolCall.Name != "" {
		toolName = p.ToolCall.Name
	}

	// Guardar timestamp de inicio en milisegundos
	tFile := timerFilePath(sessionID, stepIdx, toolName)
	_ = os.WriteFile(tFile, []byte(fmt.Sprintf("%d", time.Now().UnixMilli())), 0644)

	rolePath := defaultActiveRolePath
	if workspaceRoot != "" {
		rolePath = filepath.Join(workspaceRoot, defaultActiveRolePath)
	}
	activeRole := inferActiveSkill(envSkill, p.TranscriptPath, activeTaskPath, rolePath)
	targetPath, cmdStr := extractTargetAndCommand(p.ToolCall, workspaceRoot)

	// 1. Bloqueo de archivos sensibles
	if targetPath != "" {
		base := filepath.Base(targetPath)
		if base == ".env" || strings.HasPrefix(base, ".env.") || strings.Contains(base, "id_rsa") ||
			strings.Contains(base, "id_ed25519") || strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") {
			return marshalHookDecision("deny", fmt.Sprintf("[seb-harness BLOQUEO DE SEGURIDAD] Acceso denegado al archivo sensible: %s", base)), nil
		}
	}

	// 2. Bloqueo de comandos destructivos o evasion
	if cmdStr != "" {
		if dangerousCmdRegex.MatchString(cmdStr) {
			return marshalHookDecision("deny", "[seb-harness BLOQUEO DE SEGURIDAD] Comando peligroso o evasion de verificacion detectada."), nil
		}
		if activeRole == "implementador" || activeRole == "" {
			if shellRedirectContractRegex.MatchString(cmdStr) {
				return marshalHookDecision("deny", "[seb-harness ANTI-REWARD-HACKING] Prohibido modificar contratos SDD mediante comandos de shell."), nil
			}
		}
	}

	// 3. Verificacion de Menor Privilegio en herramientas de escritura
	if targetPath != "" {
		tl := strings.ToLower(toolName)
		if strings.Contains(tl, "write") || strings.Contains(tl, "edit") || strings.Contains(tl, "patch") ||
			strings.Contains(tl, "create") || strings.Contains(tl, "replace") || strings.Contains(tl, "save") {
			if !checkPermission(activeRole, "write", filepath.Clean(targetPath)) {
				if db != nil {
					logInternalAction(db, "", activeRole, "guard-blocked:write", targetPath, "DENIED", 0, 1)
				}
				return marshalHookDecision("deny", fmt.Sprintf("GUARDARRAIL DE MENOR PRIVILEGIO: El rol '%s' NO tiene permiso de escritura sobre '%s'.", activeRole, targetPath)), nil
			}
		}
	}

	return marshalHookDecision("allow", ""), nil
}

func marshalHookDecision(decision, reason string) string {
	resp := map[string]interface{}{
		"decision": decision,
	}
	if reason != "" {
		resp["reason"] = reason
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

// findPlannerStepMetrics busca el paso PLANNER_RESPONSE en transcript.jsonl cuyo step_index sea <= targetStepIdx (el mas cercano)
func findPlannerStepMetrics(transcriptPath string, targetStepIdx int) (plannerStepIdx int, tokIn, cacheRead, tokOut int64, found bool) {
	if transcriptPath == "" {
		return -1, 0, 0, 0, false
	}
	f, err := os.Open(transcriptPath)
	if err != nil {
		return -1, 0, 0, 0, false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	bestStep := -1
	var bIn, bCache, bOut int64

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "PLANNER_RESPONSE") {
			continue
		}
		var st TranscriptStep
		if err := json.Unmarshal([]byte(line), &st); err == nil && st.Type == "PLANNER_RESPONSE" {
			if targetStepIdx <= 0 || st.StepIndex <= targetStepIdx {
				if st.StepIndex >= bestStep {
					bestStep = st.StepIndex
					bIn = st.InputTokens
					bCache = st.CacheReadTokens
					bOut = st.OutputTokens
				}
			}
		}
	}

	if bestStep >= 0 {
		return bestStep, bIn, bCache, bOut, true
	}
	return -1, 0, 0, 0, false
}

// processHookPostTool ejecuta la logica de PostToolUse y devuelve "{}"
func processHookPostTool(db *sql.DB, payloadBytes []byte, envSkill, activeTaskPath, workspaceRoot, configPath string) (string, error) {
	nowMs := time.Now().UnixMilli()
	var p HookPayload
	if len(payloadBytes) > 0 {
		_ = json.Unmarshal(payloadBytes, &p)
	}
	if len(p.WorkspacePaths) > 0 && p.WorkspacePaths[0] != "" {
		workspaceRoot = p.WorkspacePaths[0]
	}

	sessionID := p.ConversationID
	if sessionID == "" {
		sessionID = "local-" + time.Now().Format("20060102")
	}
	stepIdx := -1
	if p.StepIdx != nil {
		stepIdx = *p.StepIdx
	}
	toolName := "tool_call"
	var toolArgs map[string]interface{}
	if p.ToolCall != nil {
		if p.ToolCall.Name != "" {
			toolName = p.ToolCall.Name
		}
		toolArgs = p.ToolCall.Args
	}

	// Calcular duracion desde archivo timer
	var durationMs int64
	tFile := timerFilePath(sessionID, stepIdx, toolName)
	if data, err := os.ReadFile(tFile); err == nil {
		var startMs int64
		if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &startMs); err == nil && startMs > 0 {
			durationMs = normalizeDuration(nowMs - startMs)
		}
		_ = os.Remove(tFile)
	}

	rolePath := defaultActiveRolePath
	if workspaceRoot != "" {
		rolePath = filepath.Join(workspaceRoot, defaultActiveRolePath)
	}
	activeSkill := inferActiveSkill(envSkill, p.TranscriptPath, activeTaskPath, rolePath)
	sanitizedParams := sanitizeToolParams(toolArgs, workspaceRoot)

	exitCode := 0
	resultSummary := "ok"
	if strings.TrimSpace(p.Error) != "" {
		exitCode = 1
		resultSummary = sanitizeStringValue(p.Error, workspaceRoot)
	}

	agentName := p.ModelName
	if agentName == "" {
		agentName = os.Getenv("SEB_AGENT")
	}
	if agentName == "" {
		agentName = "antigravity"
	}

	// Extraer tokens del paso PLANNER_RESPONSE correspondiente y deduplicar por (session_id, step_idx)
	var tokIn, cacheRead, tokOut int64
	recordedStepIdx := stepIdx
	if plannerIdx, tIn, tCache, tOut, ok := findPlannerStepMetrics(p.TranscriptPath, stepIdx); ok {
		recordedStepIdx = plannerIdx
		alreadyCounted := false
		if db != nil && plannerIdx >= 0 {
			var existing int
			err := db.QueryRow(`SELECT COUNT(*) FROM agent_actions WHERE session_id = ? AND step_idx = ? AND (tokens_in > 0 OR cache_read_tokens > 0 OR tokens_out > 0)`,
				sessionID, plannerIdx).Scan(&existing)
			if err == nil && existing > 0 {
				alreadyCounted = true
			}
		}
		if !alreadyCounted {
			tokIn = tIn
			cacheRead = tCache
			tokOut = tOut
		}
	}

	if db != nil {
		resolvedTask := resolveActiveTask("", os.Getenv("SEB_TASK_ID"), activeTaskPath, db)
		cost := calculateCost(0.0, p.ModelName, tokIn, tokOut, configPath)
		_, _ = db.Exec(`INSERT INTO agent_actions
			(session_id, task_id, agent, skill, action, parameters, result_summary, level, duration_ms, tokens_in, cache_read_tokens, tokens_out, cost_usd, exit_code, step_idx)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'autonomous', ?, ?, ?, ?, ?, ?, ?)`,
			sessionID, resolvedTask, agentName, activeSkill, toolName, sanitizedParams, resultSummary, durationMs, tokIn, cacheRead, tokOut, cost, exitCode, recordedStepIdx)
	}

	return "{}", nil
}

// processHookStop inspecciona transcriptPath al cierre de turno y registra pasos PLANNER_RESPONSE no contabilizados
func processHookStop(db *sql.DB, payloadBytes []byte, envSkill, activeTaskPath, workspaceRoot, configPath string) (string, error) {
	var p HookPayload
	if len(payloadBytes) > 0 {
		_ = json.Unmarshal(payloadBytes, &p)
	}
	if len(p.WorkspacePaths) > 0 && p.WorkspacePaths[0] != "" {
		workspaceRoot = p.WorkspacePaths[0]
	}

	sessionID := p.ConversationID
	if sessionID == "" {
		sessionID = "local-" + time.Now().Format("20060102")
	}
	agentName := p.ModelName
	if agentName == "" {
		agentName = "antigravity"
	}

	if db != nil && p.TranscriptPath != "" {
		if f, err := os.Open(p.TranscriptPath); err == nil {
			defer f.Close()
			rolePath := defaultActiveRolePath
	if workspaceRoot != "" {
		rolePath = filepath.Join(workspaceRoot, defaultActiveRolePath)
	}
	activeSkill := inferActiveSkill(envSkill, p.TranscriptPath, activeTaskPath, rolePath)
			resolvedTask := resolveActiveTask("", os.Getenv("SEB_TASK_ID"), activeTaskPath, db)

			scanner := bufio.NewScanner(f)
			buf := make([]byte, 0, 64*1024)
			scanner.Buffer(buf, 2*1024*1024)

			for scanner.Scan() {
				line := scanner.Text()
				if !strings.Contains(line, "PLANNER_RESPONSE") {
					continue
				}
				var st TranscriptStep
				if err := json.Unmarshal([]byte(line), &st); err == nil && st.Type == "PLANNER_RESPONSE" {
					if st.InputTokens == 0 && st.CacheReadTokens == 0 && st.OutputTokens == 0 {
						continue
					}
					var existing int
					err := db.QueryRow(`SELECT COUNT(*) FROM agent_actions WHERE session_id = ? AND step_idx = ?`,
						sessionID, st.StepIndex).Scan(&existing)
					if err == nil && existing == 0 {
						cost := calculateCost(0.0, p.ModelName, st.InputTokens, st.OutputTokens, configPath)
						params := fmt.Sprintf(`{"step_index":%d}`, st.StepIndex)
						_, _ = db.Exec(`INSERT INTO agent_actions
							(session_id, task_id, agent, skill, action, parameters, result_summary, level, duration_ms, tokens_in, cache_read_tokens, tokens_out, cost_usd, exit_code, step_idx)
							VALUES (?, ?, ?, ?, 'model_response', ?, 'ok', 'autonomous', 0, ?, ?, ?, ?, 0, ?)`,
							sessionID, resolvedTask, agentName, activeSkill, params, st.InputTokens, st.CacheReadTokens, st.OutputTokens, cost, st.StepIndex)
					}
				}
			}
		}
	}

	return `{"decision":"allow"}`, nil
}
