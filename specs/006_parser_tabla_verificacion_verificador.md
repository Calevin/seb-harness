# Verificador Programático — Parser Robusto de Tablas de Verificación y Prevención de Omisiones Silenciosas (Spec 006)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify 006_parser_tabla_verificacion` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-01,RF-02 | Parseo de identificadores heterogéneos y normalización de listas con o sin espacios | `direnv exec . bash -c "cd engine && go test -v -run TestParseVerifierHeterogeneousRequirements ./cmd/seb"` |
| CHK-02 | RF-03,RF-05 | Anti-Silent-Drop de filas candidatas malformadas y comandos sin backticks o vacíos | `direnv exec . bash -c "cd engine && go test -v -run TestParseVerifierMalformedCandidateRows ./cmd/seb"` |
| CHK-03 | RF-04 | Aislamiento de comandos de shell con pipes, operadores lógicos y comillas entre backticks | `direnv exec . bash -c "cd engine && go test -v -run TestParseVerifierCommandIsolationWithShellPipes ./cmd/seb"` |
| CHK-04 | RF-06 | Detección y rechazo determinista de Check IDs duplicados | `direnv exec . bash -c "cd engine && go test -v -run TestParseVerifierDuplicateCheckIDs ./cmd/seb"` |
| CHK-05 | RF-07 | Inmunidad contextual a bloques de código cercados y líneas de tabla no candidatas | `direnv exec . bash -c "cd engine && go test -v -run TestParseVerifierFencedCodeBlockImmunity ./cmd/seb"` |
| CHK-06 | RF-08,RF-10 | Integridad en plan-lock bloqueando verificadores defectuosos o sin controles ejecutables | `direnv exec . bash -c "cd engine && go test -v -run TestParseVerifierPlanLockIntegrity ./cmd/seb"` |
| CHK-07 | RF-09 | Integridad en verify reportando total exacto de controles y exigiendo 100% en PASA | `direnv exec . bash -c "cd engine && go test -v -run TestParseVerifierVerifyIntegrity ./cmd/seb"` |
| CHK-08 | RNF-01,RNF-02,RNF-03 | Evaluación en menos de 10ms, determinismo offline y compatibilidad retrospectiva con specs 001-005 | `direnv exec . bash -c "cd engine && go test -v -run TestParseVerifierNonFunctionalAndCompatibility ./cmd/seb"` |
| CHK-98 | RF-Stack | Análisis estático oficial de Go (linter sin errores) | `direnv exec . bash -c "cd engine && go vet ./..."` |
| CHK-99 | RF-GLOBAL | Compilación limpia de binario seb sin CGO y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify 006_parser_tabla_verificacion` (o `006`) reporta **100% de los controles (`CHK-01` a `CHK-99`) en estado `[PASA]`**.
