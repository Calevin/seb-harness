package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	taskModeSDD         = "SDD"
	taskModeFAST        = "FAST"
	defaultFastMaxFiles = 4
	defaultFastMaxLines = 100
)

// FastTrackConfig representa la seccion opcional "fast_track" en .seb/config.json.
type FastTrackConfig struct {
	MaxFiles *int `json:"max_files,omitempty"`
	MaxLines *int `json:"max_lines,omitempty"`
}

// ResolvedFastTrackConfig contiene los umbrales efectivos para tareas FAST.
type ResolvedFastTrackConfig struct {
	MaxFiles int
	MaxLines int
}

var (
	reSpecModeHeader = regexp.MustCompile(`^[-*>]\s*\*\*Modo:\*\*\s*(.+)$`)
	reTestSkipMarker = regexp.MustCompile(`(\bt\.Skip(?:f|Now)?\s*\(|@Ignore\b|markTestSkipped\s*\(|\.(?:skip|only)\s*\(|@pytest\.mark\.skip\b|#\[ignore\]|\bx(?:it|describe)\s*\()`)
)

// normalizeTaskMode normaliza el modo de una tarea a "FAST" o "SDD".
func normalizeTaskMode(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), taskModeFAST) {
		return taskModeFAST
	}
	return taskModeSDD
}

// loadFastTrackConfig lee y valida los umbrales FAST desde .seb/config.json.
// Aplica valores por defecto (4 archivos, 100 lineas) si el archivo o la seccion no existen.
func loadFastTrackConfig(configPath string) (ResolvedFastTrackConfig, error) {
	resolved := ResolvedFastTrackConfig{
		MaxFiles: defaultFastMaxFiles,
		MaxLines: defaultFastMaxLines,
	}
	if strings.TrimSpace(configPath) == "" {
		configPath = ".seb/config.json"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return resolved, nil
		}
		return resolved, fmt.Errorf("error leyendo configuracion '%s': %w", configPath, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return resolved, nil
	}

	var cfg EngineConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return resolved, fmt.Errorf("configuracion invalida en '%s' (JSON malformado o tipos invalidos en fast_track): %w", configPath, err)
	}
	if cfg.FastTrack == nil {
		return resolved, nil
	}

	if cfg.FastTrack.MaxFiles != nil {
		if *cfg.FastTrack.MaxFiles <= 0 {
			return resolved, fmt.Errorf("configuracion invalida en '%s': fast_track.max_files debe ser mayor a 0 (recibido: %d)", configPath, *cfg.FastTrack.MaxFiles)
		}
		resolved.MaxFiles = *cfg.FastTrack.MaxFiles
	}
	if cfg.FastTrack.MaxLines != nil {
		if *cfg.FastTrack.MaxLines <= 0 {
			return resolved, fmt.Errorf("configuracion invalida en '%s': fast_track.max_lines debe ser mayor a 0 (recibido: %d)", configPath, *cfg.FastTrack.MaxLines)
		}
		resolved.MaxLines = *cfg.FastTrack.MaxLines
	}

	return resolved, nil
}

