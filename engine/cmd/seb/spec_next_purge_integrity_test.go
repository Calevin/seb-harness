package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// setupSpecNextTestEnv crea un entorno aislado para probar reservas y purgas
func setupSpecNextTestEnv(t *testing.T) (string, *sql.DB, func()) {
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

// 015-CHK-01 (015-RF-01, 015-RF-02, 015-RNF-01, 015-CL-01)
func TestSpecNextSequentialChaining(t *testing.T) {
	_, db, cleanup := setupSpecNextTestEnv(t)
	defer cleanup()

	// Simular dos llamadas consecutivas antes de redactar la primera spec
	pathA, err := specNext(db, []string{"tarea_a"})
	if err != nil {
		t.Fatalf("Primera llamada specNext fallo: %v", err)
	}
	pathB, err := specNext(db, []string{"tarea_b"})
	if err != nil {
		t.Fatalf("Segunda llamada specNext fallo: %v", err)
	}

	expectedA := filepath.Join("specs", "001_tarea_a_spec.md")
	expectedB := filepath.Join("specs", "002_tarea_b_spec.md")
	if pathA != expectedA {
		t.Fatalf("Ruta A esperada '%s', obtenida '%s'", expectedA, pathA)
	}
	if pathB != expectedB {
		t.Fatalf("Ruta B esperada '%s', obtenida '%s'", expectedB, pathB)
	}

	// Verificar existencia fisica y cabecera minima de ambos stubs
	for _, tc := range []struct {
		path string
		id   string
		name string
	}{
		{expectedA, "001", "tarea_a"},
		{expectedB, "002", "tarea_b"},
	} {
		raw, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("No se creo el stub '%s': %v", tc.path, err)
		}
		content := string(raw)
		for _, must := range []string{
			"**Tarea ID:** `" + tc.id + "`",
			"**Identificador:** `" + tc.name + "`",
			"**Estado:** `BORRADOR`",
			"**Afecta-UC:** [PENDIENTE]",
			"[NECESITA ACLARACIÓN]",
		} {
			if !strings.Contains(content, must) {
				t.Fatalf("Stub '%s' no contiene '%s'", tc.path, must)
			}
		}
	}

	// Verificar que ambas filas coexisten en SQLite sin sobrescritura
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("Se esperaban 2 filas en tasks, obtenido count=%d err=%v", count, err)
	}

	// Verificar autonomia multi-maquina en clon limpio sin filas en SQLite
	_, _ = db.Exec(`DELETE FROM tasks`)
	pathC, err := specNext(db, []string{"tarea_c"})
	if err != nil {
		t.Fatalf("Tercera llamada specNext tras limpiar DB fallo: %v", err)
	}
	expectedC := filepath.Join("specs", "003_tarea_c_spec.md")
	if pathC != expectedC {
		t.Fatalf("En clon limpio sin DB debio derivar '%s' desde specs/, obtenido '%s'", expectedC, pathC)
	}
}

