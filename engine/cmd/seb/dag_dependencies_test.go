package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// CHK-01 (RF-01, RF-02): Parser determinista de metadata de dependencias en specs
func TestParseSpecDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Spec con directiva Dependencias y multiples tokens separados por coma
	spec1 := filepath.Join(tmpDir, "001_tarea_uno_spec.md")
	content1 := `# Spec 001 — Tarea Uno
- **Tarea ID:** 001
- **Dependencias:** 002_prefijo_secuencial_artefactos, 004_captura_telemetria_antigravity
## 1. Contexto`
	if err := os.WriteFile(spec1, []byte(content1), 0644); err != nil {
		t.Fatalf("Error creando spec1: %v", err)
	}

	deps1, err := parseSpecDependencies(spec1)
	if err != nil {
		t.Fatalf("parseSpecDependencies fallo en spec1: %v", err)
	}
	if len(deps1) != 2 || deps1[0] != "002_prefijo_secuencial_artefactos" || deps1[1] != "004_captura_telemetria_antigravity" {
		t.Errorf("Dependencias inesperadas para spec1: got %v", deps1)
	}

	// 2. Spec con directiva Depends-On y backticks
	spec2 := filepath.Join(tmpDir, "002_tarea_dos_spec.md")
	content2 := `# Spec 002 — Tarea Dos
- **Depends-On:** ` + "`001`" + `, ` + "`003`" + `
## 1. Contexto`
	if err := os.WriteFile(spec2, []byte(content2), 0644); err != nil {
		t.Fatalf("Error creando spec2: %v", err)
	}

	deps2, err := parseSpecDependencies(spec2)
	if err != nil {
		t.Fatalf("parseSpecDependencies fallo en spec2: %v", err)
	}
	if len(deps2) != 2 || deps2[0] != "001" || deps2[1] != "003" {
		t.Errorf("Dependencias inesperadas para spec2: got %v", deps2)
	}

	// 3. Spec con valor explicito 'Ninguna'
	spec3 := filepath.Join(tmpDir, "003_tarea_tres_spec.md")
	content3 := `# Spec 003 — Tarea Tres
- **Dependencias:** Ninguna
## 1. Contexto`
	if err := os.WriteFile(spec3, []byte(content3), 0644); err != nil {
		t.Fatalf("Error creando spec3: %v", err)
	}

	deps3, err := parseSpecDependencies(spec3)
	if err != nil {
		t.Fatalf("parseSpecDependencies fallo en spec3: %v", err)
	}
	if len(deps3) != 0 {
		t.Errorf("Se esperaba lista vacia para 'Ninguna', pero se obtuvo %v", deps3)
	}

	// 4. Spec sin campo de dependencias (retrocompatibilidad)
	spec4 := filepath.Join(tmpDir, "004_tarea_cuatro_spec.md")
	content4 := `# Spec 004 — Tarea Cuatro
- **Tarea ID:** 004
## 1. Contexto`
	if err := os.WriteFile(spec4, []byte(content4), 0644); err != nil {
		t.Fatalf("Error creando spec4: %v", err)
	}

	deps4, err := parseSpecDependencies(spec4)
	if err != nil {
		t.Fatalf("parseSpecDependencies fallo en spec4: %v", err)
	}
	if len(deps4) != 0 {
		t.Errorf("Se esperaba lista vacia para spec sin campo de dependencias, pero se obtuvo %v", deps4)
	}
}

// CHK-02 (RF-03): Estampado de verified: true y commit_hash en seb.lock al superar seb verify
func TestVerifyUpdatesSebLockWithVerifiedAndCommitHash(t *testing.T) {
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

	// Crear seb.lock inicial con una tarea solo planificada
	initialLock := SebLock{
		Tasks: map[string]TaskLock{
			"005_tarea_test": {
				SpecHash:     "hash_spec_123",
				VerifierHash: "hash_verif_456",
				Verified:     false,
				CommitHash:   "",
			},
		},
	}
	writeLockFile(initialLock)

	// Ejecutar estampado simulando fin exitoso de seb verify
	canonical := "005_tarea_test"
	testCommit := "abc7890"
	if err := updateTaskLockVerification(canonical, testCommit); err != nil {
		t.Fatalf("updateTaskLockVerification fallo: %v", err)
	}

	// Leer y verificar seb.lock actualizado
	updatedLock := readLockFile()
	tLock, exists := updatedLock.Tasks[canonical]
	if !exists {
		t.Fatalf("La tarea %s no existe en seb.lock actualizado", canonical)
	}
	if !tLock.Verified {
		t.Errorf("Se esperaba verified == true, pero se obtuvo false")
	}
	if tLock.CommitHash != testCommit {
		t.Errorf("Se esperaba commit_hash == %s, pero se obtuvo %s", testCommit, tLock.CommitHash)
	}
	if tLock.SpecHash != "hash_spec_123" || tLock.VerifierHash != "hash_verif_456" {
		t.Errorf("Los hashes previos fueron corrompidos: spec=%s, verif=%s", tLock.SpecHash, tLock.VerifierHash)
	}
}

