package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// CHK-01 (RF-01, RF-02, RF-03, RF-04, CL-01, CL-03)
// Gestion de roles CLI (set, get, clear), normalizacion de mayusculas/espacios, salida 1 ante roles no reconocidos e idempotencia
func TestRoleCLIManagement(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error obteniendo directorio actual: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Error cambiando al directorio temporal: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.MkdirAll(".seb", 0755); err != nil {
		t.Fatalf("Error creando directorio .seb: %v", err)
	}

	roleFile := filepath.Join(tmpDir, ".seb", "ACTIVE_ROLE")

	// 1. RF-03: Consulta inicial cuando no hay rol definido -> debe retornar "implementador"
	initialRole := getActiveRole(roleFile)
	if initialRole != "implementador" {
		t.Fatalf("Se esperaba rol por defecto 'implementador', se obtuvo '%s'", initialRole)
	}

	// 2. RF-01: Asignacion valida de rol canónico
	if err := setActiveRole("seb-iniciador", roleFile); err != nil {
		t.Fatalf("Fallo asignando rol valido 'seb-iniciador': %v", err)
	}
	data, err := os.ReadFile(roleFile)
	if err != nil || string(data) != "seb-iniciador" {
		t.Fatalf("Contenido en archivo de rol no coincide con 'seb-iniciador': %s", string(data))
	}
	if cur := getActiveRole(roleFile); cur != "seb-iniciador" {
		t.Fatalf("getActiveRole retorno '%s', se esperaba 'seb-iniciador'", cur)
	}

	// 3. CL-01: Normalizacion de espacios y mayusculas
	if err := setActiveRole("  SEB-ESPECIFICADOR  \n", roleFile); err != nil {
		t.Fatalf("Fallo asignando rol con mayusculas y espacios: %v", err)
	}
	if cur := getActiveRole(roleFile); cur != "seb-especificador" {
		t.Fatalf("No se normalizo correctamente a minusculas: '%s'", cur)
	}

	// 4. RF-02: Rechazo estricto ante rol no reconocido o vacio
	errInvalid := setActiveRole("java-springboot", roleFile)
	if errInvalid == nil {
		t.Fatalf("Se esperaba error al intentar asignar rol no canonico 'java-springboot'")
	}
	if !strings.Contains(errInvalid.Error(), "no reconocido") {
		t.Fatalf("El mensaje de error no indica diagnostico adecuado: %v", errInvalid)
	}
	// El rol previo no debe haber sido mutado
	if cur := getActiveRole(roleFile); cur != "seb-especificador" {
		t.Fatalf("El rol previo fue mutado indebidamente tras error: '%s'", cur)
	}

	errEmpty := setActiveRole("", roleFile)
	if errEmpty == nil {
		t.Fatalf("Se esperaba error al asignar rol vacio")
	}

	// 5. CL-03: Idempotencia en persistencia
	if err := setActiveRole("seb-verificador", roleFile); err != nil {
		t.Fatalf("Error en primera asignacion: %v", err)
	}
	if err := setActiveRole("seb-verificador", roleFile); err != nil {
		t.Fatalf("Error en segunda asignacion consecutiva (idempotencia): %v", err)
	}
	if cur := getActiveRole(roleFile); cur != "seb-verificador" {
		t.Fatalf("Rol esperado 'seb-verificador', obtenido '%s'", cur)
	}

	// 6. RF-04: Limpieza de rol
	if err := clearActiveRole(roleFile); err != nil {
		t.Fatalf("Error al limpiar rol activo: %v", err)
	}
	if _, err := os.Stat(roleFile); !os.IsNotExist(err) {
		t.Fatalf("El archivo ACTIVE_ROLE no fue eliminado tras clear")
	}
	if cur := getActiveRole(roleFile); cur != "implementador" {
		t.Fatalf("Tras clear, se esperaba fallback a 'implementador', se obtuvo '%s'", cur)
	}

	// 7. Prueba del despachador CLI cmdRole
	db := setupTestDB(t)
	defer db.Close()

	if err := cmdRole(db, []string{"set", "seb-clarificador"}); err != nil {
		t.Fatalf("cmdRole set fallo: %v", err)
	}
	if cur := getActiveRole(".seb/ACTIVE_ROLE"); cur != "seb-clarificador" {
		t.Fatalf("cmdRole set no persistio 'seb-clarificador': '%s'", cur)
	}

	if err := cmdRole(db, []string{"set", "invalid-role"}); err == nil {
		t.Fatalf("cmdRole set con rol invalido debio fallar")
	}

	if err := cmdRole(db, []string{"clear"}); err != nil {
		t.Fatalf("cmdRole clear fallo: %v", err)
	}
	if cur := getActiveRole(".seb/ACTIVE_ROLE"); cur != "implementador" {
		t.Fatalf("cmdRole clear no restablecio a 'implementador': '%s'", cur)
	}
}

