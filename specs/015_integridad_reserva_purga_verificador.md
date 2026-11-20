# Verificador Programático — Integridad en Reserva Secuencial y Purga de Tareas (Spec 015)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify integridad_reserva_purga` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o debilitar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| 015-CHK-01 | 015-RF-01,015-RF-02,015-RNF-01,015-CL-01 | Calculo secuencial desde specs/ y reservas previas, creacion atomica de archivo stub inicial con cabecera valida y reservas consecutivas encadenadas sin colision | `direnv exec . bash -c "cd engine && go test -v -run TestSpecNextSequentialChaining ./cmd/seb"` |
| 015-CHK-02 | 015-RF-03 | Rechazo explicito con codigo de salida 1 al intentar reservar o sincronizar un ID ya asignado a un nombre diferente sin sobrescribir el registro ni el archivo | `direnv exec . bash -c "cd engine && go test -v -run TestSpecNextCollisionRejection ./cmd/seb"` |
| 015-CHK-03 | 015-RF-04,015-RF-05,015-CL-02 | Idempotencia acotada cuando existe el stub sin editar y rechazo explicito cuando la spec con el mismo nombre ya fue redactada | `direnv exec . bash -c "cd engine && go test -v -run TestSpecNextStubIdempotencyAndEditedRejection ./cmd/seb"` |
| 015-CHK-04 | 015-RF-06,015-RF-08,015-CL-05 | Modo --dry-run sin efectos secundarios en disco ni SQLite y rechazo de nombres vacios, invalidos o con traversal | `direnv exec . bash -c "cd engine && go test -v -run TestSpecNextDryRunAndInvalidNames ./cmd/seb"` |
| 015-CHK-05 | 015-RF-07,015-RNF-02,015-CL-04 | Rollback atomico de la reserva en SQLite ante fallo de escritura en disco al crear el archivo stub | `direnv exec . bash -c "cd engine && go test -v -run TestSpecNextDiskFailureRollback ./cmd/seb"` |
| 015-CHK-06 | 015-RF-09,015-RF-10,015-RNF-02,015-CL-03 | Purga acotada estrictamente a la identidad univoca preservando tareas con distinto ID y mismo nombre, y rechazo ante referencia ambigua o inexistente | `direnv exec . bash -c "cd engine && go test -v -run TestAbortPurgeScopedToUniqueID ./cmd/seb"` |
| 015-CHK-07 | 015-RF-11 | Directiva de seb-especificador actualizada documentando la creacion previa del stub por spec-next y su completado en Fase 4 | `direnv exec . bash -c "cd engine && go test -v -run TestEspecificadorStubDirective ./cmd/seb"` |
| 015-CHK-98 | GLOBAL | Analisis estatico oficial del proyecto (go vet) sin advertencias | `direnv exec . bash -c "cd engine && go vet ./..."` |
| 015-CHK-99 | GLOBAL | Compilacion limpia de binario seb con CGO_ENABLED=0 y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify integridad_reserva_purga` (o `015`) reporta **100% de los controles (`015-CHK-01` a `015-CHK-99`) en estado `[PASA]`**.
