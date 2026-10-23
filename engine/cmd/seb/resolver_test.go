package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestArtifactPathFormatting valida que los paths de artefactos sigan el patron specs/<NNN>_<tarea>_<tipo>.md
func TestArtifactPathFormatting(t *testing.T) {
	testCases := []struct {
		id       string
		name     string
		kind     string
		expected string
	}{
		{"001", "registro_telemetria", "spec", filepath.Join("specs", "001_registro_telemetria_spec.md")},
		{"002", "prefijo_secuencial", "plan", filepath.Join("specs", "002_prefijo_secuencial_plan.md")},
		{"003", "auth_jwt", "verificador", filepath.Join("specs", "003_auth_jwt_verificador.md")},
	}

	for _, tc := range testCases {
		got := buildArtifactPath(tc.id, tc.name, tc.kind)
		if got != tc.expected {
			t.Errorf("buildArtifactPath(%q, %q, %q) = %q; expected %q", tc.id, tc.name, tc.kind, got, tc.expected)
		}
	}
}

// TestResolveTask valida la resolucion flexible por ID, nombre simple y clave canonica con manejo de ambiguedad
func TestResolveTask(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insertar tareas de prueba en base de datos
	_, err := db.Exec(`
		INSERT INTO tasks (id, name, spec_path, status) VALUES
		('001', 'registro_telemetria', 'specs/001_registro_telemetria_spec.md', 'VERIFIED'),
		('002', 'prefijo_secuencial', 'specs/002_prefijo_secuencial_spec.md', 'PLANNED'),
		('003', 'tarea_duplicada', 'specs/003_tarea_duplicada_spec.md', 'SPECIFIED'),
		('004', 'tarea_duplicada', 'specs/004_tarea_duplicada_spec.md', 'SPECIFIED');
	`)
	if err != nil {
		t.Fatalf("Error insertando fixtures de tareas: %v", err)
	}

	// 1. Resolucion por ID con tres digitos
	task, err := resolveTask(db, "002")
	if err != nil {
		t.Fatalf("Fallo resolucion por ID '002': %v", err)
	}
	if task.ID != "002" || task.Name != "prefijo_secuencial" || task.CanonicalName != "002_prefijo_secuencial" {
		t.Errorf("Datos de tarea inesperados al resolver por ID: %+v", task)
	}
	expectedPlan := filepath.Join("specs", "002_prefijo_secuencial_plan.md")
	if task.PlanPath != expectedPlan {
		t.Errorf("PlanPath esperado %q, obtenido %q", expectedPlan, task.PlanPath)
	}

	// 2. Resolucion por ID numerico no rellenado
	task2, err := resolveTask(db, "2")
	if err != nil {
		t.Fatalf("Fallo resolucion por ID '2': %v", err)
	}
	if task2.ID != "002" {
		t.Errorf("Esperado ID '002', obtenido %q", task2.ID)
	}

	// 3. Resolucion por nombre simple
	task3, err := resolveTask(db, "registro_telemetria")
	if err != nil {
		t.Fatalf("Fallo resolucion por nombre simple: %v", err)
	}
	if task3.ID != "001" || task3.CanonicalName != "001_registro_telemetria" {
		t.Errorf("Datos inesperados al resolver por nombre simple: %+v", task3)
	}

	// 4. Resolucion por clave canonica completa
	task4, err := resolveTask(db, "002_prefijo_secuencial")
	if err != nil {
		t.Fatalf("Fallo resolucion por clave canonica: %v", err)
	}
	if task4.ID != "002" {
		t.Errorf("Esperado ID '002', obtenido %q", task4.ID)
	}

	// 5. Error en tarea inexistente
	_, err = resolveTask(db, "no_existe")
	if err == nil {
		t.Error("Se esperaba error al buscar tarea inexistente, pero retorno nil")
	}

	// 6. Deteccion de ambiguedad al buscar nombre simple duplicado
	_, err = resolveTask(db, "tarea_duplicada")
	if err == nil {
		t.Error("Se esperaba error por ambiguedad al buscar nombre duplicado, pero retorno nil")
	}
	if !strings.Contains(err.Error(), "ambiguo") {
		t.Errorf("Mensaje de error debe indicar ambiguedad, obtenido: %v", err)
	}
}

// TestLockCanonicalKey comprueba el almacenamiento con clave canonica en SebLock
func TestLockCanonicalKey(t *testing.T) {
	lock := SebLock{
		Tasks: make(map[string]TaskLock),
	}

	canonicalKey := "002_prefijo_secuencial_artefactos"
	lock.Tasks[canonicalKey] = TaskLock{
		SpecHash:     "hash-spec-test",
		VerifierHash: "hash-verifier-test",
	}

	taskLock, exists := lock.Tasks[canonicalKey]
	if !exists {
		t.Fatalf("No se encontro la tarea por clave canonica %q", canonicalKey)
	}
	if taskLock.SpecHash != "hash-spec-test" || taskLock.VerifierHash != "hash-verifier-test" {
		t.Errorf("Valores de hash no coinciden: %+v", taskLock)
	}
}