// CHK-03 (RF-04, RF-05, RF-06): Guardarrail de dependencias en plan-lock
func TestPlanLockDirectDependenciesGuardrail(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error obteniendo directorio actual: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Error cambiando a directorio temporal: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.MkdirAll("specs", 0755); err != nil {
		t.Fatalf("Error creando specs/: %v", err)
	}

	// Crear base en memoria
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Error abriendo sqlite: %v", err)
	}
	defer db.Close()
	if err := initDBSchema(db); err != nil {
		t.Fatalf("Error inicializando esquema: %v", err)
	}

	// 1. Crear tarea previa satisfecha
	specDepOk := filepath.Join("specs", "001_auth_jwt_spec.md")
	_ = os.WriteFile(specDepOk, []byte("# Spec 001 — Auth\n- **Tarea ID:** 001\n"), 0644)
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('001', 'auth_jwt', ?, 'VERIFIED')`, specDepOk)

	// 2. Crear tarea previa no verificada (solo planificada)
	specDepUnverified := filepath.Join("specs", "002_reportes_spec.md")
	_ = os.WriteFile(specDepUnverified, []byte("# Spec 002 — Reportes\n- **Tarea ID:** 002\n"), 0644)
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('002', 'reportes', ?, 'PLANNED')`, specDepUnverified)

	// Configurar lockfile
	testLock := SebLock{
		Tasks: map[string]TaskLock{
			"001_auth_jwt": {
				SpecHash:     "hash_001_spec",
				VerifierHash: "hash_001_verif",
				Verified:     true,
				CommitHash:   "commit123",
			},
			"002_reportes": {
				SpecHash:     "hash_002_spec",
				VerifierHash: "hash_002_verif",
				Verified:     false,
				CommitHash:   "",
			},
		},
	}

	targetTask := TaskInfo{
		ID:            "003",
		Name:          "panel_admin",
		CanonicalName: "003_panel_admin",
		SpecPath:      filepath.Join("specs", "003_panel_admin_spec.md"),
	}

	// Caso A: Dependencia satisfecha (001) -> Debe pasar
	errOk := validateDirectDependencies(db, targetTask, []string{"001"}, testLock)
	if errOk != nil {
		t.Errorf("Se esperaba exito para dependencia satisfecha 001, pero fallo: %v", errOk)
	}

	// Caso B: Dependencia no verificada (002) -> Debe fallar con mensaje claro
	errUnverified := validateDirectDependencies(db, targetTask, []string{"002"}, testLock)
	if errUnverified == nil {
		t.Errorf("Se esperaba fallo para dependencia no verificada 002, pero retorno nil")
	} else if !strings.Contains(errUnverified.Error(), "no ha completado su verificación determinista") {
		t.Errorf("Mensaje de error inesperado para tarea no verificada: %v", errUnverified)
	}

	// Caso C: Dependencia huerfana (099) -> Debe fallar reportando inexistencia
	errMissing := validateDirectDependencies(db, targetTask, []string{"099_inexistente"}, testLock)
	if errMissing == nil {
		t.Errorf("Se esperaba fallo para dependencia inexistente 099, pero retorno nil")
	} else if !strings.Contains(errMissing.Error(), "no existe en el repositorio") {
		t.Errorf("Mensaje de error inesperado para dependencia huerfana: %v", errMissing)
	}
}

