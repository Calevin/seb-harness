# Verificador Programático — Captura Automática de Telemetría, Tokens en Caché y Saneamiento en Antigravity (Spec 004)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify 004_captura_telemetria_antigravity` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-05,RF-13 | Migración idempotente de columna cache_read_tokens, exclusión del cálculo de costo y sumatoria en reporte | `direnv exec . bash -c "cd engine && go test -v -run TestCacheReadTokensAndMigration ./cmd/seb"` |
| CHK-02 | RF-06,RF-07 | Saneamiento de rutas del workspace, enmascaramiento <EXTERNAL_PATH>/<base> y truncamiento a 256 chars con ...[TRUNCATED] | `direnv exec . bash -c "cd engine && go test -v -run TestParameterSanitizationAndTruncation ./cmd/seb"` |
| CHK-03 | RF-11 | Inferencia determinista de habilidad activa desde transcript.jsonl y fallback a implementador | `direnv exec . bash -c "cd engine && go test -v -run TestActiveSkillInference ./cmd/seb"` |
| CHK-04 | RF-01 | Interceptor PreToolUse con validación de guardarraíles, timer y salida JSON allow o deny | `direnv exec . bash -c "cd engine && go test -v -run TestHookPreToolContract ./cmd/seb"` |
| CHK-05 | RF-02,RF-03,RF-04 | Interceptor PostToolUse con extracción de tokens desde transcript.jsonl y deduplicación en herramientas paralelas del mismo paso | `direnv exec . bash -c "cd engine && go test -v -run TestHookPostToolAndDeduplication ./cmd/seb"` |
| CHK-06 | RF-08,RF-09 | Manejo de errores de herramienta (exit_code != 0) y tolerancia ante transcript.jsonl ausente o ilegible | `direnv exec . bash -c "cd engine && go test -v -run TestHookPostToolErrorsAndMissingTranscript ./cmd/seb"` |
| CHK-07 | RF-10 | Interceptor Stop registrando pasos PLANNER_RESPONSE sin herramientas bajo acción model_response | `direnv exec . bash -c "cd engine && go test -v -run TestHookStopModelResponse ./cmd/seb"` |
| CHK-08 | RF-12,RF-RNF-01,RF-RNF-03 | Aprovisionamiento y remoción de .agents/hooks.json mediante install.sh y uninstall.sh y latencia < 100ms | `direnv exec . bash -c "cd engine && go test -v -run TestHooksJsonInstallAndUninstall ./cmd/seb"` |
| CHK-98 | RF-Stack | Análisis estático oficial de Go (linter sin errores) | `direnv exec . bash -c "cd engine && go vet ./..."` |
| CHK-99 | RF-GLOBAL | Compilación limpia de binario seb sin CGO y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify 004_captura_telemetria_antigravity` (o `004`) reporta **100% de los controles (`CHK-01` a `CHK-99`) en estado `[PASA]`**.
