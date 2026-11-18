package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HookFormat identifica el contrato de entrada y salida de un cliente de IA.
type HookFormat string

const (
	FormatAntigravity HookFormat = "antigravity"
	FormatClaude      HookFormat = "claude"
)

// ClaudeHookPayload modela el payload JSON enviado por Claude Code / Desktop por stdin.
type ClaudeHookPayload struct {
	SessionID      string                 `json:"session_id"`
	TranscriptPath string                 `json:"transcript_path"`
	Cwd            string                 `json:"cwd"`
	HookEventName  string                 `json:"hook_event_name"`
	ToolName       string                 `json:"tool_name"`
	ToolInput      map[string]interface{} `json:"tool_input"`
	ToolResponse   interface{}            `json:"tool_response"`
	StopHookActive bool                   `json:"stop_hook_active"`
}

// NormalizedHookInput unifica los datos relevantes de entrada entre Antigravity y Claude.
type NormalizedHookInput struct {
	SessionID      string
	WorkspaceRoot  string
	TranscriptPath string
	ModelName      string
	StepIdx        int
	HasStepIdx     bool
	ToolName       string
	ToolArgs       map[string]interface{}
	ErrorText      string
	StopHookActive bool
	DefaultAgent   string
}

// HookDecision representa la decision interna agnostica al cliente.
type HookDecision struct {
	Decision          string // "allow" | "deny" | "block"
	Reason            string
	AdditionalContext string
}

// resolveHookFormat determina el formato efectivo aplicando precedencia:
// --format > SEB_HOOK_FORMAT > "antigravity".
func resolveHookFormat(args []string, envVal string) (HookFormat, []string, error) {
	flagVal := ""
	hasFlag := false
	rem := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--format" {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("el flag --format requiere un valor ('antigravity' o 'claude')")
			}
			flagVal = strings.TrimSpace(args[i+1])
			hasFlag = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "--format=") {
			flagVal = strings.TrimSpace(strings.TrimPrefix(arg, "--format="))
			hasFlag = true
			continue
		}
		rem = append(rem, arg)
	}

	raw := ""
	if hasFlag {
		raw = flagVal
	} else if strings.TrimSpace(envVal) != "" {
		raw = strings.TrimSpace(envVal)
	} else {
		return FormatAntigravity, rem, nil
	}

	switch HookFormat(strings.ToLower(raw)) {
	case FormatAntigravity:
		return FormatAntigravity, rem, nil
	case FormatClaude:
		return FormatClaude, rem, nil
	default:
		return "", nil, fmt.Errorf("formato de hook no soportado: %q (valores validos: antigravity, claude)", raw)
	}
}

// normalizeClaudeToolName mapea nombres de herramientas de Claude a operaciones reconocidas por seb.
func normalizeClaudeToolName(name string) string {
	switch strings.TrimSpace(name) {
	case "Bash":
		return "run_command"
	case "Edit", "MultiEdit", "NotebookEdit":
		return "edit_file"
	case "Write":
		return "write_to_file"
	case "Read":
		return "read_file"
	default:
		return name
	}
}

func parseHookInput(payloadBytes []byte, fallbackWorkspace string, format HookFormat) (NormalizedHookInput, bool) {
	in := NormalizedHookInput{
		WorkspaceRoot: fallbackWorkspace,
		StepIdx:       -1,
		DefaultAgent:  "antigravity",
	}

	if format == FormatClaude {
		in.DefaultAgent = "claude"
		if len(strings.TrimSpace(string(payloadBytes))) == 0 {
			return in, false
		}
		var cp ClaudeHookPayload
		if err := json.Unmarshal(payloadBytes, &cp); err != nil {
			return in, false
		}
		if cp.Cwd != "" {
			in.WorkspaceRoot = cp.Cwd
		}
		in.SessionID = cp.SessionID
		in.TranscriptPath = cp.TranscriptPath
		in.ToolName = cp.ToolName
		in.ToolArgs = cp.ToolInput
		in.StopHookActive = cp.StopHookActive
		return in, true
	}

	var p HookPayload
	if len(payloadBytes) > 0 {
		_ = json.Unmarshal(payloadBytes, &p)
	}
	if len(p.WorkspacePaths) > 0 && p.WorkspacePaths[0] != "" {
		in.WorkspaceRoot = p.WorkspacePaths[0]
	}
	in.SessionID = p.ConversationID
	in.TranscriptPath = p.TranscriptPath
	in.ModelName = p.ModelName
	if p.StepIdx != nil {
		in.StepIdx = *p.StepIdx
		in.HasStepIdx = true
	}
	if p.ToolCall != nil {
		in.ToolName = p.ToolCall.Name
		in.ToolArgs = p.ToolCall.Args
	}
	in.ErrorText = p.Error
	return in, true
}

