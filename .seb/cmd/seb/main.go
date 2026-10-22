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

func openDB() (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
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
		tokens_out INTEGER DEFAULT 0,
		cost_usd REAL DEFAULT 0.0,
		exit_code INTEGER DEFAULT 0,
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
		return nil, err
	}
	return db, nil
}

// 1. seb spec-next <nombre_tarea>
func cmdSpecNext(db *sql.DB, args []string) {
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
	specPath := filepath.Join("specs", fmt.Sprintf("%s_%s_spec.md", nextID, taskName))

	// 2. Sincronizar idempotentemente la base de datos local
	_, err := db.Exec(`
		INSERT INTO tasks (id, name, spec_path, status)
		VALUES (?, ?, ?, 'SPECIFIED')
		ON CONFLICT(name) DO UPDATE SET spec_path=excluded.spec_path`,
		nextID, taskName, specPath)
	if err != nil {
		fatal("Error sincronizando tarea en SQLite: %v", err)
	}

	logInternalAction(db, nextID, "seb-especificador", "spec-next", taskName, specPath, 0)
	fmt.Println(specPath)
}

// 2. seb plan-lock <nombre_tarea> (Sella hashes SHA-256 de spec y verificador)
func cmdPlanLock(db *sql.DB, args []string) {
	if len(args) < 1 {
		fatal("Uso: seb plan-lock <nombre_tarea>")
	}
	taskName := sanitizeName(args[0])

	var id, specPath string
	err := db.QueryRow(`SELECT id, spec_path FROM tasks WHERE name = ?`, taskName).Scan(&id, &specPath)
	if err != nil {
		fatal("Tarea '%s' no encontrada en SQLite. Ejecuta primero seb spec-next.", taskName)
	}

	// Verificar que la spec no tenga etiquetas [NECESITA ACLARACIÓN] pendientes
	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		fatal("No se pudo leer %s: %v", specPath, err)
	}
	if strings.Contains(string(specBytes), "[NECESITA ACLARACIÓN]") {
		fatal("BLOQUEO SDD: La spec '%s' aún contiene etiquetas [NECESITA ACLARACIÓN]. Ejecuta el skill seb-clarificador antes de planificar.", specPath)
	}

	verifPath := filepath.Join("specs", fmt.Sprintf("%s_verificador.md", taskName))
	checks, err := parseVerifierFile(verifPath)
	if err != nil || len(checks) == 0 {
		fatal("BLOQUEO SDD: '%s' no existe o no contiene comandos de verificación programática válidos.", verifPath)
	}

	specHash, _ := fileSHA256(specPath)
	verifHash, _ := fileSHA256(verifPath)

	// Guardar hashes en seb.lock versionado por Git
	lock := readLockFile()
	lock.Tasks[taskName] = TaskLock{
		SpecHash:     specHash,
		VerifierHash: verifHash,
	}
	writeLockFile(lock)

	// Marcar la tarea como activa para el pre-commit hook
	setActiveTask(taskName)

	// Actualizar estado en SQLite (solo para observabilidad local)
	_, _ = db.Exec(`UPDATE tasks SET status = 'PLANNED', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)

	logInternalAction(db, id, "seb-planificador", "plan-lock", taskName, fmt.Sprintf("checks=%d", len(checks)), 0)
	fmt.Printf("[seb-harness] Plan sellado en .seb/seb.lock y marcado como activo para '%s'.\n", taskName)
}

// 3. seb verify <nombre_tarea>
func cmdVerify(db *sql.DB, args []string) {
	if len(args) < 1 {
		fatal("Uso: seb verify <nombre_tarea>")
	}
	taskName := sanitizeName(args[0])

	var id, specPath string
	err := db.QueryRow(`SELECT id, spec_path FROM tasks WHERE name = ?`, taskName).Scan(&id, &specPath)
	if err != nil {
		fatal("Tarea '%s' no registrada en SQLite.", taskName)
	}

	verifPath := filepath.Join("specs", fmt.Sprintf("%s_verificador.md", taskName))

	// Control de Integridad Anti-Reward-Hacking
	// Leer hashes desde seb.lock versionado por Git
	lock := readLockFile()
	taskLock, exists := lock.Tasks[taskName]

	if exists {
		if taskLock.SpecHash != "" {
			currentSpecHash, _ := fileSHA256(specPath)
			if currentSpecHash != taskLock.SpecHash {
				fatal("VIOLACIÓN DE INTEGRIDAD (Anti-Reward-Hacking): '%s' modificado.", specPath)
			}
		}
		if taskLock.VerifierHash != "" {
			currentVerifHash, _ := fileSHA256(verifPath)
			if currentVerifHash != taskLock.VerifierHash {
				fatal("VIOLACIÓN DE INTEGRIDAD (Anti-Reward-Hacking): '%s' modificado.", verifPath)
			}
		}
	}

	checks, err := parseVerifierFile(verifPath)
	if err != nil || len(checks) == 0 {
		fatal("No se encontraron controles ejecutables en %s: %v", verifPath, err)
	}

	commitHash := currentGitCommit()
	passed := 0
	failed := 0

	fmt.Printf("## Reporte de Verificación Determinista: `%s` (Spec `%s`)\n\n", taskName, id)
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
			id, c.ID, c.ReqID, c.Command, status, exitCode, duration, output, commitHash)
	}

	fmt.Printf("\n**Resumen de Cobertura:** %d/%d controles en `[PASA]`, %d en `[FALLA]`.\n", passed, len(checks), failed)

	if failed > 0 {
		_, _ = db.Exec(`UPDATE tasks SET status = 'FAILED_VERIFICATION', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
		fmt.Println(failureDetails.String())
		fmt.Println("\n**VEREDICTO:** `PENDIENTE DE IMPLEMENTACIÓN` — Corrige el código de aplicación sin alterar `*_verificador.md`.")
		os.Exit(2)
	}

	// Liberar el hook pre-commit porque la tarea pasó la verificación
	clearActiveTask()
	_, _ = db.Exec(`UPDATE tasks SET status = 'VERIFIED', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	fmt.Println("\n**VEREDICTO:** `APROBADO` — Todos los controles deterministas pasaron con éxito.")
}

// Parsea filas de tabla Markdown en specs/<tarea>_verificador.md:
// Formato esperado: | CHK-01 | RF-01 | Descripción | `comando a ejecutar` |
var tableRowRegex = regexp.MustCompile(`^\|\s*(CHK-[A-Za-z0-9_-]+)\s*\|\s*(RF-[A-Za-z0-9_,-]+)\s*\|\s*([^|]+)\|\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\|`)

func parseVerifierFile(path string) ([]CheckItem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var items []CheckItem
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		matches := tableRowRegex.FindStringSubmatch(line)
		if len(matches) == 5 {
			items = append(items, CheckItem{
				ID:      strings.TrimSpace(matches[1]),
				ReqID:   strings.TrimSpace(matches[2]),
				Desc:    strings.TrimSpace(matches[3]),
				Command: strings.TrimSpace(matches[4]),
			})
		}
	}
	return items, scanner.Err()
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
	if len(args) < 3 {
		fatal("Uso: seb guard <rol_o_skill> <write|exec> <ruta_o_comando>")
	}
	role := strings.TrimSpace(args[0])
	action := strings.TrimSpace(args[1])
	target := filepath.Clean(strings.TrimSpace(args[2]))

	allowed := checkPermission(role, action, target)
	if !allowed {
		logInternalAction(db, "", role, "guard-blocked:"+action, target, "DENIED", 1)
		fatal("GUARDARRAÍL DE MENOR PRIVILEGIO: El rol '%s' NO tiene permiso para '%s' sobre '%s'.", role, action, target)
	}
}

func checkPermission(role, action, target string) bool {
	if action != "write" {
		return true
	}
	switch role {
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
		return false
	}
}

// 5. seb memory-add "<categoria>" "<texto>"
func cmdMemoryAdd(db *sql.DB, args []string) {
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
	logInternalAction(db, "", "seb-cli", "memory-add", category, entry, 0)
	fmt.Println("[seb-harness] Entrada registrada en MEMORY.md y auditada en SQLite.")
}

// 6. seb status [nombre_tarea]
func cmdStatus(db *sql.DB, args []string) {
	rows, err := db.Query(`SELECT id, name, status, spec_path, updated_at FROM tasks ORDER BY id ASC`)
	if err != nil {
		fatal("Error consultando tareas: %v", err)
	}
	defer rows.Close()

	fmt.Println("| ID | Tarea | Estado SDD | Archivo Spec | Última Actualización |")
	fmt.Println("| :--- | :--- | :--- | :--- | :--- |")
	for rows.Next() {
		var id, name, status, specPath, updated string
		_ = rows.Scan(&id, &name, &status, &specPath, &updated)
		fmt.Printf("| `%s` | `%s` | **%s** | `%s` | %s |\n", id, name, status, specPath, updated)
	}
}

// 7. seb audit <log|report>
func cmdAudit(db *sql.DB, args []string) {
	if len(args) < 1 {
		fatal("Uso: seb audit <log|report> [flags]")
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
		tokensIn := fs.Int("tokens-in", 0, "Tokens de entrada")
		tokensOut := fs.Int("tokens-out", 0, "Tokens de salida")
		costUSD := fs.Float64("cost-usd", 0.0, "Costo estimado en USD")
		exitCode := fs.Int("exit-code", 0, "Código de salida")
		_ = fs.Parse(args[1:])

		if *sessionID == "" {
			*sessionID = "local-" + time.Now().Format("20060102")
		}
		if *agent == "" {
			*agent = "unknown-agent"
		}

		_, err := db.Exec(`INSERT INTO agent_actions
			(session_id, task_id, agent, skill, action, parameters, result_summary, level, duration_ms, tokens_in, tokens_out, cost_usd, exit_code)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			*sessionID, *taskID, *agent, *skill, *action, *params, *result, *level, *duration, *tokensIn, *tokensOut, *costUSD, *exitCode)
		if err != nil {
			fatal("Error insertando audit log: %v", err)
		}

	case "report":
		fmt.Println("## 📊 Audit Trail & Observabilidad (`seb-harness`)")
		fmt.Println("\n### 1. Métricas por Tarea / Caso (Latencia, Tokens y Coste)")
		rows, err := db.Query(`
			SELECT COALESCE(NULLIF(task_id, ''), 'global'),
			       COUNT(*),
			       SUM(duration_ms),
			       SUM(tokens_in),
			       SUM(tokens_out),
			       ROUND(SUM(cost_usd), 5)
			FROM agent_actions
			GROUP BY COALESCE(NULLIF(task_id, ''), 'global')
			ORDER BY 1 ASC`)
		if err != nil {
			fatal("Error generando reporte: %v", err)
		}
		defer rows.Close()

		fmt.Println("| Caso / Task ID | Llamadas a Tools | Latencia Total (ms) | Tokens In | Tokens Out | Coste Acumulado (USD) |")
		fmt.Println("| :--- | :--- | :--- | :--- | :--- | :--- |")
		for rows.Next() {
			var task string
			var calls int
			var dur, tIn, tOut int64
			var cost float64
			_ = rows.Scan(&task, &calls, &dur, &tIn, &tOut, &cost)
			fmt.Printf("| `%s` | %d | %d ms | %d | %d | $%.5f |\n", task, calls, dur, tIn, tOut, cost)
		}

		fmt.Println("\n### 2. Últimas 10 Acciones Registradas")
		recent, err := db.Query(`
			SELECT timestamp, agent, COALESCE(NULLIF(skill,''), '-'), action, duration_ms, tokens_in+tokens_out, cost_usd, exit_code
			FROM agent_actions ORDER BY id DESC LIMIT 10`)
		if err == nil {
			defer recent.Close()
			fmt.Println("| Timestamp | Agente | Skill | Acción / Tool | Latencia | Tokens | Costo USD | Exit |")
			fmt.Println("| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |")
			for recent.Next() {
				var ts, ag, sk, act string
				var dur, tok, code int64
				var cost float64
				_ = recent.Scan(&ts, &ag, &sk, &act, &dur, &tok, &cost, &code)
				fmt.Printf("| %s | `%s` | `%s` | `%s` | %d ms | %d | $%.5f | %d |\n", ts, ag, sk, act, dur, tok, cost, code)
			}
		}
	}
}

func logInternalAction(db *sql.DB, taskID, skill, action, params, result string, exitCode int) {
	session := os.Getenv("SEB_SESSION_ID")
	if session == "" {
		session = "seb-cli-" + time.Now().Format("20060102")
	}
	agent := os.Getenv("SEB_AGENT")
	if agent == "" {
		agent = "seb-bin"
	}
	_, _ = db.Exec(`INSERT INTO agent_actions (session_id, task_id, agent, skill, action, parameters, result_summary, exit_code)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		session, taskID, agent, skill, action, params, result, exitCode)
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
  seb memory-add "<categoría>" "<texto>"    Añade aprendizaje a MEMORY.md y registra en SQLite
  seb status                                Muestra el estado de todas las specs en el proyecto
  seb audit log [flags]                     Registra llamada a tool, latencia, tokens y costo USD
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