// CHK-02 (RF-05, RF-06, CL-02)
// Jerarquia de precedencia de 4 niveles (SEB_SKILL > ACTIVE_ROLE > transcripcion seb-* > implementador) y tolerancia a archivo corrupto
func TestActiveRolePrecedenceAndHierarchy(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")
	activeTaskFile := filepath.Join(tmpDir, "ACTIVE_TASK")
	activeRoleFile := filepath.Join(tmpDir, "ACTIVE_ROLE")

	// Caso Nivel 1: SEB_SKILL tiene maxima prioridad por sobre archivo y transcripcion
	_ = os.WriteFile(activeRoleFile, []byte("seb-iniciador"), 0644)
	transcriptLines := []string{
		`{"step_index":1,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"read_file","args":{"Path":"/repo/.agents/skills/seb-especificador/SKILL.md"}}]}`,
	}
	_ = os.WriteFile(transcriptFile, []byte(strings.Join(transcriptLines, "\n")+"\n"), 0644)

	resNivel1 := inferActiveSkill("seb-verificador", transcriptFile, activeTaskFile, activeRoleFile)
	if resNivel1 != "seb-verificador" {
		t.Fatalf("Nivel 1 fallo: se esperaba 'seb-verificador' desde SEB_SKILL, se obtuvo '%s'", resNivel1)
	}

	// Caso Nivel 2: Sin variable de entorno, ACTIVE_ROLE prima sobre la transcripcion
	resNivel2 := inferActiveSkill("", transcriptFile, activeTaskFile, activeRoleFile)
	if resNivel2 != "seb-iniciador" {
		t.Fatalf("Nivel 2 fallo: se esperaba 'seb-iniciador' desde ACTIVE_ROLE, se obtuvo '%s'", resNivel2)
	}

	// Caso Nivel 3 (RF-06): Sin variable ni ACTIVE_ROLE, inferencia en transcripcion filtra estrictamente seb-*
	_ = os.Remove(activeRoleFile)
	linesWithDomainSkill := []string{
		`{"step_index":1,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"read_file","args":{"Path":"/repo/.agents/skills/seb-clarificador/SKILL.md"}}]}`,
		`{"step_index":2,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"read_file","args":{"Path":"/repo/.agents/skills/java-springboot/SKILL.md"}}]}`,
		`{"step_index":3,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"read_file","args":{"Path":"/repo/.agents/skills/docker-guidance/SKILL.md"}}]}`,
	}
	_ = os.WriteFile(transcriptFile, []byte(strings.Join(linesWithDomainSkill, "\n")+"\n"), 0644)

	resNivel3 := inferActiveSkill("", transcriptFile, activeTaskFile, activeRoleFile)
	if resNivel3 != "seb-clarificador" {
		t.Fatalf("Nivel 3 fallo: las lecturas de java-springboot y docker-guidance debieron ignorarse. Se esperaba 'seb-clarificador', se obtuvo '%s'", resNivel3)
	}

	// Caso Nivel 4: Sin coincidencias ni configuracion previa -> fallback a implementador
	emptyTranscript := filepath.Join(tmpDir, "empty_transcript.jsonl")
	_ = os.WriteFile(emptyTranscript, []byte(""), 0644)
	resNivel4 := inferActiveSkill("", emptyTranscript, activeTaskFile, activeRoleFile)
	if resNivel4 != "implementador" {
		t.Fatalf("Nivel 4 fallo: se esperaba fallback a 'implementador', se obtuvo '%s'", resNivel4)
	}

	// Caso CL-02: Archivo ACTIVE_ROLE vacio o con contenido corrupto no debe romper el sistema
	_ = os.WriteFile(activeRoleFile, []byte(""), 0644)
	resEmptyFile := inferActiveSkill("", emptyTranscript, activeTaskFile, activeRoleFile)
	if resEmptyFile != "implementador" {
		t.Fatalf("CL-02 fallo: ante archivo de rol vacio se esperaba 'implementador', se obtuvo '%s'", resEmptyFile)
	}

	_ = os.WriteFile(activeRoleFile, []byte("INVALID_CORRUPT_ROLE_DATA_123!@#"), 0644)
	resCorruptFile := inferActiveSkill("", emptyTranscript, activeTaskFile, activeRoleFile)
	if resCorruptFile != "implementador" {
		t.Fatalf("CL-02 fallo: ante archivo corrupto se esperaba 'implementador', se obtuvo '%s'", resCorruptFile)
	}
}

