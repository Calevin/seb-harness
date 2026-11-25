package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestDocumentadorSkillDefinitionAndRules valida la estructura, directivas y ausencia de emojis
// en la definicion del skill seb-documentador y en los protocolos operativos.
func TestDocumentadorSkillDefinitionAndRules(t *testing.T) {
	repoRoot := getRepoRootDir(t)

	// 1. Validar existencia del skill en payload y en .agents (dogfooding)
	skillPaths := []string{
		filepath.Join(repoRoot, "payload", ".agents", "skills", "seb-documentador", "SKILL.md"),
		filepath.Join(repoRoot, ".agents", "skills", "seb-documentador", "SKILL.md"),
	}

	emojiRegex := regexp.MustCompile(`[\x{1F600}-\x{1F64F}\x{1F300}-\x{1F5FF}\x{1F680}-\x{1F6FF}\x{1F700}-\x{1F77F}\x{1F780}-\x{1F7FF}\x{1F800}-\x{1F8FF}\x{1F900}-\x{1F9FF}\x{1FA00}-\x{1FA6F}\x{1FA70}-\x{1FAFF}\x{2600}-\x{26FF}\x{2700}-\x{27BF}]`)

	for _, sPath := range skillPaths {
		contentBytes, err := os.ReadFile(sPath)
		if err != nil {
			t.Fatalf("No se pudo leer el archivo de skill en %s: %v", sPath, err)
		}
		content := string(contentBytes)

		// Directivas obligatorias
		if !strings.Contains(content, "name: seb-documentador") {
			t.Errorf("El skill en %s debe declarar 'name: seb-documentador'", sPath)
		}
		if !strings.Contains(content, "autonumber") {
			t.Errorf("El skill en %s debe exigir la directiva 'autonumber' para diagramas de secuencia", sPath)
		}
		if !strings.Contains(content, "UC-<dominio>_<accion>.md") {
			t.Errorf("El skill en %s debe especificar el formato canónico 'UC-<dominio>_<accion>.md'", sPath)
		}
		if !strings.Contains(content, "Afecta-UC:") {
			t.Errorf("El skill en %s debe contemplar el metadato 'Afecta-UC:' para documentacion viva", sPath)
		}
		if !strings.Contains(content, "docs/use-cases/README.md") {
			t.Errorf("El skill en %s debe documentar el indice general 'docs/use-cases/README.md'", sPath)
		}

		// Prohibicion de emojis
		if emojiRegex.MatchString(content) {
			t.Errorf("El archivo de skill en %s contiene emojis prohibidos", sPath)
		}
	}

	// 2. Validar que el protocolo gestionado SEB-HARNESS.md (payload y .agents) incluye Fase 6 y matriz de menor privilegio (017-RF-01, 017-RF-13)
	protocolPaths := []string{
		filepath.Join(repoRoot, "payload", ".agents", "SEB-HARNESS.md"),
		filepath.Join(repoRoot, ".agents", "SEB-HARNESS.md"),
	}

	for _, pPath := range protocolPaths {
		contentBytes, err := os.ReadFile(pPath)
		if err != nil {
			t.Fatalf("No se pudo leer %s: %v", pPath, err)
		}
		content := string(contentBytes)

		if !strings.Contains(content, "Fase 6 — Documentación (`seb-documentador`)") {
			t.Errorf("El archivo %s debe declarar la Fase 6 para seb-documentador", pPath)
		}
		if !strings.Contains(content, "`seb-documentador`") || !strings.Contains(content, "docs/use-cases/*.md") {
			t.Errorf("El archivo %s debe incluir la fila de seb-documentador en la tabla de menor privilegio", pPath)
		}
		if emojiRegex.MatchString(content) {
			t.Errorf("El archivo %s contiene emojis prohibidos", pPath)
		}
	}

	// Validar que AGENTS.md (payload y raiz) enrutan hacia .agents/SEB-HARNESS.md sin emojis
	for _, aPath := range []string{filepath.Join(repoRoot, "payload", "AGENTS.md"), filepath.Join(repoRoot, "AGENTS.md")} {
		contentBytes, err := os.ReadFile(aPath)
		if err != nil {
			t.Fatalf("No se pudo leer %s: %v", aPath, err)
		}
		content := string(contentBytes)
		if !strings.Contains(content, ".agents/SEB-HARNESS.md") {
			t.Errorf("El enrutador %s debe referenciar .agents/SEB-HARNESS.md", aPath)
		}
		if emojiRegex.MatchString(content) {
			t.Errorf("El archivo %s contiene emojis prohibidos", aPath)
		}
	}

	// 3. Validar regla global reducida en payload/.agents/rules/00-seb-harness.md (017-RF-08, 017-RF-13)
	rulesPath := filepath.Join(repoRoot, "payload", ".agents", "rules", "00-seb-harness.md")
	ruleBytes, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("No se pudo leer %s: %v", rulesPath, err)
	}
	ruleContent := string(ruleBytes)

	if !strings.Contains(ruleContent, ".agents/SEB-HARNESS.md") || !strings.Contains(ruleContent, "seb guard") {
		t.Errorf("El archivo %s debe referenciar .agents/SEB-HARNESS.md y seb guard", rulesPath)
	}
	if emojiRegex.MatchString(ruleContent) {
		t.Errorf("El archivo %s contiene emojis prohibidos", rulesPath)
	}
}
