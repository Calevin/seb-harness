package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Operacion one-off 013: migracion de identificadores heredados (HU/RF/RNF/CL/CHK-NN)
// al formato prefijado <NNN>-<TIPO>-<sufijo>.

var oneOff013 = OneOff{
	ID:         "013",
	Title:      "Migración de identificadores heredados a formato <NNN>-<TIPO>-<NN>",
	Applicable: oneOff013Applicable,
	Run:        runOneOff013,
}

var (
	numericIDRegex = regexp.MustCompile(`^\d+$`)
	// Identificador heredado en prosa: tipo + sufijo numerico (CHK acepta un digito o mas).
	legacyProseIDRegex = regexp.MustCompile(`(^|[^A-Za-z0-9_-])((?:HU|RF|RNF|CL)-\d{2,}|CHK-\d+)\b`)
	// Elemento heredado en la columna Requisito de una matriz de verificacion.
	legacyRequirementCellRegex = regexp.MustCompile(`^(HU|RF|RNF|CL|CHK)-[A-Za-z0-9_-]+$`)
	// Declaracion prefijada en una spec: "- **013-RF-01 ..."
	prefixedDeclRegex = regexp.MustCompile(`(?m)^\s*-\s*\*\*\d{3}-(HU|RF|RNF|CL)-\d`)
	digitRunRegex     = regexp.MustCompile(`\d+`)
	sha256HexRegex    = regexp.MustCompile(`[0-9a-f]{64}`)
)

type artifactKind int

const (
	kindSpec artifactKind = iota
	kindPlan
	kindVerifier
)

// oneOff013Applicable indica si el proyecto tiene verificadores con filas de control heredadas.
func oneOff013Applicable(root string) (bool, string, error) {
	n := countLegacyVerifiers(root)
	if n == 0 {
		return false, "sin verificadores heredados", nil
	}
	return true, fmt.Sprintf("%d verificadores con identificadores heredados", n), nil
}

