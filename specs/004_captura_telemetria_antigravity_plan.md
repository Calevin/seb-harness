# Plan Técnico — Captura Automática de Telemetría, Tokens en Caché y Saneamiento en Antigravity (Spec 004)

- **Spec Origen:** `specs/004_captura_telemetria_antigravity_spec.md`
- **Verificador Asociado:** `specs/004_captura_telemetria_antigravity_verificador.md`
- **Estado:** `PLANIFICADO`

## 1. Resumen Arquitectónico y Aislamiento de Lógica

El sistema actual de telemetría no lograba capturar eventos de herramientas ni consumo de tokens en Antigravity 2.0 ni en `agy` CLI porque carecía del archivo declarativo `.agents/hooks.json`, los scripts de hooks no emitían el contrato JSON requerido en `stdout` y se asumía erróneamente que el conteo de tokens provenía del payload `stdin` en lugar del archivo `transcript.jsonl` apuntado por `transcriptPath`.

Para resolverlo manteniendo el diseño de **Go puro sin CGO** (`docs/constitution.md` Sección B) y eliminando la dependencia frágil de `jq` o heurísticas en Bash, se centraliza toda la lógica de ciclo de vida dentro del motor compilado `.seb/bin/seb` mediante un nuevo subcomando nativo (`seb hook <pre-tool|post-tool|stop>`), respaldado por wrappers livianos en `.agents/hooks/` y declarado en `.agents/hooks.json`:

1. **Declaración Formal de Ciclo de Vida (`payload/.agents/hooks.json`):**
   Configura los eventos `PreToolUse` (matcher `*`), `PostToolUse` (matcher `*`) y `Stop` apuntando a los scripts de `.agents/hooks/`, con directorio de trabajo en `.agents/` (comportamiento nativo de Antigravity, por lo que los scripts resuelven la raíz del proyecto como `..`).
2. **Motor de Intercepción y Contratos JSON (`cmdHook` en Go):**
   - `pre-tool`: Parsea el JSON camelCase de `stdin` (`conversationId`, `stepIdx`, `toolCall.name`, `toolCall.args`, `transcriptPath`), resuelve el `skill` activo (priorizando `SEB_SKILL` o infiriéndolo desde `transcriptPath`), aplica bloqueos de seguridad y `checkPermission` (`seb guard`), guarda el timestamp de inicio en `/tmp/seb_timer_<conversationId>_<stepIdx>_<toolName>` y emite `{"decision":"allow"}` o `{"decision":"deny","reason":"..."}` en `stdout`.
   - `post-tool`: Calcula `duration_ms` desde el archivo de timer, sanea `toolCall.args` (rutas relativas al workspace, `<EXTERNAL_PATH>/<basename>` para rutas absolutas externas y truncamiento a `256` caracteres con `...[TRUNCATED]`), extrae `input_tokens`, `cache_read_tokens` y `output_tokens` del paso `step_index` en `transcriptPath` (imputándolos solo si ese `step_index` no fue contabilizado previamente en la sesión) y emite `{}` en `stdout`.
   - `stop`: Escanea `transcriptPath` buscando cualquier paso `PLANNER_RESPONSE` de la sesión cuyos tokens aún no hayan sido contabilizados (como turnos de respuesta en texto sin llamada a herramientas), los registra bajo la acción `model_response` y emite `{"decision":"allow"}` en `stdout`.
3. **Migración Idempotente de Esquema SQLite (`cache_read_tokens` y `step_idx`):**
   `initDBSchema` verifica e incorpora de forma idempotente la columna `cache_read_tokens INTEGER DEFAULT 0` (y metadato de control de paso o verificación por sesión/paso) en `agent_actions` sin afectar bases de datos existentes (`CL-01`), actualizando además `seb audit report` para mostrar la sumatoria de tokens en caché manteniendo su exclusión de `cost_usd` (`RF-05`).
4. **Distribución y Reversión en Proyectos Destino (`install.sh` y `uninstall.sh`):**
   Se actualiza el bucle de copia en `install.sh` y el bucle de limpieza en `uninstall.sh` para incluir archivos regulares en la raíz de `payload/.agents/` (específicamente `.agents/hooks.json`), garantizando que proyectos como `brienne` reciban y desinstalen la configuración declarativa sin perder extensiones propias.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta | Acción (`CREAR` / `MODIFICAR`) | Responsabilidad Técnica | Requisitos Cubiertos (`RF-XX`) |
