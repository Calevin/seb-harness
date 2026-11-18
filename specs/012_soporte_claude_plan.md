# Plan Técnico — soporte_claude (Spec 012)

- **Spec Origen:** `specs/012_soporte_claude_spec.md`
- **Verificador Asociado:** `specs/012_soporte_claude_verificador.md`
- **Estado:** `PLANIFICADO`

## 1. Resumen Arquitectónico y Aislamiento de Lógica

El diseño separa estrictamente la **lógica de decisión del arnés** (reglas de seguridad, guardarraíles de menor privilegio, telemetría y control de `stop_hook_active`) de la **capa de adaptación de protocolo** (parseo de entrada y serialización de salida según formato `antigravity` o `claude`):

1. **Adaptadores de Formato en `engine/cmd/seb/`:**
   - Se introduce un tipo explícito `HookFormat` (`antigravity` por defecto, o `claude`) resuelto por precedencia estricta: flag `--format` > variable `SEB_HOOK_FORMAT` > `"antigravity"`. Cualquier otro valor falla con código de salida `1` en `stderr`.
   - Se unifica la entrada en una estructura interna `NormalizedHookInput` con funciones puras de parseo (`parseAntigravityPayload`, `parseClaudePayload`) y normalización de herramientas de Claude (`Bash`, `Edit`, `Write`, `Read`, `MultiEdit`, `NotebookEdit`).
   - La evaluación produce una estructura `HookDecision` independiente del cliente, la cual se serializa mediante `formatAntigravityOutput` (preservando byte a byte la salida actual) o `formatClaudeOutput` (cumpliendo el contrato oficial de `PreToolUse`, `PostToolUse` y `Stop`).
2. **Scripts Plantilla de Hooks (`payload/.agents/hooks/`):**
   - `pre-tool.sh`, `post-tool.sh` y `stop.sh` reenvían `"$@"` a `.seb/bin/seb hook <evento> "$@"` y detectan el formato solicitado en su fallback cuando el binario no existe.
3. **Aprovisionamiento y Reversibilidad (`install.sh` y `uninstall.sh`):**
   - `install.sh` crea por defecto `.claude/{hooks,rules,skills}` como symlinks relativos (`../.agents/<dir>`) únicamente si no existen, y crea o fusiona `.claude/settings.json` de forma idempotente y no destructiva.
   - `uninstall.sh` revierte quirúrgicamente solo los symlinks que apuntan a `.agents/` y solo los hooks de `seb-harness` en `.claude/settings.json`, respetando `--dry-run` y abortando sin modificar ante JSON corrupto.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta | Acción (`CREAR` / `MODIFICAR`) | Responsabilidad Técnica | Requisitos Cubiertos (`RF-XX`) |
| :--- | :--- | :--- | :--- |
| `engine/cmd/seb/hook_claude.go` | `CREAR` | Resolución de formato, structs de Claude, normalización de herramientas, `NormalizedHookInput`, `HookDecision` y serializadores puros | `012-RF-06` a `012-RF-15`, `012-RNF-01` a `012-RNF-03`, `012-CL-01`, `012-CL-05` |
| `engine/cmd/seb/main.go` | `MODIFICAR` | Parseo de `--format` en `cmdHook` y delegación de `processHookPreTool/PostTool/Stop` al núcleo desacoplado manteniendo compatibilidad exacta | `012-RF-06` a `012-RF-15`, `012-RNF-02`, `012-CL-01` |
| `engine/cmd/seb/claude_hook_test.go` | `CREAR` | Tests unitarios de selección de formato, regresión Antigravity, contratos Claude por evento y latencia `<10ms` | `012-RF-06` a `012-RF-15`, `012-RNF-01`, `012-RNF-02`, `012-CL-01`, `012-CL-05` |
| `payload/.agents/hooks/pre-tool.sh` | `MODIFICAR` | Reenvío `"$@"` a `seb hook pre-tool "$@"` y fallback compatible con `--format claude` | `012-RF-16`, `012-CL-04` |
| `payload/.agents/hooks/post-tool.sh` | `MODIFICAR` | Reenvío `"$@"` a `seb hook post-tool "$@"` y fallback compatible con `--format claude` | `012-RF-16`, `012-CL-04` |
| `payload/.agents/hooks/stop.sh` | `MODIFICAR` | Reenvío `"$@"` a `seb hook stop "$@"` y fallback compatible con `--format claude` | `012-RF-16`, `012-CL-04` |
| `install.sh` | `MODIFICAR` | Creación no destructiva de symlinks `.claude/{hooks,rules,skills}` y fusión idempotente de `.claude/settings.json` | `012-RF-01` a `012-RF-05`, `012-RNF-04`, `012-CL-02`, `012-CL-03` |
| `uninstall.sh` | `MODIFICAR` | Remoción quirúrgica de symlinks hacia `.agents/`, limpieza de hooks de `seb` en `.claude/settings.json` y soporte `--dry-run` | `012-RF-17`, `012-RF-18`, `012-CL-03`, `012-CL-06` |
| `engine/cmd/seb/claude_install_test.go` | `CREAR` | Tests de integración de scripts plantilla, `install.sh` y `uninstall.sh` | `012-RF-01` a `012-RF-05`, `012-RF-16` a `012-RF-18`, `012-RNF-04`, `012-CL-02` a `012-CL-04`, `012-CL-06` |
| `README.md` | `MODIFICAR` | Documentación de matriz de agentes soportados (`Antigravity`, `Claude Code`, `Claude Desktop`), uso de `--format` y validación end-to-end | `012-RF-19` |