// 015-CHK-02 (015-RF-03)
func TestSpecNextCollisionRejection(t *testing.T) {
	_, db, cleanup := setupSpecNextTestEnv(t)
	defer cleanup()

	// Registrar tarea 001 con nombre original en SQLite y disco
	origPath := filepath.Join("specs", "001_tarea_original_spec.md")
	if err := os.WriteFile(origPath, []byte(buildSpecStubContent("001", "tarea_original")), 0644); err != nil {
		t.Fatalf("Error creando spec original: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('001', 'tarea_original', ?, 'SPECIFIED')`, origPath); err != nil {
		t.Fatalf("Error insertando tarea original: %v", err)
	}

	// Intentar reservar/sincronizar el mismo ID 001 para un nombre distinto debe fallar
	err := reserveSpecInDB(db, "001", "tarea_intrusa", filepath.Join("specs", "001_tarea_intrusa_spec.md"))
	if err == nil {
		t.Fatalf("Se esperaba error de colision al reservar ID 001 con nombre diferente")
	}

	// Verificar que la fila original en SQLite y el archivo en disco permanecen intactos
	var nameInDB string
	if err := db.QueryRow(`SELECT name FROM tasks WHERE id = '001'`).Scan(&nameInDB); err != nil {
		t.Fatalf("Error consultando fila 001: %v", err)
	}
	if nameInDB != "tarea_original" {
		t.Fatalf("El registro original fue sobrescrito: '%s'", nameInDB)
	}
	if _, err := os.Stat(origPath); err != nil {
		t.Fatalf("El archivo original fue alterado o eliminado: %v", err)
	}
}

// 015-CHK-03 (015-RF-04, 015-RF-05, 015-CL-02)
func TestSpecNextStubIdempotencyAndEditedRejection(t *testing.T) {
	_, db, cleanup := setupSpecNextTestEnv(t)
	defer cleanup()

	// 1. Primera reserva crea el stub
	firstPath, err := specNext(db, []string{"mi_feature"})
	if err != nil {
		t.Fatalf("Primera llamada specNext fallo: %v", err)
	}

	// 2. Segunda llamada mientras sigue siendo stub debe ser idempotente
	secondPath, err := specNext(db, []string{"mi_feature"})
	if err != nil {
		t.Fatalf("Re-invocacion sobre stub sin editar debio ser idempotente: %v", err)
	}
	if secondPath != firstPath {
		t.Fatalf("Idempotencia devolvio ruta distinta: '%s' vs '%s'", secondPath, firstPath)
	}
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&count)
	if count != 1 {
		t.Fatalf("La re-invocacion idempotente no debio duplicar filas: count=%d", count)
	}

	// 3. Editar el archivo stub simulando redaccion de spec completa
	editedContent := "# Spec 001 — Mi Feature\n\n- **Tarea ID:** `001`\n- **Identificador:** `mi_feature`\n- **Estado:** `BORRADOR`\n- **Afecta-UC:** `nuevo`\n"
	if err := os.WriteFile(firstPath, []byte(editedContent), 0644); err != nil {
		t.Fatalf("Error editando spec: %v", err)
	}

	// 4. Invocar spec-next con el mismo nombre sobre una spec ya editada debe fallar
	if _, err := specNext(db, []string{"mi_feature"}); err == nil {
		t.Fatalf("Se esperaba rechazo al invocar spec-next sobre una spec ya editada")
	}

	// Verificar que el contenido editado sigue intacto
	afterRaw, _ := os.ReadFile(firstPath)
	if string(afterRaw) != editedContent {
		t.Fatalf("El contenido editado fue modificado tras rechazo")
	}
}

// 015-CHK-04 (015-RF-06, 015-RF-08, 015-CL-05)
func TestSpecNextDryRunAndInvalidNames(t *testing.T) {
	_, db, cleanup := setupSpecNextTestEnv(t)
	defer cleanup()

	// 1. Invocaciones repetidas de --dry-run
	expectedDry := filepath.Join("specs", "001_consulta_previa_spec.md")
	for i := 0; i < 3; i++ {
		p, err := specNext(db, []string{"--dry-run", "consulta_previa"})
		if err != nil {
			t.Fatalf("Invocacion %d de --dry-run fallo: %v", i+1, err)
		}
		if p != expectedDry {
			t.Fatalf("Ruta --dry-run esperada '%s', obtenida '%s'", expectedDry, p)
		}
	}

	if _, err := os.Stat(expectedDry); !os.IsNotExist(err) {
		t.Fatalf("--dry-run no debio crear archivo en disco")
	}
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&count)
	if count != 0 {
		t.Fatalf("--dry-run no debio insertar filas en SQLite, count=%d", count)
	}

	// 2. Rechazo de nombres vacios, invalidos o con traversal
	invalidArgs := [][]string{
		{},
		{""},
		{"   "},
		{"../escape"},
		{"sub/dir"},
		{"a\\b"},
		{"!!!"},
		{"--dry-run"},
	}
	for _, args := range invalidArgs {
		if _, err := specNext(db, args); err == nil {
			t.Fatalf("Se esperaba error para argumentos invalidos %v", args)
		}
	}
}

// 015-CHK-05 (015-RF-07, 015-RNF-02, 015-CL-04)
func TestSpecNextDiskFailureRollback(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("Prueba de permisos de solo lectura requiere usuario no-root")
	}

	_, db, cleanup := setupSpecNextTestEnv(t)
	defer cleanup()

	// Quitar permisos de escritura en specs/ para forzar fallo de disco
	if err := os.Chmod("specs", 0555); err != nil {
		t.Fatalf("Error cambiando permisos de specs/: %v", err)
	}
	defer func() {
		_ = os.Chmod("specs", 0755)
	}()

	_, err := specNext(db, []string{"fallo_disco"})
	if err == nil {
		t.Fatalf("Se esperaba error al fallar la escritura en specs/ de solo lectura")
	}

	// Verificar rollback completo en SQLite
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&count); err != nil {
		t.Fatalf("Error consultando tasks: %v", err)
	}
	if count != 0 {
		t.Fatalf("No se ejecuto rollback de la reserva en SQLite tras fallo de disco: count=%d", count)
	}
}

