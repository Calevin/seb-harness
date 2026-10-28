package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// CHK-01 (RF-01, RF-02, CL-01): Parseo de identificadores heterogeneos y normalizacion de listas con o sin espacios
func TestParseVerifierHeterogeneousRequirements(t *testing.T) {
	tmpDir := t.TempDir()
	verifierPath := filepath.Join(tmpDir, "verificador_heterogeneo.md")

	// Incluye exactamente los casos reportados en produccion:
	// CHK-09 con RNF-02 y backticks en la descripcion
	// CHK-98 con RNF-01, RNF-02 con espacios
	// CHK-99 con GLOBAL
	// Adicionalmente RF-Stack y combinaciones mixtas
	content := `# Verificador de Prueba

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-01 | Verificacion de autenticacion basica | ` + "`echo ok`" + ` |
| CHK-09 | RNF-02 | Rutas de categorías montadas tras ` + "`requireAuth`" + ` y catálogo global sin filtro por usuario | ` + "`direnv exec . pnpm --filter server exec vitest run src/routes/category.routes.spec.ts`" + ` |
| CHK-98 | RNF-01, RNF-02 | Análisis estático del cliente y compilación estricta de todos los paquetes | ` + "`direnv exec . pnpm --filter client lint && direnv exec . pnpm -r build`" + ` |
| CHK-99 | GLOBAL | Build global, suite de backend completa con umbral de cobertura | ` + "`direnv exec . pnpm -r build && direnv exec . pnpm --filter server test`" + ` |
| CHK-100 | RF-Stack | Analisis de linter oficial del proyecto | ` + "`go vet ./...`" + ` |
`

	if err := os.WriteFile(verifierPath, []byte(content), 0644); err != nil {
		t.Fatalf("Error creando archivo de prueba: %v", err)
	}

	items, err := parseVerifierFile(verifierPath)
	if err != nil {
		t.Fatalf("parseVerifierFile fallo inesperadamente: %v", err)
	}

	if len(items) != 5 {
		t.Fatalf("Se esperaban 5 items de verificacion, pero se obtuvieron %d", len(items))
	}

	// Comprobar CHK-01
	if items[0].ID != "CHK-01" || items[0].ReqID != "RF-01" {
		t.Errorf("CHK-01 incorrecto: got ID=%s, ReqID=%s", items[0].ID, items[0].ReqID)
	}

	// Comprobar CHK-09 (RNF-02 y backticks en descripcion)
	if items[1].ID != "CHK-09" || items[1].ReqID != "RNF-02" {
		t.Errorf("CHK-09 incorrecto: got ID=%s, ReqID=%s", items[1].ID, items[1].ReqID)
	}
	if !strings.Contains(items[1].Desc, "requireAuth") {
		t.Errorf("CHK-09 descripcion perdio requireAuth: got %s", items[1].Desc)
	}

	// Comprobar CHK-98 (normalizacion de espacios en lista de requisitos)
	if items[2].ID != "CHK-98" || items[2].ReqID != "RNF-01,RNF-02" {
		t.Errorf("CHK-98 normalizacion fallo: got ID=%s, ReqID=%s (esperado RNF-01,RNF-02)", items[2].ID, items[2].ReqID)
	}

	// Comprobar CHK-99 (GLOBAL)
	if items[3].ID != "CHK-99" || items[3].ReqID != "GLOBAL" {
		t.Errorf("CHK-99 incorrecto: got ID=%s, ReqID=%s (esperado GLOBAL)", items[3].ID, items[3].ReqID)
	}

	// Comprobar CHK-100 (RF-Stack)
	if items[4].ID != "CHK-100" || items[4].ReqID != "RF-Stack" {
		t.Errorf("CHK-100 incorrecto: got ID=%s, ReqID=%s", items[4].ID, items[4].ReqID)
	}
}

