package main

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// CHK-01 (RF-06, RF-07, RF-08, RNF-02, CL-01): Selector de formato y no regresion Antigravity
func TestClaudeFormatSelectionAndRegression(t *testing.T) {
	// 1. Valor por defecto sin flag ni variable de entorno
	fmtDef, rem, err := resolveHookFormat([]string{"pre-tool"}, "")
	if err != nil {
		t.Fatalf("Error inesperado en formato por defecto: %v", err)
	}
	if fmtDef != FormatAntigravity || len(rem) != 1 || rem[0] != "pre-tool" {
		t.Fatalf("Se esperaba FormatAntigravity y args ['pre-tool'], obtenido %v %v", fmtDef, rem)
	}

	// 2. Flag --format claude y --format=claude
	fmtFlag, rem, err := resolveHookFormat([]string{"pre-tool", "--format", "claude"}, "")
	if err != nil || fmtFlag != FormatClaude || len(rem) != 1 {
		t.Fatalf("Fallo resolucion con --format claude: fmt=%v rem=%v err=%v", fmtFlag, rem, err)
	}
	fmtEq, _, err := resolveHookFormat([]string{"post-tool", "--format=claude"}, "")
	if err != nil || fmtEq != FormatClaude {
		t.Fatalf("Fallo resolucion con --format=claude: fmt=%v err=%v", fmtEq, err)
	}

	// 3. Respaldo por variable de entorno SEB_HOOK_FORMAT
	fmtEnv, _, err := resolveHookFormat([]string{"stop"}, "claude")
	if err != nil || fmtEnv != FormatClaude {
		t.Fatalf("Fallo resolucion con SEB_HOOK_FORMAT=claude: fmt=%v err=%v", fmtEnv, err)
	}

	// 4. Precedencia estricta: flag --format prevalece sobre SEB_HOOK_FORMAT (CL-01)
	fmtPrec, _, err := resolveHookFormat([]string{"pre-tool", "--format", "antigravity"}, "claude")
	if err != nil || fmtPrec != FormatAntigravity {
		t.Fatalf("El flag --format debe prevalecer sobre SEB_HOOK_FORMAT, obtenido %v err=%v", fmtPrec, err)
	}

	// 5. Valor invalido rechazado con error (RF-08)
	if _, _, err := resolveHookFormat([]string{"pre-tool", "--format", "cursor"}, ""); err == nil {
		t.Fatalf("Se esperaba error para formato invalido 'cursor'")
	}
	if _, _, err := resolveHookFormat([]string{"pre-tool"}, "invalid_fmt"); err == nil {
		t.Fatalf("Se esperaba error para SEB_HOOK_FORMAT invalido")
	}

	// 6. Regresion byte a byte en Antigravity (RNF-02)
	tmpDir := t.TempDir()
	allowPayload := `{"conversationId":"conv-1","workspacePaths":["` + tmpDir + `"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"` + tmpDir + `/src/app.go"}}}`
	outAllow, err := processHookPreTool(nil, []byte(allowPayload), "implementador", "", tmpDir)
	if err != nil {
		t.Fatalf("Error en PreToolUse allow: %v", err)
	}
	if outAllow != `{"decision":"allow"}` {
		t.Fatalf("Regresion byte a byte en allow: esperado '{\"decision\":\"allow\"}', obtenido '%s'", outAllow)
	}

	denyPayload := `{"conversationId":"conv-1","workspacePaths":["` + tmpDir + `"],"toolCall":{"name":"write_to_file","args":{"TargetFile":"` + tmpDir + `/docs/constitution.md"}}}`
	outDeny, err := processHookPreTool(nil, []byte(denyPayload), "implementador", "", tmpDir)
	if err != nil {
		t.Fatalf("Error en PreToolUse deny: %v", err)
	}
	expectedDeny := `{"decision":"deny","reason":"GUARDARRAIL DE MENOR PRIVILEGIO: El rol 'implementador' NO tiene permiso de escritura sobre 'docs/constitution.md'."}`
	if outDeny != expectedDeny {
		t.Fatalf("Regresion byte a byte en deny:\nesperado: %s\nobtenido: %s", expectedDeny, outDeny)
	}

	outPost, err := processHookPostTool(nil, []byte(`{"conversationId":"conv-1"}`), "implementador", "", tmpDir, "")
	if err != nil || outPost != "{}" {
		t.Fatalf("Regresion byte a byte en PostToolUse: esperado '{}', obtenido '%s' err=%v", outPost, err)
	}

	outStop, err := processHookStop(nil, []byte(`{"conversationId":"conv-1"}`), "implementador", "", tmpDir, "")
	if err != nil || outStop != `{"decision":"allow"}` {
		t.Fatalf("Regresion byte a byte en Stop: esperado '{\"decision\":\"allow\"}', obtenido '%s' err=%v", outStop, err)
	}
}

