package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// repoRootFromEngine devuelve la ruta raiz del repositorio seb-harness desde engine/cmd/seb.
func repoRootFromEngine(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("error obteniendo cwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

// buildTestSebBin compila el binario seb actual dentro de un directorio temporal para pruebas CLI.
func buildTestSebBin(t *testing.T) string {
	t.Helper()
	root := repoRootFromEngine(t)
	engineDir := filepath.Join(root, "engine")
	binPath := filepath.Join(t.TempDir(), "seb")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/seb")
	cmd.Dir = engineDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("error compilando binario seb de prueba: %v\n%s", err, string(out))
	}
	return binPath
}

// initTempGitRepo inicializa un repositorio Git limpio con un commit inicial.
func initTempGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v fallo: %v\n%s", args, err, string(out))
		}
	}
	runGit("init")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Repo de prueba\n"), 0644); err != nil {
		t.Fatalf("error escribiendo README.md: %v", err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "chore: commit inicial")
}

// countLinesFast cuenta la cantidad de lineas de un archivo Markdown.
func countLinesFast(data []byte) int {
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "\n") + 1
}

// extractFastManagedBlockLines extrae las lineas del bloque gestionado de AGENTS.md.
func extractFastManagedBlockLines(content string) []string {
	lines := strings.Split(content, "\n")
	inBlock := false
	var out []string
	for _, l := range lines {
		tr := strings.TrimSpace(l)
		if tr == "<!-- BEGIN seb-harness:managed -->" {
			inBlock = true
		}
		if inBlock {
			out = append(out, l)
		}
		if tr == "<!-- END seb-harness:managed -->" {
			inBlock = false
		}
	}
	return out
}

// TestFastTaskSkillTemplatesAndRouting valida 018-RF-01, 018-RF-04, 018-RF-05 y 018-RNF-01.
func TestFastTaskSkillTemplatesAndRouting(t *testing.T) {
	root := repoRootFromEngine(t)

	// 1. No debe existir .agents/SEB-FAST.md duplicado
	if _, err := os.Stat(filepath.Join(root, "payload", ".agents", "SEB-FAST.md")); !os.IsNotExist(err) {
		t.Fatalf("no debe existir payload/.agents/SEB-FAST.md; la guia debe vivir en seb-fast-task/SKILL.md")
	}

	// 2. Skill seb-fast-task debe existir con criterios y prohibicion de activacion implicita
	skillPath := filepath.Join(root, "payload", ".agents", "skills", "seb-fast-task", "SKILL.md")
	skillBytes, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("no se encontro %s: %v", skillPath, err)
	}
	skillText := string(skillBytes)
	for _, mustContain := range []string{"seb-fast-task", "plan_y_verificador.md", "seb spec-next", "seb plan-lock", "implementador", "Never start a `FAST` task implicitly"} {
		if !strings.Contains(skillText, mustContain) {
			t.Errorf("SKILL.md de seb-fast-task debe contener %q", mustContain)
		}
	}

	// 3. Plantillas acotadas <= 35 lineas cada una
	specTplPath := filepath.Join(root, "payload", ".agents", "skills", "seb-fast-task", "template", "nnn_nombre_tarea_spec.md")
	specTplBytes, err := os.ReadFile(specTplPath)
	if err != nil {
		t.Fatalf("falta plantilla de spec acotada: %v", err)
	}
	if n := countLinesFast(specTplBytes); n > 35 {
		t.Errorf("plantilla spec acotada excede 35 lineas: %d", n)
	}
	if !strings.Contains(string(specTplBytes), "**Modo:** `FAST`") {
		t.Errorf("plantilla spec acotada debe incluir cabecera Modo: FAST")
	}

	pvTplPath := filepath.Join(root, "payload", ".agents", "skills", "seb-fast-task", "template", "nnn_nombre_tarea_plan_y_verificador.md")
	pvTplBytes, err := os.ReadFile(pvTplPath)
	if err != nil {
		t.Fatalf("falta plantilla de plan_y_verificador: %v", err)
	}
	if n := countLinesFast(pvTplBytes); n > 35 {
		t.Errorf("plantilla plan_y_verificador excede 35 lineas: %d", n)
	}
	if !strings.Contains(string(pvTplBytes), "<NNN>-CHK-") {
		t.Errorf("plantilla plan_y_verificador debe incluir identificadores <NNN>-CHK-")
	}

	// 4. Bloque gestionado de AGENTS.md <= 30 lineas con enrutamiento explicito a seb-fast-task
	agentsBytes, err := os.ReadFile(filepath.Join(root, "payload", "AGENTS.md"))
	if err != nil {
		t.Fatalf("error leyendo payload/AGENTS.md: %v", err)
	}
	blockLines := extractFastManagedBlockLines(string(agentsBytes))
	if len(blockLines) == 0 || len(blockLines) > 30 {
		t.Errorf("bloque gestionado de payload/AGENTS.md fuera de rango (1..30): %d lineas", len(blockLines))
	}
	blockText := strings.Join(blockLines, "\n")
	for _, needle := range []string{".agents/SEB-HARNESS.md", "seb-fast-task", ".agents/skills/seb-fast-task/SKILL.md"} {
		if !strings.Contains(blockText, needle) {
			t.Errorf("bloque gestionado de AGENTS.md debe mencionar %q", needle)
		}
	}

	// 5. Regla global <= 35 lineas protegiendo *_plan_y_verificador.md
	ruleBytes, err := os.ReadFile(filepath.Join(root, "payload", ".agents", "rules", "00-seb-harness.md"))
	if err != nil {
		t.Fatalf("error leyendo 00-seb-harness.md: %v", err)
	}
	if n := countLinesFast(ruleBytes); n > 35 {
		t.Errorf("00-seb-harness.md excede 35 lineas: %d", n)
	}
	if !strings.Contains(string(ruleBytes), "_plan_y_verificador.md") {
		t.Errorf("00-seb-harness.md debe proteger specs/*_plan_y_verificador.md")
	}
}

