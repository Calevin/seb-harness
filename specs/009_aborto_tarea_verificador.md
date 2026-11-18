# Verificador Programático — Aborto Formal y Purga de Tareas (Spec 009)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify aborto_tarea` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| 009-CHK-01 | 009-RF-01,009-RF-02,009-RF-03,009-RF-04,009-RF-05,009-RNF-03,009-CL-01,009-CL-03 | Aborto formal por defecto: transicion a ABORTED, cabecera de spec actualizada, preservacion de telemetria SQLite, actualizacion de seb.lock y limpieza de ACTIVE_TASK | `direnv exec . bash -c "cd engine && go test -v -run TestTaskAbortFormal ./cmd/seb"` |
| 009-CHK-02 | 009-RF-06,009-RF-07,009-RF-08,009-RF-09,009-RNF-02,009-RNF-04,009-CL-02,009-CL-04 | Purga atomica con --purge: eliminacion de filas SQLite, eliminacion fisica de artefactos en specs/, remocion en seb.lock y liberacion de prefijo para spec-next | `direnv exec . bash -c "cd engine && go test -v -run TestTaskAbortPurge ./cmd/seb"` |
| 009-CHK-03 | 009-RF-10,009-RF-11,009-RF-12,009-RF-13,009-RF-14,009-RNF-01 | Guardarrailes de seguridad: bloqueo de tareas verificadas o con dependencias sin --force, rechazo de IDs inexistentes/ambiguos y alias CLI | `direnv exec . bash -c "cd engine && go test -v -run TestTaskAbortGuardsAndErrors ./cmd/seb"` |
| 009-CHK-98 | 009-RNF-01,009-RNF-04 | Analisis estatico oficial del proyecto (go vet) sin advertencias | `direnv exec . bash -c "cd engine && go vet ./..."` |
| 009-CHK-99 | GLOBAL | Compilacion limpia de binario seb y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify aborto_tarea` (o `009`) reporta **100% de los controles (`009-CHK-01` a `009-CHK-99`) en estado `[PASA]`**.
