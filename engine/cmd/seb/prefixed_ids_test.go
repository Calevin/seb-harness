package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeSpecStub crea una spec minima <id>_<name>_spec.md en dir.
func writeSpecStub(t *testing.T, dir, id, name string) {
	t.Helper()
	p := filepath.Join(dir, id+"_"+name+"_spec.md")
	if err := os.WriteFile(p, []byte("# Spec "+id+"\n- **Tarea ID:** `"+id+"`\n- **Estado:** `APROBADA`\n"), 0644); err != nil {
		t.Fatalf("Error creando spec stub: %v", err)
	}
}

// writeVerifierFixture crea un verificador <id>_<name>_verificador.md con el cuerpo de tabla indicado.
func writeVerifierFixture(t *testing.T, dir, id, name, rows string) string {
	t.Helper()
	p := filepath.Join(dir, id+"_"+name+"_verificador.md")
	content := "# Verificador\n\n| Check ID | Requisito | Descripción | Comando |\n| :--- | :--- | :--- | :--- |\n" + rows
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("Error creando verificador: %v", err)
	}
	return p
}

// 013-RF-07, 013-RF-08, 013-RF-10, 013-RF-11, 013-RF-12, 013-CL-01, 013-CL-02, 013-CL-03, 013-CL-09
func TestPrefixedVerifierParsing(t *testing.T) {
	t.Run("acepta Check ID prefijado, referencias cruzadas, GLOBAL y sufijos flexibles", func(t *testing.T) {
		dir := t.TempDir()
		writeSpecStub(t, dir, "013", "tarea")
		writeSpecStub(t, dir, "005", "otra")
		p := writeVerifierFixture(t, dir, "013", "tarea",
			"| 013-CHK-01 | 013-RF-01, 005-RF-03 | Referencia cruzada | `echo a` |\n"+
				"| 013-CHK-SMOKE | 013-RF-100,013-CL-02 | Sufijos largos | `echo b` |\n"+
				"| 013-CHK-99 | GLOBAL | Global | `echo c` |\n"+
				"| 013-CHK-02 | 013-HU-01,013-RNF-01,013-CHK-01 | Todos los tipos | `echo d` |\n")
		items, err := parseVerifierFile(p)
		if err != nil {
			t.Fatalf("No se esperaba error: %v", err)
		}
		if len(items) != 4 {
			t.Fatalf("Se esperaban 4 controles, got %d", len(items))
		}
		if items[0].ID != "013-CHK-01" || items[0].ReqID != "013-RF-01,005-RF-03" {
			t.Errorf("Item 0 incorrecto: %+v", items[0])
		}
		if items[1].ID != "013-CHK-SMOKE" || items[1].ReqID != "013-RF-100,013-CL-02" {
			t.Errorf("Item 1 incorrecto: %+v", items[1])
		}
		if items[2].ReqID != "GLOBAL" {
			t.Errorf("GLOBAL debe aceptarse sin prefijo: %+v", items[2])
		}
	})

	t.Run("rechaza Check ID heredado con mensaje de migracion", func(t *testing.T) {
		dir := t.TempDir()
		writeSpecStub(t, dir, "013", "tarea")
		p := writeVerifierFixture(t, dir, "013", "tarea", "| CHK-01 | 013-RF-01 | Heredado | `echo a` |\n")
		_, err := parseVerifierFile(p)
		if err == nil {
			t.Fatal("Se esperaba rechazo del Check ID heredado")
		}
		if !errors.Is(err, errLegacyCheckID) {
			t.Errorf("El error debe envolver errLegacyCheckID: %v", err)
		}
		if !strings.Contains(err.Error(), migrationHintCommand) || !strings.Contains(err.Error(), "línea 5") {
			t.Errorf("El error debe indicar linea y comando de migracion: %v", err)
		}
	})

	t.Run("rechaza prefijo distinto a la tarea verificada", func(t *testing.T) {
		dir := t.TempDir()
		writeSpecStub(t, dir, "013", "tarea")
		writeSpecStub(t, dir, "012", "otra")
		p := writeVerifierFixture(t, dir, "013", "tarea", "| 012-CHK-01 | 013-RF-01 | Prefijo ajeno | `echo a` |\n")
		_, err := parseVerifierFile(p)
		if err == nil || !strings.Contains(err.Error(), "incoherente") || !strings.Contains(err.Error(), "012-CHK-01") {
			t.Fatalf("Se esperaba error de prefijo incoherente con el identificador, got %v", err)
		}
	})

	t.Run("rechaza prefijo malformado con apariencia de control", func(t *testing.T) {
		malformed := []string{"13-CHK-01", "0013-CHK-01", "abc-CHK-01", "013-CHK-", "013-CHK-0 1"}
		for _, id := range malformed {
			dir := t.TempDir()
			writeSpecStub(t, dir, "013", "tarea")
			p := writeVerifierFixture(t, dir, "013", "tarea", "| "+id+" | 013-RF-01 | Malformado | `echo a` |\n")
			if _, err := parseVerifierFile(p); err == nil {
				t.Errorf("Se esperaba rechazo (Anti-Silent-Drop) para '%s'", id)
			}
		}
	})

	t.Run("rechaza referencias de requisito invalidas", func(t *testing.T) {
		invalid := []string{"RF-01", "13-RF-01", "013-XX-01", "999-RF-01", "013-RF-", "global"}
		for _, ref := range invalid {
			dir := t.TempDir()
			writeSpecStub(t, dir, "013", "tarea")
			p := writeVerifierFixture(t, dir, "013", "tarea", "| 013-CHK-01 | "+ref+" | Referencia | `echo a` |\n")
			_, err := parseVerifierFile(p)
			if err == nil {
				t.Errorf("Se esperaba rechazo de la referencia '%s'", ref)
				continue
			}
			if !strings.Contains(err.Error(), "línea 5") || !strings.Contains(err.Error(), ref) {
				t.Errorf("El error debe indicar linea y referencia '%s': %v", ref, err)
			}
		}
	})

	t.Run("referencia a spec purgada es invalida", func(t *testing.T) {
		dir := t.TempDir()
		writeSpecStub(t, dir, "013", "tarea")
		// La spec 004 fue purgada: no existe archivo en disco
		p := writeVerifierFixture(t, dir, "013", "tarea", "| 013-CHK-01 | 004-RF-01 | Purgada | `echo a` |\n")
		_, err := parseVerifierFile(p)
		if err == nil || !strings.Contains(err.Error(), "004") {
			t.Fatalf("Se esperaba rechazo por spec inexistente, got %v", err)
		}
	})

	t.Run("bloques cercados ignorados en ambos formatos", func(t *testing.T) {
		dir := t.TempDir()
		writeSpecStub(t, dir, "013", "tarea")
		rows := "```markdown\n| CHK-01 | RF-01 | Ejemplo heredado | `exit 1` |\n| 099-CHK-01 | 099-RF-01 | Ejemplo ajeno | `exit 1` |\n```\n" +
			"| 013-CHK-01 | 013-RF-01 | Real | `echo real` |\n"
		p := writeVerifierFixture(t, dir, "013", "tarea", rows)
		items, err := parseVerifierFile(p)
		if err != nil {
			t.Fatalf("No se esperaba error: %v", err)
		}
		if len(items) != 1 || items[0].ID != "013-CHK-01" {
			t.Fatalf("Se esperaba solo el control real, got %+v", items)
		}
	})

	t.Run("filas no candidatas se ignoran", func(t *testing.T) {
		dir := t.TempDir()
		writeSpecStub(t, dir, "013", "tarea")
		rows := "| 013-CHK-01 | 013-RF-01 | Real | `echo real` |\n\n| Version | Fecha |\n| 1.0-1 | 2026-11-18 |\n"
		p := writeVerifierFixture(t, dir, "013", "tarea", rows)
		items, err := parseVerifierFile(p)
		if err != nil || len(items) != 1 {
			t.Fatalf("Se esperaba 1 control sin error, got %d items, err=%v", len(items), err)
		}
	})

	t.Run("nombre de verificador sin ID de tarea", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "verificador.md")
		_ = os.WriteFile(p, []byte("| 013-CHK-01 | GLOBAL | x | `echo` |\n"), 0644)
		if _, err := parseVerifierFile(p); err == nil {
			t.Fatal("Se esperaba error al no poder derivar el ID de tarea")
		}
	})
}

