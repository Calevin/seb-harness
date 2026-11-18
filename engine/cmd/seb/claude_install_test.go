package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repoRootDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("No se pudo obtener cwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

func copyFileExec(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("Error leyendo %s: %v", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		t.Fatalf("Error creando directorio %s: %v", filepath.Dir(dst), err)
	}
	if err := os.WriteFile(dst, data, 0755); err != nil {
		t.Fatalf("Error escribiendo %s: %v", dst, err)
	}
}

// CHK-03 (RF-16, CL-04): Reenvio de argumentos y fallback seguro en scripts plantilla
func TestHookScriptsArgForwardingAndFallback(t *testing.T) {
	root := repoRootDir(t)
	tmpRepo := t.TempDir()

	hooksDir := filepath.Join(tmpRepo, ".agents", "hooks")
	for _, name := range []string{"pre-tool.sh", "post-tool.sh", "stop.sh"} {
		copyFileExec(t, filepath.Join(root, "payload", ".agents", "hooks", name), filepath.Join(hooksDir, name))
	}

	// 1. Sin binario .seb/bin/seb: fallback en formato claude debe salir 0 sin stdout (CL-04)
	for _, name := range []string{"pre-tool.sh", "post-tool.sh", "stop.sh"} {
		cmd := exec.Command("bash", filepath.Join(hooksDir, name), "--format", "claude")
		cmd.Dir = tmpRepo
		var out, errBuf bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			t.Fatalf("Fallback %s --format claude fallo: %v (stderr: %s)", name, err, errBuf.String())
		}
		if strings.TrimSpace(out.String()) != "" {
			t.Fatalf("Fallback %s --format claude debe tener stdout vacio, obtenido: %q", name, out.String())
		}
	}

	// 2. Sin binario y sin --format: fallback Antigravity intacto
	cmdAnti := exec.Command("bash", filepath.Join(hooksDir, "pre-tool.sh"))
	cmdAnti.Dir = tmpRepo
	outAnti, err := cmdAnti.Output()
	if err != nil {
		t.Fatalf("Fallback Antigravity fallo: %v", err)
	}
	if strings.TrimSpace(string(outAnti)) != `{"decision":"allow"}` {
		t.Fatalf("Fallback Antigravity inesperado: %q", string(outAnti))
	}

	// 3. Con binario compilado: reenvia --format claude y bloquea edicion de constitution.md
	sebBin := filepath.Join(tmpRepo, ".seb", "bin", "seb")
	if err := os.MkdirAll(filepath.Dir(sebBin), 0755); err != nil {
		t.Fatalf("Error creando .seb/bin: %v", err)
	}
	buildCmd := exec.Command("go", "build", "-o", sebBin, "./cmd/seb")
	buildCmd.Dir = filepath.Join(root, "engine")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Error compilando seb para test: %v\n%s", err, string(out))
	}

	cmdBlock := exec.Command("bash", filepath.Join(hooksDir, "pre-tool.sh"), "--format", "claude")
	cmdBlock.Dir = tmpRepo
	cmdBlock.Stdin = strings.NewReader(`{"session_id":"s1","cwd":"` + tmpRepo + `","hook_event_name":"PreToolUse","tool_name":"Edit","tool_input":{"file_path":"` + tmpRepo + `/docs/constitution.md"}}`)
	outBlock, err := cmdBlock.Output()
	if err != nil {
		t.Fatalf("Ejecucion de pre-tool.sh --format claude fallo: %v", err)
	}
	assertClaudeDenyJSON(t, string(outBlock), "GUARDARRAIL")
}

