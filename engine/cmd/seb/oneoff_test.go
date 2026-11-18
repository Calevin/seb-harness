package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// gitRun ejecuta git en dir con identidad local de prueba.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.name=seb-test", "-c", "user.email=seb-test@localhost", "-c", "commit.gpgsign=false"}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v fallo: %v\n%s", args, err, string(out))
	}
	return string(out)
}

// gitCommitAll versiona todo el contenido actual del directorio.
func gitCommitAll(t *testing.T, dir string) {
	t.Helper()
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "--allow-empty", "-m", "fixture")
}

// newGitSebProject crea un proyecto seb inicializado dentro de un repositorio Git.
func newGitSebProject(t *testing.T, bin string) string {
	t.Helper()
	dir := newSebProject(t, bin)
	gitRun(t, dir, "init", "-q")
	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".seb/seb-audit.db*\n.seb/ACTIVE_ROLE\n.seb/ACTIVE_TASK\n"), 0644)
	return dir
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func specSet(ids ...string) map[string]bool {
	m := make(map[string]bool)
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// 013-RF-20, 013-RF-21, 013-RF-22, 013-CL-05, 013-CL-06
func TestOneOff013RewriteRules(t *testing.T) {
	existing := specSet("002", "005", "011", "014")

	t.Run("prosa de spec: propios prefijados, ajenos ambiguos, bloques cercados y marcadores intactos", func(t *testing.T) {
		in := strings.Join([]string{
			"# Spec 005",
			"- **Estado:** `ABORTED`",
			"- **HU-01:** Como usuario quiero algo.",
			"- **RF-01 (Event-Driven):** CUANDO x, EL SISTEMA DEBE y.",
			"- **RNF-02:** umbral de 10ms y 100 controles.",
			"- **CL-03 (Vacios):** comportamiento; ver RF-01 y CHK-07.",
			"Sustituye CL-03 y RF-03 de la spec 002.",
			"Cada `RF-XX` debe verificarse; decision D-01; 013-RF-01 ya prefijado.",
			"```text",
			"| CHK-01 | RF-01 | ejemplo | `echo` |",
			"```",
			"",
		}, "\n")
		out, n, amb := migrateTaskArtifact([]byte(in), kindSpec, "specs/005_x_spec.md", "005", false, existing)
		got := string(out)
		for _, want := range []string{
			"- **005-HU-01:** Como usuario",
			"- **005-RF-01 (Event-Driven):**",
			"- **005-RNF-02:** umbral de 10ms y 100 controles.",
			"- **005-CL-03 (Vacios):** comportamiento; ver 005-RF-01 y 005-CHK-07.",
			"Sustituye CL-03 y RF-03 de la spec 002.",
			"Cada `RF-XX` debe verificarse; decision D-01; 013-RF-01 ya prefijado.",
			"| CHK-01 | RF-01 | ejemplo | `echo` |",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("Falta %q en salida:\n%s", want, got)
			}
		}
		if n != 6 {
			t.Errorf("Se esperaban 6 identificadores reescritos, got %d", n)
		}
		if len(amb) != 2 || amb[0].ID != "CL-03" || amb[1].ID != "RF-03" || amb[0].Line != 7 || amb[0].Path != "specs/005_x_spec.md" {
			t.Errorf("Referencias ambiguas inesperadas: %+v", amb)
		}
		if !strings.HasSuffix(got, "\n") {
			t.Error("Debe preservarse el salto de linea final")
		}
	})

	t.Run("filas de verificador: solo Check ID y Requisito, resto byte a byte", func(t *testing.T) {
		row := "| CHK-06 | RF-15, RNF-01,GLOBAL | Integridad de la Spec 011 y CHK-01 | `bash -c 'test RF-01 = 011 | grep CHK-02'` |"
		in := "# Verificador\n\n| Check ID | Requisito | Descripción | Comando |\n| :--- | :--- | :--- | :--- |\n" + row + "\n\n- Criterio: CHK-01 a CHK-99 en [PASA].\n"
		out, n, amb := migrateTaskArtifact([]byte(in), kindVerifier, "specs/014_x_verificador.md", "014", false, existing)
		got := string(out)
		wantRow := "| 014-CHK-06 | 014-RF-15, 014-RNF-01,GLOBAL | Integridad de la Spec 011 y CHK-01 | `bash -c 'test RF-01 = 011 | grep CHK-02'` |"
		if !strings.Contains(got, wantRow) {
			t.Errorf("Fila migrada incorrecta:\n%s", got)
		}
		if !strings.Contains(got, "- Criterio: 014-CHK-01 a 014-CHK-99 en [PASA].") {
			t.Errorf("Prosa del verificador debe migrarse: %s", got)
		}
		if n != 5 || len(amb) != 0 {
			t.Errorf("Se esperaban 5 reescrituras y 0 ambiguas, got %d / %+v", n, amb)
		}
	})

	t.Run("spec ya prefijada: prosa intacta, filas heredadas del verificador migradas", func(t *testing.T) {
		spec := "- **013-RF-01 (Ubiquitous):** EL SISTEMA DEBE x.\nEjemplo heredado: `RF-01`, `CHK-01`.\n"
		if !specDeclaresPrefixedIDs([]byte(spec)) {
			t.Fatal("La spec debe detectarse como prefijada")
		}
		out, n, amb := migrateTaskArtifact([]byte(spec), kindSpec, "s", "013", true, existing)
		if string(out) != spec || n != 0 || len(amb) != 0 {
			t.Errorf("Spec prefijada no debe reescribirse: %q", string(out))
		}
		plan := "Tarea T1 cubre RF-07.\n"
		outPlan, _, _ := migrateTaskArtifact([]byte(plan), kindPlan, "p", "013", true, existing)
		if string(outPlan) != plan {
			t.Errorf("Plan de spec prefijada no debe reescribirse: %q", string(outPlan))
		}
		verif := "Formato transitorio (`CHK-NN`, `RF-NN`).\n| CHK-01 | RF-07,CL-01 | d | `echo` |\n"
		outV, nV, _ := migrateTaskArtifact([]byte(verif), kindVerifier, "v", "013", true, existing)
		if string(outV) != "Formato transitorio (`CHK-NN`, `RF-NN`).\n| 013-CHK-01 | 013-RF-07,013-CL-01 | d | `echo` |\n" || nV != 3 {
			t.Errorf("Filas heredadas deben migrarse aun con spec prefijada: %q (%d)", string(outV), nV)
		}
	})

	t.Run("artefactos documentales: mencion unica prefija, ninguna o varias es ambigua", func(t *testing.T) {
		in := strings.Join([]string{
			"- En la Spec 005 se definio RF-03 y CL-01.",
			"- Spec 014: CHK-06 fija los hashes de la spec 011.",
			"- Requisito suelto RF-09 sin contexto.",
			"- `specs/011_gobernanza_afecta_uc_spec.md` RF-02",
			"- Version 999 RF-04",
			"",
		}, "\n")
		out, n, amb := migrateDocumentArtifact([]byte(in), "MEMORY.md", existing)
		got := string(out)
		if !strings.Contains(got, "005-RF-03 y 005-CL-01") || !strings.Contains(got, "`specs/011_gobernanza_afecta_uc_spec.md` 011-RF-02") {
			t.Errorf("Mencion unica debe prefijar: %s", got)
		}
		if !strings.Contains(got, "Spec 014: CHK-06") || !strings.Contains(got, "suelto RF-09") || !strings.Contains(got, "999 RF-04") {
			t.Errorf("Lineas ambiguas deben quedar intactas: %s", got)
		}
		if n != 3 || len(amb) != 3 {
			t.Errorf("Se esperaban 3 reescrituras y 3 ambiguas, got %d / %+v", n, amb)
		}
	})

	t.Run("menciones de spec ignoran numeros pegados a letras, guiones o specs inexistentes", func(t *testing.T) {
		m := mentionedSpecs("hash e26bd606 100ms 013-RF-100 CHK-005 spec 011, 2026-11-18, 005_x", existing)
		if len(m) != 2 || !m["011"] || !m["005"] {
			t.Errorf("Menciones inesperadas: %v", m)
		}
	})
}