// TestFastTaskInstallAndUninstallLifecycle valida 018-RF-02, 018-RF-03 y 018-RNF-02.
func TestFastTaskInstallAndUninstallLifecycle(t *testing.T) {
	root := repoRootFromEngine(t)
	target := t.TempDir()
	initTempGitRepo(t, target)

	installScript := filepath.Join(root, "install.sh")
	uninstallScript := filepath.Join(root, "uninstall.sh")

	runBash := func(script string, args ...string) {
		cmd := exec.Command("bash", append([]string{script}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v fallo: %v\n%s", script, args, err, string(out))
		}
	}

	// 1. Primera instalacion
	runBash(installScript, "--no-build", target)
	installedSkill := filepath.Join(target, ".agents", "skills", "seb-fast-task", "SKILL.md")
	skillFirst, err := os.ReadFile(installedSkill)
	if err != nil {
		t.Fatalf("install.sh no instalo seb-fast-task/SKILL.md: %v", err)
	}
	agentsFirst, err := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if err != nil {
		t.Fatalf("install.sh no instalo AGENTS.md: %v", err)
	}

	// 2. Segunda ejecucion: idempotencia total
	runBash(installScript, "--no-build", target)
	skillSecond, _ := os.ReadFile(installedSkill)
	agentsSecond, _ := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if !bytes.Equal(skillFirst, skillSecond) {
		t.Errorf("install.sh no es idempotente sobre seb-fast-task/SKILL.md")
	}
	if !bytes.Equal(agentsFirst, agentsSecond) {
		t.Errorf("install.sh no es idempotente sobre AGENTS.md")
	}

	// 3. Desinstalacion --dry-run no debe modificar disco
	runBash(uninstallScript, "--dry-run", target)
	if _, err := os.Stat(installedSkill); err != nil {
		t.Errorf("uninstall.sh --dry-run elimino %s", installedSkill)
	}

	// 4. Desinstalacion estandar debe eliminar .agents/skills/seb-fast-task/
	runBash(uninstallScript, target)
	if _, err := os.Stat(filepath.Join(target, ".agents", "skills", "seb-fast-task")); !os.IsNotExist(err) {
		t.Errorf("uninstall.sh no elimino .agents/skills/seb-fast-task")
	}
}

// TestFastTaskConfigAndModeDetection valida 018-RF-06, 018-CL-02 y 018-CL-05.
func TestFastTaskConfigAndModeDetection(t *testing.T) {
	tmp := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	// 1. Sin config.json: defaults 4 archivos y 100 lineas
	cfg, err := loadFastTrackConfig(".seb/config.json")
	if err != nil {
		t.Fatalf("loadFastTrackConfig sin archivo fallo: %v", err)
	}
	if cfg.MaxFiles != 4 || cfg.MaxLines != 100 {
		t.Errorf("defaults esperados (4, 100), recibido (%d, %d)", cfg.MaxFiles, cfg.MaxLines)
	}

	// 2. Configuracion personalizada valida
	_ = os.MkdirAll(".seb", 0755)
	_ = os.WriteFile(".seb/config.json", []byte(`{"fast_track":{"max_files":2,"max_lines":40}}`), 0644)
	cfg, err = loadFastTrackConfig(".seb/config.json")
	if err != nil {
		t.Fatalf("config personalizada valida fallo: %v", err)
	}
	if cfg.MaxFiles != 2 || cfg.MaxLines != 40 {
		t.Errorf("umbrales personalizados esperados (2, 40), recibido (%d, %d)", cfg.MaxFiles, cfg.MaxLines)
	}

	// 3. Configuracion invalida (<= 0 o tipo no numerico) debe fallar
	_ = os.WriteFile(".seb/config.json", []byte(`{"fast_track":{"max_files":0,"max_lines":50}}`), 0644)
	if _, err := loadFastTrackConfig(".seb/config.json"); err == nil {
		t.Errorf("se esperaba error con max_files=0")
	}
	_ = os.WriteFile(".seb/config.json", []byte(`{"fast_track":{"max_files":"cuatro"}}`), 0644)
	if _, err := loadFastTrackConfig(".seb/config.json"); err == nil {
		t.Errorf("se esperaba error con max_files no numerico")
	}

	// 4. Deteccion FAST, SDD y rechazo de artefactos mixtos (018-CL-02)
	_ = os.MkdirAll("specs", 0755)
	_ = os.WriteFile("specs/001_sdd_spec.md", []byte("# Spec 001\n- **Estado:** `APROBADA`\n"), 0644)
	_ = os.WriteFile("specs/001_sdd_plan.md", []byte("# Plan\n"), 0644)
	_ = os.WriteFile("specs/001_sdd_verificador.md", []byte("# Verif\n"), 0644)
	mode, _, _, err := detectTaskModeAndArtifacts("001", "sdd", "specs/001_sdd_spec.md")
	if err != nil || mode != taskModeSDD {
		t.Errorf("se esperaba SDD para 001_sdd, obtenido mode=%s err=%v", mode, err)
	}

	_ = os.WriteFile("specs/002_rapida_spec.md", []byte("# Spec 002\n- **Modo:** `FAST`\n- **Estado:** `APROBADA`\n"), 0644)
	_ = os.WriteFile("specs/002_rapida_plan_y_verificador.md", []byte("# Plan y Verif\n"), 0644)
	mode, planP, verifP, err := detectTaskModeAndArtifacts("002", "rapida", "specs/002_rapida_spec.md")
	if err != nil || mode != taskModeFAST || planP != verifP {
		t.Errorf("se esperaba FAST unificado para 002_rapida, obtenido mode=%s plan=%s verif=%s err=%v", mode, planP, verifP, err)
	}

	// Mezclar artefactos SDD y FAST para la misma tarea debe fallar
	_ = os.WriteFile("specs/002_rapida_verificador.md", []byte("# Conflicto\n"), 0644)
	if _, _, _, err := detectTaskModeAndArtifacts("002", "rapida", "specs/002_rapida_spec.md"); err == nil {
		t.Errorf("se esperaba error de ambiguedad al coexistir *_plan_y_verificador.md y *_verificador.md")
	}
}

// TestFastTaskDiffThresholdsAndAntiRewardHacking valida 018-RF-08, 018-RF-11, 018-RF-12 y 018-CL-01.
func TestFastTaskDiffThresholdsAndAntiRewardHacking(t *testing.T) {
	tmp := t.TempDir()
	initTempGitRepo(t, tmp)
	origWD, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	// Artefactos propios de la tarea en specs/ y .seb/ no deben contar contra los umbrales
	_ = os.MkdirAll("specs", 0755)
	_ = os.MkdirAll(".seb", 0755)
	_ = os.WriteFile("specs/001_fix_spec.md", []byte(strings.Repeat("linea\n", 60)), 0644)
	_ = os.WriteFile("specs/001_fix_plan_y_verificador.md", []byte(strings.Repeat("linea\n", 60)), 0644)
	_ = os.WriteFile(".seb/seb.lock", []byte("{}\n"), 0644)

	// 1 archivo con 10 lineas: debe pasar (<= 4 archivos, <= 100 lineas)
	_ = os.WriteFile("app.go", []byte("package main\nfunc ok() int { return 1 }\n"), 0644)
	cfg := ResolvedFastTrackConfig{MaxFiles: 4, MaxLines: 100}
	if err := validateFastDiffThresholds("001_fix", cfg); err != nil {
		t.Fatalf("cambio pequeno debio pasar umbrales FAST: %v", err)
	}

	// Exceder max_files (5 archivos de codigo) debe fallar con mensaje de escalamiento a SDD
	for i := 1; i <= 4; i++ {
		_ = os.WriteFile(fmt.Sprintf("extra_%d.go", i), []byte("package main\n"), 0644)
	}
	err := validateFastDiffThresholds("001_fix", cfg)
	if err == nil || !strings.Contains(err.Error(), "Flujo SDD Completo") {
		t.Errorf("se esperaba bloqueo FAST con instruccion de escalamiento a SDD, obtenido: %v", err)
	}

	// Limpiar archivos extra y probar exceso de lineas (> 100)
	for i := 1; i <= 4; i++ {
		_ = os.Remove(fmt.Sprintf("extra_%d.go", i))
	}
	_ = os.WriteFile("app.go", []byte(strings.Repeat("var x = 1\n", 105)), 0644)
	if err := validateFastDiffThresholds("001_fix", cfg); err == nil {
		t.Errorf("se esperaba bloqueo FAST al superar 100 lineas")
	}

	// Probar deteccion anti-reward-hacking en archivo de test
	_ = os.WriteFile("app.go", []byte("package main\n"), 0644)
	_ = os.WriteFile("app_test.go", []byte("package main\nimport \"testing\"\nfunc TestX(t *testing.T) { t.Skip(\"omitido\") }\n"), 0644)
	if err := validateFastAntiRewardHackingDiff("001_fix"); err == nil || !strings.Contains(err.Error(), "ANTI-REWARD-HACKING") {
		t.Errorf("se esperaba bloqueo ANTI-REWARD-HACKING ante t.Skip, obtenido: %v", err)
	}
}

// TestFastTaskGuardPermissionsAndLatency valida 018-RF-09, 018-RF-12 y 018-RNF-03.
func TestFastTaskGuardPermissionsAndLatency(t *testing.T) {
	// 1. Permisos de seb-fast-task
	if !checkPermission("seb-fast-task", "write", "specs/019_typo_spec.md") {
		t.Errorf("seb-fast-task debe poder escribir specs/*_spec.md")
	}
	if !checkPermission("seb-fast-task", "write", "specs/019_typo_plan_y_verificador.md") {
		t.Errorf("seb-fast-task debe poder escribir specs/*_plan_y_verificador.md")
	}
	if !checkPermission("seb-fast-task", "write", "MEMORY.md") {
		t.Errorf("seb-fast-task debe poder escribir MEMORY.md")
	}
	for _, forbidden := range []string{"engine/cmd/seb/main.go", "docs/constitution.md", "specs/019_typo_plan.md", "specs/019_typo_verificador.md"} {
		if checkPermission("seb-fast-task", "write", forbidden) {
			t.Errorf("seb-fast-task NO debe poder escribir %s", forbidden)
		}
	}

	// 2. Implementador y roles desconocidos no pueden editar *_plan_y_verificador.md
	if checkPermission("implementador", "write", "specs/019_typo_plan_y_verificador.md") {
		t.Errorf("implementador NO debe poder escribir specs/*_plan_y_verificador.md")
	}
	if checkPermission("spring-boot-expert", "write", "specs/019_typo_plan_y_verificador.md") {
		t.Errorf("rol externo NO debe poder escribir specs/*_plan_y_verificador.md")
	}

	// 3. PreToolUse bloquea redireccion de shell a *_plan_y_verificador.md y escalada a migrador desde seb-fast-task
	payloadRedirect := []byte(`{"conversationId":"s1","toolCall":{"name":"run_command","args":{"CommandLine":"echo x > specs/019_typo_plan_y_verificador.md"}}}`)
	out, _, _ := processHookPreToolWithFormat(nil, payloadRedirect, "implementador", "", "", FormatAntigravity)
	if !strings.Contains(out, `"deny"`) {
		t.Errorf("PreToolUse debio denegar redireccion de shell a *_plan_y_verificador.md, salida: %s", out)
	}

	payloadEscalation := []byte(`{"conversationId":"s1","toolCall":{"name":"run_command","args":{"CommandLine":".seb/bin/seb role set migrador"}}}`)
	outEsc, _, _ := processHookPreToolWithFormat(nil, payloadEscalation, "seb-fast-task", "", "", FormatAntigravity)
	if !strings.Contains(outEsc, `"deny"`) {
		t.Errorf("PreToolUse debio denegar escalada a migrador desde seb-fast-task, salida: %s", outEsc)
	}

	// 4. Latencia < 10 ms en evaluacion de guard
	start := time.Now()
	for i := 0; i < 100; i++ {
		_ = checkPermission("seb-fast-task", "write", "specs/019_typo_plan_y_verificador.md")
	}
	if elapsed := time.Since(start); elapsed >= 10*time.Millisecond {
		t.Errorf("latencia de checkPermission excedio 10ms para 100 evaluaciones: %v", elapsed)
	}
}

// TestFastTaskPlanLockAndVerifyLifecycle valida 018-RF-07, 018-RF-10, 018-RF-11, 018-CL-03 y 018-CL-06.
func TestFastTaskPlanLockAndVerifyLifecycle(t *testing.T) {
	bin := buildTestSebBin(t)
	tmp := t.TempDir()
	initTempGitRepo(t, tmp)

	runSeb := func(envSkill string, args ...string) (string, error) {
		cmd := exec.Command(bin, args...)
		cmd.Dir = tmp
		cmd.Env = append(os.Environ(), "SEB_SKILL="+envSkill)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// 1. Reservar ID con spec-next
	out, err := runSeb("seb-fast-task", "spec-next", "ajuste_copy")
	if err != nil {
		t.Fatalf("spec-next fallo: %v\n%s", err, out)
	}
	specRel := strings.TrimSpace(out)
	if specRel != "specs/001_ajuste_copy_spec.md" {
		t.Fatalf("ruta inesperada: %s", specRel)
	}

	// 2. Escribir los 2 artefactos acotados (con Afecta-UC existente)
	specContent := `# Spec 001 (FAST) — Ajuste de copy

- **Tarea ID:** ` + "`001`" + `
- **Identificador:** ` + "`ajuste_copy`" + `
- **Modo:** ` + "`FAST`" + `
- **Estado:** ` + "`APROBADA`" + `
- **Afecta-UC:** ` + "`UC-cli_observability_introspection`" + `

## 1. Objetivo y Alcance
Corregir texto menor.

## 2. Criterios de Aceptación (EARS)
- **001-RF-01 (Ubiquitous):** EL SISTEMA DEBE mostrar texto corregido.

## 3. Fuera de Alcance
- Cambios de arquitectura.
`
	pvContent := `# Plan y Verificador (FAST) — ajuste_copy (Spec 001)

## 1. Resumen de Cambios y Archivos
- ` + "`README.md`" + `: actualizar copy.

## 2. Matriz de Verificación Determinista

| Check ID | Requisito | Descripción | Comando Determinista |
| :--- | :--- | :--- | :--- |
| 001-CHK-01 | 001-RF-01 | Verifica texto en README | ` + "`grep -q 'Corregido' README.md`" + ` |
| 001-CHK-99 | GLOBAL | Smoke check base | ` + "`true`" + ` |
`
	_ = os.WriteFile(filepath.Join(tmp, "specs/001_ajuste_copy_spec.md"), []byte(specContent), 0644)
	_ = os.WriteFile(filepath.Join(tmp, "specs/001_ajuste_copy_plan_y_verificador.md"), []byte(pvContent), 0644)

	// 3. Sellar con plan-lock
	out, err = runSeb("seb-fast-task", "plan-lock", "ajuste_copy")
	if err != nil {
		t.Fatalf("plan-lock FAST fallo: %v\n%s", err, out)
	}

	// Verificar seb.lock con mode: "fast" y rol transicionado a implementador
	lockBytes, err := os.ReadFile(filepath.Join(tmp, ".seb", "seb.lock"))
	if err != nil {
		t.Fatalf("no se creo .seb/seb.lock: %v", err)
	}
	var lock SebLock
	if err := json.Unmarshal(lockBytes, &lock); err != nil {
		t.Fatalf("seb.lock invalido: %v", err)
	}
	tl, ok := lock.Tasks["001_ajuste_copy"]
	if !ok || tl.Mode != "fast" || tl.SpecHash == "" || tl.VerifierHash == "" {
		t.Fatalf("seb.lock no registro mode=fast o hashes: %+v", tl)
	}
	roleBytes, _ := os.ReadFile(filepath.Join(tmp, ".seb", "ACTIVE_ROLE"))
	if strings.TrimSpace(string(roleBytes)) != "implementador" {
		t.Errorf("plan-lock debio transicionar ACTIVE_ROLE a implementador, actual: %q", string(roleBytes))
	}

	// 4. Implementar cambio menor y borrar SQLite para probar clon limpio (018-CL-06)
	_ = os.WriteFile(filepath.Join(tmp, "README.md"), []byte("# Repo de prueba\nCorregido\n"), 0644)
	_ = os.Remove(filepath.Join(tmp, ".seb", "seb-audit.db"))

	out, err = runSeb("seb-verificador", "verify", "ajuste_copy")
	if err != nil {
		t.Fatalf("verify FAST fallo en clon limpio: %v\n%s", err, out)
	}
	if !strings.Contains(out, "APROBADO") {
		t.Errorf("se esperaba veredicto APROBADO, salida:\n%s", out)
	}
}

// TestFastTaskAntiSilentDropAndTampering valida 018-RF-12 y 018-CL-04.
func TestFastTaskAntiSilentDropAndTampering(t *testing.T) {
	bin := buildTestSebBin(t)
	tmp := t.TempDir()
	initTempGitRepo(t, tmp)

	_ = os.MkdirAll(filepath.Join(tmp, "specs"), 0755)
	specContent := "# Spec 001\n- **Modo:** `FAST`\n- **Estado:** `APROBADA`\n- **Afecta-UC:** `ninguno`\n"
	_ = os.WriteFile(filepath.Join(tmp, "specs/001_mal_spec.md"), []byte(specContent), 0644)

	// 1. Fila CHK sin prefijo de spec debe rechazarse (Anti-Silent-Drop)
	badPV := "# Plan y Verificador\n| Check ID | Requisito | Descripcion | Comando |\n| :--- | :--- | :--- | :--- |\n| CHK-01 | GLOBAL | Heredado | `true` |\n"
	_ = os.WriteFile(filepath.Join(tmp, "specs/001_mal_plan_y_verificador.md"), []byte(badPV), 0644)
	cmd := exec.Command(bin, "plan-lock", "001")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("plan-lock debio rechazar fila CHK- heredada en *_plan_y_verificador.md:\n%s", string(out))
	}

	// 2. Sellar version valida y luego alterar *_plan_y_verificador.md -> verify debe rechazar tampering
	goodPV := "# Plan y Verificador\n| Check ID | Requisito | Descripcion | Comando |\n| :--- | :--- | :--- | :--- |\n| 001-CHK-01 | GLOBAL | Valido | `true` |\n"
	_ = os.WriteFile(filepath.Join(tmp, "specs/001_mal_plan_y_verificador.md"), []byte(goodPV), 0644)
	cmd = exec.Command(bin, "plan-lock", "001")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("plan-lock valido fallo: %v\n%s", err, string(out))
	}

	_ = os.WriteFile(filepath.Join(tmp, "specs/001_mal_plan_y_verificador.md"), []byte(goodPV+"\n# editado\n"), 0644)
	cmd = exec.Command(bin, "verify", "001")
	cmd.Dir = tmp
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "VIOLACIÓN DE INTEGRIDAD") {
		t.Fatalf("verify debio rechazar manipulacion de *_plan_y_verificador.md, salida:\n%s", string(out))
	}
}

