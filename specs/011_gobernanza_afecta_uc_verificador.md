# Verificador Programático — Gobernanza del Metadato `Afecta-UC:` en Especificación y Clarificación SDD (Spec 011)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify gobernanza_afecta_uc` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| 011-CHK-01 | 011-RF-01,011-RF-02,011-RF-03,011-RF-04,011-RF-10,011-CL-03 | Directivas y plantilla de seb-especificador: consulta a docs/use-cases/README.md, campo Afecta-UC con backticks y regla de division multi-UC | `direnv exec . bash -c "cd engine && go test -v -run TestEspecificadorAfectaUCDirectives ./cmd/seb"` |
| 011-CHK-02 | 011-RF-05,011-RF-06,011-RF-07,011-RF-08,011-RF-09,011-RF-10,011-CL-01,011-CL-02 | Directivas de auditoria y bloqueo de seb-clarificador: validacion de backticks, existencia en README.md, coherencia RF-XX, interrogacion sin edicion previa y Hard Gate en Fase 5 | `direnv exec . bash -c "cd engine && go test -v -run TestClarificadorAfectaUCDirectives ./cmd/seb"` |
| 011-CHK-03 | 011-RF-11,011-RNF-01 | Compatibilidad historica y distribucion: specs 001 a 010 intactas y verificadas en seb.lock, y sincronizacion entre payload/.agents y .agents sin emojis | `direnv exec . bash -c "cd engine && go test -v -run TestAfectaUCBackwardCompatibilityAndSync ./cmd/seb"` |
| 011-CHK-98 | 011-RNF-02 | Analisis estatico oficial del proyecto (go vet) sin advertencias | `direnv exec . bash -c "cd engine && go vet ./..."` |
| 011-CHK-99 | GLOBAL,011-RNF-02 | Compilacion limpia de binario seb y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify gobernanza_afecta_uc` (o `011`) reporta **100% de los controles (`011-CHK-01` a `011-CHK-99`) en estado `[PASA]`**.
