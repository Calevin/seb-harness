# Plan Técnico — registro_telemetria_acciones (Spec 001)

- **Spec Origen:** `specs/001_registro_telemetria_acciones_spec.md`
- **Verificador Asociado:** `specs/registro_telemetria_acciones_verificador.md`
- **Estado:** `PLANIFICADO`

## 1. Resumen Arquitectónico y Aislamiento de Lógica

El subsistema de observabilidad y auditoría de `seb-harness` se reestructura para desacoplar la recolección de eventos de la lógica de negocio y persistencia. La medición de tiempos y la resolución contextual se centralizan en funciones puras y deterministas en Go, eliminando la fragilidad de scripts bash distribuidos.

Se implementan cuatro componentes nucleares dentro de `engine/cmd/seb/main.go`:
1. **Resolvedor de Tarea Activa (`resolveActiveTask`):** Función pura que implementa la cascada de precedencia (flag `--task` -> variable de entorno `SEB_TASK_ID` -> archivo `.seb/ACTIVE_TASK` -> base de datos SQLite -> `global`).
2. **Motor de Cronometraje y Normalización (`normalizeDuration` / decorador de ejecución):** Asegura que comandos internos (`spec-next`, `plan-lock`, `verify`) y herramientas externas capturen `duration_ms` real con resolución en milisegundos, forzando `0` ante desfases de reloj negativos.
3. **Calculador de Costo Híbrido (`calculateCost`):** Prioriza el costo explícito provisto en el payload. Como alternativa secundaria, consulta tarifas en `.seb/config.json` si se conocen los tokens y el identificador de modelo; si no hay coincidencia, retorna `0.0`.
4. **Ingestor de Telemetría Agregada (`importUsageJsonl`):** Procesa archivos de registro de uso estructurado (`usage.jsonl`) producidos por agentes que operan en bucle autónomo, evitando duplicación de registros de manera determinista mediante claves compuestas por timestamp y tarea.

Los scripts de hooks (`pre-tool.sh`, `post-tool.sh`, `git-post-commit`) se sincronizan para utilizar identificadores de llamada a herramienta homogéneos y delegar la resolución contextual al CLI.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta | Acción (`CREAR` / `MODIFICAR`) | Responsabilidad Técnica | Requisitos Cubiertos (`RF-XX`) |
| :------------- | :----------------------------- | :---------------------- | :----------------------------- |
| `engine/cmd/seb/main.go` | `MODIFICAR` | Incorporación de flags (`-model`), comando `import-usage`, cascada de tarea, cronometraje interno y cálculo de costos | `RF-01`, `RF-02`, `RF-03`, `RF-04`, `RF-05`, `RF-06`, `RF-07`, `RF-08`, `RF-09` |
| `engine/cmd/seb/main_test.go` | `CREAR` | Suite de pruebas unitarias automatizadas para cada requisito funcional y casos límite | `RF-01`, `RF-02`, `RF-03`, `RF-04`, `RF-05`, `RF-06`, `RF-07`, `RF-08`, `RF-09` |
| `.agents/hooks/pre-tool.sh` y `payload/.agents/hooks/pre-tool.sh` | `MODIFICAR` | Generación de temporizadores unívocos por identificador de llamada o sesión | `RF-01` |
| `.agents/hooks/post-tool.sh` y `payload/.agents/hooks/post-tool.sh` | `MODIFICAR` | Extracción de tokens multi-formato (`input_tokens`/`prompt_tokens`), modelo y paso de métricas a `seb audit log` | `RF-01`, `RF-02`, `RF-03`, `RF-05` |
| `.agents/hooks/git-post-commit` y `payload/.agents/hooks/git-post-commit` | `MODIFICAR` | Vinculación automática de commits a la tarea activa mediante cascada | `RF-05` |

## 3. Contratos, Firmas y Modelo de Datos

### Estructuras de Configuración y Telemetría (Go)

```go
// ModelPricing define las tarifas monetarias por millon de tokens
type ModelPricing struct {
    PromptPricePerMillion     float64 `json:"prompt_price_per_million"`
    CompletionPricePerMillion float64 `json:"completion_price_per_million"`
}

// EngineConfig representa la configuracion local opcional de .seb/config.json
type EngineConfig struct {
    Pricing map[string]ModelPricing `json:"pricing"`
}

// UsageRecord representa la linea estructurada de un archivo usage.jsonl
type UsageRecord struct {
    Timestamp        string `json:"timestamp"`
    Turns            int    `json:"turns"`
    InputTokens      int64  `json:"input_tokens"`
    OutputTokens     int64  `json:"output_tokens"`
    ApprovalRequired bool   `json:"approval_required"`
    Outcome          string `json:"outcome"`
}
```

### Firmas de Funciones Principales