// failingStore falla en la escritura numero failAt (1-based) y delega el resto al sistema de archivos.
type failingStore struct {
	failAt int
	writes int
}

func (f *failingStore) ReadFile(p string) ([]byte, error) { return os.ReadFile(p) }
func (f *failingStore) WriteFile(p string, data []byte) error {
	f.writes++
	if f.writes == f.failAt {
		return errors.New("fallo de escritura simulado")
	}
	return osFileStore{}.WriteFile(p, data)
}

// buildMigrationFixture crea un repositorio Git con tareas heredadas selladas.
func buildMigrationFixture(t *testing.T) (string, map[string]string) {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	specs := filepath.Join(dir, "specs")
	files := map[string]string{
		"specs/001_alfa_spec.md":        "# Spec 001\n- **Estado:** `APROBADA`\n- **RF-01 (Ubiquitous):** EL SISTEMA DEBE a.\n",
		"specs/001_alfa_plan.md":        "# Plan\n- T1 cubre RF-01 (CHK-01).\n",
		"specs/001_alfa_verificador.md": "| Check ID | Requisito | Descripción | Comando |\n| :--- | :--- | :--- | :--- |\n| CHK-01 | RF-01 | a | `echo a` |\n",
		"specs/002_beta_spec.md":        "# Spec 002\n- **Estado:** `ABORTED`\n- **RF-01 (Ubiquitous):** EL SISTEMA DEBE b; ver RF-01 de la spec 001.\n",
		"specs/002_beta_verificador.md": "| Check ID | Requisito | Descripción | Comando |\n| :--- | :--- | :--- | :--- |\n| CHK-01 | RF-01 | b | `echo b` |\n",
		"docs/use-cases/UC-a_b.md":      "# UC\n- Spec 001: RF-01 cubre el flujo.\n",
		"MEMORY.md":                     "# Memoria\n- Spec 002 y 001: CHK-01 ambiguo.\n",
	}
	for rel, c := range files {
		mustWrite(t, filepath.Join(dir, rel), c)
	}
	spec1 := sha256Hex([]byte(files["specs/001_alfa_spec.md"]))
	verif1 := sha256Hex([]byte(files["specs/001_alfa_verificador.md"]))
	// La tarea 003 embebe en su comando los hashes sellados de la 001 (patron de la Spec 014)
	mustWrite(t, filepath.Join(specs, "003_gamma_spec.md"), "# Spec 003\n- **RF-01 (Ubiquitous):** EL SISTEMA DEBE c.\n")
	mustWrite(t, filepath.Join(specs, "003_gamma_verificador.md"),
		"| Check ID | Requisito | Descripción | Comando |\n| :--- | :--- | :--- | :--- |\n| CHK-01 | RF-01 | sello 001 | `bash -c 'test \"$(sha256sum specs/001_alfa_spec.md | cut -c1-64)\" = "+spec1+" && test \"$(sha256sum specs/001_alfa_verificador.md | cut -c1-64)\" = "+verif1+"'` |\n")

	lock := SebLock{Tasks: map[string]TaskLock{}}
	seal := func(canonical string, verified, aborted bool, commit string) {
		sh := sha256Hex([]byte(mustRead(t, filepath.Join(specs, canonical+"_spec.md"))))
		vh := sha256Hex([]byte(mustRead(t, filepath.Join(specs, canonical+"_verificador.md"))))
		lock.Tasks[canonical] = TaskLock{SpecHash: sh, VerifierHash: vh, Verified: verified, CommitHash: commit, Aborted: aborted}
	}
	seal("001_alfa", true, false, "aaa1111")
	seal("002_beta", false, true, "")
	seal("003_gamma", true, false, "ccc3333")
	b, _ := json.MarshalIndent(lock, "", "  ")
	mustWrite(t, filepath.Join(dir, ".seb", "seb.lock"), string(b))
	gitCommitAll(t, dir)
	return dir, map[string]string{"spec1": spec1, "verif1": verif1}
}

func snapshotTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	snap := make(map[string]string)
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || strings.Contains(p, string(filepath.Separator)+".git"+string(filepath.Separator)) {
			return nil
		}
		snap[p] = mustRead(t, p)
		return nil
	})
	return snap
}

// 013-RF-23, 013-RF-24, 013-RF-25, 013-RF-26, 013-RF-27, 013-RF-28, 013-RF-29, 013-RNF-01, 013-CL-04
func TestOneOff013TransactionalMigration(t *testing.T) {
	t.Run("dry-run no escribe archivos ni auditoria", func(t *testing.T) {
		dir, _ := buildMigrationFixture(t)
		db := setupTestDB(t)
		defer db.Close()
		before := snapshotTree(t, dir)
		rep, err := runOneOff013(OneOffContext{Root: dir, DB: db, DryRun: true, FS: osFileStore{}})
		if err != nil {
			t.Fatalf("dry-run fallo: %v", err)
		}
		if len(rep.Changes) == 0 || !rep.DryRun {
			t.Fatalf("dry-run debe reportar cambios: %+v", rep)
		}
		after := snapshotTree(t, dir)
		for p, c := range before {
			if after[p] != c {
				t.Errorf("dry-run modifico %s", p)
			}
		}
		var n int
		_ = db.QueryRow(`SELECT COUNT(*) FROM agent_actions WHERE skill = 'migrador'`).Scan(&n)
		if n != 0 {
			t.Errorf("dry-run no debe auditar, got %d entradas", n)
		}
	})

	t.Run("ejecucion: resellado, hashes embebidos, auditoria e idempotencia", func(t *testing.T) {
		dir, old := buildMigrationFixture(t)
		db := setupTestDB(t)
		defer db.Close()
		rep, err := runOneOff013(OneOffContext{Root: dir, DB: db, FS: osFileStore{}})
		if err != nil {
			t.Fatalf("migracion fallo: %v", err)
		}
		specs := filepath.Join(dir, "specs")

		// Verificadores migrados y validos con el parser estricto
		for _, v := range []string{"001_alfa", "002_beta", "003_gamma"} {
			if _, err := parseVerifierFile(filepath.Join(specs, v+"_verificador.md")); err != nil {
				t.Errorf("Verificador %s invalido tras migrar: %v", v, err)
			}
		}
		if !strings.Contains(mustRead(t, filepath.Join(specs, "001_alfa_spec.md")), "- **001-RF-01 (Ubiquitous):**") {
			t.Error("La spec 001 debe quedar prefijada")
		}
		if !strings.Contains(mustRead(t, filepath.Join(specs, "001_alfa_plan.md")), "001-RF-01 (001-CHK-01)") {
			t.Error("El plan 001 debe quedar prefijado")
		}
		if !strings.Contains(mustRead(t, filepath.Join(dir, "docs", "use-cases", "UC-a_b.md")), "Spec 001: 001-RF-01") {
			t.Error("El caso de uso con mencion unica debe prefijarse")
		}
		// Tarea abortada (013-CL-05) tambien migrada
		if !strings.Contains(mustRead(t, filepath.Join(specs, "002_beta_verificador.md")), "| 002-CHK-01 | 002-RF-01 |") {
			t.Error("La tarea abortada debe migrarse")
		}

		// Resellado preservando verified, commit_hash y aborted (013-RF-23)
		var lock SebLock
		if err := json.Unmarshal([]byte(mustRead(t, filepath.Join(dir, ".seb", "seb.lock"))), &lock); err != nil {
			t.Fatalf("seb.lock invalido: %v", err)
		}
		for canonical, tl := range lock.Tasks {
			if tl.SpecHash != sha256Hex([]byte(mustRead(t, filepath.Join(specs, canonical+"_spec.md")))) {
				t.Errorf("spec_hash de %s no coincide con disco", canonical)
			}
			if tl.VerifierHash != sha256Hex([]byte(mustRead(t, filepath.Join(specs, canonical+"_verificador.md")))) {
				t.Errorf("verifier_hash de %s no coincide con disco", canonical)
			}
		}
		if l := lock.Tasks["001_alfa"]; !l.Verified || l.CommitHash != "aaa1111" || l.Aborted {
			t.Errorf("001 perdio su estado: %+v", l)
		}
		if l := lock.Tasks["002_beta"]; l.Verified || !l.Aborted || l.CommitHash != "" {
			t.Errorf("002 perdio su estado: %+v", l)
		}

		// Hashes embebidos sustituidos (013-RF-27)
		v3 := mustRead(t, filepath.Join(specs, "003_gamma_verificador.md"))
		if strings.Contains(v3, old["spec1"]) || strings.Contains(v3, old["verif1"]) {
			t.Error("El verificador 003 conserva hashes previos")
		}
		if !strings.Contains(v3, lock.Tasks["001_alfa"].SpecHash) || !strings.Contains(v3, lock.Tasks["001_alfa"].VerifierHash) {
			t.Error("El verificador 003 debe contener los hashes nuevos de la 001")
		}
		if rep.HashSubstitutions != 2 {
			t.Errorf("Se esperaban 2 sustituciones de hash, got %d", rep.HashSubstitutions)
		}
		// Auditoria por archivo (013-RF-24)
		var rewrites, summaries int
		_ = db.QueryRow(`SELECT COUNT(*) FROM agent_actions WHERE skill = 'migrador' AND action = 'oneoff-013:rewrite'`).Scan(&rewrites)
		_ = db.QueryRow(`SELECT COUNT(*) FROM agent_actions WHERE skill = 'migrador' AND action = 'oneoff-013:summary'`).Scan(&summaries)
		if rewrites != len(rep.Changes) || summaries != 1 {
			t.Errorf("Auditoria inesperada: rewrites=%d changes=%d summaries=%d", rewrites, len(rep.Changes), summaries)
		}
		var params, result string
		_ = db.QueryRow(`SELECT parameters, result_summary FROM agent_actions WHERE action = 'oneoff-013:rewrite' AND parameters = 'specs/001_alfa_spec.md'`).Scan(&params, &result)
		if !regexp.MustCompile(`^[0-9a-f]{64}->[0-9a-f]{64}$`).MatchString(result) {
			t.Errorf("La auditoria debe registrar hash previo y nuevo: %q", result)
		}

		// Ambiguas reportadas (spec 002 menciona la 001; MEMORY menciona dos specs)
		if len(rep.Ambiguous) < 2 {
			t.Errorf("Se esperaban referencias ambiguas, got %+v", rep.Ambiguous)
		}

		// Idempotencia (013-RNF-01)
		gitCommitAll(t, dir)
		before := snapshotTree(t, dir)
		rep2, err := runOneOff013(OneOffContext{Root: dir, DB: db, FS: osFileStore{}})
		if err != nil {
			t.Fatalf("segunda ejecucion fallo: %v", err)
		}
		if len(rep2.Changes) != 0 {
			t.Errorf("La segunda ejecucion no debe modificar archivos: %+v", rep2.Changes)
		}
		after := snapshotTree(t, dir)
		for p, c := range before {
			if after[p] != c {
				t.Errorf("La segunda ejecucion modifico %s", p)
			}
		}
		var rewrites2 int
		_ = db.QueryRow(`SELECT COUNT(*) FROM agent_actions WHERE action = 'oneoff-013:rewrite'`).Scan(&rewrites2)
		if rewrites2 != rewrites {
			t.Errorf("La segunda ejecucion no debe auditar modificaciones: %d -> %d", rewrites, rewrites2)
		}
	})

	t.Run("hashes embebidos sustituidos validan con sha256sum real", func(t *testing.T) {
		dir, _ := buildMigrationFixture(t)
		if _, err := runOneOff013(OneOffContext{Root: dir, FS: osFileStore{}}); err != nil {
			t.Fatalf("migracion fallo: %v", err)
		}
		items, err := parseVerifierFile(filepath.Join(dir, "specs", "003_gamma_verificador.md"))
		if err != nil || len(items) != 1 {
			t.Fatalf("verificador 003 invalido: %v", err)
		}
		c := exec.Command("bash", "-c", items[0].Command)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Errorf("El comando con hashes sustituidos debe pasar: %v %s", err, string(out))
		}
	})

	t.Run("precondicion Git: cambios sin commitear o repositorio inexistente abortan sin escribir", func(t *testing.T) {
		dir, _ := buildMigrationFixture(t)
		mustWrite(t, filepath.Join(dir, "MEMORY.md"), "# Memoria editada\n- RF-01 de la Spec 001\n")
		before := snapshotTree(t, dir)
		if _, err := runOneOff013(OneOffContext{Root: dir, FS: osFileStore{}}); err == nil || !strings.Contains(err.Error(), "sin commitear") {
			t.Fatalf("Se esperaba aborto por cambios sin commitear, got %v", err)
		}
		after := snapshotTree(t, dir)
		for p, c := range before {
			if after[p] != c {
				t.Errorf("El aborto modifico %s", p)
			}
		}

		noGit := t.TempDir()
		mustWrite(t, filepath.Join(noGit, "specs", "001_a_spec.md"), "- **RF-01 (X):** y\n")
		if _, err := runOneOff013(OneOffContext{Root: noGit, FS: osFileStore{}}); err == nil || !strings.Contains(err.Error(), "Git") {
			t.Fatalf("Se esperaba aborto fuera de un repositorio Git, got %v", err)
		}
	})

	t.Run("rollback ante fallo de escritura", func(t *testing.T) {
		for _, failAt := range []int{1, 3, 8} {
			dir, _ := buildMigrationFixture(t)
			before := snapshotTree(t, dir)
			fs := &failingStore{failAt: failAt}
			_, err := runOneOff013(OneOffContext{Root: dir, FS: fs})
			if err == nil {
				t.Fatalf("failAt=%d: se esperaba error de escritura", failAt)
			}
			after := snapshotTree(t, dir)
			for p, c := range before {
				if after[p] != c {
					t.Errorf("failAt=%d: %s no fue restaurado", failAt, p)
				}
			}
		}
	})

	t.Run("proyecto sin specs o ya migrado termina sin cambios", func(t *testing.T) {
		dir := t.TempDir()
		gitRun(t, dir, "init", "-q")
		mustWrite(t, filepath.Join(dir, "README.md"), "x\n")
		gitCommitAll(t, dir)
		rep, err := runOneOff013(OneOffContext{Root: dir, FS: osFileStore{}})
		if err != nil || len(rep.Changes) != 0 {
			t.Fatalf("Proyecto vacio: err=%v changes=%+v", err, rep.Changes)
		}
	})

	t.Run("CLI: ambiguas producen exit 0 con advertencia en stderr", func(t *testing.T) {
		bin := buildSebForTest(t)
		dir := newGitSebProject(t, bin)
		specs := filepath.Join(dir, "specs")
		writeSpecStub(t, specs, "001", "alfa")
		writeSpecStub(t, specs, "002", "beta")
		mustWrite(t, filepath.Join(specs, "002_beta_plan.md"), "Depende de RF-04 de la spec 001.\n")
		writeVerifierFixture(t, specs, "002", "beta", "| CHK-01 | RF-01 | b | `echo b` |\n")
		gitCommitAll(t, dir)
		code, stdout, stderr := runSeb(t, bin, dir, []string{"SEB_SKILL=migrador"}, "oneoff", "013")
		if code != 0 {
			t.Fatalf("Se esperaba exit 0: %d %s %s", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "ADVERTENCIA") || !strings.Contains(stderr, "1 referencias ambiguas") {
			t.Errorf("Se esperaba advertencia de ambiguas en stderr: %q", stderr)
		}
		if !strings.Contains(stdout, "specs/002_beta_plan.md:1") {
			t.Errorf("El reporte debe listar archivo y linea de la ambigua: %q", stdout)
		}
		if !bytes.Contains([]byte(mustRead(t, filepath.Join(specs, "002_beta_verificador.md"))), []byte("| 002-CHK-01 | 002-RF-01 |")) {
			t.Error("El verificador debio migrarse")
		}
	})
}

