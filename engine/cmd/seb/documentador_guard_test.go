package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDocumentadorGuardAndRoles valida los permisos de menor privilegio y la gestion de roles para seb-documentador
func TestDocumentadorGuardAndRoles(t *testing.T) {
	// 1. Validar permisos de menor privilegio en checkPermission
	testCases := []struct {
		role     string
		action   string
		target   string
		expected bool
	}{
		// Permitido escribir en casos de uso y memoria tecnica
		{"seb-documentador", "write", filepath.Join("docs", "use-cases", "UC-auth_login.md"), true},
		{"seb-documentador", "write", filepath.Join("docs", "use-cases", "README.md"), true},
		{"seb-documentador", "write", "MEMORY.md", true},

		// Prohibido escribir en codigo de aplicacion, tests, constitucion y specs
		{"seb-documentador", "write", filepath.Join("engine", "cmd", "seb", "main.go"), false},
		{"seb-documentador", "write", filepath.Join("engine", "cmd", "seb", "guard_test.go"), false},
		{"seb-documentador", "write", filepath.Join("docs", "constitution.md"), false},
		{"seb-documentador", "write", filepath.Join("specs", "010_integracion_documentador_spec.md"), false},
		{"seb-documentador", "write", filepath.Join("specs", "010_integracion_documentador_plan.md"), false},
		{"seb-documentador", "write", filepath.Join("specs", "010_integracion_documentador_verificador.md"), false},

		// Operaciones de lectura siempre permitidas
		{"seb-documentador", "read", filepath.Join("engine", "cmd", "seb", "main.go"), true},
		{"seb-documentador", "read", filepath.Join("docs", "constitution.md"), true},
	}

	for _, tc := range testCases {
		got := checkPermission(tc.role, tc.action, tc.target)
		if got != tc.expected {
			t.Errorf("checkPermission(%q, %q, %q) = %v; se esperaba %v", tc.role, tc.action, tc.target, got, tc.expected)
		}
	}

	// 2. Validar que seb-documentador es un rol canonico y gestionable
	tmpDir := t.TempDir()
	roleFile := filepath.Join(tmpDir, ".seb", "ACTIVE_ROLE")

	if !isCanonicalRole("seb-documentador") {
		t.Fatalf("seb-documentador debe estar reconocido en canonicalRoles")
	}

	if err := setActiveRole("seb-documentador", roleFile); err != nil {
		t.Fatalf("Fallo asignando rol 'seb-documentador': %v", err)
	}

	if cur := getActiveRole(roleFile); cur != "seb-documentador" {
		t.Fatalf("getActiveRole retorno '%s', se esperaba 'seb-documentador'", cur)
	}

	// 3. Validar latencia de checkPermission (RNF-01: < 5ms por evaluacion)
	start := time.Now()
	iterations := 1000
	for i := 0; i < iterations; i++ {
		_ = checkPermission("seb-documentador", "write", "docs/use-cases/UC-test.md")
	}
	elapsed := time.Since(start)
	avgMs := float64(elapsed.Microseconds()) / (float64(iterations) * 1000.0)
	if avgMs > 5.0 {
		t.Fatalf("Latencia promedio de checkPermission excedio los 5ms: %.3f ms", avgMs)
	}

	// 4. Validar ensureDirectories en un directorio temporal aislado
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error obteniendo directorio actual: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Error cambiando a directorio temporal: %v", err)
	}
	defer os.Chdir(origDir)

	if err := ensureDirectories(); err != nil {
		t.Fatalf("ensureDirectories fallo: %v", err)
	}

	requiredDirs := []string{
		filepath.Join(".agents", "skills", "seb-documentador"),
		filepath.Join("docs", "use-cases"),
	}
	for _, reqDir := range requiredDirs {
		info, err := os.Stat(reqDir)
		if err != nil || !info.IsDir() {
			t.Fatalf("Directorio requerido no fue creado por ensureDirectories: %s", reqDir)
		}
	}
}