```go
// resolveActiveTask aplica la cascada estricta para determinar la tarea activa
func resolveActiveTask(explicitTask, envTask string, activeTaskPath string, db *sql.DB) string

// normalizeDuration asegura que la duracion sea mayor o igual a cero
func normalizeDuration(durationMs int64) int64

// calculateCost evalua el costo en USD usando valor explicito, configuracion local o cero
func calculateCost(explicitCost float64, model string, tokensIn, tokensOut int64, configPath string) float64

// importUsageJsonl ingesta registros desde un archivo usage.jsonl hacia agent_actions
func importUsageJsonl(db *sql.DB, taskID, filePath string) (int, error)
```

## 4. Decisiones Técnicas Justificadas

- **Decisión 1: Centralización de la Cascada en el Binario Go:**
  - *Justificación:* Al ubicar la cascada de resolución de tarea en el CLI Go (`seb audit log`), los hooks de shell (`post-tool.sh`, `git-post-commit`) se simplifican significativamente y se garantiza un comportamiento idéntico tanto si el evento se origina en bash como si proviene de una llamada directa en Go o Python.
  - *Alternativa descartada:* Replicar lógica en scripts bash dependientes de utilidades del sistema (`sqlite3`, `cat`, etc.) que pueden variar entre sistemas operativos.
- **Decisión 2: Tabla de Precios Local Opcional (`.seb/config.json`):**
  - *Justificación:* Mantiene el principio constitucional de determinismo y operatividad 100% offline sin peticiones HTTP a APIs de precios externas en tiempo de ejecución.
  - *Alternativa descartada:* Consultar APIs en vivo de OpenRouter o Anthropic durante la ejecución de los hooks (prohibido por latencia y riesgo de fallos de red).
- **Decisión 3: Soporte para Ingesta de `usage.jsonl`:**
  - *Justificación:* Modela el flujo probado en `referencias/agent.py` y `agent.ts`, permitiendo a agentes por lotes registrar sus métricas acumuladas de una sola vez sin overhead por cada token generado.
  - *Alternativa descartada:* Forzar a los agentes autónomos a emitir comandos shell por cada llamada individual a la API del LLM.

## 5. Secuencia de Tareas Atómicas (TDD: Test en Rojo -> Código -> Test en Verde)

_Regla para el Agente Implementador: Ejecuta UNA tarea a la vez en orden de dependencia. Primero escribe el test y verifica que falle (Rojo), luego escribe el código mínimo hasta pasar (Verde)._

- [ ] **T1. Implementar cascada estricta de tarea activa y fallback a global** (`RF-05`, `RF-08`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/main_test.go`
  - _Hecho cuando:_ Los comandos `CHK-05` y `CHK-08` de `registro_telemetria_acciones_verificador.md` devuelven exit code `0`.

- [ ] **T2. Implementar cronometraje interno de comandos del arnés y normalización de latencias** (`RF-01`, `RF-04`, `RF-07`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/main_test.go`
  - _Hecho cuando:_ Los comandos `CHK-01`, `CHK-04` y `CHK-07` de `registro_telemetria_acciones_verificador.md` devuelven exit code `0`.

- [ ] **T3. Implementar registro de tokens y cálculo de costo con tabla local y fallback** (`RF-02`, `RF-03`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/main_test.go`
  - _Hecho cuando:_ Los comandos `CHK-02` y `CHK-03` de `registro_telemetria_acciones_verificador.md` devuelven exit code `0`.

- [ ] **T4. Implementar subcomando de importación determinista e idempotente de `usage.jsonl`** (`RF-06`, `CL-05`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/main_test.go`
  - _Hecho cuando:_ El comando `CHK-06` de `registro_telemetria_acciones_verificador.md` devuelve exit code `0`.

- [ ] **T5. Actualizar reporte consolidado de observabilidad con sumatorias verificadas** (`RF-09`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/main_test.go`
  - _Hecho cuando:_ El comando `CHK-09` de `registro_telemetria_acciones_verificador.md` devuelve exit code `0`.

- [ ] **T6. Sincronizar scripts de hooks (`pre-tool.sh`, `post-tool.sh`, `git-post-commit`) en `.agents/` y `payload/`** (`RF-01`, `RF-02`, `RF-05`)
  - _Archivos:_ `payload/.agents/hooks/pre-tool.sh`, `payload/.agents/hooks/post-tool.sh`, `payload/.agents/hooks/git-post-commit`
  - _Hecho cuando:_ `git diff` entre `.agents/hooks` y `payload/.agents/hooks` es limpio y pasan los linters de shell.

- [ ] **T7. Compilación limpia y pase integral de la suite de verificación** (`RF-01` a `RF-09`)
  - _Archivos:_ `.seb/bin/seb`
  - _Hecho cuando:_ Los comandos `CHK-10` y `CHK-11` devuelven exit code `0`, y `.seb/bin/seb verify registro_telemetria_acciones` concluye con 100% de controles en `[PASA]`.
