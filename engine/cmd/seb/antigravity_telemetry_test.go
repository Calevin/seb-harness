package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// CHK-01 (RF-05, RF-13, CL-01): Migracion idempotente de cache_read_tokens, exclusion de costo y reporte
func TestCacheReadTokensAndMigration(t *testing.T) {
	// 1. Crear base de datos con esquema viejo (sin cache_read_tokens ni step_idx)
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Error abriendo sqlite en memoria: %v", err)
	}
	defer db.Close()

	legacySchema := `
	CREATE TABLE agent_actions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		task_id TEXT DEFAULT '',
		agent TEXT NOT NULL,
		skill TEXT DEFAULT '',
		action TEXT NOT NULL,
		parameters TEXT DEFAULT '',
		result_summary TEXT DEFAULT '',
		level TEXT NOT NULL DEFAULT 'autonomous',
		duration_ms INTEGER DEFAULT 0,
		tokens_in INTEGER DEFAULT 0,
		tokens_out INTEGER DEFAULT 0,
		cost_usd REAL DEFAULT 0.0,
		exit_code INTEGER DEFAULT 0,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatalf("Error creando esquema legacy: %v", err)
	}

	// Insertar fila historica
	if _, err := db.Exec(`INSERT INTO agent_actions (session_id, task_id, agent, action, tokens_in, tokens_out) VALUES ('legacy-sess', '001', 'agy', 'read_file', 100, 50)`); err != nil {
		t.Fatalf("Error insertando fila legacy: %v", err)
	}

	// 2. Ejecutar initDBSchema (debe migrar de forma idempotente dos veces sin fallar ni borrar datos)
	if err := initDBSchema(db); err != nil {
		t.Fatalf("Primera llamada a initDBSchema fallo: %v", err)
	}
	if err := initDBSchema(db); err != nil {
		t.Fatalf("Segunda llamada a initDBSchema (idempotencia) fallo: %v", err)
	}

	// Verificar que la fila historica conserva sus datos y tiene cache_read_tokens = 0
	var tokIn, cacheRead, tokOut int64
	if err := db.QueryRow(`SELECT tokens_in, cache_read_tokens, tokens_out FROM agent_actions WHERE session_id = 'legacy-sess'`).Scan(&tokIn, &cacheRead, &tokOut); err != nil {
		t.Fatalf("Error consultando cache_read_tokens tras migracion: %v", err)
	}
	if tokIn != 100 || cacheRead != 0 || tokOut != 50 {
		t.Fatalf("Valores inesperados tras migracion: in=%d, cache=%d, out=%d", tokIn, cacheRead, tokOut)
	}

	// 3. Verificar que cache_read_tokens se persiste y se excluye del calculo de cost_usd (RF-05)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	cfg := EngineConfig{
		Pricing: map[string]ModelPricing{
			"gemini-4-argon": {
				PromptPricePerMillion:     2.0,
				CompletionPricePerMillion: 10.0,
			},
		},
	}
	cfgBytes, _ := json.Marshal(cfg)
	_ = os.WriteFile(configPath, cfgBytes, 0644)

	// 500,000 tokens_in ($1.0) + 100,000 tokens_out ($1.0) = $2.0 (ignorando 5,000,000 cache_read_tokens)
	cost := calculateCost(0.0, "gemini-4-argon", 500000, 100000, configPath)
	if cost < 1.9999 || cost > 2.0001 {
		t.Fatalf("El costo calculado debe ser 2.0 excluyendo cache, se obtuvo: %f", cost)
	}

	_, err = db.Exec(`INSERT INTO agent_actions (session_id, task_id, agent, skill, action, tokens_in, cache_read_tokens, tokens_out, cost_usd)
		VALUES ('s2', '004', 'gemini-4-argon', 'implementador', 'edit_file', 500, 12000, 150, ?)`, cost)
	if err != nil {
		t.Fatalf("Error insertando fila con cache_read_tokens: %v", err)
	}

	// 4. Verificar que el reporte consolidado expone la sumatoria de cache_read_tokens
	reportOut := renderAuditReport(db)
	if !strings.Contains(reportOut, "Cache Read") || !strings.Contains(reportOut, "12000") {
		t.Fatalf("El reporte de auditoria no incluye la columna o sumatoria de Cache Read (12000):\n%s", reportOut)
	}
}

// CHK-02 (RF-06, RF-07, RNF-02, CL-03): Saneamiento de rutas del workspace, <EXTERNAL_PATH>/<base> y truncamiento a 256 chars
func TestParameterSanitizationAndTruncation(t *testing.T) {
	workspaceRoot := "/home/calevin/Github/seb-harness"
	longContent := strings.Repeat("A", 300)

	args := map[string]interface{}{
		"Path":        "/home/calevin/Github/seb-harness/specs/004_captura_telemetria_antigravity_spec.md",
		"External":    "/home/calevin/.ssh/id_ed25519",
		"OtherHost":   "/etc/passwd",
		"Content":     longContent,
		"Short":       "normal_value",
		"CommandLine": "cat /home/calevin/Github/seb-harness/README.md /home/calevin/.gemini/secret.json",
	}

	sanitizedJSON := sanitizeToolParams(args, workspaceRoot)

	// RNF-02: Prohibido exponer rutas absolutas del host en el resultado
	if strings.Contains(sanitizedJSON, "/home/calevin") || strings.Contains(sanitizedJSON, "/etc/passwd") {
		t.Fatalf("El saneamiento filtro rutas absolutas del host: %s", sanitizedJSON)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(sanitizedJSON), &parsed); err != nil {
		t.Fatalf("El resultado saneado no es JSON valido: %v -> %s", err, sanitizedJSON)
	}

	// RF-06: Ruta dentro del workspace convertida a relativa limpia
	if parsed["Path"] != "specs/004_captura_telemetria_antigravity_spec.md" {
		t.Errorf("Se esperaba ruta relativa 'specs/004_captura_telemetria_antigravity_spec.md', se obtuvo '%v'", parsed["Path"])
	}

	// RF-06: Ruta externa reemplazada por <EXTERNAL_PATH>/<nombre_base>
	if parsed["External"] != "<EXTERNAL_PATH>/id_ed25519" {
		t.Errorf("Se esperaba '<EXTERNAL_PATH>/id_ed25519', se obtuvo '%v'", parsed["External"])
	}
	if parsed["OtherHost"] != "<EXTERNAL_PATH>/passwd" {
		t.Errorf("Se esperaba '<EXTERNAL_PATH>/passwd', se obtuvo '%v'", parsed["OtherHost"])
	}

	// RF-06 en comandos embebidos
	expectedCmd := "cat README.md <EXTERNAL_PATH>/secret.json"
	if parsed["CommandLine"] != expectedCmd {
		t.Errorf("Se esperaba comando saneado '%s', se obtuvo '%v'", expectedCmd, parsed["CommandLine"])
	}

	// RF-07 y CL-03: Truncamiento exacto a 256 caracteres + ...[TRUNCATED]
	expectedTrunc := strings.Repeat("A", 256) + "...[TRUNCATED]"
	if parsed["Content"] != expectedTrunc {
		t.Errorf("Truncamiento incorrecto: longitud=%d, valor='%v'", len(parsed["Content"].(string)), parsed["Content"])
	}
}

// CHK-03 (RF-11): Inferencia determinista de habilidad activa desde transcript.jsonl y fallback a implementador
func TestActiveSkillInference(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")
	activeTaskFile := filepath.Join(tmpDir, "ACTIVE_TASK")

	// Caso 1: Variable explicita SEB_SKILL tiene prioridad
	if sk := inferActiveSkill("seb-verificador", transcriptFile, activeTaskFile); sk != "seb-verificador" {
		t.Fatalf("Se esperaba 'seb-verificador', se obtuvo '%s'", sk)
	}

	// Caso 2: Sin transcript ni ACTIVE_TASK -> fallback a implementador
	if sk := inferActiveSkill("", transcriptFile, activeTaskFile); sk != "implementador" {
		t.Fatalf("Se esperaba fallback 'implementador', se obtuvo '%s'", sk)
	}

	// Caso 3: Transcript con lectura de seb-especificador y luego seb-clarificador
	lines := []string{
		`{"step_index":1,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"read_file","args":{"Path":"/repo/.agents/skills/seb-especificador/SKILL.md"}}]}`,
		`{"step_index":5,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"read_file","args":{"Path":"\"/repo/.agents/skills/seb-clarificador/SKILL.md\""}}]}`,
	}
	_ = os.WriteFile(transcriptFile, []byte(strings.Join(lines, "\n")+"\n"), 0644)

	if sk := inferActiveSkill("", transcriptFile, activeTaskFile); sk != "seb-clarificador" {
		t.Fatalf("Se esperaba 'seb-clarificador' como ultimo skill leido, se obtuvo '%s'", sk)
	}

	// Caso 4: Si existe ACTIVE_TASK (plan sellado con plan-lock) y el ultimo skill leido fue seb-planificador, pasa a implementador
	_ = os.WriteFile(activeTaskFile, []byte("004_captura_telemetria_antigravity"), 0644)
	linesPlan := append(lines, `{"step_index":8,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"read_file","args":{"Path":"/repo/.agents/skills/seb-planificador/SKILL.md"}}]}`)
	_ = os.WriteFile(transcriptFile, []byte(strings.Join(linesPlan, "\n")+"\n"), 0644)

	if sk := inferActiveSkill("", transcriptFile, activeTaskFile); sk != "implementador" {
		t.Fatalf("Con ACTIVE_TASK sellado tras planificador, se esperaba 'implementador', se obtuvo '%s'", sk)
	}
}

// CHK-04 (RF-01): Interceptor PreToolUse con validacion de guardarrails, timer y salida JSON allow o deny
func TestHookPreToolContract(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")
	_ = os.WriteFile(transcriptFile, []byte(`{"step_index":1,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"read_file","args":{"Path":"/repo/.agents/skills/seb-especificador/SKILL.md"}}]}`+"\n"), 0644)

	// 1. Llamada permitida: seb-especificador escribiendo specs/004_test_spec.md
	allowPayload := map[string]interface{}{
		"conversationId": "conv-123",
		"workspacePaths": []string{tmpDir},
		"transcriptPath": transcriptFile,
		"modelName":      "gemini-4-argon",
		"stepIdx":        2,
		"toolCall": map[string]interface{}{
			"name": "write_file",
			"args": map[string]interface{}{
				"Path": filepath.Join(tmpDir, "specs/004_test_spec.md"),
			},
		},
	}
	payloadBytes, _ := json.Marshal(allowPayload)
	outAllow, err := processHookPreTool(db, payloadBytes, "", filepath.Join(tmpDir, "ACTIVE_TASK"), tmpDir)
	if err != nil {
		t.Fatalf("Error inesperado en pre-tool allow: %v", err)
	}
	var respAllow map[string]interface{}
	if err := json.Unmarshal([]byte(outAllow), &respAllow); err != nil || respAllow["decision"] != "allow" {
		t.Fatalf("Se esperaba JSON con decision=allow, se obtuvo: %s", outAllow)
	}

	// 2. Llamada denegada: seb-especificador intentando modificar engine/cmd/seb/main.go
	denyPayload := map[string]interface{}{
		"conversationId": "conv-123",
		"workspacePaths": []string{tmpDir},
		"transcriptPath": transcriptFile,
		"modelName":      "gemini-4-argon",
		"stepIdx":        3,
		"toolCall": map[string]interface{}{
			"name": "edit_file",
			"args": map[string]interface{}{
				"Path": filepath.Join(tmpDir, "engine/cmd/seb/main.go"),
			},
		},
	}
	denyBytes, _ := json.Marshal(denyPayload)
	outDeny, _ := processHookPreTool(db, denyBytes, "", filepath.Join(tmpDir, "ACTIVE_TASK"), tmpDir)
	var respDeny map[string]interface{}
	if err := json.Unmarshal([]byte(outDeny), &respDeny); err != nil || respDeny["decision"] != "deny" {
		t.Fatalf("Se esperaba JSON con decision=deny, se obtuvo: %s", outDeny)
	}

	// 3. Llamada denegada a archivo sensible (.env)
	envPayload := map[string]interface{}{
		"conversationId": "conv-123",
		"workspacePaths": []string{tmpDir},
		"transcriptPath": transcriptFile,
		"stepIdx":        4,
		"toolCall": map[string]interface{}{
			"name": "read_file",
			"args": map[string]interface{}{
				"Path": filepath.Join(tmpDir, ".env"),
			},
		},
	}
	envBytes, _ := json.Marshal(envPayload)
	outEnv, _ := processHookPreTool(db, envBytes, "implementador", filepath.Join(tmpDir, "ACTIVE_TASK"), tmpDir)
	var respEnv map[string]interface{}
	if err := json.Unmarshal([]byte(outEnv), &respEnv); err != nil || respEnv["decision"] != "deny" {
		t.Fatalf("Se esperaba decision=deny para archivo .env, se obtuvo: %s", outEnv)
	}
}

// CHK-05 (RF-02, RF-03, RF-04, CL-02): PostToolUse con extraccion de tokens desde transcript.jsonl y deduplicacion por paso
func TestHookPostToolAndDeduplication(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	activeFile := filepath.Join(tmpDir, "ACTIVE_TASK")
	_ = os.WriteFile(activeFile, []byte("004"), 0644)

	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")
	// En el paso 7 el modelo consume 8250 in, 12714 cache, 96 out e invoca 3 herramientas en paralelo
	transcriptContent := `{"step_index":0,"type":"USER_INPUT","content":"hola"}
{"step_index":7,"type":"PLANNER_RESPONSE","input_tokens":8250,"cache_read_tokens":12714,"output_tokens":96,"tool_calls":[{"name":"read_file"},{"name":"read_file"},{"name":"run_command"}]}
`
	_ = os.WriteFile(transcriptFile, []byte(transcriptContent), 0644)

	tools := []string{"read_file", "read_file", "run_command"}
	for i, toolName := range tools {
		step := 7 + i // Antigravity asigna stepIdx al paso de ejecucion o al paso actual
		prePayload := map[string]interface{}{
			"conversationId": "conv-parallel",
			"workspacePaths": []string{tmpDir},
			"transcriptPath": transcriptFile,
			"modelName":      "gemini-4-argon",
			"stepIdx":        step,
			"toolCall": map[string]interface{}{
				"name": toolName,
				"args": map[string]interface{}{
					"Path": filepath.Join(tmpDir, "README.md"),
				},
			},
		}
		preBytes, _ := json.Marshal(prePayload)
		_, _ = processHookPreTool(db, preBytes, "implementador", activeFile, tmpDir)
		time.Sleep(2 * time.Millisecond)

		outPost, err := processHookPostTool(db, preBytes, "implementador", activeFile, tmpDir, "")
		if err != nil {
			t.Fatalf("Error en post-tool %d: %v", i, err)
		}
		if strings.TrimSpace(outPost) != "{}" {
			t.Fatalf("PostToolUse debe emitir exactamente '{}', se obtuvo: %s", outPost)
		}
	}

	// Verificar que se registraron las 3 herramientas pero los tokens del paso 7 se imputaron SOLO UNA VEZ
	var count int
	var sumIn, sumCache, sumOut int64
	err := db.QueryRow(`SELECT COUNT(*), SUM(tokens_in), SUM(cache_read_tokens), SUM(tokens_out) FROM agent_actions WHERE session_id = 'conv-parallel'`).Scan(&count, &sumIn, &sumCache, &sumOut)
	if err != nil {
		t.Fatalf("Error consultando metricas de conv-parallel: %v", err)
	}
	if count != 3 {
		t.Fatalf("Se esperaban 3 acciones registradas, se obtuvieron %d", count)
	}
	if sumIn != 8250 || sumCache != 12714 || sumOut != 96 {
		t.Fatalf("Deduplicacion fallida: se esperaba (8250, 12714, 96), se obtuvo (%d, %d, %d)", sumIn, sumCache, sumOut)
	}
}

// CHK-06 (RF-08, RF-09): Manejo de errores de herramienta (exit_code != 0) y tolerancia ante transcript.jsonl ausente
func TestHookPostToolErrorsAndMissingTranscript(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	activeFile := filepath.Join(tmpDir, "ACTIVE_TASK")
	_ = os.WriteFile(activeFile, []byte("004"), 0644)

	postPayload := map[string]interface{}{
		"conversationId": "conv-err",
		"workspacePaths": []string{tmpDir},
		"transcriptPath": filepath.Join(tmpDir, "non_existent_transcript.jsonl"),
		"modelName":      "gemini-4-argon",
		"stepIdx":        5,
		"error":          "exit status 2: fallo en comando /home/calevin/secret.txt",
		"toolCall": map[string]interface{}{
			"name": "run_command",
			"args": map[string]interface{}{
				"CommandLine": "false",
			},
		},
	}
	postBytes, _ := json.Marshal(postPayload)
	outPost, err := processHookPostTool(db, postBytes, "implementador", activeFile, tmpDir, "")
	if err != nil {
		t.Fatalf("No debe fallar ante transcript inexistente: %v", err)
	}
	if strings.TrimSpace(outPost) != "{}" {
		t.Fatalf("Se esperaba '{}', se obtuvo: %s", outPost)
	}

	var exitCode int
	var resultSummary string
	var tokIn int64
	err = db.QueryRow(`SELECT exit_code, result_summary, tokens_in FROM agent_actions WHERE session_id = 'conv-err'`).Scan(&exitCode, &resultSummary, &tokIn)
	if err != nil {
		t.Fatalf("Error consultando accion fallida: %v", err)
	}
	if exitCode == 0 {
		t.Fatalf("Se esperaba exit_code != 0 ante error de herramienta, se obtuvo 0")
	}
	if tokIn != 0 {
		t.Fatalf("Se esperaba tokens_in = 0 ante transcript ausente, se obtuvo %d", tokIn)
	}
	if strings.Contains(resultSummary, "/home/calevin") || !strings.Contains(resultSummary, "<EXTERNAL_PATH>/secret.txt") {
		t.Fatalf("El resumen de error debe estar saneado, se obtuvo: %s", resultSummary)
	}
}

// CHK-07 (RF-10, CL-04): Interceptor Stop registrando pasos PLANNER_RESPONSE sin herramientas bajo accion model_response
func TestHookStopModelResponse(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	activeFile := filepath.Join(tmpDir, "ACTIVE_TASK")
	_ = os.WriteFile(activeFile, []byte("004"), 0644)

	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")
	// Paso 1: tuvo herramienta (ya contabilizada en PostToolUse)
	// Paso 4: respuesta final de texto sin herramientas (tool_calls vacio)
	transcriptContent := `{"step_index":1,"type":"PLANNER_RESPONSE","input_tokens":1000,"cache_read_tokens":2000,"output_tokens":100,"tool_calls":[{"name":"read_file"}]}
{"step_index":4,"type":"PLANNER_RESPONSE","input_tokens":1500,"cache_read_tokens":3000,"output_tokens":250}
`
	_ = os.WriteFile(transcriptFile, []byte(transcriptContent), 0644)

	// Primero procesamos PostToolUse para el paso 1
	postPayload := map[string]interface{}{
		"conversationId": "conv-stop",
		"workspacePaths": []string{tmpDir},
		"transcriptPath": transcriptFile,
		"modelName":      "gemini-4-argon",
		"stepIdx":        1,
		"toolCall": map[string]interface{}{
			"name": "read_file",
			"args": map[string]interface{}{"Path": "README.md"},
		},
	}
	postBytes, _ := json.Marshal(postPayload)
	_, _ = processHookPostTool(db, postBytes, "implementador", activeFile, tmpDir, "")

	// Ahora disparamos el evento Stop dos veces (para verificar idempotencia)
	stopPayload := map[string]interface{}{
		"conversationId":    "conv-stop",
		"workspacePaths":    []string{tmpDir},
		"transcriptPath":    transcriptFile,
		"modelName":         "gemini-4-argon",
		"executionNum":      1,
		"terminationReason": "NO_TOOL_CALL",
		"fullyIdle":         true,
	}
	stopBytes, _ := json.Marshal(stopPayload)
	outStop1, err := processHookStop(db, stopBytes, "implementador", activeFile, tmpDir, "")
	if err != nil {
		t.Fatalf("Error en primer Stop: %v", err)
	}
	var stopResp map[string]interface{}
	if err := json.Unmarshal([]byte(outStop1), &stopResp); err != nil || stopResp["decision"] == "continue" {
		t.Fatalf("El hook Stop debe devolver un JSON valido que permita terminar el turno, se obtuvo: %s", outStop1)
	}

	// Segunda ejecucion idempotente
	_, _ = processHookStop(db, stopBytes, "implementador", activeFile, tmpDir, "")

	// Verificar totales en la sesion: paso 1 (1000/2000/100) + paso 4 (1500/3000/250) = (2500/5000/350)
	var sumIn, sumCache, sumOut int64
	var countModelResp int
	_ = db.QueryRow(`SELECT SUM(tokens_in), SUM(cache_read_tokens), SUM(tokens_out) FROM agent_actions WHERE session_id = 'conv-stop'`).Scan(&sumIn, &sumCache, &sumOut)
	_ = db.QueryRow(`SELECT COUNT(*) FROM agent_actions WHERE session_id = 'conv-stop' AND action = 'model_response'`).Scan(&countModelResp)

	if countModelResp != 1 {
		t.Fatalf("Se esperaba exactamente 1 fila model_response para el paso sin tools, se obtuvo %d", countModelResp)
	}
	if sumIn != 2500 || sumCache != 5000 || sumOut != 350 {
		t.Fatalf("Totales incorrectos tras Stop: esperado (2500, 5000, 350), obtenido (%d, %d, %d)", sumIn, sumCache, sumOut)
	}
}

// CHK-08 (RF-12, RNF-01, RNF-03): Aprovisionamiento y remocion de .agents/hooks.json mediante install.sh y uninstall.sh y latencia < 100ms
func TestHooksJsonInstallAndUninstall(t *testing.T) {
	harnessDir := getRepoRootDir(t)
	targetDir := t.TempDir()

	// Inicializar repo git en destino
	_ = exec.Command("git", "init", targetDir).Run()

	// 1. Ejecutar install.sh --no-build hacia targetDir
	installCmd := exec.Command("bash", filepath.Join(harnessDir, "install.sh"), "--no-build", targetDir)
	var outBuf bytes.Buffer
	installCmd.Stdout = &outBuf
	installCmd.Stderr = &outBuf
	if err := installCmd.Run(); err != nil {
		t.Fatalf("install.sh fallo: %v\n%s", err, outBuf.String())
	}

	// Verificar que .agents/hooks.json fue copiado y es JSON valido con PreToolUse, PostToolUse y Stop
	hooksJsonPath := filepath.Join(targetDir, ".agents", "hooks.json")
	hooksBytes, err := os.ReadFile(hooksJsonPath)
	if err != nil {
		t.Fatalf("install.sh no aprovisiono .agents/hooks.json en el proyecto destino: %v", err)
	}
	var hooksCfg map[string]interface{}
	if err := json.Unmarshal(hooksBytes, &hooksCfg); err != nil {
		t.Fatalf(".agents/hooks.json no es JSON valido: %v", err)
	}
	rawStr := string(hooksBytes)
	for _, ev := range []string{"PreToolUse", "PostToolUse", "Stop"} {
		if !strings.Contains(rawStr, ev) {
			t.Fatalf(".agents/hooks.json no declara el evento %s", ev)
		}
	}

	// 2. Verificar RNF-03: Latencia de procesamiento de hook < 100ms
	db := setupTestDB(t)
	defer db.Close()
	start := time.Now()
	samplePayload := []byte(`{"conversationId":"perf-1","workspacePaths":["` + targetDir + `"],"modelName":"auto","stepIdx":1,"toolCall":{"name":"read_file","args":{"Path":"README.md"}}}`)
	_, _ = processHookPostTool(db, samplePayload, "implementador", "", targetDir, "")
	elapsedMs := time.Since(start).Milliseconds()
	if elapsedMs >= 100 {
		t.Fatalf("RNF-03 violado: el procesamiento de PostToolUse tardo %d ms (limite 100 ms)", elapsedMs)
	}

	// 3. Ejecutar uninstall.sh sobre targetDir y verificar que .agents/hooks.json y .agents/ desaparecen
	code, _, errStr := runUninstall(t, harnessDir, "", targetDir)
	if code != 0 {
		t.Fatalf("uninstall.sh fallo con codigo %d: %s", code, errStr)
	}
	if _, err := os.Stat(hooksJsonPath); !os.IsNotExist(err) {
		t.Fatalf(".agents/hooks.json debio ser eliminado por uninstall.sh")
	}
	if _, err := os.Stat(filepath.Join(targetDir, ".agents")); !os.IsNotExist(err) {
		t.Fatalf(".agents debio eliminarse al quedar vacio tras remover hooks.json y subdirectorios")
	}
}