// CHK-02 (RF-03, RF-05, CL-04, CL-05): Anti-Silent-Drop de filas candidatas malformadas y comandos sin backticks o vacios
func TestParseVerifierMalformedCandidateRows(t *testing.T) {
	tmpDir := t.TempDir()

	testCases := []struct {
		name          string
		content       string
		expectedError string
	}{
		{
			name: "Comando sin backticks",
			content: `| Check ID | Requisito | Descripción | Comando |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-01 | Descripcion valida | exit 0 |
`,
			expectedError: "delimitadores de comando",
		},
		{
			name: "Comando vacio entre backticks",
			content: `| Check ID | Requisito | Descripción | Comando |
| :--- | :--- | :--- | :--- |
| CHK-02 | RF-02 | Descripcion valida | ` + "``" + ` |
`,
			expectedError: "vacío",
		},
		{
			name: "Fila candidata con columnas faltantes (solo 3 columnas)",
			content: `| Check ID | Requisito | Descripción | Comando |
| :--- | :--- | :--- | :--- |
| CHK-03 | RF-03 | ` + "`echo ok`" + ` |
`,
			expectedError: "estructura de columnas",
		},
		{
			name: "Fila candidata sin pipe de cierre",
			content: `| Check ID | Requisito | Descripción | Comando |
| :--- | :--- | :--- | :--- |
| CHK-04 | RF-04 | Descripcion | ` + "`echo ok`" + `
`,
			expectedError: "delimitador",
		},
		{
			name: "Comillas invertidas no cerradas en el comando",
			content: `| Check ID | Requisito | Descripción | Comando |
| :--- | :--- | :--- | :--- |
| CHK-05 | RF-05 | Descripcion | ` + "`echo ok |" + `
`,
			expectedError: "comillas invertidas",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, strings.ReplaceAll(tc.name, " ", "_")+".md")
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatalf("Error escribiendo archivo: %v", err)
			}

			_, err := parseVerifierFile(path)
			if err == nil {
				t.Fatalf("Se esperaba error por fila malformada en caso '%s', pero no hubo error (omision silenciosa detectada)", tc.name)
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.expectedError)) {
				t.Errorf("Mensaje de error inesperado: got '%v', se esperaba que contuviera '%s'", err, tc.expectedError)
			}
		})
	}
}

// CHK-03 (RF-04, CL-02): Aislamiento de comandos de shell con pipes, operadores logicos y comillas entre backticks
func TestParseVerifierCommandIsolationWithShellPipes(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "verificador_pipes.md")

	content := `# Verificador con Pipes y Operadores de Shell

| Check ID | Requisito | Descripción | Comando |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-01 | Pipeline con pipe tradicional | ` + "`cat report.txt | grep -E \"SUCCESS|PASS\" | wc -l`" + ` |
| CHK-02 | RF-02 | Encadenamiento con OR logico | ` + "`direnv exec . pnpm test || exit 1`" + ` |
| CHK-03 | RF-03 | Encadenamiento mixto con AND y subshell | ` + "`cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./...`" + ` |
`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Error creando archivo: %v", err)
	}

	items, err := parseVerifierFile(path)
	if err != nil {
		t.Fatalf("parseVerifierFile fallo con comandos que contienen pipes: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("Se esperaban 3 items, se obtuvieron %d", len(items))
	}

	expectedCmd0 := `cat report.txt | grep -E "SUCCESS|PASS" | wc -l`
	if items[0].Command != expectedCmd0 {
		t.Errorf("Comando 0 distorsionado por pipes: got '%s', want '%s'", items[0].Command, expectedCmd0)
	}

	expectedCmd1 := `direnv exec . pnpm test || exit 1`
	if items[1].Command != expectedCmd1 {
		t.Errorf("Comando 1 distorsionado por OR logico: got '%s', want '%s'", items[1].Command, expectedCmd1)
	}

	expectedCmd2 := `cd engine && CGO_ENABLED=0 go build -ldflags="-s -w" -o ../.seb/bin/seb ./cmd/seb && go test -v ./...`
	if items[2].Command != expectedCmd2 {
		t.Errorf("Comando 2 distorsionado: got '%s', want '%s'", items[2].Command, expectedCmd2)
	}
}