// formatHookOutput serializa HookDecision al contrato de salida del formato solicitado.
func formatHookOutput(format HookFormat, event string, d HookDecision) (string, bool) {
	if format == FormatClaude {
		switch event {
		case "pre-tool":
			if d.Decision == "deny" {
				resp := map[string]interface{}{
					"hookSpecificOutput": map[string]interface{}{
						"hookEventName":            "PreToolUse",
						"permissionDecision":       "deny",
						"permissionDecisionReason": d.Reason,
					},
				}
				b, _ := json.Marshal(resp)
				return string(b), true
			}
			// En PreToolUse allow, Claude Code continua su flujo normal con exit 0 y stdout vacio.
			return "", false
		case "post-tool":
			if d.AdditionalContext != "" {
				resp := map[string]interface{}{
					"hookSpecificOutput": map[string]interface{}{
						"hookEventName":     "PostToolUse",
						"additionalContext": d.AdditionalContext,
					},
				}
				b, _ := json.Marshal(resp)
				return string(b), true
			}
			return "", false
		case "stop":
			if d.Decision == "block" {
				resp := map[string]interface{}{
					"decision": "block",
					"reason":   d.Reason,
				}
				b, _ := json.Marshal(resp)
				return string(b), true
			}
			return "", false
		default:
			return "", false
		}
	}

	// Contrato Antigravity (identico byte a byte al comportamiento original)
	switch event {
	case "pre-tool":
		return marshalHookDecision(d.Decision, d.Reason), true
	case "post-tool":
		return "{}", true
	case "stop":
		return `{"decision":"allow"}`, true
	default:
		return "{}", true
	}
}

// processHookPreToolWithFormat evalua PreToolUse y formatea segun el cliente objetivo.
func processHookPreToolWithFormat(db *sql.DB, payloadBytes []byte, envSkill, activeTaskPath, workspaceRoot string, format HookFormat) (string, bool, error) {
	in, valid := parseHookInput(payloadBytes, workspaceRoot, format)
	if format == FormatClaude && !valid {
		return "", false, nil
	}

	sessionID := in.SessionID
	if sessionID == "" {
		sessionID = "local-" + time.Now().Format("20060102")
	}
	stepIdx := 0
	if in.HasStepIdx {
		stepIdx = in.StepIdx
	}
	toolName := "unknown"
	if in.ToolName != "" {
		toolName = in.ToolName
	}

	tFile := timerFilePath(sessionID, stepIdx, toolName)
	_ = os.WriteFile(tFile, []byte(fmt.Sprintf("%d", time.Now().UnixMilli())), 0644)

	rolePath := defaultActiveRolePath
	if in.WorkspaceRoot != "" {
		rolePath = filepath.Join(in.WorkspaceRoot, defaultActiveRolePath)
	}
	activeRole := inferActiveSkill(envSkill, in.TranscriptPath, activeTaskPath, rolePath)

	tc := &HookTool{Name: toolName, Args: in.ToolArgs}
	targetPath, cmdStr := extractTargetAndCommand(tc, in.WorkspaceRoot)

	// 1. Bloqueo de archivos sensibles
	if targetPath != "" {
		base := filepath.Base(targetPath)
		if base == ".env" || strings.HasPrefix(base, ".env.") || strings.Contains(base, "id_rsa") ||
			strings.Contains(base, "id_ed25519") || strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") {
			out, emit := formatHookOutput(format, "pre-tool", HookDecision{
				Decision: "deny",
				Reason:   fmt.Sprintf("[seb-harness BLOQUEO DE SEGURIDAD] Acceso denegado al archivo sensible: %s", base),
			})
			return out, emit, nil
		}
	}

	// 2. Bloqueo de comandos destructivos o evasion
	if cmdStr != "" {
		// Anti-escalada (Spec 013): ningun agente puede ejecutar one-offs ni activar el rol migrador
		if migradorEscalationRegex.MatchString(cmdStr) {
			if db != nil {
				logInternalAction(db, "", activeRole, "guard-blocked:escalation", cmdStr, "DENIED", 0, 1)
			}
			out, emit := formatHookOutput(format, "pre-tool", HookDecision{
				Decision: "deny",
				Reason:   migradorEscalationReason,
			})
			return out, emit, nil
		}
		if dangerousCmdRegex.MatchString(cmdStr) {
			out, emit := formatHookOutput(format, "pre-tool", HookDecision{
				Decision: "deny",
				Reason:   "[seb-harness BLOQUEO DE SEGURIDAD] Comando peligroso o evasion de verificacion detectada.",
			})
			return out, emit, nil
		}
		if activeRole == "implementador" || activeRole == "" {
			if shellRedirectContractRegex.MatchString(cmdStr) {
				out, emit := formatHookOutput(format, "pre-tool", HookDecision{
					Decision: "deny",
					Reason:   "[seb-harness ANTI-REWARD-HACKING] Prohibido modificar contratos SDD mediante comandos de shell.",
				})
				return out, emit, nil
			}
		}
	}

	// 3. Verificacion de Menor Privilegio en herramientas de escritura
	if targetPath != "" {
		evalTool := toolName
		if format == FormatClaude {
			evalTool = normalizeClaudeToolName(toolName)
		}
		tl := strings.ToLower(evalTool)
		if strings.Contains(tl, "write") || strings.Contains(tl, "edit") || strings.Contains(tl, "patch") ||
			strings.Contains(tl, "create") || strings.Contains(tl, "replace") || strings.Contains(tl, "save") {
			if !checkPermission(activeRole, "write", filepath.Clean(targetPath)) {
				if db != nil {
					logInternalAction(db, "", activeRole, "guard-blocked:write", targetPath, "DENIED", 0, 1)
				}
				out, emit := formatHookOutput(format, "pre-tool", HookDecision{
					Decision: "deny",
					Reason:   fmt.Sprintf("GUARDARRAIL DE MENOR PRIVILEGIO: El rol '%s' NO tiene permiso de escritura sobre '%s'.", activeRole, targetPath),
				})
				return out, emit, nil
			}
		}
	}

	out, emit := formatHookOutput(format, "pre-tool", HookDecision{Decision: "allow"})
	return out, emit, nil
}

