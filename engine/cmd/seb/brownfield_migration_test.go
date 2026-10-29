package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Helper para ejecutar install.sh capturando salida y codigo de retorno
func runInstall(t *testing.T, harnessDir string, args ...string) (int, string, string) {
	t.Helper()
	scriptPath := filepath.Join(harnessDir, "install.sh")
	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	cmd.Dir = harnessDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("Error ejecutando install.sh: %v", err)
		}
	}
	return exitCode, stdout.String(), stderr.String()
}

// CHK-01: RF-01, RF-09, RNF-01
// Respaldo determinista de AGENTS.md sin firma a AGENTS.pre-seb.md e instalacion del protocolo oficial
func TestInstallBackupAgentsPreSeb(t *testing.T) {
	harnessDir := getRepoRootDir(t)
	targetDir := t.TempDir()

	// Inicializar repositorio Git para que install.sh pueda vincular hooks
	_ = exec.Command("git", "init", targetDir).Run()

	customAgentsContent := "# Directrices Previas de Agentes\nNo usar dependencias dinamicas."
	agentsPath := filepath.Join(targetDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(customAgentsContent), 0644); err != nil {
		t.Fatalf("Fallo creando AGENTS.md previo: %v", err)
	}

	// Ejecutar install.sh con --no-build
	code, out, errOut := runInstall(t, harnessDir, "--no-build", targetDir)
	if code != 0 {
		t.Fatalf("install.sh fallo con codigo %d:\nSTDOUT: %s\nSTDERR: %s", code, out, errOut)
	}

	// Verificar que AGENTS.pre-seb.md fue creado con el contenido original
	backupPath := filepath.Join(targetDir, "AGENTS.pre-seb.md")
	data, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("No se encontro AGENTS.pre-seb.md: %v", err)
	}
	if string(data) != customAgentsContent {
		t.Fatalf("Contenido en respaldo difiere del original:\n%s", string(data))
	}

	// Verificar que AGENTS.md ahora contiene el protocolo oficial de seb-harness
	newAgentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("No se encontro AGENTS.md instalado: %v", err)
	}
	if !strings.Contains(strings.ToLower(string(newAgentsData)), "seb-harness") {
		t.Fatalf("El nuevo AGENTS.md no contiene la firma del protocolo seb-harness")
	}

	// Verificar idempotencia (RNF-01): re-ejecutar no debe borrar ni alterar el respaldo
	code2, _, _ := runInstall(t, harnessDir, "--no-build", targetDir)
	if code2 != 0 {
		t.Fatalf("Segunda ejecucion de install.sh fallo con codigo %d", code2)
	}
	dataAfterReinstall, _ := os.ReadFile(backupPath)
	if string(dataAfterReinstall) != customAgentsContent {
		t.Fatalf("El respaldo fue sobreescrito o alterado en la segunda ejecucion")
	}
}

// CHK-02: RF-02, RF-07, RF-09
// Deteccion y respaldo de asistentes externos (GEMINI.md, CLAUDE.md) a .pre-seb.md y anti-sobrescritura
func TestInstallBackupAssistantContextFiles(t *testing.T) {
	harnessDir := getRepoRootDir(t)
	targetDir := t.TempDir()
	_ = exec.Command("git", "init", targetDir).Run()

	geminiContent := "# Gemini instructions\nPrefer standard library."
	claudeContent := "# Claude instructions\nAlways write tests."
	_ = os.WriteFile(filepath.Join(targetDir, "GEMINI.md"), []byte(geminiContent), 0644)
	_ = os.WriteFile(filepath.Join(targetDir, "CLAUDE.md"), []byte(claudeContent), 0644)

	// Ejecutar install.sh
	code, _, errOut := runInstall(t, harnessDir, "--no-build", targetDir)
	if code != 0 {
		t.Fatalf("install.sh fallo con codigo %d: %s", code, errOut)
	}

	geminiBak := filepath.Join(targetDir, "GEMINI.pre-seb.md")
	claudeBak := filepath.Join(targetDir, "CLAUDE.pre-seb.md")

	dataG, err := os.ReadFile(geminiBak)
	if err != nil || string(dataG) != geminiContent {
		t.Fatalf("GEMINI.pre-seb.md no fue respaldado correctamente: %v", err)
	}
	dataC, err := os.ReadFile(claudeBak)
	if err != nil || string(dataC) != claudeContent {
		t.Fatalf("CLAUDE.pre-seb.md no fue respaldado correctamente: %v", err)
	}

	// RF-07: Anti-sobrescritura de respaldos si ya existe *.pre-seb.md
	// Modificamos el archivo fuente y volvemos a correr: el backup original debe mantenerse
	_ = os.WriteFile(filepath.Join(targetDir, "GEMINI.md"), []byte("New modified gemini"), 0644)
	code3, _, errOut3 := runInstall(t, harnessDir, "--no-build", targetDir)
	if code3 != 0 {
		t.Fatalf("Fallo en re-instalacion: %s", errOut3)
	}
	dataG2, _ := os.ReadFile(geminiBak)
	if string(dataG2) != geminiContent {
		t.Fatalf("El respaldo existente fue sobreescrito: se esperaba el original")
	}
}