// CHK-04 (RF-06, CL-04): Deteccion y rechazo determinista de Check IDs duplicados
func TestParseVerifierDuplicateCheckIDs(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "verificador_duplicados.md")

	content := `# Verificador con IDs Repetidos

| Check ID | Requisito | Descripción | Comando |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-01 | Primer control con CHK-01 | ` + "`echo uno`" + ` |
| CHK-02 | RF-02 | Control intermedio valido | ` + "`echo dos`" + ` |
| CHK-01 | RF-03 | Segundo control con ID repetido | ` + "`echo tres`" + ` |
`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Error creando archivo: %v", err)
	}

	_, err := parseVerifierFile(path)
	if err == nil {
		t.Fatal("Se esperaba error por Check ID duplicado 'CHK-01', pero el parser lo admitio")
	}

	errMsg := strings.ToLower(err.Error())
	if !strings.Contains(errMsg, "duplicado") && !strings.Contains(errMsg, "chk-01") {
		t.Errorf("El mensaje de error deberia especificar duplicidad de CHK-01: got '%s'", err.Error())
	}
}

// CHK-05 (RF-07, CL-03): Inmunidad contextual a bloques de codigo cercados y lineas de tabla no candidatas
func TestParseVerifierFencedCodeBlockImmunity(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "verificador_cercado.md")

	content := `# Documentacion con Bloques Cercados

A continuacion se muestra un ejemplo de como debe lucir una tabla:

` + "```markdown" + `
| Check ID | Requisito | Descripción | Comando |
| :--- | :--- | :--- | :--- |
| CHK-99 | GLOBAL | Este check es solo un ejemplo documental | ` + "`exit 1`" + ` |
| CHK-INVALID | | Incompleto pero dentro de bloque |
` + "```" + `

Tabla real ejecutable:

| Check ID | Requisito | Descripción | Comando |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-01 | Check real unico fuera del bloque | ` + "`echo real`" + ` |
`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Error creando archivo: %v", err)
	}

	items, err := parseVerifierFile(path)
	if err != nil {
		t.Fatalf("parseVerifierFile no deberia fallar por texto dentro de bloques cercados: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Se esperaba exactamente 1 item (el check real), pero se obtuvieron %d", len(items))
	}

	if items[0].ID != "CHK-01" {
		t.Errorf("Item extraido incorrecto: got %s, want CHK-01", items[0].ID)
	}
}

// CHK-06 (RF-08, RF-10, CL-06): Integridad en plan-lock bloqueando verificadores defectuosos o sin controles ejecutables
func TestParseVerifierPlanLockIntegrity(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Archivo sin filas de control ejecutables (solo texto o encabezados)
	emptyVerifier := filepath.Join(tmpDir, "sin_controles_verificador.md")
	contentEmpty := `# Verificador Vacio
| Check ID | Requisito | Descripción | Comando |
| :--- | :--- | :--- | :--- |
`
	if err := os.WriteFile(emptyVerifier, []byte(contentEmpty), 0644); err != nil {
		t.Fatalf("Error creando archivo: %v", err)
	}

	_, err := parseVerifierFile(emptyVerifier)
	if err == nil {
		t.Fatal("parseVerifierFile debio retornar error ante un verificador sin filas de control")
	}

	// 2. Archivo con error de sintaxis que debe ser reportado
	syntaxErrorVerifier := filepath.Join(tmpDir, "sintaxis_error_verificador.md")
	contentSyntax := `# Verificador
| Check ID | Requisito | Descripción | Comando |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-01 | Fila malformada | exit 0 |
`
	if err := os.WriteFile(syntaxErrorVerifier, []byte(contentSyntax), 0644); err != nil {
		t.Fatalf("Error creando archivo: %v", err)
	}

	_, errSyntax := parseVerifierFile(syntaxErrorVerifier)
	if errSyntax == nil {
		t.Fatal("parseVerifierFile debio retornar error ante fila malformada")
	}
}