// legacyRowsOf extrae (Check ID, comando) de las filas heredadas de un verificador previo a la migracion.
func legacyRowsOf(t *testing.T, content string) [][2]string {
	t.Helper()
	var rows [][2]string
	inFence := false
	for _, line := range strings.Split(content, "\n") {
		tl := strings.TrimSpace(line)
		if strings.HasPrefix(tl, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		cell, ok := firstTableCell(tl)
		if !ok || !strings.HasPrefix(cell, "CHK-") {
			continue
		}
		pipes := tablePipePositions(tl)
		if len(pipes) < 5 {
			t.Fatalf("Fila heredada malformada: %s", tl)
		}
		cmd := strings.TrimSpace(tl[pipes[3]+1 : pipes[4]])
		rows = append(rows, [2]string{cell, strings.Trim(cmd, "`")})
	}
	return rows
}

var anyHashRegex = regexp.MustCompile(`[0-9a-f]{64}`)

// 013-RNF-02: equivalencia funcional de la migracion contra el snapshot pre-migracion f3f5945
func TestOneOff013RealRepositoryEquivalence(t *testing.T) {
	const snapshotCommit = "f3f5945"
	root := repoRootDir(t)
	snap := t.TempDir()

	archive := exec.Command("bash", "-c", "git -C \"$1\" archive \"$2\" specs docs MEMORY.md .seb/seb.lock | tar -x -C \"$3\"", "_", root, snapshotCommit, snap)
	if out, err := archive.CombinedOutput(); err != nil {
		t.Fatalf("No se pudo extraer el snapshot %s: %v\n%s", snapshotCommit, err, string(out))
	}
	gitRun(t, snap, "init", "-q")
	gitCommitAll(t, snap)

	var snapLock SebLock
	if err := json.Unmarshal([]byte(mustRead(t, filepath.Join(snap, ".seb", "seb.lock"))), &snapLock); err != nil {
		t.Fatalf("seb.lock del snapshot invalido: %v", err)
	}
	verifiers, _ := filepath.Glob(filepath.Join(snap, "specs", "*_verificador.md"))
	if len(verifiers) < 13 {
		t.Fatalf("Se esperaban al menos 13 verificadores historicos en el snapshot, got %d", len(verifiers))
	}
	before := make(map[string][][2]string)
	for _, v := range verifiers {
		before[filepath.Base(v)] = legacyRowsOf(t, mustRead(t, v))
	}

	rep, err := runOneOff013(OneOffContext{Root: snap, FS: osFileStore{}})
	if err != nil {
		t.Fatalf("La migracion del snapshot fallo: %v", err)
	}
	hashMap := make(map[string]string)
	for _, c := range rep.Changes {
		hashMap[c.OldHash] = c.NewHash
	}

	for _, v := range verifiers {
		name := filepath.Base(v)
		items, err := parseVerifierFile(v)
		if err != nil {
			t.Errorf("%s invalido tras migrar el snapshot: %v", name, err)
			continue
		}
		prev := before[name]
		if len(items) != len(prev) {
			t.Errorf("%s: cantidad de controles cambio %d -> %d", name, len(prev), len(items))
			continue
		}
		taskID := name[:3]
		for i, it := range items {
			if it.ID != taskID+"-"+prev[i][0] {
				t.Errorf("%s: Check ID %s debio migrar a %s-%s", name, it.ID, taskID, prev[i][0])
			}
			expected := anyHashRegex.ReplaceAllStringFunc(prev[i][1], func(h string) string {
				if nh, ok := hashMap[h]; ok {
					return nh
				}
				return h
			})
			if it.Command != expected {
				t.Errorf("%s %s: el comando cambio mas alla de la sustitucion de hashes:\n antes: %s\n despues: %s", name, it.ID, prev[i][1], it.Command)
			}
		}
	}

	// Hashes embebidos de la Spec 014 iguales a los artefactos migrados de la 011 (013-RF-27)
	checkEmbedded011 := func(base string) {
		v014 := mustRead(t, filepath.Join(base, "specs", "014_afecta_uc_multiple_verificador.md"))
		for _, f := range []string{"011_gobernanza_afecta_uc_spec.md", "011_gobernanza_afecta_uc_verificador.md"} {
			h := sha256Hex([]byte(mustRead(t, filepath.Join(base, "specs", f))))
			if !strings.Contains(v014, h) {
				t.Errorf("[%s] El verificador 014 debe embeber el hash actual de %s (%s)", base, f, h)
			}
		}
	}
	checkEmbedded011(snap)

	// Estado real del repositorio: debe coincidir con la migracion determinista del snapshot
	normalize := func(s string) string { return anyHashRegex.ReplaceAllString(s, "<HASH>") }
	for _, v := range verifiers {
		name := filepath.Base(v)
		snapItems, _ := parseVerifierFile(v)
		rootItems, err := parseVerifierFile(filepath.Join(root, "specs", name))
		if err != nil {
			t.Errorf("El repositorio no esta migrado: %s: %v", name, err)
			continue
		}
		if len(rootItems) != len(snapItems) {
			t.Errorf("%s: el repositorio tiene %d controles, se esperaban %d", name, len(rootItems), len(snapItems))
			continue
		}
		for i := range rootItems {
			a, b := rootItems[i], snapItems[i]
			if a.ID != b.ID || a.ReqID != b.ReqID || a.Desc != b.Desc || normalize(a.Command) != normalize(b.Command) {
				t.Errorf("%s: control %d difiere de la migracion determinista: %+v vs %+v", name, i, a, b)
			}
		}
	}
	checkEmbedded011(root)

	var rootLock SebLock
	if err := json.Unmarshal([]byte(mustRead(t, filepath.Join(root, ".seb", "seb.lock"))), &rootLock); err != nil {
		t.Fatalf("seb.lock del repositorio invalido: %v", err)
	}
	for canonical, prevEntry := range snapLock.Tasks {
		cur, ok := rootLock.Tasks[canonical]
		if !ok {
			t.Errorf("La tarea %s desaparecio de seb.lock", canonical)
			continue
		}
		if prevEntry.Verified && (!cur.Verified || strings.TrimSpace(cur.CommitHash) == "") {
			t.Errorf("La tarea %s perdio verified/commit_hash", canonical)
		}
		if cur.SpecHash != sha256Hex([]byte(mustRead(t, filepath.Join(root, "specs", canonical+"_spec.md")))) {
			t.Errorf("spec_hash de %s no coincide con el disco", canonical)
		}
		if cur.VerifierHash != sha256Hex([]byte(mustRead(t, filepath.Join(root, "specs", canonical+"_verificador.md")))) {
			t.Errorf("verifier_hash de %s no coincide con el disco", canonical)
		}
	}
}