## 3. Contratos, Firmas y Modelo de Datos

### 3.1 Resolución de Formato y Contratos en Go (`engine/cmd/seb/hook_claude.go`)

```go
type HookFormat string

const (
	FormatAntigravity HookFormat = "antigravity"
	FormatClaude      HookFormat = "claude"
)

// resolveHookFormat aplica precedencia: flag --format > SEB_HOOK_FORMAT > "antigravity".
// Retorna error si el valor no pertenece al conjunto permitido.
func resolveHookFormat(args []string, envVal string) (HookFormat, []string, error)

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

type HookDecision struct {
	Decision          string // "allow" | "deny" | "block"
	Reason            string
	AdditionalContext string
}

// formatHookOutput serializa HookDecision segun evento ("pre-tool", "post-tool", "stop") y formato.
// Cuando emitStdout es false, cmdHook no imprime nada y sale con codigo 0.
func formatHookOutput(format HookFormat, event string, d HookDecision) (out string, emitStdout bool)
```

### 3.2 Contrato JSON de Salida para `claude`

- **`PreToolUse` (`allow`):** `emitStdout = false`, código de salida `0`.
- **`PreToolUse` (`deny`):** `emitStdout = true`, código de salida `0`, JSON:
  `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"<motivo>"}}`
- **`PostToolUse` (normal):** `emitStdout = false`, código de salida `0`.
- **`PostToolUse` (contexto adicional):** `emitStdout = true`, código de salida `0`, JSON:
  `{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"<mensaje>"}}`
- **`Stop` (normal o `stop_hook_active: true`):** `emitStdout = false`, código de salida `0`.
- **`Stop` (`block` con `stop_hook_active: false`):** `emitStdout = true`, código de salida `0`, JSON:
  `{"decision":"block","reason":"<motivo>"}`

### 3.3 Registro en `.claude/settings.json`

