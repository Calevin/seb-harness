# Verificador Programático — Gestión de Grafo de Dependencias (DAG), Guardarraíl de Planificación y Protocolo de Suspensión en Clarificación (Spec 005)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify 005_gestion_grafo_dependencias` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-01,RF-02 | Parser determinista de metadata de dependencias en specs con soporte de IDs, nombres canónicos y retrocompatibilidad | `direnv exec . bash -c "cd engine && go test -v -run TestParseSpecDependencies ./cmd/seb"` |
| CHK-02 | RF-03 | Estampado de verified: true y commit_hash en seb.lock al superar seb verify | `direnv exec . bash -c "cd engine && go test -v -run TestVerifyUpdatesSebLockWithVerifiedAndCommitHash ./cmd/seb"` |
| CHK-03 | RF-04,RF-05,RF-06 | Guardarraíl de dependencias en plan-lock bloqueando tareas insatisfechas o no verificadas | `direnv exec . bash -c "cd engine && go test -v -run TestPlanLockDirectDependenciesGuardrail ./cmd/seb"` |
| CHK-04 | RF-07 | Detección y rechazo determinista de referencias circulares mutuas en plan-lock | `direnv exec . bash -c "cd engine && go test -v -run TestPlanLockCircularDependencyDetection ./cmd/seb"` |
| CHK-05 | RF-08,RF-09,RF-10,RF-11,RF-12,RF-13,RF-14 | Protocolo formal de bifurcación de alcance, suspensión BLOQUEADA_POR_DEPENDENCIA y contrato de desbloqueo en seb-clarificador | `direnv exec . bash -c "cd engine && go test -v -run TestClarificadorSkillProtocolDAG ./cmd/seb"` |
| CHK-06 | RF-15,RF-16 | Observabilidad de dependencias y alertas de bloqueo en seb status e integración memory-add | `direnv exec . bash -c "cd engine && go test -v -run TestStatusDisplaysDependencies ./cmd/seb"` |
| CHK-07 | RNF-01,RNF-02,RNF-03 | Evaluación de dependencias offline en < 50ms, clon limpio sin SQLite y compatibilidad con specs 001-004 | `direnv exec . bash -c "cd engine && go test -v -run TestDAGNonFunctionalRequirements ./cmd/seb"` |
| CHK-98 | RF-Stack | Análisis estático oficial de Go (linter sin errores) | `direnv exec . bash -c "cd engine && go vet ./..."` |
| CHK-99 | RF-GLOBAL | Compilación limpia de binario seb sin CGO y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify 005_gestion_grafo_dependencias` (o `005`) reporta **100% de los controles (`CHK-01` a `CHK-99`) en estado `[PASA]`**.
