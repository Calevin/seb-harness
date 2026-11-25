package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	managedBeginMarker = "<!-- BEGIN seb-harness:managed -->"
	managedEndMarker   = "<!-- END seb-harness:managed -->"
)

// extractManagedBlockLines extrae el bloque delimitado por los marcadores de gestion.
func extractManagedBlockLines(t *testing.T, content string) []string {
	t.Helper()
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	startIdx := -1
	endIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == managedBeginMarker {
			startIdx = i
		}
		if strings.TrimSpace(line) == managedEndMarker {
			endIdx = i
		}
	}
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		t.Fatalf("No se encontro un bloque gestionado valido en el contenido:\n%s", content)
	}
	return lines[startIdx : endIdx+1]
}

// 017-RF-01, 017-RF-02, 017-RF-03, 017-RF-08, 017-RF-12, 017-RNF-02
func TestManagedProtocolDistributionAndSize(t *testing.T) {
	repoRoot := getRepoRootDir(t)

	// 1. Validar protocolo SDD completo en payload/.agents/SEB-HARNESS.md
	protocolPath := filepath.Join(repoRoot, "payload", ".agents", "SEB-HARNESS.md")
	protocolBytes, err := os.ReadFile(protocolPath)
	if err != nil {
		t.Fatalf("No se pudo leer payload/.agents/SEB-HARNESS.md: %v", err)
	}
	protocolContent := string(protocolBytes)
	requiredProtocolLiterals := []string{
		"GESTIONADO POR seb-harness",
		"Fase 6 — Documentación (`seb-documentador`)",
		"`seb-documentador`",
		"docs/use-cases/*.md",
		"BORRADOR",
		"SPECIFIED",
		"seb.lock",
		"seb status",
		"seb help",
		"identificador prefijado `<NNN>-<TIPO>-<NN>`",
		"Operaciones One-Off",
		"rol `migrador`",
	}
	for _, lit := range requiredProtocolLiterals {
		if !strings.Contains(protocolContent, lit) {
			t.Errorf("payload/.agents/SEB-HARNESS.md debe contener %q", lit)
		}
	}

	// 2. Validar enrutador liviano en payload/AGENTS.md y limite de 30 lineas en bloque gestionado
	agentsTplPath := filepath.Join(repoRoot, "payload", "AGENTS.md")
	agentsTplBytes, err := os.ReadFile(agentsTplPath)
	if err != nil {
		t.Fatalf("No se pudo leer payload/AGENTS.md: %v", err)
	}
	agentsTpl := string(agentsTplBytes)
	blockLines := extractManagedBlockLines(t, agentsTpl)
	if len(blockLines) > 30 {
		t.Errorf("El bloque gestionado de payload/AGENTS.md excede 30 lineas: tiene %d", len(blockLines))
	}
	blockText := strings.Join(blockLines, "\n")
	for _, lit := range []string{"seb-harness", "docs/constitution.md", "MEMORY.md", ".agents/SEB-HARNESS.md"} {
		if !strings.Contains(blockText, lit) {
			t.Errorf("El bloque gestionado de payload/AGENTS.md debe contener %q", lit)
		}
	}

	// 3. Validar regla global reducida <= 35 lineas (017-RF-08, 017-RNF-02)
	rulesPath := filepath.Join(repoRoot, "payload", ".agents", "rules", "00-seb-harness.md")
	rulesBytes, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("No se pudo leer payload/.agents/rules/00-seb-harness.md: %v", err)
	}
	rulesContent := string(rulesBytes)
	rulesLineCount := len(strings.Split(strings.TrimRight(rulesContent, "\n"), "\n"))
	if rulesLineCount > 35 {
		t.Errorf("payload/.agents/rules/00-seb-harness.md excede 35 lineas: tiene %d", rulesLineCount)
	}
	for _, lit := range []string{"AGENTS.md", "docs/constitution.md", "MEMORY.md", ".agents/SEB-HARNESS.md"} {
		if !strings.Contains(rulesContent, lit) {
			t.Errorf("00-seb-harness.md debe referenciar %q", lit)
		}
	}

	// 4. Validar stubs en seb-iniciador apuntan a AGENTS.md sin acoplarse a .agents/SEB-HARNESS.md (017-RF-12)
	iniciadorBytes, err := os.ReadFile(filepath.Join(repoRoot, "payload", ".agents", "skills", "seb-iniciador", "SKILL.md"))
	if err != nil {
		t.Fatalf("No se pudo leer SKILL.md de seb-iniciador: %v", err)
	}
	iniciadorStr := string(iniciadorBytes)
	if !strings.Contains(iniciadorStr, "[AGENTS.md](AGENTS.md)") {
		t.Errorf("seb-iniciador debe mantener la referencia de stubs a AGENTS.md")
	}
	if strings.Contains(iniciadorStr, "SEB-HARNESS.md") {
		t.Errorf("Los stubs de seb-iniciador no deben acoplarse directamente a SEB-HARNESS.md")
	}

	// 5. Validar distribucion y sobrescritura automatica de .agents/SEB-HARNESS.md (017-RF-02)
	targetDir := t.TempDir()
	code, stdout, stderr := runInstall(t, repoRoot, "--no-build", targetDir)
	if code != 0 {
		t.Fatalf("install.sh fallo (%d): %s\n%s", code, stdout, stderr)
	}
	installedProtocol := filepath.Join(targetDir, ".agents", "SEB-HARNESS.md")
	gotProtocol, err := os.ReadFile(installedProtocol)
	if err != nil {
		t.Fatalf("install.sh no distribuyo .agents/SEB-HARNESS.md: %v", err)
	}
	if string(gotProtocol) != protocolContent {
		t.Fatalf(".agents/SEB-HARNESS.md instalado difiere de payload/.agents/SEB-HARNESS.md")
	}

	// Modificar localmente .agents/SEB-HARNESS.md y re-ejecutar install.sh: debe sobrescribirse
	if err := os.WriteFile(installedProtocol, []byte("contenido obsoleto"), 0644); err != nil {
		t.Fatalf("Error modificando protocolo local: %v", err)
	}
	code2, stdout2, stderr2 := runInstall(t, repoRoot, "--no-build", targetDir)
	if code2 != 0 {
		t.Fatalf("Segunda ejecucion de install.sh fallo (%d): %s\n%s", code2, stdout2, stderr2)
	}
	gotProtocolAfter, err := os.ReadFile(installedProtocol)
	if err != nil {
		t.Fatalf("No se pudo releer .agents/SEB-HARNESS.md: %v", err)
	}
	if string(gotProtocolAfter) != protocolContent {
		t.Fatalf("install.sh no sobrescribio .agents/SEB-HARNESS.md con la version vigente")
	}
}

