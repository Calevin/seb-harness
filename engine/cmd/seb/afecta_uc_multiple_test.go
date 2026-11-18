package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Spec 014: literales canonicos definidos en specs/014_afecta_uc_multiple_plan.md (seccion 3).
const afectaUCListTemplateLine = "- **Afecta-UC:** [`UC-<dominio>_<accion>` | `nuevo` | `ninguno` | lista: `UC-<dominio>_<accion>`, `nuevo`]"

var especificadorListLiterals = []string{
	"comma-and-space separated list",
	"each element individually wrapped in backticks",
	"`nuevo` may appear at most once",
	"`ninguno` is exclusive",
	"`nuevo` may be combined with existing `UC-<dominio>_<accion>` identifiers",
	"cohesive",
	"functionally independent",
	"A single-element list is equivalent to a single value",
}

// Literal de la regla de division obligatoria sustituida (RF-13).
const especificadorSupersededLiteral = "to preserve a single unambiguous `Afecta-UC:` value"

var clarificadorListLiterals = []string{
	"When `Afecta-UC:` declares a list",
	"each `UC-<dominio>_<accion>` element exists in `docs/use-cases/README.md`",
	"The order of the elements is irrelevant",
	"backed by at least one `RF-XX`",
	"no `RF-XX` modifies a cataloged use case absent from the list",
	"`ninguno` combined with other elements",
	"duplicate elements (including repeated `nuevo`)",
	"a separator other than comma and space",
}

const clarificadorHardGateAnchor = "STRICTLY FORBIDDEN from marking a spec as `APROBADA`"
const clarificadorHardGateListLiteral = "an invalid `Afecta-UC:` list"

var documentadorListLiterals = []string{
	"Si `Afecta-UC:` declara una lista",
	"actualizar cada caso de uso `UC-<dominio>_<accion>` declarado",
	"determinar a partir de los `RF-XX` la cantidad de casos de uso nuevos",
	"identificador `UC-<dominio>_<accion>` inexistente en el catalogo",
	"columna \"Specs de Evolución\" inmediatamente despues de \"Spec Originaria\"",
	"sin modificar su \"Spec Originaria\"",
	"completar con `-` las filas",
	"sin duplicar rutas ya registradas",
	"| Spec Originaria | Specs de Evolución |",
}

var afectaUCListEmojiRegex = regexp.MustCompile(`[\x{1F600}-\x{1F64F}\x{1F300}-\x{1F5FF}\x{1F680}-\x{1F6FF}\x{1F700}-\x{1F77F}\x{1F780}-\x{1F7FF}\x{1F800}-\x{1F8FF}\x{1F900}-\x{1F9FF}\x{1FA00}-\x{1FA6F}\x{1FA70}-\x{1FAFF}\x{2600}-\x{26FF}\x{2700}-\x{27BF}]`)

var afectaUCListModifiedFiles = []string{
	filepath.Join("skills", "seb-especificador", "SKILL.md"),
	filepath.Join("skills", "seb-especificador", "template", "nnn_nombre_tarea_spec.md"),
	filepath.Join("skills", "seb-clarificador", "SKILL.md"),
	filepath.Join("skills", "seb-documentador", "SKILL.md"),
}

// readPayloadFile lee un archivo relativo a payload/.agents y falla el test si no existe.
func readPayloadFile(t *testing.T, rel string) string {
	t.Helper()
	path := filepath.Join(getRepoRootDir(t), "payload", ".agents", rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("No se pudo leer %s: %v", path, err)
	}
	return string(data)
}

// assertLiterals verifica la presencia literal de cada token en el contenido.
func assertLiterals(t *testing.T, name, content string, literals []string) {
	t.Helper()
	for _, lit := range literals {
		if !strings.Contains(content, lit) {
			t.Errorf("%s debe contener el literal canonico %q", name, lit)
		}
	}
}

// TestEspecificadorAfectaUCList valida RF-01..RF-04, RF-13 y CL-01 de la Spec 014.
func TestEspecificadorAfectaUCList(t *testing.T) {
	skill := readPayloadFile(t, filepath.Join("skills", "seb-especificador", "SKILL.md"))
	assertLiterals(t, "seb-especificador/SKILL.md", skill, especificadorListLiterals)

	if strings.Contains(skill, especificadorSupersededLiteral) {
		t.Errorf("seb-especificador/SKILL.md no debe conservar la regla de division obligatoria sustituida: %q", especificadorSupersededLiteral)
	}

	template := readPayloadFile(t, filepath.Join("skills", "seb-especificador", "template", "nnn_nombre_tarea_spec.md"))
	if !strings.Contains(template, afectaUCListTemplateLine) {
		t.Errorf("La plantilla base debe incluir exactamente la linea %q", afectaUCListTemplateLine)
	}
}

