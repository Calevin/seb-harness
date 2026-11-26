package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const defaultLockPath = ".seb/seb.lock"

var (
	reSpecFileName   = regexp.MustCompile(`^specs/(\d{3})_(.+)_spec\.md$`)
	reEditorialState = regexp.MustCompile(`^[-*>]\s*\*\*Estado:\*\*\s*(.+)$`)
)

// DependencyStatus describe el estado resuelto de una dependencia directa en el DAG.
type DependencyStatus struct {
	Target string `json:"target"`
	State  string `json:"state"` // OK, PENDIENTE, ABORTADA, AUSENTE, AMBIGUA
}

// TaskStatusDetail consolida la vista unificada multi-fuente de una tarea SDD o FAST.
type TaskStatusDetail struct {
	ID                     string             `json:"id"`
	Name                   string             `json:"name"`
	CanonicalName          string             `json:"canonical_name"`
	Mode                   string             `json:"mode"`
	SpecPath               string             `json:"spec_path"`
	HasSpec                bool               `json:"has_spec"`
	EditorialState         string             `json:"editorial_state"`
	Clarifications         int                `json:"clarifications_pending"`
	EngineStatus           string             `json:"engine_status"`
	InSQLite               bool               `json:"in_sqlite"`
	HasPlan                bool               `json:"has_plan"`
	HasVerifier            bool               `json:"has_verifier"`
	HasUnifiedPlanVerifier bool               `json:"has_unified_plan_verifier,omitempty"`
	InLock                 bool               `json:"in_lock"`
	LockVerified           bool               `json:"lock_verified"`
	LockAborted            bool               `json:"lock_aborted"`
	LockCommit             string             `json:"lock_commit_hash,omitempty"`
	Dependencies           []DependencyStatus `json:"dependencies"`
	Alerts                 []string           `json:"alerts"`
	NextStep               string             `json:"next_step"`
	UpdatedAt              string             `json:"updated_at,omitempty"`
}

// StatusReport representa el resultado estructurado de seb status.
type StatusReport struct {
	DatabasePresent bool               `json:"database_present"`
	LockPresent     bool               `json:"lock_present"`
	Tasks           []TaskStatusDetail `json:"tasks"`
}

type specMeta struct {
	EditorialState string
	EditorialValid bool
	Clarifications int
	Dependencies   []string
}

// readLockFileStrict lee .seb/seb.lock diferenciando ausencia de archivo vs JSON corrupto.
func readLockFileStrict(lockPath string) (SebLock, bool, error) {
	lock := SebLock{Tasks: make(map[string]TaskLock)}
	b, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return lock, false, nil
		}
		return lock, false, fmt.Errorf("error leyendo %s: %w", lockPath, err)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return lock, true, nil
	}
	if err := json.Unmarshal(b, &lock); err != nil {
		return lock, true, fmt.Errorf("archivo %s corrupto o JSON invalido: %w", lockPath, err)
	}
	if lock.Tasks == nil {
		lock.Tasks = make(map[string]TaskLock)
	}
	return lock, true, nil
}

// parseSpecMetadata extrae estado editorial, aclaraciones pendientes y dependencias de un *_spec.md.
func parseSpecMetadata(specPath string) specMeta {
	meta := specMeta{
		EditorialState: "DESCONOCIDO",
		EditorialValid: false,
	}

	f, err := os.Open(specPath)
	if err != nil {
		return meta
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if lineNum <= 40 && !meta.EditorialValid {
			if m := reEditorialState.FindStringSubmatch(trimmed); len(m) == 2 {
				val := strings.TrimSpace(m[1])
				val = strings.Trim(val, "`* ")
				switch val {
				case "BORRADOR", "APROBADA", "BLOQUEADA_POR_DEPENDENCIA", "ABORTADA":
					meta.EditorialState = val
					meta.EditorialValid = true
				default:
					meta.EditorialState = "DESCONOCIDO"
					meta.EditorialValid = false
				}
			}
		}

		meta.Clarifications += strings.Count(line, "[NECESITA ACLARACIÓN]")
	}

	deps, _ := parseSpecDependencies(specPath)
	meta.Dependencies = deps
	return meta
}