// 017-RF-04, 017-RF-05, 017-RNF-01, 017-RNF-03, 017-CL-01, 017-CL-03
func TestManagedBlockUpdateAndIdempotency(t *testing.T) {
	repoRoot := getRepoRootDir(t)
	expectedBlock := strings.Join(extractManagedBlockLines(t, mustRead(t, filepath.Join(repoRoot, "payload", "AGENTS.md"))), "\n")

	// 1. Reemplazo literal preservando texto usuario antes y despues con caracteres especiales (017-RF-04, 017-CL-01)
	targetDir := t.TempDir()
	headerUser := "# Notas Previas del Proyecto\nVariables: $HOME $(whoami) & \\1 /ruta/a/archivo `echo ok`\n\n"
	footerUser := "\n\n## Reglas Locales\nConservar $PATH, &amp;, \\n y `make test` sin expansion.\n"
	staleAgents := headerUser + managedBeginMarker + "\ncontenido viejo del arnes\n" + managedEndMarker + footerUser

	agentsPath := filepath.Join(targetDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(staleAgents), 0644); err != nil {
		t.Fatalf("Error escribiendo AGENTS.md inicial: %v", err)
	}

	code, stdout, stderr := runInstall(t, repoRoot, "--no-build", targetDir)
	if code != 0 {
		t.Fatalf("install.sh fallo (%d): %s\n%s", code, stdout, stderr)
	}

	wantAgents := headerUser + expectedBlock + footerUser
	gotAgents := mustRead(t, agentsPath)
	if gotAgents != wantAgents {
		t.Fatalf("AGENTS.md actualizado no coincide exactamente.\n--- ESPERADO ---\n%q\n--- OBTENIDO ---\n%q", wantAgents, gotAgents)
	}

	// 2. Idempotencia en segunda ejecucion consecutiva (017-RNF-01)
	code2, stdout2, stderr2 := runInstall(t, repoRoot, "--no-build", targetDir)
	if code2 != 0 {
		t.Fatalf("Segunda ejecucion fallo (%d): %s\n%s", code2, stdout2, stderr2)
	}
	gotAgents2 := mustRead(t, agentsPath)
	if gotAgents2 != gotAgents {
		t.Fatalf("Segunda ejecucion altero AGENTS.md (no idempotente):\n%q\nvs\n%q", gotAgents, gotAgents2)
	}

	// 3. Archivo AGENTS.md que solo contiene el bloque gestionado sin lineas acumulativas (017-CL-03)
	targetOnlyBlock := t.TempDir()
	onlyBlockAgents := filepath.Join(targetOnlyBlock, "AGENTS.md")
	if err := os.WriteFile(onlyBlockAgents, []byte(managedBeginMarker+"\nviejo\n"+managedEndMarker+"\n"), 0644); err != nil {
		t.Fatalf("Error escribiendo AGENTS.md solo bloque: %v", err)
	}
	if code, _, errOut := runInstall(t, repoRoot, "--no-build", targetOnlyBlock); code != 0 {
		t.Fatalf("Fallo actualizacion de solo bloque: %s", errOut)
	}
	firstPass := mustRead(t, onlyBlockAgents)
	if firstPass != expectedBlock+"\n" {
		t.Fatalf("Solo bloque debe quedar exacto sin lineas extra.\nEsperado: %q\nObtenido: %q", expectedBlock+"\n", firstPass)
	}
	if code, _, errOut := runInstall(t, repoRoot, "--no-build", targetOnlyBlock); code != 0 {
		t.Fatalf("Fallo segunda actualizacion de solo bloque: %s", errOut)
	}
	secondPass := mustRead(t, onlyBlockAgents)
	if secondPass != firstPass {
		t.Fatalf("Se acumularon diferencias en segunda ejecucion de solo bloque: %q vs %q", firstPass, secondPass)
	}
}

