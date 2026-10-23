# Verificador Programático — registro_telemetria_acciones (Spec 001)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify registro_telemetria_acciones` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :------- | :-------- | :----------------------------- | :------------------- |
| CHK-01 | RF-01 | Verificación de cálculo y persistencia de duración en milisegundos | `direnv exec . bash -c "cd engine && go test -v -run TestAuditLogDuration ./..."` |
| CHK-02 | RF-02 | Verificación de registro exacto de tokens_in y tokens_out | `direnv exec . bash -c "cd engine && go test -v -run TestAuditLogTokens ./..."` |
| CHK-03 | RF-03 | Verificación de cálculo y persistencia de costo USD con tabla local y fallback | `direnv exec . bash -c "cd engine && go test -v -run TestAuditCostCalculation ./..."` |
| CHK-04 | RF-04 | Verificación de cronometraje interno de comandos del arnés (duración > 0) | `direnv exec . bash -c "cd engine && go test -v -run TestInternalActionsDuration ./..."` |
| CHK-05 | RF-05 | Verificación de cascada estricta para resolución de tarea activa | `direnv exec . bash -c "cd engine && go test -v -run TestActiveTaskCascade ./..."` |
| CHK-06 | RF-06 | Verificación de importación determinista e idempotente de usage.jsonl | `direnv exec . bash -c "cd engine && go test -v -run TestImportUsageJsonl ./..."` |
| CHK-07 | RF-07 | Verificación de normalización a cero ante duraciones negativas | `direnv exec . bash -c "cd engine && go test -v -run TestNegativeDurationNormalization ./..."` |
| CHK-08 | RF-08 | Verificación de asignación de categoría global en ausencia de tarea activa | `direnv exec . bash -c "cd engine && go test -v -run TestGlobalTaskFallback ./..."` |
| CHK-09 | RF-09 | Verificación de consolidación y sumatorias en reporte de auditoría | `direnv exec . bash -c "cd engine && go test -v -run TestAuditReportAggregation ./..."` |
| CHK-10 | RF-01,RF-09 | Verificación de análisis estático oficial (go vet) | `direnv exec . bash -c "cd engine && go vet ./..."` |
| CHK-11 | RF-01,RF-09 | Verificación de compilación estática de binario Go sin CGO | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb"` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify registro_telemetria_acciones` reporta **100% de los controles (`CHK-01` a `CHK-11`) en estado `[PASA]`**.
