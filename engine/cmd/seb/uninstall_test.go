package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Helper para obtener la ruta absoluta a la raiz del repositorio seb-harness
func getRepoRootDir(t *testing.T) string {
	t.Helper()
	// Desde engine/cmd/seb subimos dos niveles
	dir, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("No se pudo obtener la ruta raiz: %v", err)
	}
	return dir
}

// Helper para ejecutar uninstall.sh capturando salida y codigo de retorno
func runUninstall(t *testing.T, harnessDir string, stdinInput string, args ...string) (int, string, string) {
	t.Helper()
	scriptPath := filepath.Join(harnessDir, "uninstall.sh")
	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	cmd.Dir = harnessDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if stdinInput != "" {
		cmd.Stdin = strings.NewReader(stdinInput)
	}

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("Error ejecutando comando bash: %v", err)
		}
	}
	return exitCode, stdout.String(), stderr.String()
}

// CHK-01: RF-01, RF-09, RF-10, RF-11, RF-18
func TestUninstallArgsAndSecurity(t *testing.T) {
	harnessDir := getRepoRootDir(t)

	// RF-18: Ayuda con -h y --help
	for _, flag := range []string{"-h", "--help"} {
		code, out, _ := runUninstall(t, harnessDir, "", flag)
		if code != 0 {
			t.Fatalf("Se esperaba exit code 0 con %s, se obtuvo %d", flag, code)
		}
		if !strings.Contains(out, "Uso:") || !strings.Contains(out, "--purge") {
			t.Fatalf("La salida de ayuda con %s no contiene la informacion esperada:\n%s", flag, out)
		}
	}

	// RF-11: Directorio destino inexistente aborta con 1
	nonExistent := filepath.Join(t.TempDir(), "non_existent_dir_12345")
	code, _, errOut := runUninstall(t, harnessDir, "", nonExistent)
	if code != 1 {
		t.Fatalf("Se esperaba exit code 1 para destino inexistente, se obtuvo %d", code)
	}
	if !strings.Contains(errOut, "no existe") {
		t.Fatalf("Se esperaba mensaje de error de inexistencia, se obtuvo:\n%s", errOut)
	}

	// RF-09: --self con argumento posicional aborta con 1
	code, _, errOut = runUninstall(t, harnessDir, "", "--self", t.TempDir())
	if code != 1 {
		t.Fatalf("Se esperaba exit code 1 para --self con destino, se obtuvo %d", code)
	}
	if !strings.Contains(errOut, "--self") {
		t.Fatalf("Se esperaba mensaje de error sobre --self, se obtuvo:\n%s", errOut)
	}

	// RF-10: Invocacion apuntando al repo del harness sin --self aborta con 1
	code, _, errOut = runUninstall(t, harnessDir, "", harnessDir)
	if code != 1 {
		t.Fatalf("Se esperaba exit code 1 para destino igual al repo del harness sin --self, se obtuvo %d", code)
	}
	if !strings.Contains(errOut, "dogfooding") {
		t.Fatalf("Se esperaba mensaje de advertencia sobre dogfooding, se obtuvo:\n%s", errOut)
	}
}

// CHK-02: RF-02, RF-03, CL-01
func TestUninstallGitHooks(t *testing.T) {
	harnessDir := getRepoRootDir(t)
	targetDir := t.TempDir()

	// Inicializar repositorio Git simulado
	cmdInit := exec.Command("git", "init", targetDir)
	if err := cmdInit.Run(); err != nil {
		t.Fatalf("Fallo git init: %v", err)
	}

	hooksDir := filepath.Join(targetDir, ".git", "hooks")
	_ = os.MkdirAll(hooksDir, 0755)

	// Crear hook preexistente de usuario y su respaldo .bak-seb
	originalHook := filepath.Join(hooksDir, "pre-commit")
	backupHook := filepath.Join(hooksDir, "pre-commit.bak-seb")
	_ = os.WriteFile(backupHook, []byte("#!/bin/sh\necho original-user-hook\n"), 0755)

	// Enlazar hook del arnes
	harnessHookTarget := filepath.Join(targetDir, ".agents", "hooks", "git-pre-commit")
	_ = os.Symlink(harnessHookTarget, originalHook)

	// Crear hook no perteneciente al arnes (CL-01)
	customHook := filepath.Join(hooksDir, "post-checkout")
	_ = os.WriteFile(customHook, []byte("#!/bin/sh\necho keep-me\n"), 0755)

	// Ejecutar desinstalacion
	code, _, _ := runUninstall(t, harnessDir, "", targetDir)
	if code != 0 {
		t.Fatalf("Se esperaba exit code 0, se obtuvo %d", code)
	}

	// RF-02 y RF-03: hook restaurado con el contenido original
	content, err := os.ReadFile(originalHook)
	if err != nil {
		t.Fatalf("El hook pre-commit deberia existir restaurado: %v", err)
	}
	if !strings.Contains(string(content), "original-user-hook") {
		t.Fatalf("El hook no contiene el contenido del backup restaurado:\n%s", string(content))
	}
	if _, err := os.Stat(backupHook); !os.IsNotExist(err) {
		t.Fatalf("El archivo .bak-seb deberia haber sido renombrado/removido")
	}

	// CL-01: hook no perteneciente al arnes debe conservarse intacto
	if _, err := os.Stat(customHook); err != nil {
		t.Fatalf("El hook no perteneciente al arnes debe conservarse: %v", err)
	}
}