// 017-RF-06, 017-RF-09, 017-CL-04
func TestLegacyAgentsMigrationAndNoPatch(t *testing.T) {
	repoRoot := getRepoRootDir(t)
	targetDir := t.TempDir()

	legacyContent := "# Protocolo Operativo: seb-harness (Legado)\nReglas antiguas sin marcadores.\n"
	agentsPath := filepath.Join(targetDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(legacyContent), 0644); err != nil {
		t.Fatalf("Error escribiendo AGENTS.md legado: %v", err)
	}

	code, stdout, stderr := runInstall(t, repoRoot, "--no-build", targetDir)
	if code != 0 {
		t.Fatalf("install.sh fallo en migracion legada (%d): %s\n%s", code, stdout, stderr)
	}

	// Verificar respaldo en AGENTS.legacy-seb.md
	legacyBakPath := filepath.Join(targetDir, "AGENTS.legacy-seb.md")
	gotBak, err := os.ReadFile(legacyBakPath)
	if err != nil {
		t.Fatalf("No se creo respaldo AGENTS.legacy-seb.md: %v", err)
	}
	if string(gotBak) != legacyContent {
		t.Fatalf("AGENTS.legacy-seb.md no conserva el contenido legado original:\n%s", string(gotBak))
	}
	if !strings.Contains(stdout, "AGENTS.legacy-seb.md") {
		t.Errorf("La salida de install.sh debe informar la ruta del respaldo AGENTS.legacy-seb.md:\n%s", stdout)
	}

	// Verificar que AGENTS.md fue reemplazado por el enrutador con marcadores
	newAgents := mustRead(t, agentsPath)
	if !strings.Contains(newAgents, managedBeginMarker) || !strings.Contains(newAgents, managedEndMarker) {
		t.Fatalf("AGENTS.md migrado no contiene marcadores de bloque gestionado:\n%s", newAgents)
	}

	// Verificar que NO se genera AGENTS.md.patch.md (017-RF-09, 017-CL-04)
	patchPath := filepath.Join(targetDir, "AGENTS.md.patch.md")
	if _, err := os.Stat(patchPath); !os.IsNotExist(err) {
		t.Fatalf("No debe generarse AGENTS.md.patch.md")
	}

	// Si ya existe AGENTS.legacy-seb.md y vuelve a aparecer un AGENTS.md legado, no sobrescribir el respaldo previo
	if err := os.WriteFile(agentsPath, []byte("# Otro legado seb-harness\n"), 0644); err != nil {
		t.Fatalf("Error reescribiendo AGENTS.md legado: %v", err)
	}
	code2, _, stderr2 := runInstall(t, repoRoot, "--no-build", targetDir)
	if code2 != 0 {
		t.Fatalf("Segunda migracion fallo (%d): %s", code2, stderr2)
	}
	gotBakAfter := mustRead(t, legacyBakPath)
	if gotBakAfter != legacyContent {
		t.Fatalf("AGENTS.legacy-seb.md existente fue sobrescrito indebidamente")
	}
	if _, err := os.Stat(patchPath); !os.IsNotExist(err) {
		t.Fatalf("No debe generarse AGENTS.md.patch.md en re-ejecuciones")
	}
}