// CHK-03 (RF-07, RF-08)
// Aislamiento y herencia de permisos de implementador para skills de dominio externos (java-springboot) y proteccion de contratos
func TestRolePermissionsAndDomainSkillIsolation(t *testing.T) {
	// 1. Skill de dominio (java-springboot): Puede escribir codigo de aplicacion y tests
	domainRole := "java-springboot"
	if !checkPermission(domainRole, "write", "src/main/java/com/example/App.java") {
		t.Errorf("El rol de dominio '%s' debio tener permiso para escribir codigo Java", domainRole)
	}
	if !checkPermission(domainRole, "write", "pom.xml") {
		t.Errorf("El rol de dominio '%s' debio tener permiso para escribir pom.xml", domainRole)
	}
	if !checkPermission(domainRole, "write", "src/test/java/AppTest.java") {
		t.Errorf("El rol de dominio '%s' debio tener permiso para escribir tests Java", domainRole)
	}

	// 2. RF-08: Bloqueo anti-reward-hacking innegociable sobre contratos protegidos
	if checkPermission(domainRole, "write", "docs/constitution.md") {
		t.Errorf("El rol de dominio '%s' JAMAS debe poder modificar docs/constitution.md", domainRole)
	}
	if checkPermission(domainRole, "write", "specs/001_tarea_spec.md") {
		t.Errorf("El rol de dominio '%s' JAMAS debe poder modificar specs/*_spec.md", domainRole)
	}
	if checkPermission(domainRole, "write", "specs/001_tarea_verificador.md") {
		t.Errorf("El rol de dominio '%s' JAMAS debe poder modificar specs/*_verificador.md", domainRole)
	}

	// 3. Verificacion cruzada con otros roles canónicos
	if !checkPermission("seb-iniciador", "write", "docs/constitution.md") {
		t.Errorf("seb-iniciador debe tener permiso sobre docs/constitution.md")
	}
	if checkPermission("seb-iniciador", "write", "specs/001_tarea_spec.md") {
		t.Errorf("seb-iniciador no debe tener permiso sobre specs")
	}

	if !checkPermission("seb-especificador", "write", "specs/002_tarea_spec.md") {
		t.Errorf("seb-especificador debe tener permiso sobre specs/*_spec.md")
	}
	if checkPermission("seb-especificador", "write", "docs/constitution.md") {
		t.Errorf("seb-especificador no debe tener permiso sobre constitution.md")
	}

	if checkPermission("implementador", "write", "docs/constitution.md") {
		t.Errorf("implementador no debe tener permiso sobre constitution.md")
	}
}

// CHK-04 (RF-09, CL-04)
// Transicion atomica a implementador en plan-lock y limpieza automatica de ACTIVE_ROLE en comandos terminales verify e init
func TestRoleLifecyclePlanLockAndCleanup(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error obteniendo directorio actual: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Error cambiando a directorio temporal: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.MkdirAll(".seb", 0755); err != nil {
		t.Fatalf("Error creando .seb: %v", err)
	}
	if err := os.MkdirAll("specs", 0755); err != nil {
		t.Fatalf("Error creando specs: %v", err)
	}

	db := setupTestDB(t)
	defer db.Close()

	// 1. RF-09: plan-lock restablece el rol activo a 'implementador'
	roleFile := ".seb/ACTIVE_ROLE"
	_ = os.WriteFile(roleFile, []byte("seb-planificador"), 0644)

	specPath := filepath.Join("specs", "008_test_spec.md")
	verifPath := filepath.Join("specs", "008_test_verificador.md")
	planPath := filepath.Join("specs", "008_test_plan.md")

	_ = os.WriteFile(specPath, []byte("# Spec 008\n- **Tarea ID:** 008\n- **Estado:** APROBADA\n"), 0644)
	_ = os.WriteFile(verifPath, []byte("# Verificador\n| Check ID | Requisito | Descripcion | Comando |\n| :--- | :--- | :--- | :--- |\n| CHK-01 | RF-01 | Test | `echo ok` |\n"), 0644)
	_ = os.WriteFile(planPath, []byte("# Plan\n"), 0644)

	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('008', 'test', ?, 'SPECIFIED')`, specPath)

	// Ejecutar plan-lock
	cmdPlanLock(db, []string{"008"})

	// Verificar que el rol en .seb/ACTIVE_ROLE cambio a 'implementador'
	roleAfterLock := getActiveRole(roleFile)
	if roleAfterLock != "implementador" {
		t.Fatalf("RF-09 fallo: tras plan-lock el rol debio ser 'implementador', se obtuvo '%s'", roleAfterLock)
	}

	// 2. CL-04: Limpieza de rol en init
	_ = os.WriteFile(roleFile, []byte("seb-iniciador"), 0644)
	if err := clearActiveRole(roleFile); err != nil {
		t.Fatalf("Error al limpiar rol para test de init: %v", err)
	}
	if _, err := os.Stat(roleFile); !os.IsNotExist(err) {
		t.Fatalf("CL-04 fallo: clearActiveRole debio eliminar el archivo de rol")
	}
}