// CHK-04 (RF-07): Deteccion y rechazo determinista de referencias circulares mutuas
func TestPlanLockCircularDependencyDetection(t *testing.T) {
	tmpDir := t.TempDir()

	specA := filepath.Join(tmpDir, "001_tarea_a_spec.md")
	contentA := `# Spec 001
- **Dependencias:** 002_tarea_b`
	_ = os.WriteFile(specA, []byte(contentA), 0644)

	specB := filepath.Join(tmpDir, "002_tarea_b_spec.md")
	contentB := `# Spec 002
- **Dependencias:** 001_tarea_a`
	_ = os.WriteFile(specB, []byte(contentB), 0644)

	taskA := TaskInfo{
		ID:            "001",
		Name:          "tarea_a",
		CanonicalName: "001_tarea_a",
		SpecPath:      specA,
	}
	taskB := TaskInfo{
		ID:            "002",
		Name:          "tarea_b",
		CanonicalName: "002_tarea_b",
		SpecPath:      specB,
	}

	// 1. Dependencia circular mutua: A depende de B, y B declara depender de A
	errCirc := detectDirectCircularDependency(taskA, taskB)
	if errCirc == nil {
		t.Errorf("Se esperaba error por referencia circular mutua entre A y B, pero retorno nil")
	} else if !strings.Contains(errCirc.Error(), "referencia circular mutua") {
		t.Errorf("Mensaje inesperado para dependencia circular: %v", errCirc)
	}

	// 2. Dependencia consigo misma: A declara depender de A
	errSelf := detectDirectCircularDependency(taskA, taskA)
	if errSelf == nil {
		t.Errorf("Se esperaba error por dependencia circular consigo misma, pero retorno nil")
	} else if !strings.Contains(errSelf.Error(), "consigo misma") {
		t.Errorf("Mensaje inesperado para auto-dependencia: %v", errSelf)
	}

	// 3. Dependencia normal (sin ciclo reciproco)
	specC := filepath.Join(tmpDir, "003_tarea_c_spec.md")
	contentC := `# Spec 003
- **Dependencias:** Ninguna`
	_ = os.WriteFile(specC, []byte(contentC), 0644)
	taskC := TaskInfo{
		ID:            "003",
		Name:          "tarea_c",
		CanonicalName: "003_tarea_c",
		SpecPath:      specC,
	}

	errOk := detectDirectCircularDependency(taskA, taskC)
	if errOk != nil {
		t.Errorf("Se esperaba exito en relacion lineal sin ciclo, pero fallo: %v", errOk)
	}
}

// CHK-05 (RF-08 a RF-14): Protocolo de Scope Fork, suspension y desbloqueo en seb-clarificador
func TestClarificadorSkillProtocolDAG(t *testing.T) {
	repoRoot := getRepoRootDir(t)

	// Verificar existencia e integridad de directivas en payload/.agents/skills/seb-clarificador/SKILL.md
	clarificadorPath := filepath.Join(repoRoot, "payload", ".agents", "skills", "seb-clarificador", "SKILL.md")
	content, err := os.ReadFile(clarificadorPath)
	if err != nil {
		t.Fatalf("Error leyendo %s: %v", clarificadorPath, err)
	}
	text := string(content)

	mandatoryTerms := []string{
		"BLOQUEADA_POR_DEPENDENCIA",
		"Bifurcación de Alcance",
		"Scope Fork",
		"Contrato de Desbloqueo Requerido",
		"Contrato de Desbloqueo (Satisfecho por Spec XXX)",
		"memory-add",
		"Deuda Técnica / Scope Fork",
		"spec-next",
	}

	for _, term := range mandatoryTerms {
		if !strings.Contains(text, term) {
			t.Errorf("seb-clarificador/SKILL.md carece del termino mandatorio: '%s'", term)
		}
	}

	// Verificar seb-planificador/SKILL.md
	planificadorPath := filepath.Join(repoRoot, "payload", ".agents", "skills", "seb-planificador", "SKILL.md")
	pContent, err := os.ReadFile(planificadorPath)
	if err != nil {
		t.Fatalf("Error leyendo %s: %v", planificadorPath, err)
	}
	pText := string(pContent)
	if !strings.Contains(pText, "BLOQUEADA_POR_DEPENDENCIA") || !strings.Contains(pText, "verified: true") {
		t.Errorf("seb-planificador/SKILL.md no contiene los guardarrailes de dependencias DAG")
	}

	// Verificar regla global 00-seb-harness.md
	rulePath := filepath.Join(repoRoot, "payload", ".agents", "rules", "00-seb-harness.md")
	rContent, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("Error leyendo %s: %v", rulePath, err)
	}
	rText := string(rContent)
	if !strings.Contains(rText, "Integridad del Grafo de Dependencias (DAG)") || !strings.Contains(rText, "verified: true") {
		t.Errorf("00-seb-harness.md no contiene la regla de DAG")
	}
}