// 017-RF-07, 017-CL-02
func TestMalformedMarkersAbortInstall(t *testing.T) {
	repoRoot := getRepoRootDir(t)

	cases := []struct {
		name    string
		content string
	}{
		{
			name:    "begin_sin_end",
			content: "# Proyecto\n" + managedBeginMarker + "\nsin cierre\n",
		},
		{
			name:    "end_sin_begin",
			content: "# Proyecto\nsin apertura\n" + managedEndMarker + "\n",
		},
		{
			name:    "orden_invertido",
			content: managedEndMarker + "\ncontenido\n" + managedBeginMarker + "\n",
		},
		{
			name:    "multiples_pares",
			content: managedBeginMarker + "\na\n" + managedEndMarker + "\n" + managedBeginMarker + "\nb\n" + managedEndMarker + "\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			targetDir := t.TempDir()
			agentsPath := filepath.Join(targetDir, "AGENTS.md")
			if err := os.WriteFile(agentsPath, []byte(tc.content), 0644); err != nil {
				t.Fatalf("Error escribiendo caso %s: %v", tc.name, err)
			}

			code, _, stderr := runInstall(t, repoRoot, "--no-build", targetDir)
			if code != 1 {
				t.Fatalf("Se esperaba exit code 1 para %s, se obtuvo %d", tc.name, code)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Errorf("Se esperaba mensaje diagnostico en stderr para %s", tc.name)
			}
			gotAfter := mustRead(t, agentsPath)
			if gotAfter != tc.content {
				t.Fatalf("AGENTS.md fue modificado indebidamente en caso malformado %s", tc.name)
			}
		})
	}
}

