package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// setupAbortTestEnv crea un entorno temporal aislado para pruebas de aborto/purga
func setupAbortTestEnv(t *testing.T) (string, *sql.DB, func()) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error obteniendo directorio actual: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Error cambiando a directorio temporal: %v", err)
	}

	if err := os.MkdirAll(".seb", 0755); err != nil {
		t.Fatalf("Error creando .seb: %v", err)
	}
	if err := os.MkdirAll("specs", 0755); err != nil {
		t.Fatalf("Error creando specs: %v", err)
	}

	db, err := openDB()
	if err != nil {
		t.Fatalf("Error abriendo DB de prueba: %v", err)
	}

	cleanup := func() {
		db.Close()
		_ = os.Chdir(origDir)
	}

	return tmpDir, db, cleanup
}

// CHK-01 (RF-01..05, RNF-03, CL-01, CL-03)
// Aborto formal por defecto: transicion a ABORTED, cabecera de spec actualizada,
// preservacion de telemetria SQLite, actualizacion de seb.lock y limpieza de ACTIVE_TASK
func TestTaskAbortFormal(t *testing.T) {
	_, db, cleanup := setupAbortTestEnv(t)
	defer cleanup()

	// 1. Crear tarea previa con spec y telemetria
	taskID := "001"
	taskName := "tarea_demo"
	canonical := "001_tarea_demo"
	specPath := filepath.Join("specs", "001_tarea_demo_spec.md")

	specContent := `# Spec 001 — Tarea Demo
- **Tarea ID:** ` + "`001`" + `
- **Identificador:** ` + "`tarea_demo`" + `
- **Estado:** ` + "`BORRADOR`" + `

## 1. Contexto
Texto descriptivo.
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Error escribiendo spec: %v", err)
	}

	// Registrar en SQLite
	_, err := db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES (?, ?, ?, 'SPECIFIED')`,
		taskID, taskName, specPath)
	if err != nil {
		t.Fatalf("Error insertando tarea en DB: %v", err)
	}

	// Insertar telemetria (acciones y verificaciones)
	_, err = db.Exec(`INSERT INTO agent_actions (session_id, task_id, agent, action, duration_ms, tokens_in, tokens_out, cost_usd)
		VALUES ('sess-1', '001', 'mimo-agent', 'read_file', 120, 500, 150, 0.005)`)
	if err != nil {
		t.Fatalf("Error insertando accion de telemetria: %v", err)
	}

	// Sellar en seb.lock
	lock := readLockFile()
	lock.Tasks[canonical] = TaskLock{
		SpecHash:     "fake-spec-hash",
		VerifierHash: "fake-verifier-hash",
		Verified:     false,
	}
	writeLockFile(lock)

	// Configurar como ACTIVE_TASK
	setActiveTask(canonical)
	if active := readActiveTask(".seb/ACTIVE_TASK"); active != canonical {
		t.Fatalf("Se esperaba tarea activa '%s', se obtuvo '%s'", canonical, active)
	}

	// 2. Ejecutar aborto formal
	if err := cmdAbort(db, []string{"001"}); err != nil {
		t.Fatalf("cmdAbort retorno error inesperado: %v", err)
	}

	// RF-01: Estado en SQLite debe ser 'ABORTED'
	var dbStatus string
	err = db.QueryRow(`SELECT status FROM tasks WHERE id = ?`, taskID).Scan(&dbStatus)
	if err != nil || dbStatus != "ABORTED" {
		t.Fatalf("Se esperaba status 'ABORTED' en SQLite, obtenido '%s' (err: %v)", dbStatus, err)
	}

	// RF-01: Cabecera en archivo spec debe estar actualizada a 'Estado: ABORTADA'
	updatedSpecBytes, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("No se encontro el archivo spec tras aborto formal: %v", err)
	}
	if !strings.Contains(string(updatedSpecBytes), "**Estado:** `ABORTADA`") &&
		!strings.Contains(string(updatedSpecBytes), "**Estado:** ABORTADA") {
		t.Fatalf("La cabecera de la spec no fue actualizada a ABORTADA: %s", string(updatedSpecBytes))
	}

	// RF-02: Telemetria SQLite debe conservarse intacta
	var actionCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM agent_actions WHERE task_id = '001'`).Scan(&actionCount)
	if err != nil || actionCount != 1 {
		t.Fatalf("La telemetria fue alterada o borrada en aborto formal. Filas: %d", actionCount)
	}

	// RF-04 y CL-03: Limpieza de ACTIVE_TASK
	if _, err := os.Stat(".seb/ACTIVE_TASK"); !os.IsNotExist(err) {
		t.Fatalf("El archivo .seb/ACTIVE_TASK debio ser eliminado tras abortar la tarea activa")
	}

	// RF-05: seb.lock debe registrar verified: false y aborted: true
	updatedLock := readLockFile()
	tLock, exists := updatedLock.Tasks[canonical]
	if !exists {
		t.Fatalf("La tarea '%s' desaparecio de .seb/seb.lock", canonical)
	}
	if tLock.Verified || !tLock.Aborted {
		t.Fatalf("seb.lock no refleja Aborted: true y Verified: false: %+v", tLock)
	}

	// RNF-03: Idempotencia: Invocacion repetida no debe fallar
	if err := cmdAbort(db, []string{"tarea_demo"}); err != nil {
		t.Fatalf("Segunda llamada a cmdAbort (idempotente) fallo: %v", err)
	}
}

// CHK-02 (RF-06..09, RNF-02, RNF-04, CL-02, CL-04)
// Purga atomica con --purge: eliminacion de filas SQLite, eliminacion fisica de artefactos
// en specs/, remocion en seb.lock y liberacion de prefijo para spec-next
func TestTaskAbortPurge(t *testing.T) {
	_, db, cleanup := setupAbortTestEnv(t)
	defer cleanup()

	taskID := "001"
	taskName := "tarea_purgar"
	canonical := "001_tarea_purgar"
	specPath := filepath.Join("specs", "001_tarea_purgar_spec.md")
	planPath := filepath.Join("specs", "001_tarea_purgar_plan.md")
	verifPath := filepath.Join("specs", "001_tarea_purgar_verificador.md")

	_ = os.WriteFile(specPath, []byte("# Spec 001"), 0644)
	_ = os.WriteFile(planPath, []byte("# Plan 001"), 0644)
	_ = os.WriteFile(verifPath, []byte("# Verificador 001"), 0644)

	// Insertar en SQLite
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES (?, ?, ?, 'PLANNED')`,
		taskID, taskName, specPath)
	_, _ = db.Exec(`INSERT INTO agent_actions (session_id, task_id, agent, action) VALUES ('s1', '001', 'test', 'run')`)
	_, _ = db.Exec(`INSERT INTO verifications (task_id, check_id, req_id, command, status, exit_code, duration_ms)
		VALUES ('001', 'CHK-01', 'RF-01', 'echo ok', 'PASA', 0, 10)`)

	// Lock y Active Task
	lock := readLockFile()
	lock.Tasks[canonical] = TaskLock{SpecHash: "h1", VerifierHash: "h2"}
	writeLockFile(lock)
	setActiveTask(canonical)

	// Ejecutar purga con normalizacion de espacios y mayusculas (CL-04)
	if err := cmdAbort(db, []string{"--purge", "  001  "}); err != nil {
		t.Fatalf("cmdAbort con --purge fallo: %v", err)
	}

	// RF-06: Verificacion de eliminacion en SQLite
	var countTasks, countActions, countVerif int
	_ = db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE id = '001'`).Scan(&countTasks)
	_ = db.QueryRow(`SELECT COUNT(*) FROM agent_actions WHERE task_id = '001'`).Scan(&countActions)
	_ = db.QueryRow(`SELECT COUNT(*) FROM verifications WHERE task_id = '001'`).Scan(&countVerif)

	if countTasks != 0 || countActions != 0 || countVerif != 0 {
		t.Fatalf("La purga en SQLite no fue completa: tasks=%d, actions=%d, verif=%d",
			countTasks, countActions, countVerif)
	}

	// RF-07: Eliminacion fisica de archivos en specs/
	for _, f := range []string{specPath, planPath, verifPath} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Fatalf("El archivo '%s' no fue eliminado fisicamente en la purga", f)
		}
	}

	// RF-08: Remocion de clave en seb.lock
	lockAfter := readLockFile()
	if _, exists := lockAfter.Tasks[canonical]; exists {
		t.Fatalf("La clave '%s' no fue removida de .seb/seb.lock", canonical)
	}

	// Limpieza de active task
	if _, err := os.Stat(".seb/ACTIVE_TASK"); !os.IsNotExist(err) {
		t.Fatalf(".seb/ACTIVE_TASK debio ser eliminado en la purga")
	}

	// RF-09: Reutilizacion del ID en spec-next tras la purga
	// Al no haber specs, spec-next debe volver a reservar 001
	cmdSpecNext(db, []string{"nueva_tarea"})
	expectedNext := filepath.Join("specs", "001_nueva_tarea_spec.md")
	if _, err := os.Stat(expectedNext); err != nil {
		// En cmdSpecNext no se crea el archivo fisico, solo se devuelve e inserta en DB
		var nextInDB string
		_ = db.QueryRow(`SELECT id FROM tasks WHERE name = 'nueva_tarea'`).Scan(&nextInDB)
		if nextInDB != "001" {
			t.Fatalf("spec-next tras la purga no reutilizo el prefijo 001: %s", nextInDB)
		}
	}
}

// CHK-03 (RF-10..14, RNF-01)
// Guardarrailes de seguridad: bloqueo de tareas verificadas o con dependencias sin --force,
// rechazo de IDs inexistentes/ambiguos y alias CLI
func TestTaskAbortGuardsAndErrors(t *testing.T) {
	_, db, cleanup := setupAbortTestEnv(t)
	defer cleanup()

	// 1. RF-10: Error ante tarea inexistente
	errNotFound := cmdAbort(db, []string{"tarea_fantasma"})
	if errNotFound == nil || !strings.Contains(errNotFound.Error(), "no encontrada") {
		t.Fatalf("Se esperaba error 'no encontrada' para tarea inexistente: %v", errNotFound)
	}

	// 2. RF-13: Bloqueo de tarea VERIFIED sin --force
	specVerified := filepath.Join("specs", "001_tarea_verificada_spec.md")
	_ = os.WriteFile(specVerified, []byte("# Spec 001"), 0644)
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('001', 'tarea_verificada', ?, 'VERIFIED')`, specVerified)

	lock := readLockFile()
	lock.Tasks["001_tarea_verificada"] = TaskLock{
		SpecHash:   "h1",
		Verified:   true,
		CommitHash: "c0ffee",
	}
	writeLockFile(lock)

	errVerified := cmdAbort(db, []string{"001"})
	if errVerified == nil || !strings.Contains(errVerified.Error(), "--force") {
		t.Fatalf("Se esperaba bloqueo por tarea ya verificada requiriendo --force: %v", errVerified)
	}

	// Con flag --force debe permitirse
	errWithForce := cmdAbort(db, []string{"001", "--force"})
	if errWithForce != nil {
		t.Fatalf("cmdAbort con --force debio prosperar: %v", errWithForce)
	}

	// 3. RF-12: Bloqueo si otra tarea depende de la tarea a abortar
	specBase := filepath.Join("specs", "002_tarea_base_spec.md")
	_ = os.WriteFile(specBase, []byte("# Spec 002"), 0644)
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('002', 'tarea_base', ?, 'SPECIFIED')`, specBase)

	specDep := filepath.Join("specs", "003_tarea_dependiente_spec.md")
	specDepContent := `# Spec 003
