# Verificador Programático — separacion_protocolo_agents (Spec 017)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify separacion_protocolo_agents` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| 017-CHK-01 | 017-RF-01,017-RF-02,017-RF-03,017-RF-08,017-RF-12,017-RNF-02 | Protocolo completo en `.agents/SEB-HARNESS.md`, plantilla liviana `AGENTS.md` (<= 30 líneas en bloque), regla global reducida `00-seb-harness.md` (<= 35 líneas), stubs desacoplados y distribución en `install.sh` | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestManagedProtocolDistributionAndSize$' ./cmd/seb"` |
| 017-CHK-02 | 017-RF-04,017-RF-05,017-RNF-01,017-RNF-03,017-CL-01,017-CL-03 | Reemplazo literal del bloque gestionado preservando contenido usuario con caracteres especiales, instalación limpia/brownfield, bloque único sin líneas extra e idempotencia | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestManagedBlockUpdateAndIdempotency$' ./cmd/seb"` |
| 017-CHK-03 | 017-RF-06,017-RF-09,017-CL-04 | Migración automática de `AGENTS.md` legado a `AGENTS.legacy-seb.md` (sin sobrescribir respaldo previo) y ausencia total de `AGENTS.md.patch.md` | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestLegacyAgentsMigrationAndNoPatch$' ./cmd/seb"` |
| 017-CHK-04 | 017-RF-07,017-CL-02 | Aborto con exit code 1 y diagnóstico en stderr sin modificar `AGENTS.md` ante marcadores incompletos, invertidos o duplicados | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestMalformedMarkersAbortInstall$' ./cmd/seb"` |
| 017-CHK-05 | 017-RF-10,017-CL-01,017-CL-02,017-CL-05 | Desinstalación sin `--purge`: remueve `.agents/SEB-HARNESS.md` y solo el bloque gestionado en `AGENTS.md`, elimina archivo si queda vacío, respeta `--dry-run` y preserva ante marcadores malformados | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestUninstallManagedBlockAndProtocol$' ./cmd/seb"` |
| 017-CHK-06 | 017-RF-11,017-RNF-01 | Modo dogfooding (`--self`) actualiza el bloque gestionado del `AGENTS.md` raíz de forma idempotente aplicando las mismas reglas de validación | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^TestSelfModeUpdatesRootAgents$' ./cmd/seb"` |
| 017-CHK-07 | 017-RF-13 | Tests actualizados de contratos reemplazados (`010`, `013`) en verde y sin saltos de test (`t.Skip`) | `direnv exec . bash -c "cd engine && go test -count=1 -v -run '^(TestInstallDocumentadorSkillAndPatch|TestDocumentadorSkillDefinitionAndRules|TestPrefixedIdentifierDirectives)$' ./cmd/seb"` |
| 017-CHK-98 | GLOBAL | Análisis estático oficial del proyecto (`go vet`) sin advertencias | `direnv exec . bash -c "cd engine && go vet ./..."` |
| 017-CHK-99 | GLOBAL | Compilación limpia de binario `seb` (`CGO_ENABLED=0`) y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -count=1 -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify separacion_protocolo_agents` (o `017`) reporta **100% de los controles (`017-CHK-01` a `017-CHK-99`) en estado `[PASA]`**.