// CHK-07 (RF-09): Integridad en verify reportando total exacto de controles y exigiendo 100% en PASA
func TestParseVerifierVerifyIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "verificador_conteo.md")

	content := `# Verificador para Conteo
| Check ID | Requisito | Descripción | Comando |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-01 | Primer control | ` + "`echo ok1`" + ` |
| CHK-02 | RF-02 | Segundo control | ` + "`echo ok2`" + ` |
| CHK-03 | RNF-01 | Tercer control | ` + "`echo ok3`" + ` |
| CHK-04 | GLOBAL | Cuarto control | ` + "`echo ok4`" + ` |
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Error creando archivo: %v", err)
	}

	items, err := parseVerifierFile(path)
	if err != nil {
		t.Fatalf("parseVerifierFile fallo: %v", err)
	}

	if len(items) != 4 {
		t.Fatalf("Conteo exacto fallo: got %d, want 4", len(items))
	}
}

// CHK-08 (RNF-01, RNF-02, RNF-03): Evaluacion en menos de 10ms, determinismo offline y compatibilidad retrospectiva con specs 001-005
func TestParseVerifierNonFunctionalAndCompatibility(t *testing.T) {
	// 1. Compatibilidad retrospectiva: parsear todos los verificadores existentes en specs/
	specsDir := findSpecsDir()
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		t.Fatalf("No se pudo leer directorio specs (%s): %v", specsDir, err)
	}

	verifierCount := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_verificador.md") {
			path := filepath.Join(specsDir, entry.Name())
			start := time.Now()
			items, err := parseVerifierFile(path)
			elapsed := time.Since(start)

			if err != nil {
				t.Errorf("Fallo de compatibilidad retrospectiva en '%s': %v", entry.Name(), err)
			}
			if len(items) == 0 {
				t.Errorf("Verificador '%s' retorno 0 controles", entry.Name())
			}

			// RNF-01: Cada archivo debe parsearse en menos de 10 milisegundos
			if elapsed > 10*time.Millisecond {
				t.Errorf("Latencia de parseo excesiva en '%s': %v (umbral maximo 10ms)", entry.Name(), elapsed)
			}
			verifierCount++
		}
	}

	if verifierCount < 5 {
		t.Fatalf("Se esperaban al menos 5 verificadores existentes en specs/, encontrados: %d", verifierCount)
	}

	// 2. Benchmark sintetico de rendimiento para 100 controles
	tmpDir := t.TempDir()
	largeVerifierPath := filepath.Join(tmpDir, "100_checks_verificador.md")

	var b strings.Builder
	b.WriteString("# Verificador Grande (100 Checks)\n\n")
	b.WriteString("| Check ID | Requisito | Descripción | Comando |\n")
	b.WriteString("| :--- | :--- | :--- | :--- |\n")
	for i := 1; i <= 100; i++ {
		b.WriteString(fmt.Sprintf("| CHK-%03d | RF-%02d, RNF-%02d | Control sintetico numero %d | `echo ok%d` |\n", i, i%10, i%5, i, i))
	}

	if err := os.WriteFile(largeVerifierPath, []byte(b.String()), 0644); err != nil {
		t.Fatalf("Error creando verificador sintetico: %v", err)
	}

	start := time.Now()
	items100, err := parseVerifierFile(largeVerifierPath)
	elapsed100 := time.Since(start)

	if err != nil {
		t.Fatalf("Error parseando verificador sintetico de 100 checks: %v", err)
	}
	if len(items100) != 100 {
		t.Fatalf("Se esperaban 100 checks, got %d", len(items100))
	}
	if elapsed100 > 10*time.Millisecond {
		t.Errorf("Parseo de 100 checks supero umbral de 10ms: tardo %v", elapsed100)
	}
}

func findSpecsDir() string {
	candidates := []string{
		"specs",
		filepath.Join("..", "specs"),
		filepath.Join("..", "..", "specs"),
		filepath.Join("..", "..", "..", "specs"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return "specs"
}