// CHK-02 (RF-09..RF-15, CL-05): Contratos de entrada/salida Claude y normalizacion de herramientas
func TestClaudeHookContractAndPayloads(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. PreToolUse allow -> exit 0 sin stdout
	allowClaude := `{"session_id":"s1","cwd":"` + tmpDir + `","hook_event_name":"PreToolUse","tool_name":"Edit","tool_input":{"file_path":"` + tmpDir + `/engine/main.go"}}`
	out, emit, err := processHookPreToolWithFormat(nil, []byte(allowClaude), "implementador", "", tmpDir, FormatClaude)
	if err != nil || emit || out != "" {
		t.Fatalf("PreToolUse allow en Claude debe omitir stdout: emit=%v out=%q err=%v", emit, out, err)
	}

	// 2. PreToolUse deny por archivo sensible (.env)
	envClaude := `{"session_id":"s1","cwd":"` + tmpDir + `","hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":"` + tmpDir + `/.env"}}`
	outEnv, emitEnv, err := processHookPreToolWithFormat(nil, []byte(envClaude), "implementador", "", tmpDir, FormatClaude)
	if err != nil || !emitEnv {
		t.Fatalf("PreToolUse deny (.env) en Claude debe emitir JSON: emit=%v err=%v", emitEnv, err)
	}
	assertClaudeDenyJSON(t, outEnv, ".env")

	// 3. Normalizacion de herramientas de escritura Claude (Edit, Write, MultiEdit, NotebookEdit)
	for _, tool := range []string{"Edit", "Write", "MultiEdit", "NotebookEdit"} {
		inputKey := "file_path"
		target := tmpDir + "/docs/constitution.md"
		if tool == "NotebookEdit" {
			inputKey = "notebook_path"
		}
		payload := `{"session_id":"s1","cwd":"` + tmpDir + `","hook_event_name":"PreToolUse","tool_name":"` + tool + `","tool_input":{"` + inputKey + `":"` + target + `"}}`
		outTool, emitTool, err := processHookPreToolWithFormat(nil, []byte(payload), "implementador", "", tmpDir, FormatClaude)
		if err != nil || !emitTool {
			t.Fatalf("Herramienta %s sobre archivo protegido debio bloquearse: emit=%v err=%v", tool, emitTool, err)
		}
		assertClaudeDenyJSON(t, outTool, "GUARDARRAIL")
	}

	// 4. Normalizacion de Bash con comando peligroso y redireccion anti-reward-hacking
	bashPayload := `{"session_id":"s1","cwd":"` + tmpDir + `","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo hack > specs/001_test_verificador.md"}}`
	outBash, emitBash, err := processHookPreToolWithFormat(nil, []byte(bashPayload), "implementador", "", tmpDir, FormatClaude)
	if err != nil || !emitBash {
		t.Fatalf("Bash con redireccion a verificador debio bloquearse: emit=%v err=%v", emitBash, err)
	}
	assertClaudeDenyJSON(t, outBash, "ANTI-REWARD-HACKING")

	// 5. PostToolUse normal con registro en SQLite y agente "claude" por defecto
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Error abriendo sqlite: %v", err)
	}
	defer db.Close()
	if err := initDBSchema(db); err != nil {
		t.Fatalf("Error creando esquema: %v", err)
	}
	postClaude := `{"session_id":"s-claude-1","cwd":"` + tmpDir + `","hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"go test ./..."},"tool_response":{"exitCode":0}}`
	outPost, emitPost, err := processHookPostToolWithFormat(db, []byte(postClaude), "implementador", "", tmpDir, "", FormatClaude)
	if err != nil || emitPost || outPost != "" {
		t.Fatalf("PostToolUse normal en Claude debe omitir stdout: emit=%v out=%q err=%v", emitPost, outPost, err)
	}
	var agentSaved, actionSaved string
	if err := db.QueryRow(`SELECT agent, action FROM agent_actions WHERE session_id = 's-claude-1'`).Scan(&agentSaved, &actionSaved); err != nil {
		t.Fatalf("PostToolUse no registro accion en SQLite: %v", err)
	}
	if agentSaved != "claude" || actionSaved != "Bash" {
		t.Fatalf("Telemetria esperada agent='claude', action='Bash', obtenido agent=%q action=%q", agentSaved, actionSaved)
	}

	// 6. PostToolUse con additionalContext via formatHookOutput
	ctxOut, ctxEmit := formatHookOutput(FormatClaude, "post-tool", HookDecision{
		Decision:          "allow",
		AdditionalContext: "Verifica los tests antes de continuar.",
	})
	if !ctxEmit {
		t.Fatalf("PostToolUse con additionalContext debe emitir JSON")
	}
	var ctxParsed map[string]interface{}
	if err := json.Unmarshal([]byte(ctxOut), &ctxParsed); err != nil {
		t.Fatalf("JSON invalido en PostToolUse additionalContext: %v", err)
	}
	hso, ok := ctxParsed["hookSpecificOutput"].(map[string]interface{})
	if !ok || hso["hookEventName"] != "PostToolUse" || hso["additionalContext"] != "Verifica los tests antes de continuar." {
		t.Fatalf("Estructura PostToolUse additionalContext incorrecta: %s", ctxOut)
	}

	// 7. Stop con stop_hook_active: true nunca debe emitir block (RF-15, CL-05)
	stopActivePayload := `{"session_id":"s1","cwd":"` + tmpDir + `","hook_event_name":"Stop","stop_hook_active":true}`
	outStop, emitStop, err := processHookStopWithFormat(db, []byte(stopActivePayload), "implementador", "", tmpDir, "", FormatClaude)
	if err != nil || emitStop || outStop != "" {
		t.Fatalf("Stop con stop_hook_active=true no debe emitir salida bloqueante: emit=%v out=%q err=%v", emitStop, outStop, err)
	}
	// Verificar tambien que formatHookOutput respeta decisiones de Stop cuando corresponde
	blkOut, blkEmit := formatHookOutput(FormatClaude, "stop", HookDecision{Decision: "block", Reason: "Faltan pruebas"})
	if !blkEmit || !strings.Contains(blkOut, `"decision":"block"`) {
		t.Fatalf("Stop block sin stop_hook_active debe serializar block: emit=%v out=%q", blkEmit, blkOut)
	}

	// 8. JSON malformado o vacio no provoca panic ni bloqueo falso (RF-11)
	for _, ev := range []string{"pre-tool", "post-tool", "stop"} {
		switch ev {
		case "pre-tool":
			o, em, e := processHookPreToolWithFormat(nil, []byte("{corrupt"), "implementador", "", tmpDir, FormatClaude)
			if e != nil || em || o != "" {
				t.Fatalf("JSON malformado en pre-tool debe fallar seguro: o=%q em=%v e=%v", o, em, e)
			}
		case "post-tool":
			o, em, e := processHookPostToolWithFormat(nil, []byte(""), "implementador", "", tmpDir, "", FormatClaude)
			if e != nil || em || o != "" {
				t.Fatalf("JSON vacio en post-tool debe fallar seguro: o=%q em=%v e=%v", o, em, e)
			}
		case "stop":
			o, em, e := processHookStopWithFormat(nil, []byte("{corrupt"), "implementador", "", tmpDir, "", FormatClaude)
			if e != nil || em || o != "" {
				t.Fatalf("JSON malformado en stop debe fallar seguro: o=%q em=%v e=%v", o, em, e)
			}
		}
	}
}