// countLegacyVerifiers cuenta verificadores con al menos una fila cuya primera celda comienza con "CHK-".
func countLegacyVerifiers(root string) int {
	files, _ := filepath.Glob(filepath.Join(root, "specs", "*_verificador.md"))
	count := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		inFence := false
		for _, line := range strings.Split(string(data), "\n") {
			tl := strings.TrimSpace(line)
			if strings.HasPrefix(tl, "```") {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			if cell, ok := firstTableCell(tl); ok && strings.HasPrefix(cell, "CHK-") {
				count++
				break
			}
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// Capa pura de reescritura (sin I/O)
// ---------------------------------------------------------------------------

// specDeclaresPrefixedIDs indica si una spec ya declara sus identificadores en formato prefijado.
func specDeclaresPrefixedIDs(content []byte) bool {
	return prefixedDeclRegex.Match(content)
}

// mentionedSpecs devuelve los IDs de specs existentes mencionados en una linea
// (series de exactamente 3 digitos no precedidas por letra, digito o '-', y no seguidas por letra o digito).
func mentionedSpecs(line string, existingSpecs map[string]bool) map[string]bool {
	found := make(map[string]bool)
	for _, loc := range digitRunRegex.FindAllStringIndex(line, -1) {
		if loc[1]-loc[0] != 3 {
			continue
		}
		if loc[0] > 0 {
			prev := line[loc[0]-1]
			if isASCIIAlnum(prev) || prev == '-' {
				continue
			}
		}
		if loc[1] < len(line) && isASCIIAlnum(line[loc[1]]) {
			continue
		}
		id := line[loc[0]:loc[1]]
		if existingSpecs[id] {
			found[id] = true
		}
	}
	return found
}

func isASCIIAlnum(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// legacyIDsInLine devuelve los identificadores heredados de una linea con sus posiciones.
func legacyIDsInLine(line string) [][2]int {
	var out [][2]int
	for _, m := range legacyProseIDRegex.FindAllStringSubmatchIndex(line, -1) {
		out = append(out, [2]int{m[4], m[5]})
	}
	return out
}

// prefixLegacyIDs antepone prefix a cada identificador heredado de la linea.
func prefixLegacyIDs(line, prefix string) (string, int) {
	locs := legacyIDsInLine(line)
	if len(locs) == 0 {
		return line, 0
	}
	var b strings.Builder
	last := 0
	for _, l := range locs {
		b.WriteString(line[last:l[0]])
		b.WriteString(prefix + "-")
		b.WriteString(line[l[0]:l[1]])
		last = l[1]
	}
	b.WriteString(line[last:])
	return b.String(), len(locs)
}

// tablePipePositions devuelve las posiciones de los '|' que delimitan celdas, ignorando los que estan entre backticks.
func tablePipePositions(line string) []int {
	var pos []int
	inBacktick := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '`':
			inBacktick = !inBacktick
		case '|':
			if !inBacktick {
				pos = append(pos, i)
			}
		}
	}
	return pos
}

// rewriteCellPreservingSpace aplica fn al contenido recortado de una celda conservando sus espacios.
func rewriteCellPreservingSpace(cell string, fn func(string) (string, int)) (string, int) {
	trimmed := strings.TrimSpace(cell)
	if trimmed == "" {
		return cell, 0
	}
	start := strings.Index(cell, trimmed)
	newTrimmed, n := fn(trimmed)
	return cell[:start] + newTrimmed + cell[start+len(trimmed):], n
}

// migrateVerifierRow reescribe solo las celdas Check ID y Requisito de una fila de control.
func migrateVerifierRow(line, ownerID string) (string, int) {
	pipes := tablePipePositions(line)
	if len(pipes) < 3 {
		return line, 0
	}
	total := 0
	cell1 := line[pipes[0]+1 : pipes[1]]
	cell2 := line[pipes[1]+1 : pipes[2]]

	newCell1, n1 := rewriteCellPreservingSpace(cell1, func(s string) (string, int) {
		if strings.HasPrefix(s, "CHK-") {
			return ownerID + "-" + s, 1
		}
		return s, 0
	})
	total += n1

	parts := strings.Split(cell2, ",")
	for i, p := range parts {
		np, n := rewriteCellPreservingSpace(p, func(s string) (string, int) {
			if legacyRequirementCellRegex.MatchString(s) {
				return ownerID + "-" + s, 1
			}
			return s, 0
		})
		parts[i] = np
		total += n
	}
	newCell2 := strings.Join(parts, ",")

	if total == 0 {
		return line, 0
	}
	return line[:pipes[0]+1] + newCell1 + "|" + newCell2 + line[pipes[2]:], total
}

// splitLines divide el contenido conservando el estado de salto de linea final.
func splitLines(content []byte) ([]string, bool) {
	s := string(content)
	trailing := strings.HasSuffix(s, "\n")
	if trailing {
		s = s[:len(s)-1]
	}
	return strings.Split(s, "\n"), trailing
}

func joinLines(lines []string, trailing bool) []byte {
	out := strings.Join(lines, "\n")
	if trailing {
		out += "\n"
	}
	return []byte(out)
}

// migrateTaskArtifact reescribe un artefacto de tarea (spec, plan o verificador).
// Las filas de control de un verificador siempre se migran con el prefijo de la tarea.
// La prosa se migra solo si la spec de la tarea aun no declara identificadores prefijados.
func migrateTaskArtifact(content []byte, kind artifactKind, path, ownerID string, ownerSpecPrefixed bool, existingSpecs map[string]bool) ([]byte, int, []AmbiguousRef) {
	lines, trailing := splitLines(content)
	rewritten := 0
	var ambiguous []AmbiguousRef
	inFence := false
	for i, line := range lines {
		tl := strings.TrimSpace(line)
		if strings.HasPrefix(tl, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if kind == kindVerifier {
			if cell, ok := firstTableCell(tl); ok && isCandidateFirstCell(cell) {
				nl, n := migrateVerifierRow(line, ownerID)
				lines[i] = nl
				rewritten += n
				continue
			}
		}
		if ownerSpecPrefixed {
			continue
		}
		locs := legacyIDsInLine(line)
		if len(locs) == 0 {
			continue
		}
		mentions := mentionedSpecs(line, existingSpecs)
		foreign := false
		for id := range mentions {
			if id != ownerID {
				foreign = true
				break
			}
		}
		if foreign {
			for _, l := range locs {
				ambiguous = append(ambiguous, AmbiguousRef{Path: path, Line: i + 1, ID: line[l[0]:l[1]], Text: tl})
			}
			continue
		}
		nl, n := prefixLegacyIDs(line, ownerID)
		lines[i] = nl
		rewritten += n
	}
	return joinLines(lines, trailing), rewritten, ambiguous
}

// migrateDocumentArtifact reescribe un artefacto documental (casos de uso, MEMORY.md):
// un identificador heredado se prefija solo si la linea menciona exactamente una spec existente.
func migrateDocumentArtifact(content []byte, path string, existingSpecs map[string]bool) ([]byte, int, []AmbiguousRef) {
	lines, trailing := splitLines(content)
	rewritten := 0
	var ambiguous []AmbiguousRef
	inFence := false
	for i, line := range lines {
		tl := strings.TrimSpace(line)
		if strings.HasPrefix(tl, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		locs := legacyIDsInLine(line)
		if len(locs) == 0 {
			continue
		}
		mentions := mentionedSpecs(line, existingSpecs)
		if len(mentions) != 1 {
			for _, l := range locs {
				ambiguous = append(ambiguous, AmbiguousRef{Path: path, Line: i + 1, ID: line[l[0]:l[1]], Text: tl})
			}
			continue
		}
		var owner string
		for id := range mentions {
			owner = id
		}
		nl, n := prefixLegacyIDs(line, owner)
		lines[i] = nl
		rewritten += n
	}
	return joinLines(lines, trailing), rewritten, ambiguous
}

// substituteEmbeddedHashes reemplaza, solo en la celda de comando de las filas de control,
// los hashes sellados previos por sus hashes nuevos (013-RF-27).
func substituteEmbeddedHashes(content []byte, oldToNew map[string]string) ([]byte, int) {
	if len(oldToNew) == 0 {
		return content, 0
	}
	lines, trailing := splitLines(content)
	total := 0
	inFence := false
	for i, line := range lines {
		tl := strings.TrimSpace(line)
		if strings.HasPrefix(tl, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		cell, ok := firstTableCell(tl)
		if !ok || !isCandidateFirstCell(cell) {
			continue
		}
		pipes := tablePipePositions(line)
		if len(pipes) < 5 {
			continue
		}
		cmdStart, cmdEnd := pipes[3]+1, pipes[4]
		cmdCell := line[cmdStart:cmdEnd]
		n := 0
		newCmd := sha256HexRegex.ReplaceAllStringFunc(cmdCell, func(h string) string {
			if nh, ok := oldToNew[h]; ok && nh != h {
				n++
				return nh
			}
			return h
		})
		if n > 0 {
			lines[i] = line[:cmdStart] + newCmd + line[cmdEnd:]
			total += n
		}
	}
	return joinLines(lines, trailing), total
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// Capa transaccional
// ---------------------------------------------------------------------------

type migrationTask struct {
	ID        string
	Canonical string
	Spec      string
	Plan      string
	Verifier  string
}

// gitPreconditionCheck exige que los artefactos a migrar no tengan cambios sin commitear.
func gitPreconditionCheck(root string) error {
	cmd := exec.Command("git", "-C", root, "status", "--porcelain", "--", "specs", "docs/use-cases", "MEMORY.md", ".seb/seb.lock")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("precondición Git no satisfecha: no se pudo consultar el estado del repositorio (%v); la migración requiere un repositorio Git", err)
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		return fmt.Errorf("precondición Git no satisfecha: existen cambios sin commitear en artefactos a migrar:\n%s", s)
	}
	return nil
}

func discoverMigrationTasks(root string) []migrationTask {
	files, _ := filepath.Glob(filepath.Join(root, "specs", "*_*_spec.md"))
	var tasks []migrationTask
	for _, f := range files {
		base := filepath.Base(f)
		m := specFileIDRegex.FindStringSubmatch(base)
		if len(m) != 2 {
			continue
		}
		canonical := strings.TrimSuffix(base, "_spec.md")
		t := migrationTask{ID: m[1], Canonical: canonical, Spec: f}
		if p := filepath.Join(root, "specs", canonical+"_plan.md"); fileExists(p) {
			t.Plan = p
		}
		if v := filepath.Join(root, "specs", canonical+"_verificador.md"); fileExists(v) {
			t.Verifier = v
		}
		tasks = append(tasks, t)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Canonical < tasks[j].Canonical })
	return tasks
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func readLockAt(fs fileStore, path string) (SebLock, []byte, error) {
	var lock SebLock
	lock.Tasks = make(map[string]TaskLock)
	raw, err := fs.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return lock, nil, nil
		}
		return lock, nil, err
	}
	if err := json.Unmarshal(raw, &lock); err != nil {
		return lock, raw, fmt.Errorf("seb.lock inválido: %v", err)
	}
	if lock.Tasks == nil {
		lock.Tasks = make(map[string]TaskLock)
	}
	return lock, raw, nil
}

func relPath(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil {
		return filepath.ToSlash(r)
	}
	return filepath.ToSlash(p)
}

// runOneOff013 ejecuta la migracion completa con precondicion Git, resellado, auditoria y rollback.
func runOneOff013(ctx OneOffContext) (OneOffReport, error) {
	start := time.Now()
	report := OneOffReport{DryRun: ctx.DryRun}
	root := ctx.Root
	fs := ctx.FS
	if fs == nil {
		fs = osFileStore{}
	}

	if err := gitPreconditionCheck(root); err != nil {
		return report, err
	}

	lockPath := filepath.Join(root, ".seb", "seb.lock")
	lock, originalLock, err := readLockAt(fs, lockPath)
	if err != nil {
		return report, err
	}

	existing := existingSpecIDs(filepath.Join(root, "specs"))
	tasks := discoverMigrationTasks(root)

	oldContent := make(map[string][]byte)
	newContent := make(map[string][]byte)
	rewrittenBy := make(map[string]int)
	taskOf := make(map[string]string)
	var order []string
	verifiers := []string{}

	addFile := func(path string) ([]byte, error) {
		b, err := fs.ReadFile(path)
		if err != nil {
			return nil, err
		}
		oldContent[path] = b
		order = append(order, path)
		return b, nil
	}

	for _, t := range tasks {
		specBytes, err := addFile(t.Spec)
		if err != nil {
			return report, err
		}
		prefixed := specDeclaresPrefixedIDs(specBytes)
		artifacts := []struct {
			path string
			kind artifactKind
		}{{t.Spec, kindSpec}, {t.Plan, kindPlan}, {t.Verifier, kindVerifier}}
		for _, a := range artifacts {
			if a.path == "" {
				continue
			}
			var b []byte
			if a.path == t.Spec {
				b = specBytes
			} else if b, err = addFile(a.path); err != nil {
				return report, err
			}
			out, n, amb := migrateTaskArtifact(b, a.kind, relPath(root, a.path), t.ID, prefixed, existing)
			newContent[a.path] = out
			rewrittenBy[a.path] = n
			taskOf[a.path] = t.ID
			report.Ambiguous = append(report.Ambiguous, amb...)
			if a.kind == kindVerifier {
				verifiers = append(verifiers, a.path)
			}
		}
	}

	docs, _ := filepath.Glob(filepath.Join(root, "docs", "use-cases", "*.md"))
	sort.Strings(docs)
	if mem := filepath.Join(root, "MEMORY.md"); fileExists(mem) {
		docs = append(docs, mem)
	}
	for _, d := range docs {
		b, err := addFile(d)
		if err != nil {
			return report, err
		}
		out, n, amb := migrateDocumentArtifact(b, relPath(root, d), existing)
		newContent[d] = out
		rewrittenBy[d] = n
		report.Ambiguous = append(report.Ambiguous, amb...)
	}

	// Sustitucion de hashes embebidos hasta punto fijo
	taskByCanonical := make(map[string]migrationTask)
	for _, t := range tasks {
		taskByCanonical[t.Canonical] = t
	}
	converged := false
	for iter := 0; iter <= len(verifiers)+1; iter++ {
		oldToNew := make(map[string]string)
		for canonical, tl := range lock.Tasks {
			t, ok := taskByCanonical[canonical]
			if !ok {
				continue
			}
			if tl.SpecHash != "" {
				if nh := sha256Hex(newContent[t.Spec]); nh != tl.SpecHash && !bytes.Equal(newContent[t.Spec], oldContent[t.Spec]) {
					oldToNew[tl.SpecHash] = nh
				}
			}
			if tl.VerifierHash != "" && t.Verifier != "" {
				if nh := sha256Hex(newContent[t.Verifier]); nh != tl.VerifierHash && !bytes.Equal(newContent[t.Verifier], oldContent[t.Verifier]) {
					oldToNew[tl.VerifierHash] = nh
				}
			}
		}
		changed := false
		for _, v := range verifiers {
			out, n := substituteEmbeddedHashes(newContent[v], oldToNew)
			if n > 0 && !bytes.Equal(out, newContent[v]) {
				newContent[v] = out
				report.HashSubstitutions += n
				changed = true
			}
		}
		if !changed {
			converged = true
			break
		}
	}
	if !converged {
		return report, fmt.Errorf("la sustitución de hashes embebidos no convergió")
	}

	for _, p := range order {
		if bytes.Equal(oldContent[p], newContent[p]) {
			continue
		}
		report.Changes = append(report.Changes, FileChange{
			Path:      relPath(root, p),
			TaskID:    taskOf[p],
			OldHash:   sha256Hex(oldContent[p]),
			NewHash:   sha256Hex(newContent[p]),
			Rewritten: rewrittenBy[p],
		})
		report.Rewritten += rewrittenBy[p]
	}

	if ctx.DryRun || len(report.Changes) == 0 {
		return report, nil
	}

	// Resellado de hashes preservando verified, commit_hash y aborted
	newLock := SebLock{Tasks: make(map[string]TaskLock)}
	for canonical, tl := range lock.Tasks {
		if t, ok := taskByCanonical[canonical]; ok {
			if tl.SpecHash != "" && !bytes.Equal(newContent[t.Spec], oldContent[t.Spec]) {
				tl.SpecHash = sha256Hex(newContent[t.Spec])
			}
			if tl.VerifierHash != "" && t.Verifier != "" && !bytes.Equal(newContent[t.Verifier], oldContent[t.Verifier]) {
				tl.VerifierHash = sha256Hex(newContent[t.Verifier])
			}
		}
		newLock.Tasks[canonical] = tl
	}
	lockBytes, _ := json.MarshalIndent(newLock, "", "  ")

	// Escritura con respaldo y rollback ante cualquier fallo
	var written []string
	rollback := func() error {
		var errs []string
		for _, p := range written {
			if err := fs.WriteFile(p, oldContent[p]); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", p, err))
			}
		}
		if originalLock != nil {
			if err := fs.WriteFile(lockPath, originalLock); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", lockPath, err))
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("rollback incompleto: %s", strings.Join(errs, "; "))
		}
		return nil
	}
	for _, c := range report.Changes {
		p := filepath.Join(root, filepath.FromSlash(c.Path))
		if err := fs.WriteFile(p, newContent[p]); err != nil {
			rbErr := rollback()
			if rbErr != nil {
				return report, fmt.Errorf("fallo escribiendo %s: %v; %v", c.Path, err, rbErr)
			}
			return report, fmt.Errorf("fallo escribiendo %s: %v; se restauró el estado previo", c.Path, err)
		}
		written = append(written, p)
	}
	if originalLock != nil && !bytes.Equal(lockBytes, originalLock) {
		if err := fs.WriteFile(lockPath, lockBytes); err != nil {
			rbErr := rollback()
			if rbErr != nil {
				return report, fmt.Errorf("fallo escribiendo seb.lock: %v; %v", err, rbErr)
			}
			return report, fmt.Errorf("fallo escribiendo seb.lock: %v; se restauró el estado previo", err)
		}
	}

	// Auditoria local por archivo modificado y resumen
	if ctx.DB != nil {
		for _, c := range report.Changes {
			logInternalAction(ctx.DB, c.TaskID, migradorRole, "oneoff-013:rewrite", c.Path, c.OldHash+"->"+c.NewHash, 0, 0)
		}
		logInternalAction(ctx.DB, "", migradorRole, "oneoff-013:summary", "",
			fmt.Sprintf("files=%d rewritten=%d hashes=%d ambiguous=%d", len(report.Changes), report.Rewritten, report.HashSubstitutions, len(report.Ambiguous)),
			time.Since(start).Milliseconds(), 0)
	}
	return report, nil
}