// 017-RF-10, 017-CL-01, 017-CL-02, 017-CL-05
func TestUninstallManagedBlockAndProtocol(t *testing.T) {
	repoRoot := getRepoRootDir(t)

	// Caso A: Desinstalacion con contenido propio antes y despues del bloque gestionado
	targetWithUser := t.TempDir()
	if code, _, errOut := runInstall(t, repoRoot, "--no-build", targetWithUser); code != 0 {
		t.Fatalf("Fallo instalacion previa: %s", errOut)
	}
	userTop := "# Top Proyecto\nExpr: $HOME & \\1 /dir `cmd`\n"
	userBottom := "## Notas Finales\nNo tocar $VAR ni &.\n"
	agentsPath := filepath.Join(targetWithUser, "AGENTS.md")
	installedBlock := strings.Join(extractManagedBlockLines(t, mustRead(t, agentsPath)), "\n")
	if err := os.WriteFile(agentsPath, []byte(userTop+installedBlock+"\n"+userBottom), 0644); err != nil {
		t.Fatalf("Error preparando AGENTS.md con contenido propio: %v", err)
	}

	// Dry-run no debe modificar nada
	codeDry, outDry, _ := runUninstall(t, repoRoot, "", "--dry-run", targetWithUser)
	if codeDry != 0 {
		t.Fatalf("uninstall.sh --dry-run fallo con codigo %d", codeDry)
	}
	if !strings.Contains(outDry, "[dry-run]") {
		t.Errorf("Salida de dry-run debe incluir prefijo [dry-run]:\n%s", outDry)
	}
	if _, err := os.Stat(filepath.Join(targetWithUser, ".agents", "SEB-HARNESS.md")); err != nil {
		t.Fatalf("dry-run no debe eliminar .agents/SEB-HARNESS.md")
	}
	if !strings.Contains(mustRead(t, agentsPath), managedBeginMarker) {
		t.Fatalf("dry-run no debe modificar AGENTS.md")
	}

	// Desinstalacion real sin --purge
	codeUn, _, errUn := runUninstall(t, repoRoot, "", targetWithUser)
	if codeUn != 0 {
		t.Fatalf("uninstall.sh fallo con codigo %d: %s", codeUn, errUn)
	}
	if _, err := os.Stat(filepath.Join(targetWithUser, ".agents", "SEB-HARNESS.md")); !os.IsNotExist(err) {
		t.Fatalf(".agents/SEB-HARNESS.md debio ser eliminado al desinstalar")
	}
	afterAgents := mustRead(t, agentsPath)
	if strings.Contains(afterAgents, managedBeginMarker) || strings.Contains(afterAgents, managedEndMarker) {
		t.Fatalf("El bloque gestionado debio removerse de AGENTS.md:\n%s", afterAgents)
	}
	if !strings.Contains(afterAgents, "Expr: $HOME & \\1 /dir `cmd`") || !strings.Contains(afterAgents, "No tocar $VAR ni &.") {
		t.Fatalf("Se perdio o altero contenido propio del proyecto al desinstalar:\n%s", afterAgents)
	}

	// Caso B: AGENTS.md solo con bloque gestionado -> debe eliminarse al quedar vacio (017-CL-05)
	targetOnlyBlock := t.TempDir()
	if code, _, errOut := runInstall(t, repoRoot, "--no-build", targetOnlyBlock); code != 0 {
		t.Fatalf("Fallo instalacion en targetOnlyBlock: %s", errOut)
	}
	onlyAgentsPath := filepath.Join(targetOnlyBlock, "AGENTS.md")
	if err := os.WriteFile(onlyAgentsPath, []byte(installedBlock+"\n"), 0644); err != nil {
		t.Fatalf("Error preparando AGENTS.md solo bloque: %v", err)
	}
	if code, _, errOut := runUninstall(t, repoRoot, "", "--dry-run", targetOnlyBlock); code != 0 {
		t.Fatalf("dry-run fallo en targetOnlyBlock: %s", errOut)
	}
	if _, err := os.Stat(onlyAgentsPath); err != nil {
		t.Fatalf("dry-run no debio eliminar AGENTS.md")
	}
	if code, _, errOut := runUninstall(t, repoRoot, "", targetOnlyBlock); code != 0 {
		t.Fatalf("uninstall fallo en targetOnlyBlock: %s", errOut)
	}
	if _, err := os.Stat(onlyAgentsPath); !os.IsNotExist(err) {
		t.Fatalf("AGENTS.md sin contenido propio debio eliminarse al desinstalar")
	}

	// Caso C: AGENTS.md con marcadores malformados -> preservar intacto y advertir (017-CL-02)
	targetMalformed := t.TempDir()
	malformedContent := "# Propio\n" + managedBeginMarker + "\nsin cierre\n"
	malformedPath := filepath.Join(targetMalformed, "AGENTS.md")
	if err := os.WriteFile(malformedPath, []byte(malformedContent), 0644); err != nil {
		t.Fatalf("Error creando AGENTS.md malformado: %v", err)
	}
	codeM, _, errM := runUninstall(t, repoRoot, "", targetMalformed)
	if codeM != 0 {
		t.Fatalf("uninstall.sh debio completar sin abortar ante marcadores malformados, obtuvo %d: %s", codeM, errM)
	}
	if strings.TrimSpace(errM) == "" {
		t.Errorf("uninstall.sh debio emitir advertencia en stderr ante marcadores malformados")
	}
	if mustRead(t, malformedPath) != malformedContent {
		t.Fatalf("uninstall.sh altero AGENTS.md con marcadores malformados")
	}
}