// CHK-03: RF-08, RNF-01
// Restauracion automatica de archivos *.pre-seb.* a su denominacion original al ejecutar uninstall.sh
func TestUninstallRestorePreSebBackups(t *testing.T) {
	harnessDir := getRepoRootDir(t)
	targetDir := t.TempDir()
	_ = exec.Command("git", "init", targetDir).Run()

	// Instalar primero
	code, _, _ := runInstall(t, harnessDir, "--no-build", targetDir)
	if code != 0 {
		t.Fatalf("install.sh fallo al preparar entorno")
	}

	// Simular existencia de varios respaldos creados previamente
	agentsBak := filepath.Join(targetDir, "AGENTS.pre-seb.md")
	geminiBak := filepath.Join(targetDir, "GEMINI.pre-seb.md")
	claudeBak := filepath.Join(targetDir, "CLAUDE.pre-seb.md")
	constBak := filepath.Join(targetDir, "docs", "constitution.pre-seb.md")

	_ = os.WriteFile(agentsBak, []byte("Original Agents"), 0644)
	_ = os.WriteFile(geminiBak, []byte("Original Gemini"), 0644)
	_ = os.WriteFile(claudeBak, []byte("Original Claude"), 0644)
	_ = os.MkdirAll(filepath.Join(targetDir, "docs"), 0755)
	_ = os.WriteFile(constBak, []byte("Original Constitution"), 0644)

	// Probar dry-run: no debe restaurar aun
	codeDry, outDry, _ := runUninstall(t, harnessDir, "", "--dry-run", targetDir)
	if codeDry != 0 {
		t.Fatalf("uninstall.sh --dry-run fallo con codigo %d", codeDry)
	}
	if !strings.Contains(outDry, "[dry-run]") || !strings.Contains(outDry, "restauraria") {
		t.Fatalf("uninstall.sh --dry-run no reporto la restauracion planeada:\n%s", outDry)
	}
	if _, err := os.Stat(agentsBak); os.IsNotExist(err) {
		t.Fatalf("dry-run elimino prematuramente AGENTS.pre-seb.md")
	}

	// Ejecutar uninstall real
	codeUn, outUn, errUn := runUninstall(t, harnessDir, "", targetDir)
	if codeUn != 0 {
		t.Fatalf("uninstall.sh fallo con codigo %d: %s\n%s", codeUn, outUn, errUn)
	}

	// Verificar que los archivos restaurados existen con su nombre original
	agentsRestored, err := os.ReadFile(filepath.Join(targetDir, "AGENTS.md"))
	if err != nil || string(agentsRestored) != "Original Agents" {
		t.Fatalf("Fallo restauracion de AGENTS.md: %v (contenido: %s)", err, string(agentsRestored))
	}

	geminiRestored, err := os.ReadFile(filepath.Join(targetDir, "GEMINI.md"))
	if err != nil || string(geminiRestored) != "Original Gemini" {
		t.Fatalf("Fallo restauracion de GEMINI.md: %v", err)
	}

	claudeRestored, err := os.ReadFile(filepath.Join(targetDir, "CLAUDE.md"))
	if err != nil || string(claudeRestored) != "Original Claude" {
		t.Fatalf("Fallo restauracion de CLAUDE.md: %v", err)
	}

	constRestored, err := os.ReadFile(filepath.Join(targetDir, "docs", "constitution.md"))
	if err != nil || string(constRestored) != "Original Constitution" {
		t.Fatalf("Fallo restauracion de constitution.md: %v", err)
	}

	// Verificar que los *.pre-seb.* ya no existen
	for _, f := range []string{agentsBak, geminiBak, claudeBak, constBak} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Fatalf("El archivo de respaldo %s no debio conservarse tras ser restaurado", f)
		}
	}
}