Estructura agregada de forma no destructiva por `install.sh`:
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "$CLAUDE_PROJECT_DIR/.claude/hooks/pre-tool.sh --format claude"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "$CLAUDE_PROJECT_DIR/.claude/hooks/post-tool.sh --format claude"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "$CLAUDE_PROJECT_DIR/.claude/hooks/stop.sh --format claude"
          }
        ]
      }
    ]
  }
}
```

## 4. Decisiones Técnicas Justificadas

- **Decisión 1: Separación de serializadores manteniendo intactas las funciones base de Antigravity.**
  - _Alternativa descartada:_ Modificar `marshalHookDecision` directamente con condicionales por formato. Se descartó para evitar regresiones en `antigravity_telemetry_test.go` y mantener la serialización pura y testeable de forma aislada.
- **Decisión 2: Emisión de salida vacía con `exit 0` en `PreToolUse` (`allow`) para Claude Code.**
  - _Alternativa descartada:_ Emitir `"permissionDecision": "allow"`. Según el contrato vigente de Claude Code, `"allow"` explícito omite los prompts interactivos de confirmación configurados por el usuario, mientras que `exit 0` sin salida permite continuar la evaluación estándar de Claude sin relajar sus controles propios.
- **Decisión 3: Fusión/limpieza de `.claude/settings.json` con parser JSON estructurado en `install.sh` y `uninstall.sh`.**
  - _Alternativa descartada:_ Manipulación textual con `sed`/`awk` o plantillas estáticas. Se descartó porque rompería configuraciones previas del usuario o fallaría silenciosamente ante JSON inválido (`012-CL-02`, `012-CL-03`).

## 5. Secuencia de Tareas Atómicas (TDD: Test en Rojo -> Código -> Test en Verde)

_Regla para el Agente Implementador: Ejecuta UNA tarea a la vez en orden de dependencia. Primero escribe el test y verifica que falle (Rojo), luego escribe el código mínimo hasta pasar (Verde)._

- [ ] **T1. Resolución explícita de formato (`--format` / `SEB_HOOK_FORMAT`) y no regresión Antigravity** (`012-RF-06`, `012-RF-07`, `012-RF-08`, `012-RNF-02`, `012-CL-01`)
  - _Archivos:_ `engine/cmd/seb/claude_hook_test.go`, `engine/cmd/seb/hook_claude.go`, `engine/cmd/seb/main.go`
  - _Hecho cuando:_ `012-CHK-01` de `specs/012_soporte_claude_verificador.md` devuelve exit code `0`.
- [ ] **T2. Parseo de payload Claude, normalización de herramientas y serializadores por evento** (`012-RF-09` a `012-RF-15`, `012-CL-05`)
  - _Archivos:_ `engine/cmd/seb/claude_hook_test.go`, `engine/cmd/seb/hook_claude.go`, `engine/cmd/seb/main.go`
  - _Hecho cuando:_ `012-CHK-02` de `specs/012_soporte_claude_verificador.md` devuelve exit code `0`.
- [ ] **T3. Presupuesto de latencia `<10ms` en evaluación Claude** (`012-RNF-01`)
  - _Archivos:_ `engine/cmd/seb/claude_hook_test.go`
  - _Hecho cuando:_ `012-CHK-07` de `specs/012_soporte_claude_verificador.md` devuelve exit code `0`.
- [ ] **T4. Reenvío de argumentos y fallback seguro en `.agents/hooks/*.sh`** (`012-RF-16`, `012-CL-04`)
  - _Archivos:_ `engine/cmd/seb/claude_install_test.go`, `payload/.agents/hooks/pre-tool.sh`, `payload/.agents/hooks/post-tool.sh`, `payload/.agents/hooks/stop.sh`
  - _Hecho cuando:_ `012-CHK-03` de `specs/012_soporte_claude_verificador.md` devuelve exit code `0`.
- [ ] **T5. Aprovisionamiento idempotente `.claude/` y fusión de `settings.json` en `install.sh`** (`012-RF-01` a `012-RF-05`, `012-RNF-04`, `012-CL-02`, `012-CL-03`)
  - _Archivos:_ `engine/cmd/seb/claude_install_test.go`, `install.sh`
  - _Hecho cuando:_ `012-CHK-04` de `specs/012_soporte_claude_verificador.md` devuelve exit code `0`.
- [ ] **T6. Reversión quirúrgica de `.claude/` y `settings.json` en `uninstall.sh`** (`012-RF-17`, `012-RF-18`, `012-CL-03`, `012-CL-06`)
  - _Archivos:_ `engine/cmd/seb/claude_install_test.go`, `uninstall.sh`
  - _Hecho cuando:_ `012-CHK-05` de `specs/012_soporte_claude_verificador.md` devuelve exit code `0`.
- [ ] **T7. Documentación de matriz de agentes y verificación end-to-end en `README.md`** (`012-RF-19`)
  - _Archivos:_ `README.md`
  - _Hecho cuando:_ `012-CHK-06`, `012-CHK-98` y `012-CHK-99` de `specs/012_soporte_claude_verificador.md` devuelven exit code `0`.