// TestFastTaskPreCommitGate valida 018-RF-13 y 018-CL-01 sobre el hook git-pre-commit real.
func TestFastTaskPreCommitGate(t *testing.T) {
	root := repoRootFromEngine(t)
	bin := buildTestSebBin(t)
	tmp := t.TempDir()
	initTempGitRepo(t, tmp)

	// Instalar binario y hook pre-commit en el repo temporal
	_ = os.MkdirAll(filepath.Join(tmp, ".seb", "bin"), 0755)
	binData, _ := os.ReadFile(bin)
	_ = os.WriteFile(filepath.Join(tmp, ".seb", "bin", "seb"), binData, 0755)
	hookSrc, err := os.ReadFile(filepath.Join(root, "payload", ".agents", "hooks", "git-pre-commit"))
	if err != nil {
		t.Fatalf("error leyendo git-pre-commit: %v", err)
	}
	_ = os.WriteFile(filepath.Join(tmp, ".git", "hooks", "pre-commit"), hookSrc, 0755)

	_ = os.MkdirAll(filepath.Join(tmp, "specs"), 0755)
	_ = os.WriteFile(filepath.Join(tmp, "specs/001_fastgate_spec.md"), []byte("# Spec 001\n- **Modo:** `FAST`\n- **Estado:** `APROBADA`\n- **Afecta-UC:** `ninguno`\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmp, "specs/001_fastgate_plan_y_verificador.md"), []byte("# PV\n| Check ID | Requisito | Descripcion | Comando |\n| :--- | :--- | :--- | :--- |\n| 001-CHK-01 | GLOBAL | OK | `true` |\n"), 0644)

	lockCmd := exec.Command(filepath.Join(tmp, ".seb", "bin", "seb"), "plan-lock", "001")
	lockCmd.Dir = tmp
	if out, err := lockCmd.CombinedOutput(); err != nil {
		t.Fatalf("plan-lock fallo: %v\n%s", err, string(out))
	}

	runGit := func(args ...string) (string, error) {
		c := exec.Command("git", args...)
		c.Dir = tmp
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		out, err := c.CombinedOutput()
		return string(out), err
	}

	// 1. Exceder umbral (5 archivos): pre-commit debe rechazar el commit
	for i := 1; i <= 5; i++ {
		_ = os.WriteFile(filepath.Join(tmp, fmt.Sprintf("f%d.txt", i)), []byte("cambio\n"), 0644)
	}
	_, _ = runGit("add", "f1.txt", "f2.txt", "f3.txt", "f4.txt", "f5.txt")
	if out, err := runGit("commit", "-m", "feat: excede umbral"); err == nil {
		t.Fatalf("pre-commit debio rechazar commit con 5 archivos (>4):\n%s", out)
	}

	// 2. Reducir a 1 archivo dentro de umbrales: pre-commit debe aprobar
	_, _ = runGit("reset", "HEAD")
	for i := 2; i <= 5; i++ {
		_ = os.Remove(filepath.Join(tmp, fmt.Sprintf("f%d.txt", i)))
	}
	_, _ = runGit("add", "f1.txt")
	if out, err := runGit("commit", "-m", "fix: cambio acotado"); err != nil {
		t.Fatalf("pre-commit debio aprobar commit FAST dentro de umbrales: %v\n%s", err, out)
	}
}

// TestFastTaskAbortAndPurge valida 018-RF-17.
func TestFastTaskAbortAndPurge(t *testing.T) {
	bin := buildTestSebBin(t)
	tmp := t.TempDir()
	initTempGitRepo(t, tmp)

	_ = os.MkdirAll(filepath.Join(tmp, "specs"), 0755)
	specPath := filepath.Join(tmp, "specs/001_cancelar_spec.md")
	pvPath := filepath.Join(tmp, "specs/001_cancelar_plan_y_verificador.md")
	_ = os.WriteFile(specPath, []byte("# Spec 001\n- **Modo:** `FAST`\n- **Estado:** `APROBADA`\n- **Afecta-UC:** `ninguno`\n"), 0644)
	_ = os.WriteFile(pvPath, []byte("# PV\n| Check ID | Requisito | Descripcion | Comando |\n| :--- | :--- | :--- | :--- |\n| 001-CHK-01 | GLOBAL | OK | `true` |\n"), 0644)

	runSeb := func(args ...string) (string, error) {
		c := exec.Command(bin, args...)
		c.Dir = tmp
		out, err := c.CombinedOutput()
		return string(out), err
	}

	if out, err := runSeb("plan-lock", "001"); err != nil {
		t.Fatalf("plan-lock fallo: %v\n%s", err, out)
	}
	if out, err := runSeb("abort", "001"); err != nil {
		t.Fatalf("abort formal FAST fallo: %v\n%s", err, out)
	}
	if out, err := runSeb("abort", "001", "--purge"); err != nil {
		t.Fatalf("abort --purge FAST fallo: %v\n%s", err, out)
	}
	if _, err := os.Stat(specPath); !os.IsNotExist(err) {
		t.Errorf("abort --purge no elimino %s", specPath)
	}
	if _, err := os.Stat(pvPath); !os.IsNotExist(err) {
		t.Errorf("abort --purge no elimino %s", pvPath)
	}
}

// TestFastTaskAuditReportDistinction valida 018-RF-14, 018-RF-15 y 018-RNF-04.
func TestFastTaskAuditReportDistinction(t *testing.T) {
	tmp := t.TempDir()
	dbFile := filepath.Join(tmp, "audit.db")
	db, err := sql.Open("sqlite", dbFile)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	if err := initDBSchema(db); err != nil {
		t.Fatalf("initDBSchema: %v", err)
	}

	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status, mode) VALUES ('001', 'normal', 'specs/001_normal_spec.md', 'VERIFIED', 'SDD')`)
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status, mode) VALUES ('002', 'rapida', 'specs/002_rapida_spec.md', 'VERIFIED', 'FAST')`)
	_, _ = db.Exec(`INSERT INTO agent_actions (session_id, task_id, agent, skill, action, duration_ms, tokens_in, tokens_out, cost_usd) VALUES ('s1', '001', 'agy', 'seb-planificador', 'plan-lock', 120, 1000, 200, 0.01)`)
	_, _ = db.Exec(`INSERT INTO agent_actions (session_id, task_id, agent, skill, action, duration_ms, tokens_in, tokens_out, cost_usd) VALUES ('s2', '002', 'agy', 'seb-fast-task', 'plan-lock', 40, 300, 80, 0.002)`)

	report := renderAuditReport(db)
	for _, needle := range []string{"| `001` | `SDD` |", "| `002` | `FAST` |", "Resumen Comparativo por Modo (FAST vs SDD)"} {
		if !strings.Contains(report, needle) {
			t.Errorf("audit report debe contener %q\nReporte:\n%s", needle, report)
		}
	}
	// Privacidad (018-RNF-04): rutas externas deben sanitizarse antes de persistir
	sanitized := sanitizeStringValue("/home/secret-user/private/token.txt", tmp)
	if strings.Contains(sanitized, "secret-user") {
		t.Errorf("sanitizeStringValue filtro ruta privada: %s", sanitized)
	}
}