| :--- | :--- | :--- | :--- |
| `engine/cmd/seb/main.go` | `MODIFICAR` | Migración SQLite (`cache_read_tokens`), subcomando `seb hook <pre-tool\|post-tool\|stop>`, parser de `transcript.jsonl`, deduplicación por `step_index`, inferencia de `skill`, saneamiento de parámetros y reporte con caché. | `RF-01` a `RF-11`, `RF-13`, `RNF-01` a `RNF-03` |
| `payload/.agents/hooks.json` | `CREAR` | Declaración oficial de hooks de ciclo de vida (`PreToolUse`, `PostToolUse`, `Stop`) para Antigravity 2.0 y `agy` CLI. | `RF-01`, `RF-02`, `RF-10`, `RF-12` |
| `payload/.agents/hooks/pre-tool.sh` | `MODIFICAR` | Wrapper ligero que resuelve la raíz del workspace y delega `stdin`/`stdout` a `.seb/bin/seb hook pre-tool`. | `RF-01`, `RF-11` |
| `payload/.agents/hooks/post-tool.sh` | `MODIFICAR` | Wrapper ligero que resuelve la raíz del workspace y delega `stdin`/`stdout` a `.seb/bin/seb hook post-tool`. | `RF-02` a `RF-09` |
| `payload/.agents/hooks/stop.sh` | `CREAR` | Wrapper ligero que delega el cierre de turno a `.seb/bin/seb hook stop`. | `RF-10` |
| `install.sh` | `MODIFICAR` | Copiar archivos de primer nivel en `payload/.agents/` (como `hooks.json`) además de subdirectorios hacia `$TARGET/.agents/`. | `RF-12` |
| `uninstall.sh` | `MODIFICAR` | Eliminar archivos de primer nivel en `$TARGET/.agents/` provistos por `payload/.agents/` (como `hooks.json`) preservando los del usuario. | `RF-12` |
| `engine/cmd/seb/antigravity_telemetry_test.go` | `CREAR` | Suite de pruebas unitarias y de integración deterministas para `RF-01` a `RF-13` y casos límite `CL-01` a `CL-04`. | `RF-01` a `RF-13`, `RNF-01` a `RNF-03` |

## 3. Contratos, Firmas y Modelo de Datos

### Estructura de Payload de Hooks Antigravity (Go)

```go
// HookPayload modela el JSON camelCase enviado por Antigravity en stdin
type HookPayload struct {
	ConversationID        string       `json:"conversationId"`
	WorkspacePaths        []string     `json:"workspacePaths"`
	TranscriptPath        string       `json:"transcriptPath"`
	ArtifactDirectoryPath string       `json:"artifactDirectoryPath"`
	ModelName             string       `json:"modelName"`
	StepIdx               *int         `json:"stepIdx"`
	ToolCall              *HookTool    `json:"toolCall"`
	Error                 string       `json:"error"`
	ExecutionNum          int          `json:"executionNum"`
	TerminationReason     string       `json:"terminationReason"`
}

type HookTool struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// TranscriptStep modela una linea relevante de transcript.jsonl
type TranscriptStep struct {
	StepIndex        int        `json:"step_index"`
	Type             string     `json:"type"`
	InputTokens      int64      `json:"input_tokens"`
	CacheReadTokens  int64      `json:"cache_read_tokens"`
	OutputTokens     int64      `json:"output_tokens"`
	ToolCalls        []HookTool `json:"tool_calls"`
}
```

### Firmas de Funciones Principales

```go
// migrateDBSchema garantiza que cache_read_tokens y step_idx existan en bases preexistentes
func migrateDBSchema(db *sql.DB) error

// sanitizeToolParams relativiza rutas del workspace, enmascara rutas externas (<EXTERNAL_PATH>/<base>) y trunca >256 chars (...[TRUNCATED])
func sanitizeToolParams(args map[string]interface{}, workspaceRoot string) string

// inferActiveSkill obtiene SEB_SKILL o escanea transcriptPath buscando el ultimo .agents/skills/<name>/SKILL.md leido
func inferActiveSkill(envSkill, transcriptPath, activeTaskPath string) string

// extractStepMetrics extrae tokens del stepIdx (o del paso PLANNER_RESPONSE correspondiente) en transcriptPath
func extractStepMetrics(transcriptPath string, stepIdx int) (tokensIn, cacheRead, tokensOut int64, found bool)
```

## 4. Decisiones Técnicas Justificadas

