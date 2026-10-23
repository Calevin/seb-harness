# Verificador Programático — Prefijo Secuencial en Artefactos SDD (Spec 002)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify <nombre_tarea>` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-01,RF-02,RF-03 | Validación de formato canónico de rutas de la tríada specs/<NNN>_<tarea>_<tipo>.md | `direnv exec . bash -c "cd engine && go test -v -run TestArtifactPathFormatting ./cmd/seb"` |
| CHK-02 | RF-06,RF-07 | Resolución flexible de tareas por ID, nombre simple y canónico con detección de ambigüedad | `direnv exec . bash -c "cd engine && go test -v -run TestResolveTask ./cmd/seb"` |
| CHK-03 | RF-04,RF-05,RF-08 | Uso de clave canónica <NNN>_<tarea> en seb.lock y ACTIVE_TASK durante plan-lock y verify | `direnv exec . bash -c "cd engine && go test -v -run TestLockCanonicalKey ./cmd/seb"` |
| CHK-04 | RF-09,RF-10 | Guardarraíl de menor privilegio autorizando planificador y bloqueando implementador en rutas prefijadas | `direnv exec . bash -c "cd engine && go test -v -run TestGuardPermissionsPrefixed ./cmd/seb"` |
| CHK-05 | RF-RNF-01 | Verificación determinista de la tarea 001 migrada a rutas prefijadas | `direnv exec . bash -c "test -f specs/001_registro_telemetria_acciones_plan.md && test -f specs/001_registro_telemetria_acciones_verificador.md && .seb/bin/seb verify 001"` |
| CHK-06 | RF-RNF-02 | Verificación estática de ausencia de rutas obsoletas <tarea>_plan.md en documentación y templates | `direnv exec . bash -c "! grep -rnE 'specs/<tarea>_(plan|verificador)\.md' docs/ payload/ .agents/"` |
| CHK-98 | RF-Stack | Análisis estático oficial de Go (linter sin errores) | `direnv exec . bash -c "cd engine && go vet ./..."` |
| CHK-99 | RF-GLOBAL | Compilación limpia de binario seb y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify 002` (o `002_prefijo_secuencial_artefactos`) reporta **100% de los controles (`CHK-01` a `CHK-99`) en estado `[PASA]`**.
