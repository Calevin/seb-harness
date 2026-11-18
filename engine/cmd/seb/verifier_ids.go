package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Identificadores prefijados <NNN>-<TIPO>-<sufijo> en matrices de verificacion (Spec 013).
var (
	legacyCheckIDRegex   = regexp.MustCompile(`^CHK-`)
	prefixedCheckIDRegex = regexp.MustCompile(`^(\d{3})-CHK-[A-Za-z0-9_-]+$`)
	requirementRefRegex  = regexp.MustCompile(`^(\d{3})-(HU|RF|RNF|CL|CHK)-[A-Za-z0-9_-]+$`)
	verifierTaskIDRegex  = regexp.MustCompile(`^(\d{3})_.+_verificador\.md$`)
	specFileIDRegex      = regexp.MustCompile(`^(\d{3})_.+_spec\.md$`)
)

// migrationHintCommand es el comando exacto que se sugiere ante identificadores heredados.
const migrationHintCommand = ".seb/bin/seb role set migrador && .seb/bin/seb oneoff 013"

// errLegacyCheckID identifica un Check ID en formato heredado (CHK-NN sin prefijo de spec).
var errLegacyCheckID = errors.New("identificador de control heredado")

// firstTableCell devuelve el contenido de la primera celda de una fila de tabla Markdown.
// Retorna ok=false si la linea no comienza con '|'.
func firstTableCell(trimmedLine string) (string, bool) {
	if !strings.HasPrefix(trimmedLine, "|") {
		return "", false
	}
	rest := trimmedLine[1:]
	if idx := strings.Index(rest, "|"); idx >= 0 {
		rest = rest[:idx]
	}
	return strings.TrimSpace(rest), true
}

// isCandidateFirstCell determina si la primera celda corresponde a una fila de control:
// contiene "-CHK-" (formato prefijado) o comienza con "CHK-" (formato heredado, para rechazarlo).
func isCandidateFirstCell(cell string) bool {
	return strings.Contains(cell, "-CHK-") || strings.HasPrefix(cell, "CHK-")
}

// taskIDFromVerifierPath deriva el ID de tarea de 3 digitos del nombre del verificador.
func taskIDFromVerifierPath(path string) (string, error) {
	m := verifierTaskIDRegex.FindStringSubmatch(filepath.Base(path))
	if len(m) != 2 {
		return "", fmt.Errorf("no se puede determinar el ID de tarea desde '%s' (se espera <NNN>_<tarea>_verificador.md)", filepath.Base(path))
	}
	return m[1], nil
}

// existingSpecIDs devuelve los IDs de spec presentes en disco dentro del directorio indicado.
func existingSpecIDs(dir string) map[string]bool {
	ids := make(map[string]bool)
	files, _ := filepath.Glob(filepath.Join(dir, "*_*_spec.md"))
	for _, f := range files {
		if m := specFileIDRegex.FindStringSubmatch(filepath.Base(f)); len(m) == 2 {
			ids[m[1]] = true
		}
	}
	return ids
}

// validateCheckID exige el formato <NNN>-CHK-<sufijo> con NNN igual a la tarea verificada.
func validateCheckID(id, taskID string) error {
	if legacyCheckIDRegex.MatchString(id) {
		return fmt.Errorf("%w '%s'; el proyecto requiere ejecutar la operación one-off 013: %s", errLegacyCheckID, id, migrationHintCommand)
	}
	m := prefixedCheckIDRegex.FindStringSubmatch(id)
	if len(m) != 2 {
		return fmt.Errorf("formato de identificador de control inválido '%s' (se espera %s-CHK-<sufijo>)", id, taskID)
	}
	if m[1] != taskID {
		return fmt.Errorf("identificador de control incoherente '%s': el prefijo '%s' no coincide con la tarea verificada '%s'", id, m[1], taskID)
	}
	return nil
}

// validateRequirementRef acepta GLOBAL o <NNN>-<TIPO>-<sufijo> con NNN de una spec existente.
func validateRequirementRef(ref string, specs map[string]bool) error {
	if ref == "GLOBAL" {
		return nil
	}
	m := requirementRefRegex.FindStringSubmatch(ref)
	if len(m) != 3 {
		return fmt.Errorf("referencia de requisito inválida '%s' (se espera GLOBAL o <NNN>-<TIPO>-<sufijo> con TIPO en HU, RF, RNF, CL, CHK)", ref)
	}
	if !specs[m[1]] {
		return fmt.Errorf("referencia de requisito inválida '%s': no existe una spec con ID '%s'", ref, m[1])
	}
	return nil
}