// CHK-04 (RF-01..RF-05, RNF-04, CL-02, CL-03): Instalacion idempotente para Claude
func TestInstallClaudeIntegration(t *testing.T) {
	root := repoRootDir(t)
	installSh := filepath.Join(root, "install.sh")

	// Caso A: Proyecto limpio + idempotencia en doble ejecucion
	repoClean := t.TempDir()
	for i := 0; i < 2; i++ {
		cmd := exec.Command("bash", installSh, "--no-build", repoClean)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("install.sh ejecucion %d fallo: %v\n%s", i+1, err, string(out))
		}
	}

	for _, dir := range []string{"hooks", "rules", "skills"} {
		linkPath := filepath.Join(repoClean, ".claude", dir)
		target, err := os.Readlink(linkPath)
		if err != nil {
			t.Fatalf("Se esperaba symlink en %s: %v", linkPath, err)
		}
		if target != "../.agents/"+dir {
			t.Fatalf("Symlink %s apunta a %q, esperado %q", linkPath, target, "../.agents/"+dir)
		}
	}

	settingsPath := filepath.Join(repoClean, ".claude", "settings.json")
	settingsBytes, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("No se creo .claude/settings.json: %v", err)
	}
	var settingsMap map[string]interface{}
	if err := json.Unmarshal(settingsBytes, &settingsMap); err != nil {
		t.Fatalf(".claude/settings.json invalido: %v", err)
	}
	hooksMap, ok := settingsMap["hooks"].(map[string]interface{})
	if !ok {
		t.Fatalf("Falta clave 'hooks' en settings.json")
	}
	for _, ev := range []string{"PreToolUse", "PostToolUse", "Stop"} {
		arr, ok := hooksMap[ev].([]interface{})
		if !ok || len(arr) != 1 {
			t.Fatalf("Evento %s debe tener exactamente 1 entrada tras doble instalacion, obtenido %v", ev, hooksMap[ev])
		}
	}
	if !strings.Contains(string(settingsBytes), `$CLAUDE_PROJECT_DIR/.claude/hooks/pre-tool.sh --format claude`) {
		t.Fatalf("settings.json no usa $CLAUDE_PROJECT_DIR con --format claude: %s", string(settingsBytes))
	}

	// Caso B: Preservacion de directorio real existente y fusion con settings.json previo (CL-02)
	repoMerge := t.TempDir()
	customHooksDir := filepath.Join(repoMerge, ".claude", "hooks")
	if err := os.MkdirAll(customHooksDir, 0755); err != nil {
		t.Fatalf("Error creando directorio previo: %v", err)
	}
	customScript := filepath.Join(customHooksDir, "custom.sh")
	_ = os.WriteFile(customScript, []byte("#!/bin/sh\n"), 0755)
	prevSettings := `{"permissions":{"allow":["Bash(ls:*)"]},"hooks":{"PreToolUse":[{"matcher":"Custom","hooks":[{"type":"command","command":"echo custom"}]}]}}`
	_ = os.WriteFile(filepath.Join(repoMerge, ".claude", "settings.json"), []byte(prevSettings), 0644)

	cmdMerge := exec.Command("bash", installSh, "--no-build", repoMerge)
	if out, err := cmdMerge.CombinedOutput(); err != nil {
		t.Fatalf("install.sh en repoMerge fallo: %v\n%s", err, string(out))
	}
	// .claude/hooks debe seguir siendo directorio real (no symlink) y conservar custom.sh
	fi, err := os.Lstat(customHooksDir)
	if err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Fatalf(".claude/hooks preexistente no debio ser reemplazado por symlink")
	}
	if _, err := os.Stat(customScript); err != nil {
		t.Fatalf("Archivo preexistente custom.sh fue eliminado")
	}
	// .claude/rules y .claude/skills si deben haberse creado como symlinks
	if _, err := os.Readlink(filepath.Join(repoMerge, ".claude", "rules")); err != nil {
		t.Fatalf(".claude/rules debio crearse como symlink")
	}
	mergedBytes, _ := os.ReadFile(filepath.Join(repoMerge, ".claude", "settings.json"))
	if !strings.Contains(string(mergedBytes), "Bash(ls:*)") || !strings.Contains(string(mergedBytes), "echo custom") ||
		!strings.Contains(string(mergedBytes), `$CLAUDE_PROJECT_DIR/.claude/hooks/pre-tool.sh --format claude`) {
		t.Fatalf("settings.json fusionado perdio configuracion previa o no agrego seb: %s", string(mergedBytes))
	}

	// Caso C: JSON corrupto aborta sin modificar el archivo (CL-03)
	repoCorrupt := t.TempDir()
	_ = os.MkdirAll(filepath.Join(repoCorrupt, ".claude"), 0755)
	corruptContent := `{invalid json`
	_ = os.WriteFile(filepath.Join(repoCorrupt, ".claude", "settings.json"), []byte(corruptContent), 0644)
	cmdCorrupt := exec.Command("bash", installSh, "--no-build", repoCorrupt)
	if err := cmdCorrupt.Run(); err == nil {
		t.Fatalf("install.sh debio abortar ante settings.json corrupto")
	}
	afterCorrupt, _ := os.ReadFile(filepath.Join(repoCorrupt, ".claude", "settings.json"))
	if string(afterCorrupt) != corruptContent {
		t.Fatalf("settings.json corrupto fue alterado: %q", string(afterCorrupt))
	}
}