- **Dependencias:** 002_tarea_base
`
	_ = os.WriteFile(specDep, []byte(specDepContent), 0644)
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('003', 'tarea_dependiente', ?, 'SPECIFIED')`, specDep)

	errDep := cmdAbort(db, []string{"002"})
	if errDep == nil || !strings.Contains(errDep.Error(), "dependen") {
		t.Fatalf("Se esperaba bloqueo por dependencias activas: %v", errDep)
	}

	// Con -f debe permitirlo
	errDepForce := cmdAbort(db, []string{"002", "-f"})
	if errDepForce != nil {
		t.Fatalf("cmdAbort con -f ante dependencias debio prosperar: %v", errDepForce)
	}

	// 4. RF-14: Soporte de sintaxis alias 'task abort'
	specAlias := filepath.Join("specs", "004_tarea_alias_spec.md")
	_ = os.WriteFile(specAlias, []byte("# Spec 004"), 0644)
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('004', 'tarea_alias', ?, 'SPECIFIED')`, specAlias)

	// Simular invocacion de 'seb task abort 004'
	errAlias := handleTaskSubcommand(db, []string{"abort", "004"})
	if errAlias != nil {
		t.Fatalf("handleTaskSubcommand fallo con 'abort 004': %v", errAlias)
	}
}