// CHK-04: RF-03, RF-04, RF-05, RF-06
// Verificacion de especificacion y guardarrailes del skill seb-iniciador
func TestIniciadorSkillSpecificationAndStubs(t *testing.T) {
	harnessDir := getRepoRootDir(t)
	skillPath := filepath.Join(harnessDir, "payload", ".agents", "skills", "seb-iniciador", "SKILL.md")

	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("No se pudo leer SKILL.md de seb-iniciador: %v", err)
	}
	content := string(data)

	// Verificar que los archivos .pre-seb y de asistentes estan en la inspeccion
	requiredReferences := []string{
		"AGENTS.pre-seb.md",
		"GEMINI.md",
		"GEMINI.pre-seb.md",
		"CLAUDE.md",
		"CLAUDE.pre-seb.md",
		"docs/constitution.pre-seb.md",
	}
	for _, ref := range requiredReferences {
		if !strings.Contains(content, ref) {
			t.Fatalf("SKILL.md de seb-iniciador no incluye la referencia obligatoria a '%s'", ref)
		}
	}

	// Verificar autorizacion de stubs en Allowed Writes
	if !strings.Contains(content, "GEMINI.md") || !strings.Contains(content, "CLAUDE.md") {
		t.Fatalf("SKILL.md no contempla los stubs de asistentes en sus permisos de escritura")
	}

	// Verificar protocolo de preservacion de docs/constitution.md preexistente con firma
	if !strings.Contains(content, "seb-harness") || !strings.Contains(strings.ToLower(content), "preserv") {
		t.Fatalf("SKILL.md no estipula la regla de preservacion de constitucion con firma seb-harness")
	}
}

// CHK-05: CL-01, CL-02, RNF-02
// Casos limite: archivos vacios de 0 bytes y proyectos greenfield limpios
func TestBrownfieldEdgeCasesAndGreenfield(t *testing.T) {
	harnessDir := getRepoRootDir(t)

	// Caso CL-01: Archivo de 0 bytes
	targetEmpty := t.TempDir()
	_ = exec.Command("git", "init", targetEmpty).Run()
	emptyAgents := filepath.Join(targetEmpty, "AGENTS.md")
	_ = os.WriteFile(emptyAgents, []byte(""), 0644)

	code, _, errOut := runInstall(t, harnessDir, "--no-build", targetEmpty)
	if code != 0 {
		t.Fatalf("Fallo instalacion con AGENTS.md vacio: %s", errOut)
	}
	bakPath := filepath.Join(targetEmpty, "AGENTS.pre-seb.md")
	if info, err := os.Stat(bakPath); err != nil || info.Size() != 0 {
		t.Fatalf("AGENTS.pre-seb.md debio ser creado preservando los 0 bytes: %v", err)
	}

	// Caso CL-02: Proyecto Greenfield (sin AGENTS.md ni asistentes previos)
	targetGreen := t.TempDir()
	_ = exec.Command("git", "init", targetGreen).Run()

	codeG, _, errOutG := runInstall(t, harnessDir, "--no-build", targetGreen)
	if codeG != 0 {
		t.Fatalf("Fallo instalacion en proyecto greenfield: %s", errOutG)
	}
	// Comprobar que NO se creo ningun *.pre-seb.*
	errWalk := filepath.Walk(targetGreen, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(filepath.Base(path), ".pre-seb.") {
			t.Fatalf("Se encontro un archivo espurio de respaldo en greenfield: %s", path)
		}
		return nil
	})
	if errWalk != nil {
		t.Fatalf("Error recorriendo targetGreen: %v", errWalk)
	}
}