// CHK-03: RF-04, RF-05, CL-02, CL-03
func TestUninstallGitignoreAndSebDir(t *testing.T) {
	harnessDir := getRepoRootDir(t)
	targetDir := t.TempDir()

	// Crear .gitignore con lineas del usuario y lineas del arnes
	gitignorePath := filepath.Join(targetDir, ".gitignore")
	initialGitignore := "node_modules/\n.seb/bin/\n*.log\n.seb/seb-audit.db*\nbuild/\n"
	_ = os.WriteFile(gitignorePath, []byte(initialGitignore), 0644)

	// Crear directorio .seb con contenido
	sebDir := filepath.Join(targetDir, ".seb")
	_ = os.MkdirAll(filepath.Join(sebDir, "bin"), 0755)
	_ = os.WriteFile(filepath.Join(sebDir, "bin", "seb"), []byte("dummy-bin"), 0755)
	_ = os.WriteFile(filepath.Join(sebDir, "seb-audit.db"), []byte("dummy-db"), 0644)
	_ = os.WriteFile(filepath.Join(sebDir, "VERSION"), []byte("1.0.0"), 0644)

	// Ejecutar desinstalacion
	code, _, _ := runUninstall(t, harnessDir, "", targetDir)
	if code != 0 {
		t.Fatalf("Se esperaba exit code 0, se obtuvo %d", code)
	}

	// RF-05: El directorio .seb debe ser eliminado por completo
	if _, err := os.Stat(sebDir); !os.IsNotExist(err) {
		t.Fatalf("El directorio .seb aun existe y debio ser eliminado")
	}

	// RF-04: .gitignore debe conservar lineas del usuario y no contener lineas del arnes
	updatedGitignore, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("No se pudo leer .gitignore: %v", err)
	}
	contentStr := string(updatedGitignore)
	if strings.Contains(contentStr, ".seb/bin/") || strings.Contains(contentStr, ".seb/seb-audit.db*") {
		t.Fatalf(".gitignore aun contiene entradas del arnes:\n%s", contentStr)
	}
	if !strings.Contains(contentStr, "node_modules/") || !strings.Contains(contentStr, "build/") {
		t.Fatalf(".gitignore perdio entradas del usuario:\n%s", contentStr)
	}

	// CL-03: .gitignore que solo tenia entradas del arnes
	targetDir2 := t.TempDir()
	gitignoreOnlySeb := filepath.Join(targetDir2, ".gitignore")
	_ = os.WriteFile(gitignoreOnlySeb, []byte(".seb/bin/\n.seb/seb-audit.db*\n"), 0644)
	code, _, _ = runUninstall(t, harnessDir, "", targetDir2)
	if code != 0 {
		t.Fatalf("Se esperaba exit code 0 para gitignore solo con seb, se obtuvo %d", code)
	}
	if data, err := os.ReadFile(gitignoreOnlySeb); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		t.Fatalf("El archivo .gitignore debio quedar vacio o limpio, tiene: %s", string(data))
	}
}

