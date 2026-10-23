package main

import (
	"path/filepath"
	"testing"
)

// TestGuardPermissionsPrefixed valida que las reglas de menor privilegio apliquen correctamente
// a rutas con prefijo secuencial specs/<NNN>_<tarea>_<tipo>.md
func TestGuardPermissionsPrefixed(t *testing.T) {
	testCases := []struct {
		role     string
		action   string
		target   string
		expected bool
	}{
		// seb-planificador debe poder escribir plan y verificador prefijados
		{"seb-planificador", "write", filepath.Join("specs", "002_tarea_plan.md"), true},
		{"seb-planificador", "write", filepath.Join("specs", "002_tarea_verificador.md"), true},
		{"seb-planificador", "write", filepath.Join("specs", "002_tarea_spec.md"), false},

		// implementador jamas debe poder escribir spec ni verificador prefijados
		{"implementador", "write", filepath.Join("specs", "002_tarea_spec.md"), false},
		{"implementador", "write", filepath.Join("specs", "002_tarea_verificador.md"), false},
		{"implementador", "write", filepath.Join("specs", "002_tarea_plan.md"), true},
		{"implementador", "write", filepath.Join("engine", "main.go"), true},

		// default/vacio (implementador por defecto)
		{"", "write", filepath.Join("specs", "001_tarea_verificador.md"), false},
		{"", "write", filepath.Join("specs", "001_tarea_spec.md"), false},
	}

	for _, tc := range testCases {
		got := checkPermission(tc.role, tc.action, tc.target)
		if got != tc.expected {
			t.Errorf("checkPermission(%q, %q, %q) = %v; expected %v", tc.role, tc.action, tc.target, got, tc.expected)
		}
	}
}