// processHookPostToolWithFormat registra telemetria en PostToolUse y formatea segun el cliente.
func processHookPostToolWithFormat(db *sql.DB, payloadBytes []byte, envSkill, activeTaskPath, workspaceRoot, configPath string, format HookFormat) (string, bool, error) {
	nowMs := time.Now().UnixMilli()
	in, valid := parseHookInput(payloadBytes, workspaceRoot, format)
	if format == FormatClaude && !valid {
		return "", false, nil
	}

	sessionID := in.SessionID
	if sessionID == "" {
		sessionID = "local-" + time.Now().Format("20060102")
	}
	stepIdx := -1
	if in.HasStepIdx {
		stepIdx = in.StepIdx
	}
	toolName := "tool_call"
	if in.ToolName != "" {
		toolName = in.ToolName
	}

	var durationMs int64
	tFile := timerFilePath(sessionID, stepIdx, toolName)
	if data, err := os.ReadFile(tFile); err == nil {
		var startMs int64
		if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &startMs); err == nil && startMs > 0 {
			durationMs = normalizeDuration(nowMs - startMs)
		}
		_ = os.Remove(tFile)
	}

	rolePath := defaultActiveRolePath
	if in.WorkspaceRoot != "" {
		rolePath = filepath.Join(in.WorkspaceRoot, defaultActiveRolePath)
	}
	activeSkill := inferActiveSkill(envSkill, in.TranscriptPath, activeTaskPath, rolePath)
	sanitizedParams := sanitizeToolParams(in.ToolArgs, in.WorkspaceRoot)

	exitCode := 0
	resultSummary := "ok"
	if strings.TrimSpace(in.ErrorText) != "" {
		exitCode = 1
		resultSummary = sanitizeStringValue(in.ErrorText, in.WorkspaceRoot)
	}

	agentName := in.ModelName
	if agentName == "" {
		agentName = os.Getenv("SEB_AGENT")
	}
	if agentName == "" {
		agentName = in.DefaultAgent
	}

	var tokIn, cacheRead, tokOut int64
	recordedStepIdx := stepIdx
	if plannerIdx, tIn, tCache, tOut, ok := findPlannerStepMetrics(in.TranscriptPath, stepIdx); ok {
		recordedStepIdx = plannerIdx
		alreadyCounted := false
		if db != nil && plannerIdx >= 0 {
			var existing int
			err := db.QueryRow(`SELECT COUNT(*) FROM agent_actions WHERE session_id = ? AND step_idx = ? AND (tokens_in > 0 OR cache_read_tokens > 0 OR tokens_out > 0)`,
				sessionID, plannerIdx).Scan(&existing)
			if err == nil && existing > 0 {
				alreadyCounted = true
			}
		}
		if !alreadyCounted {
			tokIn = tIn
			cacheRead = tCache
			tokOut = tOut
		}
	}

	if db != nil {
		resolvedTask := resolveActiveTask("", os.Getenv("SEB_TASK_ID"), activeTaskPath, db)
		cost := calculateCost(0.0, in.ModelName, tokIn, tokOut, configPath)
		_, _ = db.Exec(`INSERT INTO agent_actions
			(session_id, task_id, agent, skill, action, parameters, result_summary, level, duration_ms, tokens_in, cache_read_tokens, tokens_out, cost_usd, exit_code, step_idx)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'autonomous', ?, ?, ?, ?, ?, ?, ?)`,
			sessionID, resolvedTask, agentName, activeSkill, toolName, sanitizedParams, resultSummary, durationMs, tokIn, cacheRead, tokOut, cost, exitCode, recordedStepIdx)
	}

	out, emit := formatHookOutput(format, "post-tool", HookDecision{Decision: "allow"})
	return out, emit, nil
}

