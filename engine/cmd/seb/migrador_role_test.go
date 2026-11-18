package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 013-RF-14, 013-RF-17, 013-RF-19, 013-RF-33
func TestMigradorRoleAndGuard(t *testing.T) {
	t.Run("migrador es rol canonico en role set/get/clear y SEB_SKILL", func(t *testing.T) {
		if !isCanonicalRole("migrador") {
			t.Fatal("migrador debe ser rol canonico")
		}
		rolePath := filepath.Join(t.TempDir(), ".seb", "ACTIVE_ROLE")
		if err := setActiveRole("migrador", rolePath); err != nil {
			t.Fatalf("role set migrador fallo: %v", err)
		}
		if got := getActiveRole(rolePath); got != "migrador" {
			t.Fatalf("role get debio devolver migrador, got %s", got)
		}
		if got := inferActiveSkill("", "", "", rolePath); got != "migrador" {
			t.Errorf("ACTIVE_ROLE migrador debe resolverse en nivel 2, got %s", got)
		}
		if got := inferActiveSkill("migrador", "", "", filepath.Join(t.TempDir(), "none")); got != "migrador" {
			t.Errorf("SEB_SKILL=migrador debe resolverse en nivel 1, got %s", got)
		}
		if err := clearActiveRole(rolePath); err != nil {
			t.Fatalf("role clear fallo: %v", err)
		}
		if got := getActiveRole(rolePath); got != "implementador" {
			t.Fatalf("tras clear el rol debe ser implementador, got %s", got)
		}
		t.Setenv("SEB_SKILL", "migrador")
		if got := resolveCLIRole(rolePath); got != "migrador" {
			t.Errorf("resolveCLIRole debe priorizar SEB_SKILL, got %s", got)
		}
		t.Setenv("SEB_SKILL", "")
		if got := resolveCLIRole(rolePath); got != "implementador" {
			t.Errorf("resolveCLIRole sin rol debe devolver implementador, got %s", got)
		}
	})

	t.Run("politica de escritura del migrador", func(t *testing.T) {
		allowed := []string{
			"specs/001_tarea_spec.md", "specs/001_tarea_plan.md", "specs/001_tarea_verificador.md",
			"docs/use-cases/README.md", "docs/use-cases/UC-x_y.md", "MEMORY.md", ".seb/seb.lock",
		}
		denied := []string{
			"docs/constitution.md", "engine/cmd/seb/main.go", "engine/cmd/seb/main_test.go",
			"install.sh", "AGENTS.md", "specs/sub/001_x_spec.md", "specs/notas.md", ".seb/ACTIVE_ROLE",
		}
		for _, p := range allowed {
			if !checkPermission("migrador", "write", p) {
				t.Errorf("migrador debe poder escribir %s", p)
			}
		}
		for _, p := range denied {
			if checkPermission("migrador", "write", p) {
				t.Errorf("migrador NO debe poder escribir %s", p)
			}
		}
	})

	t.Run("one-off denegada para roles distintos de migrador", func(t *testing.T) {
		bin := buildSebForTest(t)
		dir := newSebProject(t, bin)
		// Implementador por defecto (sin ACTIVE_ROLE ni SEB_SKILL)
		code, _, stderr := runSeb(t, bin, dir, nil, "oneoff", "013")
		if code != 1 || !strings.Contains(stderr, "GUARDARRAÍL") || !strings.Contains(stderr, "seb role set migrador") || !strings.Contains(stderr, "implementador") {
			t.Errorf("implementador por defecto debio ser denegado: %d %q", code, stderr)
		}
		// Rol activo de otro skill
		_ = os.WriteFile(filepath.Join(dir, ".seb", "ACTIVE_ROLE"), []byte("seb-planificador"), 0644)
		code, _, stderr = runSeb(t, bin, dir, nil, "oneoff", "013", "--dry-run")
		if code != 1 || !strings.Contains(stderr, "seb-planificador") {
			t.Errorf("seb-planificador debio ser denegado: %d %q", code, stderr)
		}
		// SEB_SKILL de otro rol
		code, _, _ = runSeb(t, bin, dir, []string{"SEB_SKILL=seb-verificador"}, "oneoff", "013")
		if code != 1 {
			t.Errorf("SEB_SKILL=seb-verificador debio ser denegado, got %d", code)
		}
	})

	t.Run("anti-escalada en PreToolUse", func(t *testing.T) {
		tmpDir := t.TempDir()
		blocked := []string{
			".seb/bin/seb oneoff 013",
			"SEB_SKILL=migrador .seb/bin/seb oneoff 013 --dry-run",
			".seb/bin/seb role set migrador",
			"export SEB_SKILL=\\\"migrador\\\" && ls",
		}
		for _, envSkill := range []string{"implementador", "migrador", "seb-planificador"} {
			for _, c := range blocked {
				payload := `{"session_id":"s1","cwd":"` + tmpDir + `","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"` + c + `"}}`
				out, emit, err := processHookPreToolWithFormat(nil, []byte(payload), envSkill, "", tmpDir, FormatClaude)
				if err != nil || !emit {
					t.Fatalf("Comando '%s' debio emitir decision: emit=%v err=%v", c, emit, err)
				}
				assertClaudeDenyJSON(t, out, "ANTI-ESCALADA")
			}
		}
		// El listado sin ID es de solo lectura y se permite
		payload := `{"session_id":"s1","cwd":"` + tmpDir + `","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":".seb/bin/seb oneoff"}}`
		out, _, err := processHookPreToolWithFormat(nil, []byte(payload), "implementador", "", tmpDir, FormatClaude)
		if err != nil || strings.Contains(out, "ANTI-ESCALADA") {
			t.Errorf("El listado 'seb oneoff' debe permitirse: out=%q err=%v", out, err)
		}
	})
}

