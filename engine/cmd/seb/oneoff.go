package main

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Operaciones one-off de actualizacion del arnes (Spec 013).
// Cada operacion se identifica por el ID de la spec que la define y solo puede
// ejecutarse con el rol "migrador" activado por el humano.

// fileStore abstrae la lectura y escritura de archivos para permitir simular fallos (rollback).
type fileStore interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte) error
}

// osFileStore escribe de forma atomica mediante archivo temporal + rename en el mismo directorio.
type osFileStore struct{}

func (osFileStore) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (osFileStore) WriteFile(path string, data []byte) error {
	mode := os.FileMode(0644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".seb-oneoff-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// OneOffContext agrupa las dependencias inyectables de una operacion one-off.
type OneOffContext struct {
	Root   string
	DB     *sql.DB
	DryRun bool
	Stdout io.Writer
	Stderr io.Writer
	FS     fileStore
}

// AmbiguousRef es una referencia heredada cuya spec propietaria no puede determinarse.
type AmbiguousRef struct {
	Path string
	Line int
	ID   string
	Text string
}

// FileChange describe un archivo reescrito por una operacion one-off.
type FileChange struct {
	Path      string
	TaskID    string
	OldHash   string
	NewHash   string
	Rewritten int
}

// OneOffReport resume el resultado de una operacion one-off.
type OneOffReport struct {
	Changes           []FileChange
	Rewritten         int
	HashSubstitutions int
	Ambiguous         []AmbiguousRef
	DryRun            bool
}

// OneOff define una operacion registrada.
type OneOff struct {
	ID         string
	Title      string
	Applicable func(root string) (bool, string, error)
	Run        func(ctx OneOffContext) (OneOffReport, error)
}

// oneOffRegistry indexa las operaciones one-off por el ID de la spec que las define.
var oneOffRegistry = map[string]OneOff{
	"013": oneOff013,
}

func sortedOneOffIDs() []string {
	ids := make([]string, 0, len(oneOffRegistry))
	for id := range oneOffRegistry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// cmdOneOff implementa 'seb oneoff [<id>] [--dry-run]' y devuelve el codigo de salida.
func cmdOneOff(db *sql.DB, args []string, stdout, stderr io.Writer) int {
	start := time.Now()
	id := ""
	dryRun := false
	for _, a := range args {
		switch {
		case a == "--dry-run":
			dryRun = true
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "[seb-harness ERROR] Flag no reconocido '%s'. Uso: seb oneoff [<id_spec>] [--dry-run]\n", a)
			return 1
		case id == "":
			id = strings.TrimSpace(a)
		default:
			fmt.Fprintf(stderr, "[seb-harness ERROR] Argumento inesperado '%s'. Uso: seb oneoff [<id_spec>] [--dry-run]\n", a)
			return 1
		}
	}

	// Listado sin ID: lectura pura, permitido para cualquier rol
	if id == "" {
		if dryRun {
			fmt.Fprintln(stderr, "[seb-harness ERROR] --dry-run requiere indicar el ID de la operación one-off.")
			return 1
		}
		listOneOffs(".", stdout)
		return 0
	}

	// Normalizar ID numerico a 3 digitos
	if numericIDRegex.MatchString(id) {
		var num int
		fmt.Sscanf(id, "%d", &num)
		id = fmt.Sprintf("%03d", num)
	}

	role := resolveCLIRole()
	if role != migradorRole {
		if db != nil {
			logInternalAction(db, "", role, "guard-blocked:oneoff", id, "DENIED", time.Since(start).Milliseconds(), 1)
		}
		fmt.Fprintf(stderr, "[seb-harness ERROR] GUARDARRAÍL DE MENOR PRIVILEGIO: El rol '%s' NO tiene permiso para ejecutar la operación one-off '%s'. Solo el humano puede activarla con: .seb/bin/seb role set migrador\n", role, id)
		return 1
	}

	op, ok := oneOffRegistry[id]
	if !ok {
		fmt.Fprintf(stderr, "[seb-harness ERROR] No existe una operación one-off registrada con ID '%s'. IDs disponibles: %s\n", id, strings.Join(sortedOneOffIDs(), ", "))
		return 1
	}

	ctx := OneOffContext{Root: ".", DB: db, DryRun: dryRun, Stdout: stdout, Stderr: stderr, FS: osFileStore{}}
	report, err := op.Run(ctx)
	if err != nil {
		if db != nil && !dryRun {
			logInternalAction(db, "", migradorRole, "oneoff-"+id+":error", "", err.Error(), time.Since(start).Milliseconds(), 1)
		}
		fmt.Fprintf(stderr, "[seb-harness ERROR] Operación one-off %s abortada: %v\n", id, err)
		return 1
	}
	printOneOffReport(stdout, op, report)
	if len(report.Ambiguous) > 0 {
		fmt.Fprintf(stderr, "[seb-harness ADVERTENCIA] %d referencias ambiguas requieren revisión manual.\n", len(report.Ambiguous))
	}
	return 0
}

func listOneOffs(root string, w io.Writer) {
	fmt.Fprintln(w, "| ID | Operación | Estado | Detalle |")
	fmt.Fprintln(w, "| :--- | :--- | :--- | :--- |")
	for _, id := range sortedOneOffIDs() {
		op := oneOffRegistry[id]
		state := "NO APLICABLE"
		applicable, detail, err := op.Applicable(root)
		if err != nil {
			state = "ERROR"
			detail = err.Error()
		} else if applicable {
			state = "APLICABLE"
		}
		fmt.Fprintf(w, "| %s | %s | %s | %s |\n", op.ID, op.Title, state, detail)
	}
}

func printOneOffReport(w io.Writer, op OneOff, r OneOffReport) {
	mode := "EJECUCIÓN"
	if r.DryRun {
		mode = "SIMULACIÓN (sin escrituras)"
	}
	fmt.Fprintf(w, "## Operación one-off %s: %s\n\n", op.ID, op.Title)
	fmt.Fprintf(w, "- **Modo:** %s\n", mode)
	fmt.Fprintf(w, "- **Archivos modificados:** %d\n", len(r.Changes))
	fmt.Fprintf(w, "- **Identificadores reescritos:** %d\n", r.Rewritten)
	fmt.Fprintf(w, "- **Hashes embebidos sustituidos:** %d\n", r.HashSubstitutions)
	fmt.Fprintf(w, "- **Referencias ambiguas:** %d\n", len(r.Ambiguous))
	if len(r.Changes) > 0 {
		fmt.Fprintln(w, "\n| Archivo | Identificadores | Hash previo | Hash nuevo |")
		fmt.Fprintln(w, "| :--- | :--- | :--- | :--- |")
		for _, c := range r.Changes {
			fmt.Fprintf(w, "| `%s` | %d | `%s` | `%s` |\n", c.Path, c.Rewritten, shortHash(c.OldHash), shortHash(c.NewHash))
		}
	}
	if len(r.Ambiguous) > 0 {
		fmt.Fprintln(w, "\n### Referencias ambiguas (revisión manual)")
		for _, a := range r.Ambiguous {
			fmt.Fprintf(w, "- `%s:%d`: `%s` — %s\n", a.Path, a.Line, a.ID, a.Text)
		}
	}
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