// 017-RF-11, 017-RNF-01
func TestSelfModeUpdatesRootAgents(t *testing.T) {
	repoRoot := getRepoRootDir(t)

	// Replicar estructura minima del repo seb-harness en directorio temporal
	tempHarness := t.TempDir()
	if err := exec.Command("git", "init", tempHarness).Run(); err != nil {
		t.Fatalf("Fallo git init en replica: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tempHarness, "engine", "cmd", "seb"), 0755); err != nil {
		t.Fatalf("Error creando engine/cmd/seb: %v", err)
	}
	// Copiar install.sh y payload/
	installBytes := mustRead(t, filepath.Join(repoRoot, "install.sh"))
	if err := os.WriteFile(filepath.Join(tempHarness, "install.sh"), []byte(installBytes), 0755); err != nil {
		t.Fatalf("Error copiando install.sh: %v", err)
	}
	if err := exec.Command("cp", "-R", filepath.Join(repoRoot, "payload"), filepath.Join(tempHarness, "payload")).Run(); err != nil {
		t.Fatalf("Error copiando payload/: %v", err)
	}

	// Preparar AGENTS.md raiz con bloque desactualizado y nota propia
	customFooter := "\n## Nota Dogfooding\nMantener sincronizado.\n"
	rootAgentsPath := filepath.Join(tempHarness, "AGENTS.md")
	if err := os.WriteFile(rootAgentsPath, []byte(managedBeginMarker+"\nviejo\n"+managedEndMarker+customFooter), 0644); err != nil {
		t.Fatalf("Error creando AGENTS.md raiz: %v", err)
	}

	code, stdout, stderr := runInstall(t, tempHarness, "--self", "--no-build")
	if code != 0 {
		t.Fatalf("install.sh --self fallo (%d): %s\n%s", code, stdout, stderr)
	}

	expectedBlock := strings.Join(extractManagedBlockLines(t, mustRead(t, filepath.Join(repoRoot, "payload", "AGENTS.md"))), "\n")
	wantRoot := expectedBlock + customFooter
	gotRoot := mustRead(t, rootAgentsPath)
	if gotRoot != wantRoot {
		t.Fatalf("install.sh --self no actualizo correctamente AGENTS.md raiz.\nEsperado:\n%q\nObtenido:\n%q", wantRoot, gotRoot)
	}

	// Idempotencia en segunda ejecucion --self
	code2, _, stderr2 := runInstall(t, tempHarness, "--self", "--no-build")
	if code2 != 0 {
		t.Fatalf("Segunda ejecucion --self fallo (%d): %s", code2, stderr2)
	}
	if mustRead(t, rootAgentsPath) != gotRoot {
		t.Fatalf("install.sh --self no es idempotente sobre AGENTS.md raiz")
	}

	// Marcadores malformados en --self deben abortar con exit 1
	malformed := managedBeginMarker + "\nsin fin\n"
	if err := os.WriteFile(rootAgentsPath, []byte(malformed), 0644); err != nil {
		t.Fatalf("Error escribiendo AGENTS.md malformado en --self: %v", err)
	}
	codeBad, _, errBad := runInstall(t, tempHarness, "--self", "--no-build")
	if codeBad != 1 {
		t.Fatalf("Se esperaba exit code 1 en --self con marcadores malformados, se obtuvo %d", codeBad)
	}
	if strings.TrimSpace(errBad) == "" {
		t.Errorf("Se esperaba diagnostico en stderr en --self con marcadores malformados")
	}
	if mustRead(t, rootAgentsPath) != malformed {
		t.Fatalf("install.sh --self altero AGENTS.md con marcadores malformados")
	}
}