// buildSebForTest compila el binario seb en un directorio temporal y devuelve su ruta.
func buildSebForTest(t *testing.T) string {
	t.Helper()
	root := repoRootDir(t)
	bin := filepath.Join(t.TempDir(), "seb")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/seb")
	cmd.Dir = filepath.Join(root, "engine")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Error compilando seb: %v\n%s", err, string(out))
	}
	return bin
}

// runSeb ejecuta el binario en dir y devuelve exit code, stdout y stderr.
func runSeb(t *testing.T, bin, dir string, env []string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(filteredEnv("SEB_SKILL"), env...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("Error ejecutando seb %v: %v", args, err)
		}
	}
	return code, stdout.String(), stderr.String()
}

// filteredEnv devuelve el entorno actual sin las variables indicadas.
func filteredEnv(drop ...string) []string {
	var out []string
	for _, kv := range os.Environ() {
		keep := true
		for _, d := range drop {
			if strings.HasPrefix(kv, d+"=") {
				keep = false
			}
		}
		if keep {
			out = append(out, kv)
		}
	}
	return out
}

// newSebProject crea un proyecto temporal inicializado con seb init.
func newSebProject(t *testing.T, bin string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "specs"), 0755); err != nil {
		t.Fatalf("Error creando specs: %v", err)
	}
	if code, _, stderr := runSeb(t, bin, dir, nil, "init"); code != 0 {
		t.Fatalf("seb init fallo: %s", stderr)
	}
	return dir
}

