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
	Aborted      bool   `json:"aborted,omitempty"`
	Mode         string `json:"mode,omitempty"`
}

// ModelPricing define las tarifas monetarias por millon de tokens
type ModelPricing struct {
	PromptPricePerMillion     float64 `json:"prompt_price_per_million"`
	CompletionPricePerMillion float64 `json:"completion_price_per_million"`
}

// EngineConfig representa la configuracion local opcional de .seb/config.json
type EngineConfig struct {
	Pricing   map[string]ModelPricing `json:"pricing"`
	FastTrack *FastTrackConfig        `json:"fast_track,omitempty"`
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

	// Sistema de ayuda bajo demanda sin tocar ni crear .seb/seb-audit.db
	if cmd == "help" {
		os.Exit(runHelpCommand(args, os.Stdout, os.Stderr))
	}
	if cmd == "--help" || cmd == "-h" {
		os.Exit(runHelpCommand(nil, os.Stdout, os.Stderr))
	}
	if hasHelpFlag(args) {
		os.Exit(runHelpCommand([]string{cmd}, os.Stdout, os.Stderr))
	}

	// seb status opera en lectura pura; no crea .seb/seb-audit.db si no existe
	if cmd == "status" {
		var statusDB *sql.DB
		if _, statErr := os.Stat(dbPath); statErr == nil {
			if roDB, err := sql.Open("sqlite", dbPath+"?_pragma=query_only(ON)&_pragma=busy_timeout(5000)"); err == nil {
				statusDB = roDB
				defer statusDB.Close()
			}
		}
		os.Exit(runStatusCommand(statusDB, args, os.Stdout, os.Stderr))
	}

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
	case "abort":
		if err := cmdAbort(db, args); err != nil {
			fatal("%v", err)
		}
	case "task":
		if err := handleTaskSubcommand(db, args); err != nil {
			fatal("%v", err)
		}
	case "oneoff":
		code := cmdOneOff(db, args, os.Stdout, os.Stderr)
		db.Close()
		os.Exit(code)
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
		".agents/skills/seb-fast-task",
		".agents/skills/seb-verificador",
		".agents/skills/seb-documentador",
		".agents/hooks",
		"docs",
		"docs/use-cases",
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
		mode TEXT NOT NULL DEFAULT 'SDD',
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

	// Migracion idempotente de columna mode en tabla tasks (Spec 018)
	tRows, err := db.Query(`PRAGMA table_info(tasks)`)
	if err != nil {
		return err
	}
	defer tRows.Close()

	hasTaskMode := false
	for tRows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := tRows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
			if name == "mode" {
				hasTaskMode = true
			}
		}
	}
	if !hasTaskMode {
		if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN mode TEXT NOT NULL DEFAULT 'SDD'`); err != nil {
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
	FastPath      string
	Mode          string
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
	t.FastPath = buildArtifactPath(t.ID, t.Name, "plan_y_verificador")
	mode, planPath, verifierPath, detectErr := detectTaskModeAndArtifacts(t.ID, t.Name, t.SpecPath)
	if detectErr != nil {
		return TaskInfo{}, detectErr
	}
	t.Mode = mode
	t.PlanPath = planPath
	t.VerifierPath = verifierPath

	return t, nil
}

// buildSpecStubContent construye el contenido minimo valido del stub inicial de especificacion
func buildSpecStubContent(id, taskName string) string {
	return fmt.Sprintf(`# Spec %s — %s

- **Tarea ID:** `+"`%s`"+`
- **Identificador:** `+"`%s`"+`
- **Origen:** [Pendiente]
- **Estado:** `+"`BORRADOR`"+`
- **Afecta-UC:** [PENDIENTE]

## 10. Dudas Abiertas y Puntos por Clarificar

- [NECESITA ACLARACIÓN] Stub recién reservado por `+"`seb spec-next`"+`; pendiente redactar especificación completa.
`, id, taskName, id, taskName)
}

// parseSpecNextArgs valida y extrae el nombre de tarea y el flag --dry-run
func parseSpecNextArgs(args []string) (string, bool, error) {
	dryRun := false
	var rawName string
	for _, a := range args {
		if a == "--dry-run" {
			dryRun = true
			continue
		}
		if strings.HasPrefix(a, "-") {
			return "", false, fmt.Errorf("flag desconocido en spec-next: %s", a)
		}
		if rawName != "" {
			return "", false, errors.New("demasiados argumentos: Uso: seb spec-next [--dry-run] <nombre_tarea>")
		}
		rawName = a
	}

	trimmed := strings.TrimSpace(rawName)
	if trimmed == "" {
		return "", false, errors.New("nombre de tarea vacio o faltante: Uso: seb spec-next [--dry-run] <nombre_tarea>")
	}
	if strings.Contains(trimmed, "..") || strings.ContainsAny(trimmed, "/\\") {
		return "", false, fmt.Errorf("nombre de tarea invalido (secuencia de ruta no permitida): '%s'", rawName)
	}

	taskName := sanitizeName(trimmed)
	if taskName == "" {
		return "", false, fmt.Errorf("nombre de tarea invalido tras saneamiento: '%s'", rawName)
	}
	return taskName, dryRun, nil
}

// reserveSpecInDB inserta una reserva rechazando colisiones de ID con nombre distinto
func reserveSpecInDB(db *sql.DB, id, taskName, specPath string) error {
	if db == nil {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := reserveSpecInTx(tx, id, taskName, specPath); err != nil {
		return err
	}
	return tx.Commit()
}

func reserveSpecInTx(tx *sql.Tx, id, taskName, specPath string) error {
	var existingName string
	err := tx.QueryRow(`SELECT name FROM tasks WHERE id = ?`, id).Scan(&existingName)
	if err == nil {
		if existingName != taskName {
			return fmt.Errorf("colision de ID '%s': ya asignado a la tarea '%s'", id, existingName)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = tx.Exec(
		`INSERT INTO tasks (id, name, spec_path, status) VALUES (?, ?, ?, 'SPECIFIED')`,
		id, taskName, specPath,
	)
	return err
}

// specNext calcula el siguiente ID secuencial, crea el stub en disco y registra la reserva de forma atomica
func specNext(db *sql.DB, args []string) (string, error) {
	start := time.Now()
	taskName, dryRun, err := parseSpecNextArgs(args)
	if err != nil {
		return "", err
	}

	// 1. Escanear specs/ para detectar existencia previa con el mismo nombre y calcular maxID
	files, _ := filepath.Glob("specs/*_*_spec.md")
	maxID := 0
	re := regexp.MustCompile(`^specs/(\d{3})_(.+)_spec\.md$`)

	for _, f := range files {
		matches := re.FindStringSubmatch(f)
		if len(matches) != 3 {
			continue
		}
		idStr := matches[1]
		nameStr := matches[2]

		var id int
		fmt.Sscanf(idStr, "%d", &id)
		if id > maxID {
			maxID = id
		}

		// Idempotencia acotada al stub sin redactar; rechazar si ya fue editada
		if nameStr == taskName {
			raw, readErr := os.ReadFile(f)
			if readErr != nil {
				return "", fmt.Errorf("error leyendo spec existente '%s': %w", f, readErr)
			}
			normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
			expectedStub := strings.ReplaceAll(buildSpecStubContent(idStr, taskName), "\r\n", "\n")
			if normalized == expectedStub {
				if !dryRun && db != nil {
					if err := reserveSpecInDB(db, idStr, taskName, f); err != nil {
						return "", err
					}
				}
				return f, nil
			}
			return "", fmt.Errorf("ya existe una especificacion redactada para '%s' en '%s'", taskName, f)
		}
	}

	// 2. Considerar reservas registradas en SQLite para evitar colisiones entre llamadas consecutivas
	if db != nil {
		rows, qErr := db.Query(`SELECT id, name FROM tasks`)
		if qErr == nil {
			defer rows.Close()
			for rows.Next() {
				var dbIDStr, dbName string
				if scanErr := rows.Scan(&dbIDStr, &dbName); scanErr == nil {
					var id int
					fmt.Sscanf(dbIDStr, "%d", &id)
					if id > maxID {
						maxID = id
					}
				}
			}
		}
	}

	nextID := fmt.Sprintf("%03d", maxID+1)
	specPath := buildArtifactPath(nextID, taskName, "spec")

	// 3. Modo --dry-run: devolver ruta propuesta sin efectos secundarios
	if dryRun {
		return specPath, nil
	}

	// 4. Validar que el ID calculado no colisione en disco con otro nombre
	existingWithID, _ := filepath.Glob(filepath.Join("specs", nextID+"_*_spec.md"))
	for _, f := range existingWithID {
		if f != specPath {
			return "", fmt.Errorf("colision de ID '%s': ya existe '%s' en disco", nextID, f)
		}
	}

	// 5. Transaccion SQLite + creacion atomica de archivo stub en disco con rollback ante fallo
	var tx *sql.Tx
	if db != nil {
		tx, err = db.Begin()
		if err != nil {
			return "", fmt.Errorf("error iniciando transaccion SQLite: %w", err)
		}
		defer tx.Rollback()

		if err := reserveSpecInTx(tx, nextID, taskName, specPath); err != nil {
			return "", fmt.Errorf("error reservando tarea en SQLite: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(specPath), 0755); err != nil {
		return "", fmt.Errorf("error creando directorio specs/: %w", err)
	}

	f, err := os.OpenFile(specPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("error creando archivo stub '%s': %w", specPath, err)
	}
	stubContent := buildSpecStubContent(nextID, taskName)
	_, writeErr := f.WriteString(stubContent)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(specPath)
		if writeErr != nil {
			return "", fmt.Errorf("error escribiendo archivo stub '%s': %w", specPath, writeErr)
		}
		return "", fmt.Errorf("error cerrando archivo stub '%s': %w", specPath, closeErr)
	}

	if tx != nil {
		if err := tx.Commit(); err != nil {
			_ = os.Remove(specPath)
			return "", fmt.Errorf("error confirmando reserva en SQLite: %w", err)
		}
	}

	dur := time.Since(start).Milliseconds()
	if db != nil {
		logInternalAction(db, nextID, "seb-especificador", "spec-next", taskName, specPath, dur, 0)
	}
	return specPath, nil
}

// 1. seb spec-next [--dry-run] <nombre_tarea>
func cmdSpecNext(db *sql.DB, args []string) {
	specPath, err := specNext(db, args)
	if err != nil {
		fatal("%v", err)
	}
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
	if task.Mode == taskModeFAST {
		meta := parseSpecMetadata(task.SpecPath)
		if meta.EditorialState != "APROBADA" {
			fatal("BLOQUEO FAST: La spec acotada '%s' debe declarar 'Estado: APROBADA' antes de sellar (estado actual: %s).", task.SpecPath, meta.EditorialState)
		}
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

	lockMode := ""
	if task.Mode == taskModeFAST {
		lockMode = "fast"
	}

	existingLock := lock.Tasks[task.CanonicalName]
	lock.Tasks[task.CanonicalName] = TaskLock{
		SpecHash:     specHash,
		VerifierHash: verifHash,
		Verified:     existingLock.Verified,
		CommitHash:   existingLock.CommitHash,
		Mode:         lockMode,
	}
	writeLockFile(lock)

	// Marcar la tarea como activa para el pre-commit hook con clave canonica
	setActiveTask(task.CanonicalName)
	_ = setActiveRole("implementador")

	// Actualizar estado y modo en SQLite (solo para observabilidad local)
	_, _ = db.Exec(`
		INSERT INTO tasks (id, name, spec_path, status, mode, spec_sha256, verifier_sha256, updated_at)
		VALUES (?, ?, ?, 'PLANNED', ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET status = 'PLANNED', mode = excluded.mode, spec_sha256 = excluded.spec_sha256, verifier_sha256 = excluded.verifier_sha256, updated_at = CURRENT_TIMESTAMP`,
		task.ID, task.Name, task.SpecPath, normalizeTaskMode(task.Mode), specHash, verifHash)

	dur := time.Since(start).Milliseconds()
	lockSkill := "seb-planificador"
	if task.Mode == taskModeFAST {
		lockSkill = "seb-fast-task"
	}
	logInternalAction(db, task.ID, lockSkill, "plan-lock", task.CanonicalName, fmt.Sprintf("mode=%s checks=%d", normalizeTaskMode(task.Mode), len(checks)), dur, 0)
	fmt.Printf("[seb-harness] Plan sellado en .seb/seb.lock (modo %s) y marcado como activo para '%s'.\n", normalizeTaskMode(task.Mode), task.CanonicalName)
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

	isFast := task.Mode == taskModeFAST || strings.EqualFold(taskLock.Mode, "fast")
	effectiveMode := taskModeSDD
	if isFast {
		effectiveMode = taskModeFAST
		fastCfg, cfgErr := loadFastTrackConfig(".seb/config.json")
		if cfgErr != nil {
			fatal("%v", cfgErr)
		}
		if diffErr := validateFastDiffThresholds(task.CanonicalName, fastCfg); diffErr != nil {
			fatal("%v", diffErr)
		}
		if arhErr := validateFastAntiRewardHackingDiff(task.CanonicalName); arhErr != nil {
			fatal("%v", arhErr)
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
		logInternalAction(db, task.ID, "seb-verificador", "verify", task.CanonicalName, fmt.Sprintf("mode=%s passed=%d failed=%d", effectiveMode, passed, failed), durVerify, 2)
		_, _ = db.Exec(`
			INSERT INTO tasks (id, name, spec_path, status, mode, updated_at)
			VALUES (?, ?, ?, 'FAILED_VERIFICATION', ?, CURRENT_TIMESTAMP)
			ON CONFLICT(id) DO UPDATE SET status = 'FAILED_VERIFICATION', mode = excluded.mode, updated_at = CURRENT_TIMESTAMP`,
			task.ID, task.Name, task.SpecPath, effectiveMode)
		fmt.Println(failureDetails.String())
		fmt.Println("\n**VEREDICTO:** `PENDIENTE DE IMPLEMENTACIÓN` — Corrige el código de aplicación sin alterar `*_verificador.md` ni `*_plan_y_verificador.md`.")
		os.Exit(2)
	}

	// Liberar el hook pre-commit porque la tarea pasó la verificación
	clearActiveTask()
	_ = clearActiveRole()
	_ = updateTaskLockVerification(task.CanonicalName, commitHash)
	logInternalAction(db, task.ID, "seb-verificador", "verify", task.CanonicalName, fmt.Sprintf("mode=%s passed=%d failed=%d", effectiveMode, passed, failed), durVerify, 0)
	_, _ = db.Exec(`
		INSERT INTO tasks (id, name, spec_path, status, mode, updated_at)
		VALUES (?, ?, ?, 'VERIFIED', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET status = 'VERIFIED', mode = excluded.mode, updated_at = CURRENT_TIMESTAMP`,
		task.ID, task.Name, task.SpecPath, effectiveMode)
	fmt.Println("\n**VEREDICTO:** `APROBADO` — Todos los controles deterministas pasaron con éxito.")
}

// Analizador de tablas Markdown en specs/<NNN>_<tarea>_verificador.md.
// El ID de tarea se deriva del nombre del archivo y la existencia de specs se resuelve
// en el directorio del propio verificador (Spec 013).
func parseVerifierFile(path string) ([]CheckItem, error) {
	taskID, err := taskIDFromVerifierPath(path)
	if err != nil {
		return nil, err
	}
	knownSpecs := existingSpecIDs(filepath.Dir(path))

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

		// Filtrar lineas que no correspondan a una fila candidata a control:
		// primera celda que contiene "-CHK-" o comienza con "CHK-"
		firstCell, isRow := firstTableCell(trimmedLine)
		if !isRow || !isCandidateFirstCell(firstCell) {
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
		if err := validateCheckID(checkID, taskID); err != nil {
			return nil, fmt.Errorf("línea %d: %w", lineNum, err)
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
			if err := validateRequirementRef(trimmedP, knownSpecs); err != nil {
				return nil, fmt.Errorf("línea %d: %w", lineNum, err)
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
	case "seb-fast-task":
		return (strings.HasPrefix(target, "specs/") && (strings.HasSuffix(target, "_spec.md") || strings.HasSuffix(target, "_plan_y_verificador.md"))) || target == "MEMORY.md"
	case "seb-verificador":
		return target == "MEMORY.md"
	case "seb-documentador":
		normalizedTarget := filepath.ToSlash(filepath.Clean(target))
		return strings.HasPrefix(normalizedTarget, "docs/use-cases/") ||
			normalizedTarget == "docs/use-cases/README.md" ||
			normalizedTarget == "MEMORY.md"
	case "migrador":
		// Rol one-off (Spec 013): solo artefactos SDD, documentales y seb.lock; nunca codigo, tests ni constitucion
		return isMigradorWritableTarget(target)
	case "implementador", "default", "":
		// El agente implementador jamás puede tocar la constitución, specs ni verificadores (SDD o FAST)
		if target == "docs/constitution.md" ||
			(strings.HasPrefix(target, "specs/") && (strings.HasSuffix(target, "_spec.md") || strings.HasSuffix(target, "_verificador.md") || strings.HasSuffix(target, "_plan_y_verificador.md"))) {
			return false
		}
		return true
	default:
		// Habilidades de soporte o dominio desconocidas (RF-07) heredan la política de implementador:
		// Tienen permiso para modificar código de aplicación pero NO contratos protegidos (RF-08).
		if target == "docs/constitution.md" ||
			(strings.HasPrefix(target, "specs/") && (strings.HasSuffix(target, "_spec.md") || strings.HasSuffix(target, "_verificador.md") || strings.HasSuffix(target, "_plan_y_verificador.md"))) {
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

// 6. seb status implementado en status.go (runStatusCommand)

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
	b.WriteString("## Audit Trail & Observabilidad (`seb-harness`)\n\n")
	b.WriteString("### 1. Métricas por Tarea / Caso (Modo, Latencia, Tokens y Coste)\n")
	rows, err := db.Query(`
		SELECT COALESCE(NULLIF(a.task_id, ''), 'global') AS tid,
		       COALESCE(t.mode, CASE WHEN SUM(CASE WHEN a.skill = 'seb-fast-task' THEN 1 ELSE 0 END) > 0 THEN 'FAST' ELSE 'SDD' END) AS mode,
		       COUNT(*),
		       COALESCE(SUM(a.duration_ms), 0),
		       COALESCE(SUM(a.tokens_in), 0),
		       COALESCE(SUM(a.cache_read_tokens), 0),
		       COALESCE(SUM(a.tokens_out), 0),
		       ROUND(COALESCE(SUM(a.cost_usd), 0.0), 5)
		FROM agent_actions a
		LEFT JOIN tasks t ON t.id = a.task_id OR (t.id || '_' || t.name) = a.task_id
		GROUP BY tid
		ORDER BY 1 ASC`)
	if err != nil {
		fatal("Error generando reporte: %v", err)
	}
	defer rows.Close()

	type modeAgg struct {
		tasks int
		calls int
		dur   int64
		tIn   int64
		tOut  int64
		cost  float64
	}
	byMode := map[string]*modeAgg{
		taskModeSDD:  {},
		taskModeFAST: {},
	}

	b.WriteString("| Caso / Task ID | Modo | Llamadas a Tools | Latencia Total (ms) | Tokens In | Cache Read | Tokens Out | Coste Acumulado (USD) |\n")
	b.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")
	for rows.Next() {
		var task, mode string
		var calls int
		var dur, tIn, tCache, tOut int64
		var cost float64
		_ = rows.Scan(&task, &mode, &calls, &dur, &tIn, &tCache, &tOut, &cost)
		normMode := normalizeTaskMode(mode)
		if task == "global" {
			normMode = "-"
		} else if agg, ok := byMode[normMode]; ok {
			agg.tasks++
			agg.calls += calls
			agg.dur += dur
			agg.tIn += tIn
			agg.tOut += tOut
			agg.cost += cost
		}
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | %d | %d ms | %d | %d | %d | $%.5f |\n", task, normMode, calls, dur, tIn, tCache, tOut, cost))
	}

	b.WriteString("\n### 2. Resumen Comparativo por Modo (FAST vs SDD)\n")
	b.WriteString("| Modo | Tareas | Llamadas | Latencia Total (ms) | Tokens In | Tokens Out | Coste Acumulado (USD) |\n")
	b.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")
	for _, m := range []string{taskModeSDD, taskModeFAST} {
		agg := byMode[m]
		b.WriteString(fmt.Sprintf("| `%s` | %d | %d | %d ms | %d | %d | $%.5f |\n", m, agg.tasks, agg.calls, agg.dur, agg.tIn, agg.tOut, agg.cost))
	}

	b.WriteString("\n### 3. Últimas 10 Acciones Registradas\n")
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
	renderHelpCatalog(os.Stdout)
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

func readActiveTask(path string) string {
	if data, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// handleTaskSubcommand despacha subcomandos estructurados bajo 'seb task'
func handleTaskSubcommand(db *sql.DB, args []string) error {
	if len(args) == 0 {
		return errors.New("subcomando de tarea no especificado. Uso: seb task <abort> [flags] <tarea>")
	}
	switch args[0] {
	case "abort":
		return cmdAbort(db, args[1:])
	default:
		return fmt.Errorf("subcomando de tarea '%s' desconocido. Uso: seb task <abort>", args[0])
	}
}

// parseAbortFlags procesa argumentos y banderas para el comando abort
func parseAbortFlags(args []string) (taskQuery string, purge bool, force bool, err error) {
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" {
			continue
		}
		switch trimmed {
		case "--purge":
			purge = true
		case "--force", "-f":
			force = true
		default:
			if strings.HasPrefix(trimmed, "-") {
				return "", false, false, fmt.Errorf("bandera desconocida: '%s'", trimmed)
			}
			if taskQuery != "" {
				return "", false, false, fmt.Errorf("se proporciono mas de un identificador de tarea: '%s' y '%s'", taskQuery, trimmed)
			}
			taskQuery = trimmed
		}
	}
	if taskQuery == "" {
		return "", false, false, errors.New("identificador de tarea no especificado. Uso: seb abort <tarea> [--purge] [--force|-f]")
	}
	return taskQuery, purge, force, nil
}

// checkTaskDependents escanea las specs existentes para detectar tareas dependientes
func checkTaskDependents(targetTask TaskInfo) ([]string, error) {
	files, err := filepath.Glob("specs/*_*_spec.md")
	if err != nil {
		return nil, err
	}
	var dependents []string
	for _, f := range files {
		if filepath.Clean(f) == filepath.Clean(targetTask.SpecPath) {
			continue
		}
		deps, err := parseSpecDependencies(f)
		if err != nil {
			continue
		}
		for _, dep := range deps {
			cleanDep := strings.TrimSpace(dep)
			if matched, _ := regexp.MatchString(`^\d+$`, cleanDep); matched {
				var num int
				fmt.Sscanf(cleanDep, "%d", &num)
				cleanDep = fmt.Sprintf("%03d", num)
			}
			if cleanDep == targetTask.ID || cleanDep == targetTask.Name || cleanDep == targetTask.CanonicalName {
				base := filepath.Base(f)
				dependents = append(dependents, strings.TrimSuffix(base, "_spec.md"))
				break
			}
		}
	}
	return dependents, nil
}

// updateSpecHeaderStatus actualiza la linea de Estado en el archivo Markdown de la spec
func updateSpecHeaderStatus(specPath, newStatus string) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	reState := regexp.MustCompile(`^(\s*-\s*\*\*Estado:\*\*)\s*.*$`)
	replaced := false
	for i, line := range lines {
		if reState.MatchString(line) {
			lines[i] = fmt.Sprintf("- **Estado:** `%s`", newStatus)
			replaced = true
			break
		}
	}
	if !replaced {
		var newLines []string
		for i, line := range lines {
			newLines = append(newLines, line)
			if i == 0 && strings.HasPrefix(line, "# ") {
				newLines = append(newLines, fmt.Sprintf("- **Estado:** `%s`", newStatus))
				replaced = true
			}
		}
		if replaced {
			lines = newLines
		}
	}
	return os.WriteFile(specPath, []byte(strings.Join(lines, "\n")), 0644)
}

// cleanActiveTaskIfMatches elimina el puntero ACTIVE_TASK si corresponde a la tarea
func cleanActiveTaskIfMatches(task TaskInfo) {
	if activeData, err := os.ReadFile(".seb/ACTIVE_TASK"); err == nil {
		active := strings.TrimSpace(string(activeData))
		if active == task.ID || active == task.Name || active == task.CanonicalName {
			_ = os.Remove(".seb/ACTIVE_TASK")
		}
	}
}

// cmdAbort orquesta el aborto formal o la purga de una tarea
func cmdAbort(db *sql.DB, args []string) error {
	taskQuery, purge, force, err := parseAbortFlags(args)
	if err != nil {
		return err
	}

	task, err := resolveTask(db, taskQuery)
	if err != nil {
		return fmt.Errorf("tarea no encontrada: %w", err)
	}

	lock := readLockFile()

	// 1. Guardarrail de tarea verificada (RF-13)
	isVerified := (task.Status == "VERIFIED")
	tLock, inLock := lock.Tasks[task.CanonicalName]
	if !inLock {
		tLock = lock.Tasks[task.Name]
	}
	if tLock.Verified && strings.TrimSpace(tLock.CommitHash) != "" {
		isVerified = true
	}
	if isVerified && !force {
		return fmt.Errorf("BLOQUEO DE SEGURIDAD: La tarea '%s' ya esta formalmente verificada. Usa --force para forzar la operacion.", task.CanonicalName)
	}

	// 2. Guardarrail de tareas dependientes en el DAG (RF-12)
	dependents, err := checkTaskDependents(task)
	if err == nil && len(dependents) > 0 && !force {
		return fmt.Errorf("BLOQUEO DE SEGURIDAD: Las siguientes tareas dependen de '%s': %s. Usa --force para forzar la operacion.",
			task.CanonicalName, strings.Join(dependents, ", "))
	}

	// 3. Ejecutar segun modalidad
	if purge {
		return executeTaskPurge(db, task, lock)
	}
	return executeTaskAbortFormal(db, task, lock)
}

// executeTaskPurge ejecuta la eliminacion atómica y completa de la tarea
func executeTaskPurge(db *sql.DB, task TaskInfo, lock SebLock) error {
	// A. SQLite: eliminacion transaccional
	if db != nil {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("error iniciando transaccion: %w", err)
		}
		defer tx.Rollback()

		if _, err := tx.Exec(`DELETE FROM verifications WHERE task_id = ? OR task_id = ?`,
			task.ID, task.CanonicalName); err != nil {
			return fmt.Errorf("error purgando verifications: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM agent_actions WHERE task_id = ? OR task_id = ?`,
			task.ID, task.CanonicalName); err != nil {
			return fmt.Errorf("error purgando agent_actions: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM tasks WHERE id = ?`,
			task.ID); err != nil {
			return fmt.Errorf("error purgando tasks: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("error confirmando transaccion SQLite: %w", err)
		}
	}

	// B. Archivos fisicos en specs/ (incluyendo artefacto unificado FAST si existe)
	seenPaths := make(map[string]bool)
	for _, p := range []string{task.SpecPath, task.PlanPath, task.VerifierPath, task.FastPath} {
		if p != "" && !seenPaths[p] {
			seenPaths[p] = true
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("error eliminando '%s': %w", p, err)
			}
		}
	}

	// C. seb.lock
	delete(lock.Tasks, task.CanonicalName)
	delete(lock.Tasks, task.Name)
	writeLockFile(lock)

	// D. ACTIVE_TASK
	cleanActiveTaskIfMatches(task)

	fmt.Printf("[seb-harness] Tarea '%s' purgada completamente del sistema.\n", task.CanonicalName)
	return nil
}

// executeTaskAbortFormal cancela formalmente la tarea preservando telemetria
func executeTaskAbortFormal(db *sql.DB, task TaskInfo, lock SebLock) error {
	// A. SQLite: actualizar estado a ABORTED
	if db != nil {
		_, err := db.Exec(`
			INSERT INTO tasks (id, name, spec_path, status, updated_at)
			VALUES (?, ?, ?, 'ABORTED', CURRENT_TIMESTAMP)
			ON CONFLICT(id) DO UPDATE SET status = 'ABORTED', updated_at = CURRENT_TIMESTAMP`,
			task.ID, task.Name, task.SpecPath)
		if err != nil {
			return fmt.Errorf("error actualizando estado a ABORTED en SQLite: %w", err)
		}
	}

	// B. Archivo spec en disco: actualizar cabecera
	if task.SpecPath != "" {
		if _, err := os.Stat(task.SpecPath); err == nil {
			_ = updateSpecHeaderStatus(task.SpecPath, "ABORTADA")
		}
	}

	// C. seb.lock: marcar como cancelada
	tLock := lock.Tasks[task.CanonicalName]
	tLock.Verified = false
	tLock.Aborted = true
	lock.Tasks[task.CanonicalName] = tLock
	writeLockFile(lock)

	// D. ACTIVE_TASK
	cleanActiveTaskIfMatches(task)

	fmt.Printf("[seb-harness] Tarea '%s' abortada formalmente (telemetria preservada).\n", task.CanonicalName)
	return nil
}

const defaultActiveRolePath = ".seb/ACTIVE_ROLE"

var canonicalRoles = map[string]bool{
	"seb-iniciador":     true,
	"seb-especificador": true,
	"seb-clarificador":  true,
	"seb-planificador":  true,
	"seb-fast-task":     true,
	"seb-verificador":   true,
	"seb-documentador":  true,
	"implementador":     true,
	"migrador":          true,
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
		return fmt.Errorf("rol '%s' no reconocido. Roles canónicos válidos: seb-iniciador, seb-especificador, seb-clarificador, seb-planificador, seb-fast-task, seb-verificador, seb-documentador, implementador, migrador", role)
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
			return fmt.Errorf("debe especificar un rol válido. Roles canónicos: seb-iniciador, seb-especificador, seb-clarificador, seb-planificador, seb-fast-task, seb-verificador, seb-documentador, implementador, migrador")
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
		if exists && tLock.Aborted {
			return fmt.Errorf("la tarea prerrequisito '%s' fue abortada y no satisface el requisito", depTask.CanonicalName)
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

// 8. seb hook <pre-tool|post-tool|stop> [--format antigravity|claude]
func cmdHook(db *sql.DB, args []string) {
	format, remArgs, err := resolveHookFormat(args, os.Getenv("SEB_HOOK_FORMAT"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		os.Exit(1)
	}
	if len(remArgs) < 1 {
		if format == FormatAntigravity {
			fmt.Println("{}")
		}
		return
	}
	sub := remArgs[0]
	var payload []byte
	if stat, err := os.Stdin.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		payload, _ = io.ReadAll(os.Stdin)
	}
	cwd, _ := os.Getwd()
	envSkill := os.Getenv("SEB_SKILL")

	switch sub {
	case "pre-tool":
		out, emit, _ := processHookPreToolWithFormat(db, payload, envSkill, ".seb/ACTIVE_TASK", cwd, format)
		if emit {
			fmt.Println(out)
		}
	case "post-tool":
		out, emit, _ := processHookPostToolWithFormat(db, payload, envSkill, ".seb/ACTIVE_TASK", cwd, ".seb/config.json", format)
		if emit {
			fmt.Println(out)
		}
	case "stop":
		out, emit, _ := processHookStopWithFormat(db, payload, envSkill, ".seb/ACTIVE_TASK", cwd, ".seb/config.json", format)
		if emit {
			fmt.Println(out)
		}
	default:
		if format == FormatAntigravity {
			fmt.Println("{}")
		}
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
	if hasActiveLockedTask && (lastSkill == "seb-planificador" || lastSkill == "seb-fast-task") {
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
		if kl == "path" || kl == "file_path" || kl == "targetfile" || kl == "filepath" || kl == "notebook_path" {
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
var shellRedirectContractRegex = regexp.MustCompile(`(>|tee|sed\s+-i).*(constitution\.md|_spec\.md|_verificador\.md|_plan_y_verificador\.md)`)

// processHookPreTool ejecuta la logica de PreToolUse en formato Antigravity
func processHookPreTool(db *sql.DB, payloadBytes []byte, envSkill, activeTaskPath, workspaceRoot string) (string, error) {
	out, _, err := processHookPreToolWithFormat(db, payloadBytes, envSkill, activeTaskPath, workspaceRoot, FormatAntigravity)
	return out, err
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

// processHookPostTool ejecuta la logica de PostToolUse en formato Antigravity
func processHookPostTool(db *sql.DB, payloadBytes []byte, envSkill, activeTaskPath, workspaceRoot, configPath string) (string, error) {
	out, _, err := processHookPostToolWithFormat(db, payloadBytes, envSkill, activeTaskPath, workspaceRoot, configPath, FormatAntigravity)
	return out, err
}

// processHookStop inspecciona transcriptPath al cierre de turno en formato Antigravity
func processHookStop(db *sql.DB, payloadBytes []byte, envSkill, activeTaskPath, workspaceRoot, configPath string) (string, error) {
	out, _, err := processHookStopWithFormat(db, payloadBytes, envSkill, activeTaskPath, workspaceRoot, configPath, FormatAntigravity)
	return out, err
}