// processHookStopWithFormat procesa el evento Stop respetando stop_hook_active en Claude.
func processHookStopWithFormat(db *sql.DB, payloadBytes []byte, envSkill, activeTaskPath, workspaceRoot, configPath string, format HookFormat) (string, bool, error) {
	in, valid := parseHookInput(payloadBytes, workspaceRoot, format)
	if format == FormatClaude && !valid {
		return "", false, nil
	}

	sessionID := in.SessionID
	if sessionID == "" {
		sessionID = "local-" + time.Now().Format("20060102")
	}
	agentName := in.ModelName
	if agentName == "" {
		agentName = in.DefaultAgent
	}

	if db != nil && in.TranscriptPath != "" {
		if f, err := os.Open(in.TranscriptPath); err == nil {
			defer f.Close()
			rolePath := defaultActiveRolePath
			if in.WorkspaceRoot != "" {
				rolePath = filepath.Join(in.WorkspaceRoot, defaultActiveRolePath)
			}
			activeSkill := inferActiveSkill(envSkill, in.TranscriptPath, activeTaskPath, rolePath)
			resolvedTask := resolveActiveTask("", os.Getenv("SEB_TASK_ID"), activeTaskPath, db)

			scanner := bufio.NewScanner(f)
			buf := make([]byte, 0, 64*1024)
			scanner.Buffer(buf, 2*1024*1024)

			for scanner.Scan() {
				line := scanner.Text()
				if !strings.Contains(line, "PLANNER_RESPONSE") {
					continue
				}
				var st TranscriptStep
				if err := json.Unmarshal([]byte(line), &st); err == nil && st.Type == "PLANNER_RESPONSE" {
					if st.InputTokens == 0 && st.CacheReadTokens == 0 && st.OutputTokens == 0 {
						continue
					}
					var existing int
					err := db.QueryRow(`SELECT COUNT(*) FROM agent_actions WHERE session_id = ? AND step_idx = ?`,
						sessionID, st.StepIndex).Scan(&existing)
					if err == nil && existing == 0 {
						cost := calculateCost(0.0, in.ModelName, st.InputTokens, st.OutputTokens, configPath)
						params := fmt.Sprintf(`{"step_index":%d}`, st.StepIndex)
						_, _ = db.Exec(`INSERT INTO agent_actions
							(session_id, task_id, agent, skill, action, parameters, result_summary, level, duration_ms, tokens_in, cache_read_tokens, tokens_out, cost_usd, exit_code, step_idx)
							VALUES (?, ?, ?, ?, 'model_response', ?, 'ok', 'autonomous', 0, ?, ?, ?, ?, 0, ?)`,
							sessionID, resolvedTask, agentName, activeSkill, params, st.InputTokens, st.CacheReadTokens, st.OutputTokens, cost, st.StepIndex)
					}
				}
			}
		}
	}

	// Si stop_hook_active es true, nunca emitir block para prevenir bucles infinitos.
	if format == FormatClaude && in.StopHookActive {
		return "", false, nil
	}

	out, emit := formatHookOutput(format, "stop", HookDecision{Decision: "allow"})
	return out, emit, nil
}
