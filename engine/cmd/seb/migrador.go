package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Rol one-off "migrador" (Spec 013): unico autorizado a ejecutar operaciones one-off de actualizacion del arnes.
const migradorRole = "migrador"

// migradorEscalationRegex detecta intentos de ejecutar una one-off con ID o de activar el rol migrador.
// Se evalua en PreToolUse: los agentes nunca pueden escalar a migrador (solo el humano desde su terminal).
var migradorEscalationRegex = regexp.MustCompile(`(?i)(\bseb\s+oneoff\s+\d)|(\bseb\s+role\s+set\s+migrador\b)|(SEB_SKILL\s*=\s*["']?migrador\b)`)

const migradorEscalationReason = "[seb-harness ANTI-ESCALADA] Las operaciones one-off y el rol 'migrador' solo pueden ser activados por el humano desde su terminal."

// isMigradorWritableTarget define la politica de escritura del rol migrador.
func isMigradorWritableTarget(target string) bool {
	t := filepath.ToSlash(filepath.Clean(target))
	if t == "MEMORY.md" || t == ".seb/seb.lock" {
		return true
	}
	if strings.HasPrefix(t, "docs/use-cases/") {
		return true
	}
	if strings.HasPrefix(t, "specs/") && !strings.Contains(strings.TrimPrefix(t, "specs/"), "/") {
		return strings.HasSuffix(t, "_spec.md") || strings.HasSuffix(t, "_plan.md") || strings.HasSuffix(t, "_verificador.md")
	}
	return false
}

// resolveCLIRole resuelve el rol para subcomandos CLI: SEB_SKILL > .seb/ACTIVE_ROLE canonico > implementador.
func resolveCLIRole(rolePaths ...string) string {
	if env := strings.TrimSpace(os.Getenv("SEB_SKILL")); env != "" {
		return normalizeRole(env)
	}
	return getActiveRole(rolePaths...)
}
