package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type directiveLiteral struct {
	file    string
	literal string
}

func prefixedDirectiveLiterals() []directiveLiteral {
	const (
		specTpl    = ".agents/skills/seb-especificador/template/nnn_nombre_tarea_spec.md"
		verifTpl   = ".agents/skills/seb-planificador/template/nnn_tarea_verificador.md"
		planTpl    = ".agents/skills/seb-planificador/template/nnn_tarea_plan.md"
		espSkill   = ".agents/skills/seb-especificador/SKILL.md"
		planSkill  = ".agents/skills/seb-planificador/SKILL.md"
		clarSkill  = ".agents/skills/seb-clarificador/SKILL.md"
		docSkill   = ".agents/skills/seb-documentador/SKILL.md"
		rules      = ".agents/rules/00-seb-harness.md"
		agentsFile = "AGENTS.md"
	)
	lits := []directiveLiteral{
		// 013-RF-01
		{specTpl, "**<NNN>-HU-01:**"},
		{specTpl, "**<NNN>-RF-01 ("},
		{specTpl, "**<NNN>-RNF-01:**"},
		{specTpl, "**<NNN>-CL-01 ("},
		// 013-RF-02
		{espSkill, "replace the `<NNN>` placeholder of every identifier with the three-digit ID returned by `seb spec-next`"},
		// 013-RF-03
		{verifTpl, "| <NNN>-CHK-01 | <NNN>-RF-01 |"},
		{planTpl, "`<NNN>-RF-01`"},
		{planSkill, "`| <NNN>-CHK-XX | <NNN>-RF-XX | Descripción |"},
		// 013-RF-04
		{planSkill, "Check IDs must start with the `<NNN>-CHK-` prefix of the source spec"},
		{planSkill, "reference each requirement with the prefixed identifier of the spec that owns it"},
		// 013-RF-05, 013-RF-06
		{clarSkill, "every declared `HU`, `RF`, `RNF` and `CL` identifier uses the `<NNN>-` prefix matching the spec `Tarea ID`"},
		{clarSkill, "an identifier without the `<NNN>-` prefix, with a prefix different from its `Tarea ID`, or duplicated"},
		// 013-RF-31
		{docSkill, "usar el identificador prefijado `<NNN>-<TIPO>-<NN>` de la spec propietaria"},
	}
	// 013-RF-32, 013-RF-33
	for _, f := range []string{rules, agentsFile} {
		lits = append(lits,
			directiveLiteral{f, "identificador prefijado `<NNN>-<TIPO>-<NN>`"},
			directiveLiteral{f, "solo en texto (comentarios y documentos), nunca como parte de identificadores de código"},
			directiveLiteral{f, "Operaciones One-Off"},
			directiveLiteral{f, "rol `migrador`"},
			directiveLiteral{f, "identificadas por el ID de la spec que las define"},
			directiveLiteral{f, "única excepción autorizada a la inmutabilidad"},
		)
	}
	return lits
}

func assertDirectiveLiterals(t *testing.T, base string) {
	t.Helper()
	for _, l := range prefixedDirectiveLiterals() {
		content := mustRead(t, filepath.Join(base, l.file))
		if !strings.Contains(content, l.literal) {
			t.Errorf("%s debe contener el literal %q", l.file, l.literal)
		}
	}
	specTpl := mustRead(t, filepath.Join(base, ".agents/skills/seb-especificador/template/nnn_nombre_tarea_spec.md"))
	for _, forbidden := range []string{"- **RF-01 (", "- **HU-01:**", "- **RNF-01:**", "- **CL-01 ("} {
		if strings.Contains(specTpl, forbidden) {
			t.Errorf("La plantilla de spec no debe declarar identificadores heredados: %q", forbidden)
		}
	}
	verifTpl := mustRead(t, filepath.Join(base, ".agents/skills/seb-planificador/template/nnn_tarea_verificador.md"))
	if regexp.MustCompile(`(?m)^\s*\|\s*CHK-`).MatchString(verifTpl) {
		t.Error("La plantilla de verificador no debe contener filas que empiecen con '| CHK-'")
	}
}

// 013-RF-01..013-RF-06, 013-RF-31, 013-RF-32, 013-RF-33, 013-RNF-04
func TestPrefixedIdentifierDirectives(t *testing.T) {
	root := repoRootDir(t)

	t.Run("literales canonicos en payload", func(t *testing.T) {
		assertDirectiveLiterals(t, filepath.Join(root, "payload"))
	})

	t.Run("AGENTS.md de raiz y payload identicos", func(t *testing.T) {
		if mustRead(t, filepath.Join(root, "AGENTS.md")) != mustRead(t, filepath.Join(root, "payload", "AGENTS.md")) {
			t.Error("AGENTS.md y payload/AGENTS.md deben ser identicos byte a byte")
		}
	})

	t.Run("dogfooding: .agents apunta a payload/.agents", func(t *testing.T) {
		resolved, err := filepath.EvalSymlinks(filepath.Join(root, ".agents"))
		if err != nil {
			t.Fatalf("No se pudo resolver .agents: %v", err)
		}
		want, _ := filepath.EvalSymlinks(filepath.Join(root, "payload", ".agents"))
		if resolved != want {
			t.Errorf(".agents debe resolver a payload/.agents: %s vs %s", resolved, want)
		}
	})

	t.Run("distribucion via install.sh --no-build", func(t *testing.T) {
		target := t.TempDir()
		code, stdout, stderr := runInstall(t, root, "--no-build", target)
		if code != 0 {
			t.Fatalf("install.sh fallo: %d\n%s\n%s", code, stdout, stderr)
		}
		if _, err := os.Stat(filepath.Join(target, "AGENTS.md")); err != nil {
			t.Fatalf("install.sh debe distribuir AGENTS.md: %v", err)
		}
		assertDirectiveLiterals(t, target)
	})
}