// parseStatusArgs separa flags (--compact, --json) y el filtro opcional <tarea>.
func parseStatusArgs(args []string) (compact bool, asJSON bool, filter string, err error) {
	for _, a := range args {
		switch a {
		case "--compact":
			compact = true
		case "--json":
			asJSON = true
		default:
			if strings.HasPrefix(a, "-") {
				return false, false, "", fmt.Errorf("flag desconocido en 'seb status': %s", a)
			}
			if filter != "" {
				return false, false, "", fmt.Errorf("demasiados argumentos en 'seb status': '%s' (solo se acepta un filtro <tarea>)", a)
			}
			filter = a
		}
	}
	return compact, asJSON, filter, nil
}

// buildStatusReport cruza specs/, .seb/seb.lock y SQLite (si db != nil) en modo de lectura pura.
func buildStatusReport(db *sql.DB, lockPath string, filter string) (*StatusReport, error) {
	lock, lockPresent, err := readLockFileStrict(lockPath)
	if err != nil {
		return nil, err
	}

	tasksByCanonical := make(map[string]*TaskStatusDetail)
	ensureTask := func(canonical, id, name string) *TaskStatusDetail {
		if t, ok := tasksByCanonical[canonical]; ok {
			return t
		}
		t := &TaskStatusDetail{
			ID:             id,
			Name:           name,
			CanonicalName:  canonical,
			Mode:           taskModeSDD,
			SpecPath:       buildArtifactPath(id, name, "spec"),
			EditorialState: "NO_SPEC",
			EngineStatus:   "NO_REGISTRADA",
			Dependencies:   []DependencyStatus{},
			Alerts:         []string{},
		}
		tasksByCanonical[canonical] = t
		return t
	}

	// 1. Escanear specs/*_*_spec.md en disco
	specFiles, _ := filepath.Glob("specs/*_*_spec.md")
	sort.Strings(specFiles)
	for _, f := range specFiles {
		slashPath := filepath.ToSlash(f)
		m := reSpecFileName.FindStringSubmatch(slashPath)
		if len(m) != 3 {
			continue
		}
		id, name := m[1], m[2]
		canonical := fmt.Sprintf("%s_%s", id, name)
		t := ensureTask(canonical, id, name)
		t.HasSpec = true
		t.SpecPath = slashPath

		meta := parseSpecMetadata(slashPath)
		t.EditorialState = meta.EditorialState
		t.Clarifications = meta.Clarifications
		if parseSpecMode(slashPath) == taskModeFAST {
			t.Mode = taskModeFAST
		}
		for _, d := range meta.Dependencies {
			t.Dependencies = append(t.Dependencies, DependencyStatus{Target: d})
		}

		if !meta.EditorialValid {
			t.Alerts = append(t.Alerts, "Cabecera Estado: ausente o invalida en "+slashPath+" (reportada como DESCONOCIDO)")
		}
	}

	// 2. Cruzar registros de .seb/seb.lock
	for key, tLock := range lock.Tasks {
		id, name := splitCanonicalOrLegacyKey(key)
		canonical := key
		if id != "" && name != "" {
			canonical = fmt.Sprintf("%s_%s", id, name)
		}
		t := ensureTask(canonical, id, name)
		t.InLock = true
		t.LockVerified = tLock.Verified
		t.LockAborted = tLock.Aborted
		t.LockCommit = tLock.CommitHash
		if strings.EqualFold(tLock.Mode, "fast") {
			t.Mode = taskModeFAST
		}
	}

	// 3. Cruzar registros de SQLite si existe base local (en solo lectura)
	if db != nil {
		rows, qErr := db.Query(`SELECT id, name, status, spec_path, updated_at, mode FROM tasks ORDER BY id ASC`)
		if qErr == nil {
			defer rows.Close()
			for rows.Next() {
				var id, name, status, specPath, updatedAt, mode string
				if sErr := rows.Scan(&id, &name, &status, &specPath, &updatedAt, &mode); sErr == nil {
					canonical := fmt.Sprintf("%s_%s", id, name)
					t := ensureTask(canonical, id, name)
					t.InSQLite = true
					t.EngineStatus = status
					t.UpdatedAt = updatedAt
					if normalizeTaskMode(mode) == taskModeFAST {
						t.Mode = taskModeFAST
					}
					if specPath != "" && !t.HasSpec {
						t.SpecPath = specPath
					}
				}
			}
		} else {
			// Compatibilidad con bases SQLite creadas antes de la columna mode
			rowsLegacy, lErr := db.Query(`SELECT id, name, status, spec_path, updated_at FROM tasks ORDER BY id ASC`)
			if lErr == nil {
				defer rowsLegacy.Close()
				for rowsLegacy.Next() {
					var id, name, status, specPath, updatedAt string
					if sErr := rowsLegacy.Scan(&id, &name, &status, &specPath, &updatedAt); sErr == nil {
						canonical := fmt.Sprintf("%s_%s", id, name)
						t := ensureTask(canonical, id, name)
						t.InSQLite = true
						t.EngineStatus = status
						t.UpdatedAt = updatedAt
						if specPath != "" && !t.HasSpec {
							t.SpecPath = specPath
						}
					}
				}
			}
		}
	}

	// 4. Completar presencia de plan/verificador (SDD o FAST), detectar ambiguedad, dependencias DAG y alertas
	for _, t := range tasksByCanonical {
		if t.ID != "" && t.Name != "" {
			if _, err := os.Stat(buildArtifactPath(t.ID, t.Name, "plan")); err == nil {
				t.HasPlan = true
			}
			if _, err := os.Stat(buildArtifactPath(t.ID, t.Name, "verificador")); err == nil {
				t.HasVerifier = true
			}
			fastPath := buildArtifactPath(t.ID, t.Name, "plan_y_verificador")
			if _, err := os.Stat(fastPath); err == nil {
				if t.HasPlan || t.HasVerifier {
					return nil, fmt.Errorf("ambiguedad de artefactos para '%s': coexisten '%s' con artefactos SDD estandar (*_plan.md o *_verificador.md)", t.CanonicalName, fastPath)
				}
				t.HasUnifiedPlanVerifier = true
				t.HasPlan = true
				t.HasVerifier = true
				t.Mode = taskModeFAST
			}
		}
	}

	for _, t := range tasksByCanonical {
		resolveTaskDependencies(t, tasksByCanonical)
		detectTaskDrifts(t)
		t.NextStep = computeTaskNextStep(t)
	}

	// 5. Ordenar deterministamente por clave canonica
	var keys []string
	for k := range tasksByCanonical {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var allTasks []TaskStatusDetail
	for _, k := range keys {
		allTasks = append(allTasks, *tasksByCanonical[k])
	}

	// 6. Aplicar filtro opcional <tarea> mediante resolucion flexible offline
	if filter != "" {
		matched, err := filterSingleTask(allTasks, filter)
		if err != nil {
			return nil, err
		}
		allTasks = []TaskStatusDetail{matched}
	}

	return &StatusReport{
		DatabasePresent: db != nil,
		LockPresent:     lockPresent,
		Tasks:           allTasks,
	}, nil
}

func splitCanonicalOrLegacyKey(key string) (string, string) {
	parts := strings.SplitN(key, "_", 2)
	if len(parts) == 2 && len(parts[0]) == 3 && isAllDigits(parts[0]) {
		return parts[0], parts[1]
	}
	return "", key
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func normalizeTaskQuery(q string) string {
	trimmed := strings.TrimSpace(q)
	if isAllDigits(trimmed) {
		var num int
		fmt.Sscanf(trimmed, "%d", &num)
		return fmt.Sprintf("%03d", num)
	}
	return trimmed
}

func filterSingleTask(tasks []TaskStatusDetail, query string) (TaskStatusDetail, error) {
	q := normalizeTaskQuery(query)
	if q == "" {
		return TaskStatusDetail{}, fmt.Errorf("identificador de tarea vacio")
	}

	var matches []TaskStatusDetail
	for _, t := range tasks {
		if t.ID == q || t.Name == q || t.CanonicalName == q {
			matches = append(matches, t)
		}
	}
	if len(matches) == 0 {
		return TaskStatusDetail{}, fmt.Errorf("tarea no encontrada: '%s'", query)
	}
	if len(matches) > 1 {
		var names []string
		for _, m := range matches {
			names = append(names, m.CanonicalName)
		}
		return TaskStatusDetail{}, fmt.Errorf("identificador ambiguo '%s'; candidatos: %s", query, strings.Join(names, ", "))
	}
	return matches[0], nil
}

func resolveTaskDependencies(t *TaskStatusDetail, all map[string]*TaskStatusDetail) {
	for i := range t.Dependencies {
		depQuery := normalizeTaskQuery(t.Dependencies[i].Target)
		var matched []*TaskStatusDetail
		for _, cand := range all {
			if cand.ID == depQuery || cand.Name == depQuery || cand.CanonicalName == depQuery {
				matched = append(matched, cand)
			}
		}
		if len(matched) == 0 {
			t.Dependencies[i].State = "AUSENTE"
			continue
		}
		if len(matched) > 1 {
			t.Dependencies[i].State = "AMBIGUA"
			continue
		}
		dep := matched[0]
		if dep.LockAborted || dep.EditorialState == "ABORTADA" || dep.EngineStatus == "ABORTED" {
			t.Dependencies[i].State = "ABORTADA"
		} else if dep.LockVerified && dep.LockCommit != "" {
			t.Dependencies[i].State = "OK"
		} else {
			t.Dependencies[i].State = "PENDIENTE"
		}
	}
}

func detectTaskDrifts(t *TaskStatusDetail) {
	if !t.HasSpec && t.InSQLite {
		t.Alerts = append(t.Alerts, fmt.Sprintf("Desfase disco/SQLite: registrada en SQLite (%s) sin archivo *_spec.md en disco", t.EngineStatus))
	}
	if !t.HasSpec && t.InLock {
		t.Alerts = append(t.Alerts, "Desfase disco/lock: entrada en .seb/seb.lock sin archivo *_spec.md en disco")
	}

	specAborted := t.HasSpec && t.EditorialState == "ABORTADA"
	if (specAborted || t.LockAborted) && t.InSQLite && t.EngineStatus != "ABORTED" {
		t.Alerts = append(t.Alerts, fmt.Sprintf("Desfase de aborto: disco/lock indica ABORTADA pero SQLite reporta '%s'", t.EngineStatus))
	}
	if t.InSQLite && t.EngineStatus == "ABORTED" && t.HasSpec && t.EditorialState != "ABORTADA" {
		t.Alerts = append(t.Alerts, fmt.Sprintf("Desfase de aborto: SQLite indica ABORTED pero *_spec.md indica '%s'", t.EditorialState))
	}

	if t.LockVerified && t.InSQLite && t.EngineStatus != "VERIFIED" {
		t.Alerts = append(t.Alerts, fmt.Sprintf("Desfase de verificacion: .seb/seb.lock marca verified=true pero SQLite reporta '%s'", t.EngineStatus))
	}
	if t.InSQLite && t.EngineStatus == "VERIFIED" && !t.LockVerified {
		t.Alerts = append(t.Alerts, "Desfase de verificacion: SQLite indica VERIFIED pero .seb/seb.lock no tiene verified=true")
	}

	if t.InLock && !t.LockAborted && (!t.HasPlan || !t.HasVerifier) {
		t.Alerts = append(t.Alerts, "Desfase de artefactos: tarea sellada en .seb/seb.lock pero falta *_plan.md o *_verificador.md en disco")
	}
	if t.InSQLite && (t.EngineStatus == "PLANNED" || t.EngineStatus == "VERIFIED") && (!t.HasPlan || !t.HasVerifier) {
		t.Alerts = append(t.Alerts, fmt.Sprintf("Desfase de artefactos: SQLite indica '%s' pero falta *_plan.md o *_verificador.md en disco", t.EngineStatus))
	}
}

func computeTaskNextStep(t *TaskStatusDetail) string {
	if !t.HasSpec {
		return "Revisar integridad: falta *_spec.md en disco (o purgar registro huerfano con 'seb abort <tarea> --purge')."
	}
	if t.EditorialState == "ABORTADA" || t.LockAborted || t.EngineStatus == "ABORTED" {
		return "Ninguno (tarea abortada)."
	}
	if t.LockVerified && t.LockCommit != "" {
		return fmt.Sprintf("Ejecutar skill 'seb-documentador' para actualizar casos de uso (tarea verificada en commit %s).", t.LockCommit)
	}
	for _, d := range t.Dependencies {
		if d.State != "OK" {
			return fmt.Sprintf("Resolver dependencia bloqueante '%s' (estado: %s) antes de continuar.", d.Target, d.State)
		}
	}
	if t.EditorialState == "BLOQUEADA_POR_DEPENDENCIA" {
		return "Desbloquear dependencias y reauditar con 'seb-clarificador'."
	}
	if t.EditorialState == "BORRADOR" || t.EditorialState == "DESCONOCIDO" || t.Clarifications > 0 {
		return "Ejecutar skill 'seb-clarificador' para auditar, resolver dudas pendientes y pasar Estado a APROBADA."
	}
	if t.EditorialState == "APROBADA" && (!t.HasPlan || !t.HasVerifier || !t.InLock) {
		if t.Mode == taskModeFAST {
			return fmt.Sprintf("Ejecutar skill 'seb-fast-task' para generar *_plan_y_verificador.md y sellar con 'seb plan-lock %s'.", t.Name)
		}
		return fmt.Sprintf("Ejecutar skill 'seb-planificador' y sellar con 'seb plan-lock %s'.", t.Name)
	}
	if t.EngineStatus == "FAILED_VERIFICATION" {
		return fmt.Sprintf("Corregir implementacion segun fallos y re-ejecutar 'seb verify %s'.", t.Name)
	}
	if t.Mode == taskModeFAST {
		return fmt.Sprintf("Implementar segun *_plan_y_verificador.md (rol implementador) y ejecutar skill 'seb-verificador' ('seb verify %s').", t.Name)
	}
	return fmt.Sprintf("Implementar segun *_plan.md (rol implementador) y ejecutar skill 'seb-verificador' ('seb verify %s').", t.Name)
}

func renderStatusCompact(w io.Writer, report *StatusReport) {
	if len(report.Tasks) == 0 {
		fmt.Fprintln(w, "No hay tareas ni especificaciones registradas en el proyecto.")
		return
	}
	fmt.Fprintln(w, "| ID | Clave Canonica | Editorial | Motor | Lock | Dependencias | Alertas | Modo |")
	fmt.Fprintln(w, "| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |")
	for _, t := range report.Tasks {
		lockStr := "NO"
		if t.LockAborted {
			lockStr = "ABORTED"
		} else if t.LockVerified {
			lockStr = "VERIFIED"
		} else if t.InLock {
			lockStr = "SEALED"
		}

		depStr := "-"
		if len(t.Dependencies) > 0 {
			var parts []string
			for _, d := range t.Dependencies {
				parts = append(parts, fmt.Sprintf("%s[%s]", d.Target, d.State))
			}
			depStr = strings.Join(parts, ", ")
		}

		alertStr := "0"
		if len(t.Alerts) > 0 {
			alertStr = fmt.Sprintf("%d", len(t.Alerts))
		}

		fmt.Fprintf(w, "| `%s` | `%s` | %s | %s | %s | %s | %s | %s |\n",
			t.ID, t.CanonicalName, t.EditorialState, t.EngineStatus, lockStr, depStr, alertStr, normalizeTaskMode(t.Mode))
	}
}

func renderStatusFull(w io.Writer, report *StatusReport) {
	if len(report.Tasks) == 0 {
		fmt.Fprintln(w, "No hay tareas ni especificaciones registradas en el proyecto.")
		return
	}
	if !report.DatabasePresent {
		fmt.Fprintln(w, "[Nota] Base local .seb/seb-audit.db no encontrada (clon limpio); estado reconstruido desde specs/ y .seb/seb.lock en modo solo lectura.")
		fmt.Fprintln(w)
	}
	for idx, t := range report.Tasks {
		if idx > 0 {
			fmt.Fprintln(w, "---")
		}
		fmt.Fprintf(w, "Tarea: %s (ID: %s, Nombre: %s, Modo: %s)\n", t.CanonicalName, t.ID, t.Name, normalizeTaskMode(t.Mode))
		fmt.Fprintf(w, "  Spec: %s (presente: %v)\n", t.SpecPath, t.HasSpec)
		fmt.Fprintf(w, "  Estado Editorial (Markdown): %s | Aclaraciones pendientes: %d\n", t.EditorialState, t.Clarifications)
		fmt.Fprintf(w, "  Estado Motor (SQLite): %s\n", t.EngineStatus)
		if t.Mode == taskModeFAST {
			fmt.Fprintf(w, "  Artefactos: plan_y_verificador=%v\n", t.HasUnifiedPlanVerifier)
		} else {
			fmt.Fprintf(w, "  Artefactos: plan=%v, verificador=%v\n", t.HasPlan, t.HasVerifier)
		}
		commitDisplay := t.LockCommit
		if commitDisplay == "" {
			commitDisplay = "-"
		}
		fmt.Fprintf(w, "  Estado seb.lock: sellada=%v, verified=%v, aborted=%v, commit=%s\n", t.InLock, t.LockVerified, t.LockAborted, commitDisplay)

		if len(t.Dependencies) == 0 {
			fmt.Fprintln(w, "  Dependencias DAG: Ninguna")
		} else {
			var depParts []string
			for _, d := range t.Dependencies {
				depParts = append(depParts, fmt.Sprintf("%s [%s]", d.Target, d.State))
			}
			fmt.Fprintf(w, "  Dependencias DAG: %s\n", strings.Join(depParts, ", "))
		}

		if len(t.Alerts) > 0 {
			fmt.Fprintln(w, "  Alertas de Desfase:")
			for _, a := range t.Alerts {
				fmt.Fprintf(w, "    - [ALERTA] %s\n", a)
			}
		} else {
			fmt.Fprintln(w, "  Alertas de Desfase: Ninguna")
		}

		fmt.Fprintf(w, "  Siguiente paso esperado: %s\n", t.NextStep)
	}
}

func renderStatusJSON(w io.Writer, report *StatusReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// runStatusCommand ejecuta seb status en modo lectura pura retornando el codigo de salida.
func runStatusCommand(db *sql.DB, args []string, stdout, stderr io.Writer) int {
	compact, asJSON, filter, err := parseStatusArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "[seb-harness ERROR] %v\n", err)
		return 1
	}

	report, err := buildStatusReport(db, defaultLockPath, filter)
	if err != nil {
		fmt.Fprintf(stderr, "[seb-harness ERROR] %v\n", err)
		return 1
	}

	if asJSON {
		if err := renderStatusJSON(stdout, report); err != nil {
			fmt.Fprintf(stderr, "[seb-harness ERROR] Error serializando JSON: %v\n", err)
			return 1
		}
		return 0
	}

	if compact {
		renderStatusCompact(stdout, report)
		return 0
	}

	renderStatusFull(stdout, report)
	return 0
}
