package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupStatusWorkspace(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd fallo: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir fallo: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})
	if err := os.MkdirAll("specs", 0755); err != nil {
		t.Fatalf("mkdir specs fallo: %v", err)
	}
	if err := os.MkdirAll(".seb", 0755); err != nil {
		t.Fatalf("mkdir .seb fallo: %v", err)
	}
	return tmpDir
}

func openTestMemoryDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite memory fallo: %v", err)
	}
	if err := initDBSchema(db); err != nil {
		t.Fatalf("initDBSchema fallo: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestStatusUnifiedFullView verifica 016-CHK-03 (016-RF-01): cruce de specs/, seb.lock y SQLite.
func TestStatusUnifiedFullView(t *testing.T) {
	setupStatusWorkspace(t)
	db := openTestMemoryDB(t)

	_ = os.WriteFile("specs/001_base_spec.md", []byte("# Spec 001\n- **Estado:** `APROBADA`\n"), 0644)
	_ = os.WriteFile("specs/001_base_plan.md", []byte("# Plan 001\n"), 0644)
	_ = os.WriteFile("specs/001_base_verificador.md", []byte("# Verif 001\n"), 0644)
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('001', 'base', 'specs/001_base_spec.md', 'VERIFIED')`)

	writeLockFile(SebLock{
		Tasks: map[string]TaskLock{
			"001_base": {SpecHash: "h1", VerifierHash: "v1", Verified: true, CommitHash: "abc1234"},
		},
	})

	var out, errBuf bytes.Buffer
	code := runStatusCommand(db, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("codigo esperado 0, obtenido %d (stderr: %s)", code, errBuf.String())
	}
	got := out.String()
	for _, want := range []string{"001_base", "APROBADA", "VERIFIED", "abc1234", "plan=true", "verificador=true"} {
		if !strings.Contains(got, want) {
			t.Errorf("la salida completa debe contener %q; salida:\n%s", want, got)
		}
	}
}

// TestStatusTaskFilter verifica 016-CHK-04 (016-RF-02): resolucion flexible por ID, nombre o clave canonica.
func TestStatusTaskFilter(t *testing.T) {
	setupStatusWorkspace(t)
	_ = os.WriteFile("specs/001_alpha_spec.md", []byte("# Spec 001\n- **Estado:** `BORRADOR`\n"), 0644)
	_ = os.WriteFile("specs/002_beta_spec.md", []byte("# Spec 002\n- **Estado:** `APROBADA`\n"), 0644)

	for _, query := range []string{"2", "002", "beta", "002_beta"} {
		var out, errBuf bytes.Buffer
		code := runStatusCommand(nil, []string{query}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("consulta %q fallo con %d: %s", query, code, errBuf.String())
		}
		got := out.String()
		if !strings.Contains(got, "002_beta") || strings.Contains(got, "001_alpha") {
			t.Errorf("filtro %q no aislo exactamente 002_beta:\n%s", query, got)
		}
	}
}

// TestStatusCompact verifica 016-CHK-05 (016-RF-03): tabla resumida minima con --compact.
func TestStatusCompact(t *testing.T) {
	setupStatusWorkspace(t)
	_ = os.WriteFile("specs/001_alpha_spec.md", []byte("# Spec 001\n- **Estado:** `APROBADA`\n"), 0644)

	var out, errBuf bytes.Buffer
	code := runStatusCommand(nil, []string{"--compact"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("codigo esperado 0, obtenido %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "| ID | Clave Canonica | Editorial | Motor | Lock | Dependencias | Alertas |") {
		t.Fatalf("encabezado compacto inesperado:\n%s", got)
	}
	if strings.Contains(got, "Siguiente paso esperado:") {
		t.Fatalf("el modo --compact no debe imprimir bloques detallados")
	}
}

// TestStatusCleanCloneNoDB verifica 016-CHK-06 (016-RF-04): clon limpio sin seb-audit.db y sin crearla.
func TestStatusCleanCloneNoDB(t *testing.T) {
	tmpDir := setupStatusWorkspace(t)
	_ = os.RemoveAll(filepath.Join(tmpDir, ".seb"))
	_ = os.WriteFile("specs/003_clon_spec.md", []byte("# Spec 003\n- **Estado:** `APROBADA`\n"), 0644)

	var out, errBuf bytes.Buffer
	code := runStatusCommand(nil, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("codigo esperado 0 en clon limpio, obtenido %d (stderr: %s)", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "003_clon") || !strings.Contains(out.String(), "NO_REGISTRADA") {
		t.Fatalf("salida inesperada en clon limpio:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(tmpDir, dbPath)); !os.IsNotExist(err) {
		t.Fatalf("seb status no debe crear .seb/seb-audit.db en clones limpios")
	}
}

// TestStatusFullFieldsAndNextStep verifica 016-CHK-07 (016-RF-05): todos los campos y siguiente paso esperado.
func TestStatusFullFieldsAndNextStep(t *testing.T) {
	setupStatusWorkspace(t)
	_ = os.WriteFile("specs/001_base_spec.md", []byte("# Spec 001\n- **Estado:** `APROBADA`\n"), 0644)
	_ = os.WriteFile("specs/002_feat_spec.md", []byte("# Spec 002\n- **Estado:** `BORRADOR`\n- **Dependencias:** 001_base\n[NECESITA ACLARACIÓN] duda 1\n"), 0644)

	writeLockFile(SebLock{
		Tasks: map[string]TaskLock{
			"001_base": {SpecHash: "h1", VerifierHash: "v1", Verified: true, CommitHash: "c123"},
		},
	})

	var out, errBuf bytes.Buffer
	code := runStatusCommand(nil, []string{"002"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("codigo esperado 0, obtenido %d", code)
	}
	got := out.String()
	checks := []string{
		"Estado Editorial (Markdown): BORRADOR | Aclaraciones pendientes: 1",
		"Estado Motor (SQLite): NO_REGISTRADA",
		"Artefactos: plan=false, verificador=false",
		"Estado seb.lock: sellada=false",
		"Dependencias DAG: 001_base [OK]",
		"Siguiente paso esperado: Ejecutar skill 'seb-clarificador'",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("falta %q en salida:\n%s", want, got)
		}
	}
}

// TestStatusDriftDetection verifica 016-CHK-08 (016-RF-06): alertas por desfase entre disco, lock y SQLite.
func TestStatusDriftDetection(t *testing.T) {
	setupStatusWorkspace(t)
	db := openTestMemoryDB(t)

	_ = os.WriteFile("specs/001_abort_spec.md", []byte("# Spec 001\n- **Estado:** `ABORTADA`\n"), 0644)
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('001', 'abort', 'specs/001_abort_spec.md', 'PLANNED')`)

	var out, errBuf bytes.Buffer
	code := runStatusCommand(db, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("codigo esperado 0, obtenido %d", code)
	}
	if !strings.Contains(out.String(), "[ALERTA] Desfase de aborto") {
		t.Fatalf("se esperaba alerta de desfase de aborto:\n%s", out.String())
	}
}

// TestStatusInvalidOrAmbiguousTask verifica 016-CHK-09 (016-RF-07): codigo 1 ante tarea inexistente o ambigua.
func TestStatusInvalidOrAmbiguousTask(t *testing.T) {
	setupStatusWorkspace(t)
	_ = os.WriteFile("specs/001_shared_spec.md", []byte("# Spec 001\n- **Estado:** `BORRADOR`\n"), 0644)
	_ = os.WriteFile("specs/002_shared_spec.md", []byte("# Spec 002\n- **Estado:** `BORRADOR`\n"), 0644)

	var out1, err1 bytes.Buffer
	if code := runStatusCommand(nil, []string{"999"}, &out1, &err1); code != 1 || err1.Len() == 0 {
		t.Fatalf("tarea inexistente debe retornar 1 con error en stderr (code=%d)", code)
	}

	var out2, err2 bytes.Buffer
	if code := runStatusCommand(nil, []string{"shared"}, &out2, &err2); code != 1 || !strings.Contains(err2.String(), "ambiguo") {
		t.Fatalf("tarea ambigua debe retornar 1 con mensaje de ambiguedad (code=%d, stderr=%s)", code, err2.String())
	}
}

// TestStatusJSON verifica 016-CHK-10 (016-RF-08): salida JSON valida con --json.
func TestStatusJSON(t *testing.T) {
	setupStatusWorkspace(t)
	_ = os.WriteFile("specs/001_json_spec.md", []byte("# Spec 001\n- **Estado:** `APROBADA`\n"), 0644)

	var out, errBuf bytes.Buffer
	code := runStatusCommand(nil, []string{"--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("codigo esperado 0, obtenido %d (stderr: %s)", code, errBuf.String())
	}
	var parsed StatusReport
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("salida JSON invalida: %v\n%s", err, out.String())
	}
	if len(parsed.Tasks) != 1 || parsed.Tasks[0].CanonicalName != "001_json" {
		t.Fatalf("estructura JSON inesperada: %+v", parsed)
	}
}

// TestStatusAndHelpLatency100Specs verifica 016-CHK-20 (016-RNF-01): < 500ms con 100 specs.
func TestStatusAndHelpLatency100Specs(t *testing.T) {
	setupStatusWorkspace(t)
	for i := 1; i <= 100; i++ {
		path := fmt.Sprintf("specs/%03d_tarea_%d_spec.md", i, i)
		_ = os.WriteFile(path, []byte("# Spec\n- **Estado:** `APROBADA`\n"), 0644)
	}

	start := time.Now()
	var out, errBuf bytes.Buffer
	if code := runStatusCommand(nil, nil, &out, &errBuf); code != 0 {
		t.Fatalf("status fallo con %d", code)
	}
	if code := runHelpCommand([]string{"status"}, &out, &errBuf); code != 0 {
		t.Fatalf("help fallo con %d", code)
	}
	elapsed := time.Since(start)
	if elapsed >= 500*time.Millisecond {
		t.Fatalf("latencia excedio 500ms: %v", elapsed)
	}
}

// TestStatusAndHelpReadOnlyImmutability verifica 016-CHK-21 (016-RNF-02): sin escritura en specs, lock, ACTIVE_TASK o DB.
func TestStatusAndHelpReadOnlyImmutability(t *testing.T) {
	tmpDir := setupStatusWorkspace(t)
	_ = os.RemoveAll(filepath.Join(tmpDir, ".seb"))
	specPath := filepath.Join("specs", "001_ro_spec.md")
	origSpec := []byte("# Spec 001\n- **Estado:** `BORRADOR`\n")
	_ = os.WriteFile(specPath, origSpec, 0644)

	var out, errBuf bytes.Buffer
	_ = runStatusCommand(nil, nil, &out, &errBuf)
	_ = runHelpCommand([]string{"plan-lock"}, &out, &errBuf)

	for _, forbidden := range []string{dbPath, defaultLockPath, ".seb/ACTIVE_TASK"} {
		if _, err := os.Stat(forbidden); !os.IsNotExist(err) {
			t.Errorf("operacion de lectura creo archivo prohibido: %s", forbidden)
		}
	}
	afterSpec, _ := os.ReadFile(specPath)
	if !bytes.Equal(origSpec, afterSpec) {
		t.Errorf("spec modificada durante consulta de solo lectura")
	}
}

// TestStatusEmptyRepo verifica 016-CHK-23 (016-CL-01): repo vacio retorna codigo 0.
func TestStatusEmptyRepo(t *testing.T) {
	setupStatusWorkspace(t)
	var out, errBuf bytes.Buffer
	code := runStatusCommand(nil, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("codigo esperado 0 en repo vacio, obtenido %d", code)
	}
	if !strings.Contains(out.String(), "No hay tareas") {
		t.Fatalf("mensaje inesperado en repo vacio: %s", out.String())
	}
}

// TestStatusUnknownEditorialState verifica 016-CHK-24 (016-CL-02): estado faltante/desconocido sin abortar.
func TestStatusUnknownEditorialState(t *testing.T) {
	setupStatusWorkspace(t)
	_ = os.WriteFile("specs/001_sin_estado_spec.md", []byte("# Spec 001 sin cabecera de estado\n"), 0644)
	_ = os.WriteFile("specs/002_valida_spec.md", []byte("# Spec 002\n- **Estado:** `APROBADA`\n"), 0644)

	var out, errBuf bytes.Buffer
	code := runStatusCommand(nil, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("codigo esperado 0, obtenido %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "DESCONOCIDO") || !strings.Contains(got, "002_valida") {
		t.Fatalf("debe reportar DESCONOCIDO y continuar con 002_valida:\n%s", got)
	}
}

// TestStatusMultipleSimultaneousDrifts verifica 016-CHK-26 (016-CL-04): reporta todas las alertas sin ocultar ninguna.
func TestStatusMultipleSimultaneousDrifts(t *testing.T) {
	setupStatusWorkspace(t)
	db := openTestMemoryDB(t)

	_ = os.WriteFile("specs/001_multi_spec.md", []byte("# Spec 001\n- **Estado:** `ABORTADA`\n"), 0644)
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('001', 'multi', 'specs/001_multi_spec.md', 'VERIFIED')`)

	var out, errBuf bytes.Buffer
	code := runStatusCommand(db, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("codigo esperado 0, obtenido %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "Desfase de aborto") || !strings.Contains(got, "Desfase de verificacion") || !strings.Contains(got, "Desfase de artefactos") {
		t.Fatalf("deben reportarse los 3 desfases simultaneamente:\n%s", got)
	}
}

// TestStatusCorruptLockFile verifica 016-CHK-27 (016-CL-05): seb.lock corrupto falla con stderr y codigo 1.
func TestStatusCorruptLockFile(t *testing.T) {
	setupStatusWorkspace(t)
	_ = os.WriteFile(defaultLockPath, []byte("{corrupto"), 0644)

	var out, errBuf bytes.Buffer
	code := runStatusCommand(nil, nil, &out, &errBuf)
	if code != 1 {
		t.Fatalf("codigo esperado 1 ante seb.lock corrupto, obtenido %d", code)
	}
	if errBuf.Len() == 0 {
		t.Fatalf("se esperaba mensaje de error por stderr")
	}
}
