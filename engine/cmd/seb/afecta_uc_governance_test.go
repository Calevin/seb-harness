package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestEspecificadorAfectaUCDirectives valida las directivas y la plantilla de seb-especificador para Afecta-UC.
func TestEspecificadorAfectaUCDirectives(t *testing.T) {
	repoRoot := getRepoRootDir(t)

	skillPath := filepath.Join(repoRoot, "payload", ".agents", "skills", "seb-especificador", "SKILL.md")
	skillBytes, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("No se pudo leer seb-especificador/SKILL.md: %v", err)
	}
	skillContent := string(skillBytes)

	requiredSkillTokens := []string{
		"docs/use-cases/README.md",
		"- **Afecta-UC:**",
		"`UC-<dominio>_<accion>`",
		"`nuevo`",
		"`ninguno`",
		"backticks",
	}
	for _, token := range requiredSkillTokens {
		if !strings.Contains(skillContent, token) {
			t.Errorf("seb-especificador/SKILL.md debe contener la directiva %q", token)
		}
	}

	// Verificar regla de division cuando afecta multiples casos de uso
	if !strings.Contains(skillContent, "more than one independent `UC-<dominio>_<accion>`") {
		t.Errorf("seb-especificador/SKILL.md debe instruir la division cuando afecte multiples UCs")
	}

	templatePath := filepath.Join(repoRoot, "payload", ".agents", "skills", "seb-especificador", "template", "nnn_nombre_tarea_spec.md")
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("No se pudo leer la plantilla nnn_nombre_tarea_spec.md: %v", err)
	}
	templateContent := string(templateBytes)

	expectedHeaderLine := "- **Afecta-UC:** [`UC-<dominio>_<accion>` | `nuevo` | `ninguno`]"
	if !strings.Contains(templateContent, expectedHeaderLine) {
		t.Errorf("La plantilla base debe incluir exactamente la linea %q", expectedHeaderLine)
	}
}

// TestClarificadorAfectaUCDirectives valida la auditoria, interrogacion y bloqueo duro en seb-clarificador.
func TestClarificadorAfectaUCDirectives(t *testing.T) {
	repoRoot := getRepoRootDir(t)

	skillPath := filepath.Join(repoRoot, "payload", ".agents", "skills", "seb-clarificador", "SKILL.md")
	skillBytes, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("No se pudo leer seb-clarificador/SKILL.md: %v", err)
	}
	skillContent := string(skillBytes)

	requiredTokens := []string{
		"docs/use-cases/README.md",
		"Afecta-UC:",
		"`UC-<dominio>_<accion>`",
		"`nuevo`",
		"`ninguno`",
		"backticks",
		"without editing the spec beforehand",
		"STRICTLY FORBIDDEN from marking a spec as `APROBADA`",
		"001",
		"010",
	}
	for _, token := range requiredTokens {
		if !strings.Contains(skillContent, token) {
			t.Errorf("seb-clarificador/SKILL.md debe contener la directiva %q", token)
		}
	}
}

// TestAfectaUCBackwardCompatibilityAndSync valida la compatibilidad con specs 001..010 y la sincronizacion payload/.agents.
func TestAfectaUCBackwardCompatibilityAndSync(t *testing.T) {
	repoRoot := getRepoRootDir(t)

	// 1. Verificar sincronizacion exacta entre payload/.agents y .agents y ausencia de emojis
	emojiRegex := regexp.MustCompile(`[\x{1F600}-\x{1F64F}\x{1F300}-\x{1F5FF}\x{1F680}-\x{1F6FF}\x{1F700}-\x{1F77F}\x{1F780}-\x{1F7FF}\x{1F800}-\x{1F8FF}\x{1F900}-\x{1F9FF}\x{1FA00}-\x{1FA6F}\x{1FA70}-\x{1FAFF}\x{2600}-\x{26FF}\x{2700}-\x{27BF}]`)

	relativeFiles := []string{
		filepath.Join("skills", "seb-especificador", "SKILL.md"),
		filepath.Join("skills", "seb-especificador", "template", "nnn_nombre_tarea_spec.md"),
		filepath.Join("skills", "seb-clarificador", "SKILL.md"),
	}

	for _, rel := range relativeFiles {
		payloadFile := filepath.Join(repoRoot, "payload", ".agents", rel)
		localFile := filepath.Join(repoRoot, ".agents", rel)

		payloadBytes, err := os.ReadFile(payloadFile)
		if err != nil {
			t.Fatalf("No se pudo leer %s: %v", payloadFile, err)
		}
		localBytes, err := os.ReadFile(localFile)
		if err != nil {
			t.Fatalf("No se pudo leer %s: %v", localFile, err)
		}
		if string(payloadBytes) != string(localBytes) {
			t.Errorf("Desincronizacion entre %s y %s", payloadFile, localFile)
		}
		if emojiRegex.Match(payloadBytes) {
			t.Errorf("Se detectaron emojis prohibidos en %s", payloadFile)
		}
	}

	// 2. Verificar que las specs historicas 001..010 sigan verificadas en seb.lock y aprobadas en disco
	lockPath := filepath.Join(repoRoot, ".seb", "seb.lock")
	lockBytes, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("No se pudo leer .seb/seb.lock: %v", err)
	}

	var lockData struct {
		Tasks map[string]struct {
			Verified   bool   `json:"verified"`
			CommitHash string `json:"commit_hash"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(lockBytes, &lockData); err != nil {
		t.Fatalf("Error al parsear .seb/seb.lock: %v", err)
	}

	historicalKeys := []string{
		"001_registro_telemetria_acciones",
		"002_prefijo_secuencial_artefactos",
		"003_desinstalacion_arnes",
		"004_captura_telemetria_antigravity",
		"005_gestion_grafo_dependencias",
		"006_parser_tabla_verificacion",
		"007_migracion_contexto_brownfield",
		"008_estado_sesion_determinista",
		"009_aborto_tarea",
		"010_integracion_documentador",
	}

	for _, key := range historicalKeys {
		entry, ok := lockData.Tasks[key]
		if !ok {
			t.Errorf("Falta la tarea historica %s en .seb/seb.lock", key)
			continue
		}
		if !entry.Verified || strings.TrimSpace(entry.CommitHash) == "" {
			t.Errorf("La tarea historica %s perdio su estado verified/commit_hash en seb.lock", key)
		}

		specFile := filepath.Join(repoRoot, "specs", key+"_spec.md")
		specBytes, err := os.ReadFile(specFile)
		if err != nil {
			t.Errorf("No se pudo leer spec historica %s: %v", specFile, err)
			continue
		}
		if !strings.Contains(string(specBytes), "**Estado:** `APROBADA`") {
			t.Errorf("La spec historica %s debe conservar Estado APROBADA", specFile)
		}
	}
}