// 013-RF-15, 013-RF-16, 013-RF-18
func TestOneOffRegistryCommand(t *testing.T) {
	bin := buildSebForTest(t)
	migrador := []string{"SEB_SKILL=migrador"}

	t.Run("listado sin ID con aplicabilidad y sin escrituras", func(t *testing.T) {
		dir := newSebProject(t, bin)
		specs := filepath.Join(dir, "specs")
		writeSpecStub(t, specs, "001", "tarea")
		vPath := writeVerifierFixture(t, specs, "001", "tarea", "| CHK-01 | RF-01 | Heredado | `echo ok` |\n")
		before, _ := os.ReadFile(vPath)
		code, stdout, stderr := runSeb(t, bin, dir, nil, "oneoff")
		if code != 0 {
			t.Fatalf("listado debio salir con 0: %d %s", code, stderr)
		}
		if !strings.Contains(stdout, "| 013 |") || !strings.Contains(stdout, "APLICABLE") || !strings.Contains(stdout, "1 verificadores") {
			t.Errorf("listado debe mostrar 013 aplicable con detalle: %q", stdout)
		}
		after, _ := os.ReadFile(vPath)
		if string(before) != string(after) {
			t.Error("el listado no debe modificar archivos")
		}

		clean := newSebProject(t, bin)
		_, stdoutClean, _ := runSeb(t, bin, clean, nil, "oneoff")
		if !strings.Contains(stdoutClean, "NO APLICABLE") {
			t.Errorf("proyecto sin heredados debe mostrar NO APLICABLE: %q", stdoutClean)
		}
	})

	t.Run("ejecucion por ID de spec", func(t *testing.T) {
		dir := newGitSebProject(t, bin)
		specs := filepath.Join(dir, "specs")
		writeSpecStub(t, specs, "001", "tarea")
		writeVerifierFixture(t, specs, "001", "tarea", "| CHK-01 | RF-01 | Heredado | `echo ok` |\n")
		gitCommitAll(t, dir)
		for _, id := range []string{"013", "13"} {
			code, stdout, stderr := runSeb(t, bin, dir, migrador, "oneoff", id, "--dry-run")
			if code != 0 || !strings.Contains(stdout, "Operación one-off 013") {
				t.Errorf("oneoff %s debio ejecutar la operacion 013: %d %q %q", id, code, stdout, stderr)
			}
		}
	})

	t.Run("ID no registrado lista los disponibles", func(t *testing.T) {
		dir := newSebProject(t, bin)
		code, _, stderr := runSeb(t, bin, dir, migrador, "oneoff", "999")
		if code != 1 || !strings.Contains(stderr, "999") || !strings.Contains(stderr, "IDs disponibles: 013") {
			t.Errorf("ID inexistente debio abortar con lista: %d %q", code, stderr)
		}
	})
}