func assertClaudeDenyJSON(t *testing.T, raw string, expectedReasonSubstr string) {
	t.Helper()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("Salida Claude deny no es JSON valido (%v): %s", err, raw)
	}
	if _, hasTopDecision := parsed["decision"]; hasTopDecision {
		t.Fatalf("PreToolUse Claude no debe incluir campo deprecado 'decision' en nivel superior: %s", raw)
	}
	if _, hasTopReason := parsed["reason"]; hasTopReason {
		t.Fatalf("PreToolUse Claude no debe incluir campo deprecado 'reason' en nivel superior: %s", raw)
	}
	hso, ok := parsed["hookSpecificOutput"].(map[string]interface{})
	if !ok {
		t.Fatalf("Falta hookSpecificOutput en respuesta Claude: %s", raw)
	}
	if hso["hookEventName"] != "PreToolUse" {
		t.Fatalf("hookEventName esperado 'PreToolUse', obtenido %v", hso["hookEventName"])
	}
	if hso["permissionDecision"] != "deny" {
		t.Fatalf("permissionDecision esperado 'deny', obtenido %v", hso["permissionDecision"])
	}
	reason, _ := hso["permissionDecisionReason"].(string)
	if !strings.Contains(reason, expectedReasonSubstr) {
		t.Fatalf("permissionDecisionReason '%s' no contiene '%s'", reason, expectedReasonSubstr)
	}
}

// CHK-07 (RNF-01): Presupuesto de latencia < 10ms para evaluacion Claude
func TestClaudeHookLatencyUnder10ms(t *testing.T) {
	tmpDir := t.TempDir()
	payload := []byte(`{"session_id":"s-perf","cwd":"` + tmpDir + `","hook_event_name":"PreToolUse","tool_name":"Edit","tool_input":{"file_path":"` + tmpDir + `/docs/constitution.md"}}`)

	// Warmup
	_, _, _ = processHookPreToolWithFormat(nil, payload, "implementador", "", tmpDir, FormatClaude)

	start := time.Now()
	const iterations = 20
	for i := 0; i < iterations; i++ {
		_, emit, err := processHookPreToolWithFormat(nil, payload, "implementador", "", tmpDir, FormatClaude)
		if err != nil || !emit {
			t.Fatalf("Error en iteracion %d: err=%v emit=%v", i, err, emit)
		}
	}
	avg := time.Since(start) / iterations
	if avg >= 10*time.Millisecond {
		t.Fatalf("Latencia promedio %v excede el limite de 10ms (RNF-01)", avg)
	}
}