// 015-CHK-06 (015-RF-09, 015-RF-10, 015-RNF-02, 015-CL-03)
func TestAbortPurgeScopedToUniqueID(t *testing.T) {
	_, db, cleanup := setupSpecNextTestEnv(t)
	defer cleanup()

	// Insertar dos tareas con distinto ID pero exactamente el mismo name
	spec003 := filepath.Join("specs", "003_mi_tarea_spec.md")
	spec007 := filepath.Join("specs", "007_mi_tarea_spec.md")
	_ = os.WriteFile(spec003, []byte("# Spec 003"), 0644)
	_ = os.WriteFile(spec007, []byte("# Spec 007"), 0644)

	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('003', 'mi_tarea', ?, 'SPECIFIED')`, spec003)
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('007', 'mi_tarea', ?, 'SPECIFIED')`, spec007)
	_, _ = db.Exec(`INSERT INTO agent_actions (session_id, task_id, agent, action) VALUES ('s1', '003', 'test', 'run'), ('s1', '007', 'test', 'run')`)
	_, _ = db.Exec(`INSERT INTO verifications (task_id, check_id, req_id, command, status, exit_code, duration_ms) VALUES ('003', '003-CHK-01', '003-RF-01', 'true', 'PASA', 0, 1), ('007', '007-CHK-01', '007-RF-01', 'true', 'PASA', 0, 1)`)

	// 1. Referencia ambigua por nombre simple debe rechazarse sin borrar nada
	if err := cmdAbort(db, []string{"--purge", "mi_tarea"}); err == nil {
		t.Fatalf("Se esperaba error ante referencia ambigua 'mi_tarea'")
	}

	// 2. Referencia inexistente debe rechazarse sin borrar nada
	if err := cmdAbort(db, []string{"--purge", "inexistente"}); err == nil {
		t.Fatalf("Se esperaba error ante tarea inexistente")
	}

	// 3. Purga por ID univoco 003 debe eliminar solo 003 y preservar intacta 007
	if err := cmdAbort(db, []string{"--purge", "003"}); err != nil {
		t.Fatalf("Purga de 003 fallo: %v", err)
	}

	var count003, count007 int
	_ = db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE id = '003'`).Scan(&count003)
	_ = db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE id = '007'`).Scan(&count007)
	if count003 != 0 || count007 != 1 {
		t.Fatalf("Purga colateral detectada en tasks: count003=%d, count007=%d", count003, count007)
	}

	var actions007, verif007 int
	_ = db.QueryRow(`SELECT COUNT(*) FROM agent_actions WHERE task_id = '007'`).Scan(&actions007)
	_ = db.QueryRow(`SELECT COUNT(*) FROM verifications WHERE task_id = '007'`).Scan(&verif007)
	if actions007 != 1 || verif007 != 1 {
		t.Fatalf("Se eliminaron registros de telemetria de 007: actions=%d, verif=%d", actions007, verif007)
	}

	if _, err := os.Stat(spec007); err != nil {
		t.Fatalf("El archivo de spec007 fue eliminado por error: %v", err)
	}
}

// 015-CHK-07 (015-RF-11)
func TestEspecificadorStubDirective(t *testing.T) {
	skillPath := filepath.Join("..", "..", "..", "payload", ".agents", "skills", "seb-especificador", "SKILL.md")
	raw, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("No se pudo leer %s: %v", skillPath, err)
	}
	content := string(raw)
	if !strings.Contains(content, "stub") {
		t.Fatalf("SKILL.md de seb-especificador debe documentar la creacion del archivo stub inicial por spec-next")
	}
	if !strings.Contains(content, "complete") && !strings.Contains(content, "overwrite") {
		t.Fatalf("SKILL.md de seb-especificador debe indicar que el agente debe completar/sobrescribir el stub inicial")
	}
}