// parseSpecMode inspecciona las primeras 40 lineas de una spec para detectar "Modo: FAST".
func parseSpecMode(specPath string) string {
	f, err := os.Open(specPath)
	if err != nil {
		return taskModeSDD
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() && lineNum < 40 {
		lineNum++
		trimmed := strings.TrimSpace(scanner.Text())
		if m := reSpecModeHeader.FindStringSubmatch(trimmed); len(m) == 2 {
			val := strings.Trim(strings.TrimSpace(m[1]), "`* ")
			if strings.EqualFold(val, taskModeFAST) {
				return taskModeFAST
			}
			return taskModeSDD
		}
	}
	return taskModeSDD
}

// detectTaskModeAndArtifacts determina el modo (FAST o SDD) y rutas de plan/verificador,
// rechazando la coexistencia ambigua de *_plan_y_verificador.md con artefactos SDD estandar.
func detectTaskModeAndArtifacts(id, name, specPath string) (mode, planPath, verifierPath string, err error) {
	stdPlan := buildArtifactPath(id, name, "plan")
	stdVerif := buildArtifactPath(id, name, "verificador")
	fastPath := buildArtifactPath(id, name, "plan_y_verificador")

	_, errStdPlan := os.Stat(stdPlan)
	_, errStdVerif := os.Stat(stdVerif)
	_, errFast := os.Stat(fastPath)

	hasStdPlan := errStdPlan == nil
	hasStdVerif := errStdVerif == nil
	hasFast := errFast == nil

	if hasFast && (hasStdPlan || hasStdVerif) {
		return "", "", "", fmt.Errorf("ambiguedad de artefactos para '%s_%s': coexisten '%s' con artefactos SDD estandar (*_plan.md o *_verificador.md)", id, name, fastPath)
	}

	if hasFast {
		return taskModeFAST, fastPath, fastPath, nil
	}

	if specPath != "" && parseSpecMode(specPath) == taskModeFAST && !hasStdPlan && !hasStdVerif {
		return taskModeFAST, fastPath, fastPath, nil
	}

	return taskModeSDD, stdPlan, stdVerif, nil
}

// isFastExcludedPath determina si un archivo pertenece a los artefactos de gobernanza excluidos del conteo FAST.
func isFastExcludedPath(relPath, taskCanonical string) bool {
	p := filepath.ToSlash(filepath.Clean(strings.TrimSpace(relPath)))
	if p == "" || p == "." {
		return true
	}
	if strings.HasPrefix(p, ".seb/") || p == ".seb" {
		return true
	}
	if taskCanonical != "" && strings.HasPrefix(p, "specs/"+taskCanonical+"_") && strings.HasSuffix(p, ".md") {
		return true
	}
	return false
}

// isTestFilePath identifica archivos de pruebas donde aplican controles anti-reward-hacking.
func isTestFilePath(relPath string) bool {
	p := strings.ToLower(filepath.ToSlash(filepath.Clean(relPath)))
	base := filepath.Base(p)
	if strings.HasSuffix(base, "_test.go") ||
		strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") ||
		strings.HasPrefix(base, "test_") ||
		strings.HasSuffix(base, "_test.py") ||
		strings.HasSuffix(base, "test.java") ||
		strings.HasSuffix(base, "test.php") ||
		strings.Contains(p, "/tests/") ||
		strings.HasPrefix(p, "tests/") {
		return true
	}
	return false
}

// measureFastGitDiff calcula cantidad de archivos y lineas modificadas (inserciones + eliminaciones)
// en el repositorio Git del directorio actual, excluyendo los artefactos de la tarea FAST y .seb/.
func measureFastGitDiff(taskCanonical string) (filesCount int, linesCount int, err error) {
	checkGit := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	if out, gErr := checkGit.Output(); gErr != nil || strings.TrimSpace(string(out)) != "true" {
		return 0, 0, nil
	}

	hasHead := exec.Command("git", "rev-parse", "--verify", "HEAD").Run() == nil

	seenFiles := make(map[string]bool)
	var diffOut []byte
	if hasHead {
		diffOut, err = exec.Command("git", "diff", "--numstat", "HEAD").Output()
	} else {
		diffOut, err = exec.Command("git", "diff", "--cached", "--numstat").Output()
	}
	if err != nil {
		return 0, 0, fmt.Errorf("error ejecutando git diff --numstat: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(diffOut))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		path := parts[len(parts)-1]
		if isFastExcludedPath(path, taskCanonical) {
			continue
		}
		seenFiles[path] = true
		if add, convErr := strconv.Atoi(parts[0]); convErr == nil {
			linesCount += add
		}
		if del, convErr := strconv.Atoi(parts[1]); convErr == nil {
			linesCount += del
		}
	}

	// Incorporar archivos nuevos no trackeados en el conteo determinista
	untrackedOut, uErr := exec.Command("git", "ls-files", "--others", "--exclude-standard").Output()
	if uErr == nil {
		uScanner := bufio.NewScanner(bytes.NewReader(untrackedOut))
		for uScanner.Scan() {
			path := strings.TrimSpace(uScanner.Text())
			if path == "" || isFastExcludedPath(path, taskCanonical) || seenFiles[path] {
				continue
			}
			seenFiles[path] = true
			if raw, rErr := os.ReadFile(path); rErr == nil {
				linesCount += countNonEmptyFileLines(raw)
			}
		}
	}

	return len(seenFiles), linesCount, nil
}

func countNonEmptyFileLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := bytes.Count(data, []byte("\n"))
	if data[len(data)-1] != '\n' {
		lines++
	}
	return lines
}

// validateFastDiffThresholds valida deterministamente que el diff de la tarea FAST no exceda max_files ni max_lines.
func validateFastDiffThresholds(taskCanonical string, cfg ResolvedFastTrackConfig) error {
	filesCount, linesCount, err := measureFastGitDiff(taskCanonical)
	if err != nil {
		return err
	}
	if filesCount > cfg.MaxFiles || linesCount > cfg.MaxLines {
		return fmt.Errorf(
			"BLOQUEO FAST: El cambio excede los umbrales permitidos para Flujo Rapido (archivos=%d/%d, lineas=%d/%d). Escala en caliente al Flujo SDD Completo conservando '%s': amplia specs/%s_spec.md y reemplaza specs/%s_plan_y_verificador.md por *_plan.md y *_verificador.md (o ejecuta seb-especificador)",
			filesCount, cfg.MaxFiles, linesCount, cfg.MaxLines, taskCanonical, taskCanonical, taskCanonical,
		)
	}
	return nil
}

// validateFastAntiRewardHackingDiff inspecciona lineas agregadas en archivos de test para bloquear omisiones de pruebas.
func validateFastAntiRewardHackingDiff(taskCanonical string) error {
	checkGit := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	if out, gErr := checkGit.Output(); gErr != nil || strings.TrimSpace(string(out)) != "true" {
		return nil
	}

	hasHead := exec.Command("git", "rev-parse", "--verify", "HEAD").Run() == nil
	var diffOut []byte
	var err error
	if hasHead {
		diffOut, err = exec.Command("git", "diff", "-U0", "HEAD").Output()
	} else {
		diffOut, err = exec.Command("git", "diff", "--cached", "-U0").Output()
	}
	if err != nil {
		return fmt.Errorf("error inspeccionando diff anti-reward-hacking: %w", err)
	}

	currentFile := ""
	scanner := bufio.NewScanner(bytes.NewReader(diffOut))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "+++ b/") {
			currentFile = strings.TrimPrefix(line, "+++ b/")
			continue
		}
		if currentFile == "" || isFastExcludedPath(currentFile, taskCanonical) || !isTestFilePath(currentFile) {
			continue
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added := line[1:]
			if m := reTestSkipMarker.FindString(added); m != "" {
				return fmt.Errorf("VIOLACION ANTI-REWARD-HACKING: Patron de omision de test '%s' detectado en '%s'", strings.TrimSpace(m), currentFile)
			}
		}
	}

	// Revisar tambien archivos de test nuevos no trackeados
	untrackedOut, uErr := exec.Command("git", "ls-files", "--others", "--exclude-standard").Output()
	if uErr == nil {
		uScanner := bufio.NewScanner(bytes.NewReader(untrackedOut))
		for uScanner.Scan() {
			path := strings.TrimSpace(uScanner.Text())
			if path == "" || isFastExcludedPath(path, taskCanonical) || !isTestFilePath(path) {
				continue
			}
			if raw, rErr := os.ReadFile(path); rErr == nil {
				if m := reTestSkipMarker.FindString(string(raw)); m != "" {
					return fmt.Errorf("VIOLACION ANTI-REWARD-HACKING: Patron de omision de test '%s' detectado en '%s'", strings.TrimSpace(m), path)
				}
			}
		}
	}

	return nil
}