// CHK-06 (RF-15, RF-16): Observabilidad de dependencias y alertas en seb status
func TestStatusDisplaysDependencies(t *testing.T) {
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
		t.Fatalf("Error creando specs/: %v", err)
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Error abriendo sqlite: %v", err)
	}
	defer db.Close()
	if err := initDBSchema(db); err != nil {
		t.Fatalf("Error inicializando esquema: %v", err)
	}

	// Crear spec 001 verificada
	spec001 := filepath.Join("specs", "001_base_spec.md")
	_ = os.WriteFile(spec001, []byte("# Spec 001\n- **Dependencias:** Ninguna\n"), 0644)
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('001', 'base', ?, 'VERIFIED')`, spec001)

	// Crear spec 002 con dependencia 001
	spec002 := filepath.Join("specs", "002_consumidor_spec.md")
	_ = os.WriteFile(spec002, []byte("# Spec 002\n- **Dependencias:** 001_base\n"), 0644)
	_, _ = db.Exec(`INSERT INTO tasks (id, name, spec_path, status) VALUES ('002', 'consumidor', ?, 'PLANNED')`, spec002)

	// Lockfile con 001 verificada
	lock := SebLock{
		Tasks: map[string]TaskLock{
			"001_base": {
				SpecHash:   "h1",
				Verified:   true,
				CommitHash: "c1",
			},
		},
	}
	writeLockFile(lock)

	// Comprobar que parseSpecDependencies extrae las dependencias y se formatean como [OK]
	deps, err := parseSpecDependencies(spec002)
	if err != nil || len(deps) != 1 || deps[0] != "001_base" {
		t.Fatalf("Fallo extrayendo dependencias de spec002: %v", deps)
	}

	depTask, err := resolveTask(db, deps[0])
	if err != nil {
		t.Fatalf("resolveTask fallo para %s: %v", deps[0], err)
	}
	tLock := lock.Tasks[depTask.CanonicalName]
	if !tLock.Verified || tLock.CommitHash == "" {
		t.Errorf("Se esperaba que 001_base figure como verificada en el status")
	}
}

// CHK-07 (RNF-01, RNF-02, RNF-03): Requisitos no funcionales del DAG
func TestDAGNonFunctionalRequirements(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error obteniendo directorio actual: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Error cambiando a directorio temporal: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.MkdirAll("specs", 0755); err != nil {
		t.Fatalf("Error creando specs/: %v", err)
	}

	// 1. RNF-03: Multi-Maquina con db = nil (clon limpio sin SQLite previo)
	specPath := filepath.Join("specs", "001_auth_spec.md")
	_ = os.WriteFile(specPath, []byte("# Spec 001\n"), 0644)

	lock := SebLock{
		Tasks: map[string]TaskLock{
			"001_auth": {
				SpecHash:     "h",
				VerifierHash: "v",
				Verified:     true,
				CommitHash:   "commit123",
			},
		},
	}

	target := TaskInfo{
		ID:            "002",
		Name:          "dashboard",
		CanonicalName: "002_dashboard",
		SpecPath:      filepath.Join("specs", "002_dashboard_spec.md"),
	}

	errOffline := validateDirectDependencies(nil, target, []string{"001_auth"}, lock)
	if errOffline != nil {
		t.Errorf("RNF-03 Violado: Fallo la validacion offline sin base de datos SQLite: %v", errOffline)
	}

	// 2. RNF-01: Rendimiento < 50ms para la comprobacion del DAG
	start := time.Now()
	for i := 0; i < 100; i++ {
		_ = validateDirectDependencies(nil, target, []string{"001_auth"}, lock)
	}
	dur := time.Since(start)
	if dur > 50*time.Millisecond {
		t.Errorf("RNF-01 Violado: 100 comprobaciones tomaron %v (limite 50ms)", dur)
	}

	// 3. RNF-02: Retrocompatibilidad con specs que no declaran dependencias
	legacyContent := `# Spec 001 — Registro
- **Tarea ID:** 001
- **Identificador:** registro_telemetria
## 1. Contexto`
	legacyPath := filepath.Join(tmpDir, "specs", "001_legacy_spec.md")
	_ = os.WriteFile(legacyPath, []byte(legacyContent), 0644)

	legacyDeps, err := parseSpecDependencies(legacyPath)
	if err != nil {
		t.Errorf("RNF-02 Violado: Fallo al parsear spec legacy: %v", err)
	}
	if len(legacyDeps) != 0 {
		t.Errorf("RNF-02 Violado: Se esperaban 0 dependencias para spec legacy, got %v", legacyDeps)
	}
}