// TestClarificadorAfectaUCList valida RF-05..RF-07 y CL-02..CL-06 de la Spec 014.
func TestClarificadorAfectaUCList(t *testing.T) {
	skill := readPayloadFile(t, filepath.Join("skills", "seb-clarificador", "SKILL.md"))
	assertLiterals(t, "seb-clarificador/SKILL.md", skill, clarificadorListLiterals)

	anchorIdx := strings.Index(skill, clarificadorHardGateAnchor)
	if anchorIdx < 0 {
		t.Fatalf("seb-clarificador/SKILL.md debe conservar el Hard Gate %q", clarificadorHardGateAnchor)
	}
	if !strings.Contains(skill[anchorIdx:], clarificadorHardGateListLiteral) {
		t.Errorf("El Hard Gate de seb-clarificador debe incluir %q despues de %q", clarificadorHardGateListLiteral, clarificadorHardGateAnchor)
	}
}

// TestDocumentadorAfectaUCList valida RF-08..RF-12 y CL-07 de la Spec 014.
func TestDocumentadorAfectaUCList(t *testing.T) {
	skill := readPayloadFile(t, filepath.Join("skills", "seb-documentador", "SKILL.md"))
	assertLiterals(t, "seb-documentador/SKILL.md", skill, documentadorListLiterals)
}

// TestAfectaUCListDistribution valida RNF-01: distribucion via install.sh a .agents y .claude y ausencia de emojis.
func TestAfectaUCListDistribution(t *testing.T) {
	root := getRepoRootDir(t)

	for _, rel := range afectaUCListModifiedFiles {
		content := readPayloadFile(t, rel)
		if afectaUCListEmojiRegex.MatchString(content) {
			t.Errorf("Se detectaron emojis prohibidos en payload/.agents/%s", rel)
		}
		local, err := os.ReadFile(filepath.Join(root, ".agents", rel))
		if err != nil {
			t.Fatalf("No se pudo leer .agents/%s: %v", rel, err)
		}
		if string(local) != content {
			t.Errorf("Desincronizacion entre payload/.agents/%s y .agents/%s", rel, rel)
		}
	}

	target := t.TempDir()
	cmd := exec.Command("bash", filepath.Join(root, "install.sh"), "--no-build", target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("install.sh fallo: %v\n%s", err, string(out))
	}

	checks := map[string]string{
		filepath.Join("skills", "seb-especificador", "SKILL.md"): especificadorListLiterals[0],
		filepath.Join("skills", "seb-clarificador", "SKILL.md"):  clarificadorListLiterals[0],
		filepath.Join("skills", "seb-documentador", "SKILL.md"):  documentadorListLiterals[4],
	}
	for _, base := range []string{".agents", ".claude"} {
		for rel, lit := range checks {
			path := filepath.Join(target, base, rel)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("No se distribuyo %s: %v", path, err)
				continue
			}
			if !strings.Contains(string(data), lit) {
				t.Errorf("%s no contiene el literal distribuido %q", path, lit)
			}
		}
	}
}

// TestAfectaUCListBackwardCompatibility valida RF-17: specs previas con valor simple conservan sello y estado.
func TestAfectaUCListBackwardCompatibility(t *testing.T) {
	root := getRepoRootDir(t)

	lockBytes, err := os.ReadFile(filepath.Join(root, ".seb", "seb.lock"))
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

	previousKeys := []string{
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
		"011_gobernanza_afecta_uc",
		"012_soporte_claude",
	}
	for _, key := range previousKeys {
		entry, ok := lockData.Tasks[key]
		if !ok {
			t.Errorf("Falta la tarea previa %s en .seb/seb.lock", key)
			continue
		}
		if !entry.Verified || strings.TrimSpace(entry.CommitHash) == "" {
			t.Errorf("La tarea previa %s perdio su estado verified/commit_hash en seb.lock", key)
		}
		specBytes, err := os.ReadFile(filepath.Join(root, "specs", key+"_spec.md"))
		if err != nil {
			t.Errorf("No se pudo leer la spec previa %s: %v", key, err)
			continue
		}
		if !strings.Contains(string(specBytes), "**Estado:** `APROBADA`") {
			t.Errorf("La spec previa %s debe conservar Estado APROBADA", key)
		}
	}
}