// CHK-04: RF-06, RF-07, CL-04
func TestUninstallAgentsDirectory(t *testing.T) {
	harnessDir := getRepoRootDir(t)
	targetDir := t.TempDir()

	agentsDir := filepath.Join(targetDir, ".agents")
	// Crear carpetas oficiales del payload
	_ = os.MkdirAll(filepath.Join(agentsDir, "skills", "seb-especificador"), 0755)
	_ = os.WriteFile(filepath.Join(agentsDir, "skills", "seb-especificador", "SKILL.md"), []byte("spec skill"), 0644)
	_ = os.MkdirAll(filepath.Join(agentsDir, "hooks"), 0755)
	_ = os.WriteFile(filepath.Join(agentsDir, "hooks", "git-pre-commit"), []byte("hook"), 0755)

	// Crear carpeta y skill propio del usuario
	_ = os.MkdirAll(filepath.Join(agentsDir, "skills", "mi-skill-propio"), 0755)
	_ = os.WriteFile(filepath.Join(agentsDir, "skills", "mi-skill-propio", "SKILL.md"), []byte("mi skill"), 0644)

	// Ejecutar desinstalacion
	code, _, _ := runUninstall(t, harnessDir, "", targetDir)
	if code != 0 {
		t.Fatalf("Se esperaba exit code 0, se obtuvo %d", code)
	}

	// RF-06: componente oficial debe desaparecer
	officialSkill := filepath.Join(agentsDir, "skills", "seb-especificador")
	if _, err := os.Stat(officialSkill); !os.IsNotExist(err) {
		t.Fatalf("El skill oficial seb-especificador debio ser eliminado")
	}

	// RF-06: componente propio del usuario debe mantenerse intacto
	customSkill := filepath.Join(agentsDir, "skills", "mi-skill-propio", "SKILL.md")
	if _, err := os.Stat(customSkill); err != nil {
		t.Fatalf("El skill propio del usuario debio conservarse: %v", err)
	}

	// RF-07: Si solo existian componentes oficiales, .agents debe borrarse por completo
	targetDirClean := t.TempDir()
	agentsClean := filepath.Join(targetDirClean, ".agents")
	_ = os.MkdirAll(filepath.Join(agentsClean, "skills", "seb-iniciador"), 0755)
	_ = os.WriteFile(filepath.Join(agentsClean, "skills", "seb-iniciador", "SKILL.md"), []byte("iniciador"), 0644)
	code, _, _ = runUninstall(t, harnessDir, "", targetDirClean)
	if code != 0 {
		t.Fatalf("Se esperaba exit code 0, se obtuvo %d", code)
	}
	if _, err := os.Stat(agentsClean); !os.IsNotExist(err) {
		t.Fatalf(".agents debio ser borrado por completo al quedar vacio")
	}

	// CL-04: .agents como symlink en target sin --self debe fallar
	targetDirSymlink := t.TempDir()
	dummyTarget := filepath.Join(targetDirSymlink, "some_target")
	_ = os.MkdirAll(dummyTarget, 0755)
	_ = os.Symlink(dummyTarget, filepath.Join(targetDirSymlink, ".agents"))
	code, _, errOut := runUninstall(t, harnessDir, "", targetDirSymlink)
	if code != 1 {
		t.Fatalf("Se esperaba exit code 1 para .agents symlink inesperado, se obtuvo %d", code)
	}
	if !strings.Contains(errOut, "simbolico") {
		t.Fatalf("Se esperaba advertencia sobre enlace simbolico inesperado:\n%s", errOut)
	}
}