- **Decisión 1: Delegar el procesamiento de hooks al binario Go (`seb hook <evento>`) en lugar de Bash + `jq`.**
  - *Justificación:* En NixOS y proyectos destino minimalistas, `jq` puede no estar instalado en el entorno global. Además, el parseo de `transcript.jsonl`, el saneamiento recursivo de rutas JSON y la deduplicación transaccional en SQLite son órdenes de magnitud más rápidos (`<5ms`), seguros y testeables en Go puro que en scripts Bash.
  - *Alternativa descartada:* Parsear `transcript.jsonl` con `grep`/`sed`/`jq` en `post-tool.sh`.
- **Decisión 2: Deduplicación de tokens por `(session_id, step_idx)` en SQLite.**
  - *Justificación:* En el archivo `transcript.jsonl` de Antigravity, un paso `PLANNER_RESPONSE` tiene un `step_index` (ej. `7`) y las herramientas invocadas en ese turno se ejecutan referenciando el paso. Al guardar o verificar si los tokens del paso de modelo correspondiente ya se imputaron en `agent_actions` para esa `session_id`, garantizamos que si el modelo invoca 5 herramientas en paralelo (`CL-02`), solo la primera registre `tokens_in`, `cache_read_tokens` y `tokens_out`, y las 4 restantes registren `0`. Asimismo, en el evento `Stop` (`RF-10`), cualquier `PLANNER_RESPONSE` cuyo `step_index` no esté registrado en la sesión se inserta como `model_response`.
  - *Alternativa descartada:* Archivos temporales en `/tmp` que podrían perder coherencia frente a la base de datos SQLite.

## 5. Secuencia de Tareas Atómicas (TDD: Test en Rojo -> Código -> Test en Verde)

- [x] **T1. Migración idempotente de SQLite (`cache_read_tokens`) y actualización de `seb audit log` / `seb audit report`** (`RF-05`, `RF-13`, `CL-01`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/antigravity_telemetry_test.go`
  - _Hecho cuando:_ El comando `CHK-01` devuelve exit code `0`.

- [x] **T2. Saneamiento determinista de parámetros y resultados (rutas relativas, `<EXTERNAL_PATH>/<base>` y truncamiento `...[TRUNCATED]`)** (`RF-06`, `RF-07`, `RNF-02`, `CL-03`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/antigravity_telemetry_test.go`
  - _Hecho cuando:_ El comando `CHK-02` devuelve exit code `0`.

- [x] **T3. Inferencia de habilidad activa (`skill`) desde `transcript.jsonl` y fallback a `implementador`** (`RF-11`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/antigravity_telemetry_test.go`
  - _Hecho cuando:_ El comando `CHK-03` devuelve exit code `0`.

- [x] **T4. Interceptor `seb hook pre-tool` con contrato JSON (`allow`/`deny`), guardarraíles y cronometraje** (`RF-01`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `payload/.agents/hooks/pre-tool.sh`, `engine/cmd/seb/antigravity_telemetry_test.go`
  - _Hecho cuando:_ El comando `CHK-04` devuelve exit code `0`.

- [x] **T5. Interceptor `seb hook post-tool` con extracción de `transcript.jsonl`, deduplicación por `step_index`, manejo de errores y tolerancia a fallos** (`RF-02`, `RF-03`, `RF-04`, `RF-08`, `RF-09`, `CL-02`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `payload/.agents/hooks/post-tool.sh`, `engine/cmd/seb/antigravity_telemetry_test.go`
  - _Hecho cuando:_ Los comandos `CHK-05` y `CHK-06` devuelven exit code `0`.

- [x] **T6. Interceptor `seb hook stop` para contabilizar pasos `PLANNER_RESPONSE` sin herramientas (`model_response`)** (`RF-10`, `CL-04`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `payload/.agents/hooks/stop.sh`, `engine/cmd/seb/antigravity_telemetry_test.go`
  - _Hecho cuando:_ El comando `CHK-07` devuelve exit code `0`.

- [x] **T7. Aprovisionamiento y desinstalación de `.agents/hooks.json` en `install.sh` y `uninstall.sh`** (`RF-12`, `RNF-01`, `RNF-03`)
  - _Archivos:_ `payload/.agents/hooks.json`, `install.sh`, `uninstall.sh`, `engine/cmd/seb/antigravity_telemetry_test.go`
  - _Hecho cuando:_ El comando `CHK-08` devuelve exit code `0`.

- [x] **T8. Compilación limpia sin CGO, análisis estático (`go vet`) y pase integral sin regresiones** (`RF-01` a `RF-13`)
  - _Archivos:_ `.seb/bin/seb`
  - _Hecho cuando:_ Los comandos `CHK-98` y `CHK-99` devuelven exit code `0`.