// CHK-05 (RF-17, RF-18, CL-03, CL-06): Desinstalacion quirurgica de Claude
func TestUninstallClaudeIntegration(t *testing.T) {
	root := repoRootDir(t)
	installSh := filepath.Join(root, "install.sh")
	uninstallSh := filepath.Join(root, "uninstall.sh")

	// Caso A: Desinstalacion con configuracion mixta (dry-run y ejecucion real)
	repoMixed := t.TempDir()
	if out, err := exec.Command("bash", installSh, "--no-build", repoMixed).CombinedOutput(); err != nil {
		t.Fatalf("install.sh fallo: %v\n%s", err, string(out))
	}
	settingsPath := filepath.Join(repoMixed, ".claude", "settings.json")
	raw, _ := os.ReadFile(settingsPath)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	m["theme"] = "dark"
	hooks := m["hooks"].(map[string]interface{})
	pre := hooks["PreToolUse"].([]interface{})
	hooks["PreToolUse"] = append(pre, map[string]interface{}{
		"matcher": "Bash",
		"hooks": []interface{}{
			map[string]interface{}{"type": "command", "command": "echo user-hook"},
		},
	})
	updated, _ := json.MarshalIndent(m, "", "  ")
	_ = os.WriteFile(settingsPath, updated, 0644)

	// Dry-run no debe modificar nada
	if out, err := exec.Command("bash", uninstallSh, "--dry-run", repoMixed).CombinedOutput(); err != nil {
		t.Fatalf("uninstall.sh --dry-run fallo: %v\n%s", err, string(out))
	}
	if _, err := os.Readlink(filepath.Join(repoMixed, ".claude", "skills")); err != nil {
		t.Fatalf("dry-run elimino symlink .claude/skills")
	}

	// Ejecucion real
	if out, err := exec.Command("bash", uninstallSh, repoMixed).CombinedOutput(); err != nil {
		t.Fatalf("uninstall.sh fallo: %v\n%s", err, string(out))
	}
	for _, dir := range []string{"hooks", "rules", "skills"} {
		if _, err := os.Lstat(filepath.Join(repoMixed, ".claude", dir)); !os.IsNotExist(err) {
			t.Fatalf("Symlink .claude/%s debio ser removido", dir)
		}
	}
	afterBytes, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json con config de usuario debio preservarse: %v", err)
	}
	afterStr := string(afterBytes)
	if !strings.Contains(afterStr, `"theme": "dark"`) || !strings.Contains(afterStr, "echo user-hook") {
		t.Fatalf("Se perdio configuracion de usuario tras uninstall: %s", afterStr)
	}
	if strings.Contains(afterStr, ".claude/hooks/pre-tool.sh") || strings.Contains(afterStr, ".claude/hooks/stop.sh") {
		t.Fatalf("Quedaron hooks de seb en settings.json tras uninstall: %s", afterStr)
	}

	// Caso B: Repo limpio sin config de usuario debe remover settings.json y .claude/ vacio (CL-06)
	repoPure := t.TempDir()
	if out, err := exec.Command("bash", installSh, "--no-build", repoPure).CombinedOutput(); err != nil {
		t.Fatalf("install.sh en repoPure fallo: %v\n%s", err, string(out))
	}
	if out, err := exec.Command("bash", uninstallSh, repoPure).CombinedOutput(); err != nil {
		t.Fatalf("uninstall.sh en repoPure fallo: %v\n%s", err, string(out))
	}
	if _, err := os.Stat(filepath.Join(repoPure, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("Directorio .claude vacio debio limpiarse en repoPure")
	}
}