// CHK-05: RF-08
func TestUninstallSelfMode(t *testing.T) {
	harnessDir := getRepoRootDir(t)

	// Creamos una replica del arnes para no mutar el repo activo durante el test
	tempHarness := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempHarness, "payload", ".agents"), 0755)
	_ = os.MkdirAll(filepath.Join(tempHarness, "engine"), 0755)

	// Crear script uninstall.sh en la raiz de la replica
	originalScript, _ := os.ReadFile(filepath.Join(harnessDir, "uninstall.sh"))
	_ = os.WriteFile(filepath.Join(tempHarness, "uninstall.sh"), originalScript, 0755)

	// Inicializar git simulado y symlink .agents
	_ = exec.Command("git", "init", tempHarness).Run()
	_ = os.Symlink("payload/.agents", filepath.Join(tempHarness, ".agents"))

	// Crear hook de desarrollo
	hooksDir := filepath.Join(tempHarness, ".git", "hooks")
	_ = os.MkdirAll(hooksDir, 0755)
	_ = os.Symlink(filepath.Join(tempHarness, ".agents", "hooks", "git-pre-commit"), filepath.Join(hooksDir, "pre-commit"))

	// Ejecutar ./uninstall.sh --self
	code, _, _ := runUninstall(t, tempHarness, "", "--self")
	if code != 0 {
		t.Fatalf("Se esperaba exit code 0 con --self, se obtuvo %d", code)
	}

	// RF-08: symlink .agents eliminado
	if _, err := os.Lstat(filepath.Join(tempHarness, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("El enlace .agents debio ser eliminado en --self")
	}

	// RF-08: payload y engine deben permanecer intactos
	if _, err := os.Stat(filepath.Join(tempHarness, "payload")); err != nil {
		t.Fatalf("payload/ no debe ser borrado en --self: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempHarness, "engine")); err != nil {
		t.Fatalf("engine/ no debe ser borrado en --self: %v", err)
	}
}

// CHK-06: RF-12
func TestUninstallDryRun(t *testing.T) {
	harnessDir := getRepoRootDir(t)
	targetDir := t.TempDir()

	// Crear estructura completa del arnes en el proyecto destino
	_ = os.MkdirAll(filepath.Join(targetDir, ".seb"), 0755)
	_ = os.WriteFile(filepath.Join(targetDir, ".seb", "VERSION"), []byte("1.0.0"), 0644)
	_ = os.WriteFile(filepath.Join(targetDir, ".gitignore"), []byte(".seb/bin/\n"), 0644)
	_ = os.MkdirAll(filepath.Join(targetDir, ".agents", "skills", "seb-especificador"), 0755)
	_ = os.WriteFile(filepath.Join(targetDir, ".agents", "skills", "seb-especificador", "SKILL.md"), []byte("data"), 0644)

	// Ejecutar con --dry-run
	code, out, _ := runUninstall(t, harnessDir, "", "--dry-run", targetDir)
	if code != 0 {
		t.Fatalf("Se esperaba exit code 0 en --dry-run, se obtuvo %d", code)
	}
	if !strings.Contains(out, "[dry-run]") {
		t.Fatalf("Se esperaba prefijo [dry-run] en la salida:\n%s", out)
	}

	// RF-12: Nada debe haberse modificado en disco
	if _, err := os.Stat(filepath.Join(targetDir, ".seb", "VERSION")); err != nil {
		t.Fatalf(".seb/VERSION no debio ser borrado en dry-run")
	}
	if _, err := os.Stat(filepath.Join(targetDir, ".agents", "skills", "seb-especificador", "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md no debio ser borrado en dry-run")
	}
}

// CHK-07: RF-13, RF-14, RF-15, RF-16, RF-17, CL-05, CL-06
func TestUninstallPurgeAndDocs(t *testing.T) {
	harnessDir := getRepoRootDir(t)

	// RF-17: Sin --purge se preservan specs, docs, AGENTS.md y MEMORY.md
	targetNormal := t.TempDir()
	_ = os.MkdirAll(filepath.Join(targetNormal, "specs"), 0755)
	_ = os.WriteFile(filepath.Join(targetNormal, "specs", "001_test_spec.md"), []byte("spec"), 0644)
	_ = os.MkdirAll(filepath.Join(targetNormal, "docs"), 0755)
	_ = os.WriteFile(filepath.Join(targetNormal, "docs", "constitution.md"), []byte("ley"), 0644)
	_ = os.WriteFile(filepath.Join(targetNormal, "AGENTS.md"), []byte("agents"), 0644)
	_ = os.WriteFile(filepath.Join(targetNormal, "MEMORY.md"), []byte("memory"), 0644)

	code, _, _ := runUninstall(t, harnessDir, "", targetNormal)
	if code != 0 {
		t.Fatalf("Se esperaba exit code 0, se obtuvo %d", code)
	}
	if _, err := os.Stat(filepath.Join(targetNormal, "specs", "001_test_spec.md")); err != nil {
		t.Fatalf("specs/ debe conservarse sin --purge: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetNormal, "docs", "constitution.md")); err != nil {
		t.Fatalf("constitution.md debe conservarse sin --purge: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetNormal, "AGENTS.md")); err != nil {
		t.Fatalf("AGENTS.md debe conservarse sin --purge: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetNormal, "MEMORY.md")); err != nil {
		t.Fatalf("MEMORY.md debe conservarse sin --purge: %v", err)
	}

	// RF-15 y CL-05: --purge en stdin no-interactivo sin --force debe fallar con 1
	targetNoForce := t.TempDir()
	_ = os.WriteFile(filepath.Join(targetNoForce, "AGENTS.md"), []byte("agents"), 0644)
	code, _, errOut := runUninstall(t, harnessDir, "", "--purge", targetNoForce)
	if code != 1 {
		t.Fatalf("Se esperaba exit code 1 en purga desatendida sin --force, se obtuvo %d", code)
	}
	if !strings.Contains(errOut, "--force") {
		t.Fatalf("Se esperaba mensaje requiriendo --force:\n%s", errOut)
	}

	// RF-13, RF-14, RF-16: --purge --force con docs solo conteniendo constitution.md
	targetPurgeOnlyConst := t.TempDir()
	_ = os.MkdirAll(filepath.Join(targetPurgeOnlyConst, "specs"), 0755)
	_ = os.WriteFile(filepath.Join(targetPurgeOnlyConst, "specs", "001_spec.md"), []byte("spec"), 0644)
	_ = os.MkdirAll(filepath.Join(targetPurgeOnlyConst, "docs"), 0755)
	_ = os.WriteFile(filepath.Join(targetPurgeOnlyConst, "docs", "constitution.md"), []byte("ley"), 0644)
	_ = os.WriteFile(filepath.Join(targetPurgeOnlyConst, "AGENTS.md"), []byte("agents"), 0644)
	_ = os.WriteFile(filepath.Join(targetPurgeOnlyConst, "MEMORY.md"), []byte("memory"), 0644)

	code, _, _ = runUninstall(t, harnessDir, "", "--purge", "--force", targetPurgeOnlyConst)
	if code != 0 {
		t.Fatalf("Se esperaba exit code 0 con --purge --force, se obtuvo %d", code)
	}
	if _, err := os.Stat(filepath.Join(targetPurgeOnlyConst, "specs")); !os.IsNotExist(err) {
		t.Fatalf("specs/ debio eliminarse con --purge")
	}
	if _, err := os.Stat(filepath.Join(targetPurgeOnlyConst, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("AGENTS.md debio eliminarse con --purge")
	}
	if _, err := os.Stat(filepath.Join(targetPurgeOnlyConst, "MEMORY.md")); !os.IsNotExist(err) {
		t.Fatalf("MEMORY.md debio eliminarse con --purge")
	}
	if _, err := os.Stat(filepath.Join(targetPurgeOnlyConst, "docs")); !os.IsNotExist(err) {
		t.Fatalf("docs/ debio eliminarse al quedar vacio")
	}

	// CL-06: --purge --force con docs conteniendo archivos ajenos
	targetDocsExtra := t.TempDir()
	_ = os.MkdirAll(filepath.Join(targetDocsExtra, "docs"), 0755)
	_ = os.WriteFile(filepath.Join(targetDocsExtra, "docs", "constitution.md"), []byte("ley"), 0644)
	_ = os.WriteFile(filepath.Join(targetDocsExtra, "docs", "api.md"), []byte("api doc"), 0644)

	code, _, _ = runUninstall(t, harnessDir, "", "--purge", "-f", targetDocsExtra)
	if code != 0 {
		t.Fatalf("Se esperaba exit code 0 con -f, se obtuvo %d", code)
	}
	if _, err := os.Stat(filepath.Join(targetDocsExtra, "docs", "constitution.md")); !os.IsNotExist(err) {
		t.Fatalf("constitution.md debio eliminarse")
	}
	if _, err := os.Stat(filepath.Join(targetDocsExtra, "docs", "api.md")); err != nil {
		t.Fatalf("api.md debio conservarse: %v", err)
	}
}

// CHK-08: RNF-01, RNF-02
func TestUninstallIdempotency(t *testing.T) {
	harnessDir := getRepoRootDir(t)
	targetDir := t.TempDir()

	// RNF-01: Primera ejecucion sobre carpeta limpia
	code1, _, _ := runUninstall(t, harnessDir, "", targetDir)
	if code1 != 0 {
		t.Fatalf("Primera ejecucion debio retornar 0, retorno %d", code1)
	}

	// RNF-01: Segunda ejecucion consecutiva (idempotencia)
	code2, _, _ := runUninstall(t, harnessDir, "", targetDir)
	if code2 != 0 {
		t.Fatalf("Segunda ejecucion debio retornar 0, retorno %d", code2)
	}

	// RNF-02: Validar directiva bash y modo estricto en script
	scriptPath := filepath.Join(harnessDir, "uninstall.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("No se pudo leer uninstall.sh: %v", err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "#!/usr/bin/env bash") {
		t.Fatalf("uninstall.sh debe comenzar con shebang #!/usr/bin/env bash")
	}
	if !strings.Contains(content, "set -euo pipefail") {
		t.Fatalf("uninstall.sh debe incluir 'set -euo pipefail'")
	}
}