// 013-RF-09, 013-RF-13, 013-CL-07, 013-CL-08
func TestPrefixedVerifyAndPlanLockCLI(t *testing.T) {
	bin := buildSebForTest(t)

	t.Run("verificador heredado rechazado por plan-lock y verify con comando de migracion", func(t *testing.T) {
		dir := newSebProject(t, bin)
		specs := filepath.Join(dir, "specs")
		writeSpecStub(t, specs, "001", "tarea")
		writeVerifierFixture(t, specs, "001", "tarea", "| CHK-01 | RF-01 | Heredado | `echo ok` |\n")
		for _, sub := range []string{"plan-lock", "verify"} {
			code, _, stderr := runSeb(t, bin, dir, nil, sub, "001")
			if code != 1 {
				t.Errorf("%s debio salir con 1, got %d", sub, code)
			}
			if !strings.Contains(stderr, migrationHintCommand) || !strings.Contains(stderr, "línea 5") || !strings.Contains(stderr, "CHK-01") {
				t.Errorf("%s: stderr debe indicar linea, identificador y comando de migracion: %q", sub, stderr)
			}
		}
	})

	t.Run("verificador prefijado aceptado aunque la spec contenga menciones heredadas", func(t *testing.T) {
		dir := newSebProject(t, bin)
		specs := filepath.Join(dir, "specs")
		specContent := "# Spec 001\n- **Tarea ID:** `001`\n- **Estado:** `APROBADA`\n\nEjemplo historico: RF-05 y CHK-02 sin prefijo.\n"
		if err := os.WriteFile(filepath.Join(specs, "001_tarea_spec.md"), []byte(specContent), 0644); err != nil {
			t.Fatalf("Error escribiendo spec: %v", err)
		}
		writeVerifierFixture(t, specs, "001", "tarea", "| 001-CHK-01 | 001-RF-01 | Prefijado | `echo ok` |\n")
		if code, _, stderr := runSeb(t, bin, dir, nil, "plan-lock", "001"); code != 0 {
			t.Fatalf("plan-lock debio pasar: %d %s", code, stderr)
		}
		if code, stdout, stderr := runSeb(t, bin, dir, nil, "verify", "001"); code != 0 || !strings.Contains(stdout, "001-CHK-01") {
			t.Fatalf("verify debio aprobar: %d %s %s", code, stdout, stderr)
		}
	})

	t.Run("prefijo incoherente rechazado de forma identica en ambos subcomandos", func(t *testing.T) {
		dir := newSebProject(t, bin)
		specs := filepath.Join(dir, "specs")
		writeSpecStub(t, specs, "001", "tarea")
		writeSpecStub(t, specs, "002", "otra")
		writeVerifierFixture(t, specs, "001", "tarea", "| 002-CHK-01 | 001-RF-01 | Ajeno | `echo ok` |\n")
		var msgs []string
		for _, sub := range []string{"plan-lock", "verify"} {
			code, _, stderr := runSeb(t, bin, dir, nil, sub, "001")
			if code != 1 || !strings.Contains(stderr, "incoherente") || !strings.Contains(stderr, "002-CHK-01") {
				t.Errorf("%s debio rechazar el prefijo incoherente: %d %q", sub, code, stderr)
			}
			if idx := strings.Index(stderr, "línea"); idx >= 0 {
				msgs = append(msgs, stderr[idx:])
			}
		}
		if len(msgs) == 2 && msgs[0] != msgs[1] {
			t.Errorf("Los diagnosticos de plan-lock y verify deben coincidir: %q vs %q", msgs[0], msgs[1])
		}
	})
}
