package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHelpCatalog verifica 016-CHK-11 (016-RF-09): catalogo general de todos los subcomandos activos.
func TestHelpCatalog(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runHelpCommand(nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("codigo esperado 0, obtenido %d (stderr: %s)", code, errBuf.String())
	}

	expectedCmds := []string{
		"init", "spec-next", "plan-lock", "verify", "status",
		"abort", "task", "guard", "role", "memory-add",
		"audit", "hook", "oneoff", "help",
	}
	got := out.String()
	for _, cmd := range expectedCmds {
		if !strings.Contains(got, cmd) {
			t.Errorf("el catalogo de 'seb help' omite el subcomando '%s'", cmd)
		}
	}
	if !strings.Contains(got, "seb help <subcomando>") {
		t.Errorf("el catalogo debe indicar como consultar la ayuda individual")
	}
}

// TestHelpSubcommandAndFlags verifica 016-CHK-12 (016-RF-10): seb help <cmd>, --help y -h retornan 0 sin efectos secundarios.
func TestHelpSubcommandAndFlags(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir fallo: %v", err)
	}
	defer os.Chdir(origDir)

	var out1, err1 bytes.Buffer
	if code := runHelpCommand([]string{"status"}, &out1, &err1); code != 0 {
		t.Fatalf("seb help status fallo con codigo %d", code)
	}
	if !strings.Contains(out1.String(), "Subcomando: seb status") {
		t.Fatalf("salida inesperada para 'seb help status': %s", out1.String())
	}

	if !hasHelpFlag([]string{"--help"}) || !hasHelpFlag([]string{"foo", "-h"}) {
		t.Fatalf("hasHelpFlag debe detectar --help y -h")
	}
	if hasHelpFlag([]string{"--compact", "001"}) {
		t.Fatalf("hasHelpFlag no debe activarse sin --help/-h")
	}

	if _, err := os.Stat(filepath.Join(tmpDir, dbPath)); !os.IsNotExist(err) {
		t.Fatalf("la consulta de ayuda no debe crear .seb/seb-audit.db")
	}
}

// TestHelpTechnicalSheetFields verifica 016-CHK-13 (016-RF-11): todas las secciones obligatorias en cada ficha.
func TestHelpTechnicalSheetFields(t *testing.T) {
	requiredSections := []string{
		"Proposito:",
		"Sintaxis:",
		"Flags / Argumentos:",
		"Archivos y tablas que lee:",
		"Archivos y tablas que modifica:",
		"Precondiciones y guardarrailes:",
		"Transicion de estado:",
		"Codigos de salida:",
	}

	for _, spec := range commandCatalog {
		var out, errBuf bytes.Buffer
		code := runHelpCommand([]string{spec.Name}, &out, &errBuf)
		if code != 0 {
			t.Fatalf("seb help %s retorno %d", spec.Name, code)
		}
		text := out.String()
		for _, sec := range requiredSections {
			if !strings.Contains(text, sec) {
				t.Errorf("la ficha de '%s' omite la seccion '%s'", spec.Name, sec)
			}
		}
	}
}

// TestHelpUnknownSubcommand verifica 016-CHK-14 (016-RF-12): rechazo con codigo 1 ante subcomando inexistente.
func TestHelpUnknownSubcommand(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runHelpCommand([]string{"no_existe_cmd"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("codigo esperado 1, obtenido %d", code)
	}
	if errBuf.Len() == 0 {
		t.Fatalf("se esperaba mensaje de error por stderr")
	}
	if out.Len() != 0 {
		t.Fatalf("no debe emitirse salida por stdout ante error: %s", out.String())
	}
}

// TestHelpFlagPrecedence verifica 016-CHK-25 (016-CL-03): --help / -h tiene precedencia incluso con args invalidos.
func TestHelpFlagPrecedence(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runHelpCommand([]string{"verify", "--flag-invalido", "-h"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("codigo esperado 0 con -h, obtenido %d (stderr: %s)", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "Subcomando: seb verify") {
		t.Fatalf("se esperaba ficha de verify, obtenido: %s", out.String())
	}
}

// TestRootUsageCompact verifica 016-CHK-22 (016-RNF-03): salida compacta de catalogo sin volcar fichas completas.
func TestRootUsageCompact(t *testing.T) {
	var out bytes.Buffer
	renderHelpCatalog(&out)
	text := out.String()
	if strings.Contains(text, "Archivos y tablas que lee:") {
		t.Fatalf("el catalogo base no debe volcar fichas tecnicas completas")
	}
	if len(strings.Split(strings.TrimSpace(text), "\n")) > 25 {
		t.Fatalf("el catalogo base excede el tamano compacto esperado")
	}
}

// TestGuardAndHookAllowStatusAndHelp verifica 016-CHK-19 (016-RF-17): todos los roles pueden ejecutar status y help.
func TestGuardAndHookAllowStatusAndHelp(t *testing.T) {
	roles := []string{
		"seb-iniciador",
		"seb-especificador",
		"seb-clarificador",
		"seb-planificador",
		"implementador",
		"seb-verificador",
		"seb-documentador",
	}
	cmds := []string{
		".seb/bin/seb status",
		".seb/bin/seb status --json",
		".seb/bin/seb help",
		".seb/bin/seb help oneoff",
		".seb/bin/seb verify --help",
	}

	tmpDir := t.TempDir()
	for _, role := range roles {
		for _, c := range cmds {
			if !checkPermission(role, "exec", c) {
				t.Errorf("checkPermission denego exec '%s' para rol '%s'", c, role)
			}
			payload := `{"session_id":"s1","cwd":"` + tmpDir + `","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"` + c + `"}}`
			out, _, err := processHookPreToolWithFormat(nil, []byte(payload), role, "", tmpDir, FormatAntigravity)
			if err != nil {
				t.Fatalf("error en PreToolUse (%s / %s): %v", role, c, err)
			}
			if strings.Contains(out, `"deny"`) {
				t.Errorf("PreToolUse denego '%s' para rol '%s': %s", c, role, out)
			}
		}
	}
}