// TestFastTaskStatusAndHelpObservability valida 018-RF-16, 018-CL-02 y 018-CL-06.
func TestFastTaskStatusAndHelpObservability(t *testing.T) {
	tmp := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	_ = os.MkdirAll("specs", 0755)
	_ = os.WriteFile("specs/001_std_spec.md", []byte("# Spec 001\n- **Estado:** `APROBADA`\n"), 0644)
	_ = os.WriteFile("specs/002_rapida_spec.md", []byte("# Spec 002\n- **Modo:** `FAST`\n- **Estado:** `APROBADA`\n"), 0644)
	_ = os.WriteFile("specs/002_rapida_plan_y_verificador.md", []byte("# PV\n"), 0644)

	// 1. seb status en clon limpio (db == nil): full, --compact y --json
	var outFull, errBuf bytes.Buffer
	if code := runStatusCommand(nil, nil, &outFull, &errBuf); code != 0 {
		t.Fatalf("status full fallo (%d): %s", code, errBuf.String())
	}
	if !strings.Contains(outFull.String(), "Modo: FAST") || !strings.Contains(outFull.String(), "Modo: SDD") {
		t.Errorf("status full debe distinguir Modo: FAST y Modo: SDD:\n%s", outFull.String())
	}

	var outCompact bytes.Buffer
	if code := runStatusCommand(nil, []string{"--compact"}, &outCompact, &errBuf); code != 0 {
		t.Fatalf("status --compact fallo (%d): %s", code, errBuf.String())
	}
	if !strings.Contains(outCompact.String(), "| Modo |") || !strings.Contains(outCompact.String(), "| FAST |") {
		t.Errorf("status --compact debe mostrar columna Modo y valor FAST:\n%s", outCompact.String())
	}

	var outJSON bytes.Buffer
	if code := runStatusCommand(nil, []string{"--json"}, &outJSON, &errBuf); code != 0 {
		t.Fatalf("status --json fallo (%d): %s", code, errBuf.String())
	}
	var parsed StatusReport
	if err := json.Unmarshal(outJSON.Bytes(), &parsed); err != nil {
		t.Fatalf("status --json invalido: %v", err)
	}
	if len(parsed.Tasks) != 2 || parsed.Tasks[0].Mode != "SDD" || parsed.Tasks[1].Mode != "FAST" {
		t.Errorf("modos inesperados en JSON: %+v", parsed.Tasks)
	}

	// 2. seb help documenta flujo FAST y *_plan_y_verificador.md
	var helpOut bytes.Buffer
	if code := runHelpCommand([]string{"verify"}, &helpOut, &errBuf); code != 0 {
		t.Fatalf("help verify fallo (%d): %s", code, errBuf.String())
	}
	if !strings.Contains(helpOut.String(), "_plan_y_verificador.md") || !strings.Contains(helpOut.String(), "FAST") {
		t.Errorf("help verify debe documentar FAST y _plan_y_verificador.md:\n%s", helpOut.String())
	}

	// 3. Artefactos mixtos deben provocar codigo de salida 1 en seb status (018-CL-02)
	_ = os.WriteFile("specs/002_rapida_verificador.md", []byte("# Mezcla\n"), 0644)
	errBuf.Reset()
	if code := runStatusCommand(nil, nil, &outFull, &errBuf); code != 1 {
		t.Errorf("status debio fallar con codigo 1 ante artefactos mixtos, obtuvo %d", code)
	}
}
