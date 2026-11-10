package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallDocumentadorSkillAndPatch valida la instalacion en proyectos nuevos,
// la generacion de AGENTS.md.patch.md en proyectos existentes y la preservacion en uninstall.
func TestInstallDocumentadorSkillAndPatch(t *testing.T) {
	repoRoot := getRepoRootDir(t)

	installScript := filepath.Join(repoRoot, "install.sh")
	uninstallScript := filepath.Join(repoRoot, "uninstall.sh")

	// 1. Proyecto nuevo (Greenfield): install.sh --no-build
	tmpGreenfield := t.TempDir()
	cmdGreenfield := exec.Command("bash", installScript, "--no-build", tmpGreenfield)
	outGreenfield, err := cmdGreenfield.CombinedOutput()
	if err != nil {
		t.Fatalf("Fallo install.sh en greenfield: %v\nSalida:\n%s", err, string(outGreenfield))
	}

	docSkillPath := filepath.Join(tmpGreenfield, ".agents", "skills", "seb-documentador", "SKILL.md")
	if _, err := os.Stat(docSkillPath); os.IsNotExist(err) {
		t.Fatalf("install.sh no instalo el skill en: %s", docSkillPath)
	}

	useCasesDir := filepath.Join(tmpGreenfield, "docs", "use-cases")
	if info, err := os.Stat(useCasesDir); os.IsNotExist(err) || !info.IsDir() {
		t.Fatalf("install.sh no creo el directorio de casos de uso en: %s", useCasesDir)
	}

	patchFileGreenfield := filepath.Join(tmpGreenfield, "AGENTS.md.patch.md")
	if _, err := os.Stat(patchFileGreenfield); !os.IsNotExist(err) {
		t.Fatalf("En greenfield no debe generarse AGENTS.md.patch.md")
	}

	// 2. Proyecto existente (Brownfield) con AGENTS.md firmado sin seb-documentador
	tmpBrownfield := t.TempDir()
	customAgentsContent := `# Protocolo Operativo: seb-harness (Personalizado)
Fase 1: Especificacion
Fase 5: Verificacion (seb-verificador)
`
	agentsPath := filepath.Join(tmpBrownfield, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(customAgentsContent), 0644); err != nil {
		t.Fatalf("Error creando AGENTS.md previo: %v", err)
	}

	cmdBrownfield := exec.Command("bash", installScript, "--no-build", tmpBrownfield)
	outBrownfield, err := cmdBrownfield.CombinedOutput()
	if err != nil {
		t.Fatalf("Fallo install.sh en brownfield: %v\nSalida:\n%s", err, string(outBrownfield))
	}

	// Verificar que AGENTS.md original fue respetado
	currentAgents, err := os.ReadFile(agentsPath)
	if err != nil || string(currentAgents) != customAgentsContent {
		t.Fatalf("AGENTS.md personalizado fue sobrescrito indebidamente: %s", string(currentAgents))
	}

	// Verificar que se genero AGENTS.md.patch.md
	patchFileBrownfield := filepath.Join(tmpBrownfield, "AGENTS.md.patch.md")
	patchBytes, err := os.ReadFile(patchFileBrownfield)
	if err != nil {
		t.Fatalf("No se genero AGENTS.md.patch.md en actualizacion: %v", err)
	}
	patchStr := string(patchBytes)
	if !strings.Contains(patchStr, "seb-documentador") || !strings.Contains(patchStr, "Fase 6") {
		t.Fatalf("El archivo patch no contiene la descripcion de Fase 6 o seb-documentador:\n%s", patchStr)
	}

	// 3. Idempotencia: segunda ejecucion no falla
	cmdIdempotent := exec.Command("bash", installScript, "--no-build", tmpBrownfield)
	outIdempotent, err := cmdIdempotent.CombinedOutput()
	if err != nil {
		t.Fatalf("Segunda ejecucion de install.sh fallo: %v\nSalida:\n%s", err, string(outIdempotent))
	}

	// 4. Desinstalacion (uninstall.sh): retira skill pero preserva docs/use-cases/
	dummyUseCase := filepath.Join(tmpBrownfield, "docs", "use-cases", "UC-auth_login.md")
	if err := os.WriteFile(dummyUseCase, []byte("# UC-auth_login"), 0644); err != nil {
		t.Fatalf("Error creando caso de uso de prueba: %v", err)
	}

	cmdUninstall := exec.Command("bash", uninstallScript, tmpBrownfield)
	outUninstall, err := cmdUninstall.CombinedOutput()
	if err != nil {
		t.Fatalf("uninstall.sh fallo: %v\nSalida:\n%s", err, string(outUninstall))
	}

	// Skill debe haberse removido
	skillInBrownfield := filepath.Join(tmpBrownfield, ".agents", "skills", "seb-documentador")
	if _, err := os.Stat(skillInBrownfield); !os.IsNotExist(err) {
		t.Fatalf("uninstall.sh debio remover el skill seb-documentador de: %s", skillInBrownfield)
	}

	// Caso de uso debe PRESERVARSE
	if _, err := os.Stat(dummyUseCase); os.IsNotExist(err) {
		t.Fatalf("uninstall.sh elimino indebidamente el caso de uso en: %s", dummyUseCase)
	}
}
