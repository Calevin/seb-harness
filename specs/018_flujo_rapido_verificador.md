# Verificador Programático — flujo_rapido (Spec 018)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify flujo_rapido` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| 018-CHK-01 | 018-RF-01,018-RF-04,018-RF-05,018-RNF-01 | Skill `seb-fast-task` sin `.agents/SEB-FAST.md` duplicado, plantillas acotadas (`<= 35` líneas c/u) y enrutamiento explícito en bloque gestionado de `AGENTS.md` (`<= 30` líneas) | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestFastTaskSkillTemplatesAndRouting$' ./cmd/seb"` |
| 018-CHK-02 | 018-RF-02,018-RF-03,018-RNF-02 | Instalación/actualización (`--self` incluido) idempotente de `seb-fast-task` y remoción limpia en `uninstall.sh` respetando `--dry-run` | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestFastTaskInstallAndUninstallLifecycle$' ./cmd/seb"` |
| 018-CHK-03 | 018-RF-06,018-CL-02,018-CL-05 | Configuración por defecto (`<= 4` archivos, `<= 100` líneas) y personalizada en `.seb/config.json`, rechazo ante valores inválidos y rechazo de artefactos mixtos | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestFastTaskConfigAndModeDetection$' ./cmd/seb"` |
| 018-CHK-04 | 018-RF-08,018-RF-11,018-RF-12,018-CL-01 | Validación determinista de umbrales de diff (`max_files`/`max_lines`), diagnóstico de escalamiento en caliente a SDD y detección anti-reward-hacking en diff | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestFastTaskDiffThresholdsAndAntiRewardHacking$' ./cmd/seb"` |
| 018-CHK-05 | 018-RF-09,018-RF-12,018-RNF-03 | Menor privilegio en `seb guard` y `PreToolUse` para `seb-fast-task` e `implementador` (protegiendo `*_plan_y_verificador.md`, `constitution.md` y anti-escalada a `migrador`) con latencia `< 10 ms` | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestFastTaskGuardPermissionsAndLatency$' ./cmd/seb"` |
| 018-CHK-06 | 018-RF-07,018-RF-10,018-RF-11,018-CL-03,018-CL-06 | Ciclo completo `spec-next` -> `plan-lock` -> `verify` sobre los 2 artefactos acotados (`*_spec.md` + `*_plan_y_verificador.md`), sellado en `seb.lock` con `mode: fast`, clon limpio sin SQLite y compatibilidad con SDD estándar y `Afecta-UC` | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestFastTaskPlanLockAndVerifyLifecycle$' ./cmd/seb"` |
| 018-CHK-07 | 018-RF-12,018-CL-04 | Política Anti-Silent-Drop en `*_plan_y_verificador.md` (rechazo de filas malformadas/sin prefijo/duplicadas) y bloqueo ante manipulación de artefactos sellados | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestFastTaskAntiSilentDropAndTampering$' ./cmd/seb"` |
| 018-CHK-08 | 018-RF-13,018-CL-01 | Integración del hook `pre-commit` con tarea `FAST`: aprueba con `seb verify` exitoso y rechaza commits con umbrales excedidos o verificación fallida | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestFastTaskPreCommitGate$' ./cmd/seb"` |
| 018-CHK-09 | 018-RF-17 | `seb abort` y `seb abort --purge` gestionan atómicamente `*_spec.md` y `*_plan_y_verificador.md` preservando guardarraíles del DAG | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestFastTaskAbortAndPurge$' ./cmd/seb"` |
| 018-CHK-10 | 018-RF-14,018-RF-15,018-RNF-04 | Registro auditable de tareas `FAST` en `.seb/seb-audit.db` y diferenciación `FAST` vs `SDD` en `seb audit report` sin fuga de secretos ni rutas privadas | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestFastTaskAuditReportDistinction$' ./cmd/seb"` |
| 018-CHK-11 | 018-RF-16,018-CL-02,018-CL-06 | `seb status` (estándar, `--compact`, `--json`) distingue `FAST` vs `SDD` en clon limpio/con DB, rechaza artefactos mixtos y `seb help` documenta el flujo `FAST` | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestFastTaskStatusAndHelpObservability$' ./cmd/seb"` |
| 018-CHK-98 | GLOBAL | Análisis estático oficial del proyecto (`go vet`) sin advertencias | `direnv exec . bash -c "cd engine && go vet ./..."` |
| 018-CHK-99 | GLOBAL | Compilación limpia de binario `seb` (`CGO_ENABLED=0`) y suite completa de tests sin regresiones ni omisiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -count=1 -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify flujo_rapido` (o `018`) reporta **100% de los controles (`018-CHK-01` a `018-CHK-99`) en estado `[PASA]`**.
