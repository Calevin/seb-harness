# Verificador Programático — Integración de Skill Documentador Funcional (Spec 010)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify integracion_documentador` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| 010-CHK-01 | 010-RF-01,010-RF-02,010-RF-03,010-RNF-01,010-CL-01 | Guardarrail de menor privilegio y registro de roles: seb-documentador autorizado solo en docs/use-cases/ y MEMORY.md, bloqueado en codigo/specs y aceptado como rol canonico | `direnv exec . bash -c "cd engine && go test -v -run TestDocumentadorGuardAndRoles ./cmd/seb"` |
| 010-CHK-02 | 010-RF-09,010-RF-10,010-RF-11,010-RF-12,010-RNF-03,010-CL-01,010-CL-03 | Aprovisionamiento en scripts: install.sh crea docs/use-cases/, copia skill, emite AGENTS.md.patch.md ante proyectos existentes y uninstall.sh preserva casos de uso | `direnv exec . bash -c "cd engine && go test -v -run TestInstallDocumentadorSkillAndPatch ./cmd/seb"` |
| 010-CHK-03 | 010-RF-04,010-RF-05,010-RF-06,010-RF-07,010-RF-08,010-RF-13,010-RNF-02,010-CL-02,010-CL-04 | Integridad de skill y reglas: definicion de seb-documentador con directivas obligatorias (autonumber, sin emojis, ingles/espanol) y reglas en AGENTS.md / 00-seb-harness.md | `direnv exec . bash -c "cd engine && go test -v -run TestDocumentadorSkillDefinitionAndRules ./cmd/seb"` |
| 010-CHK-98 | 010-RNF-01,010-RNF-03 | Analisis estatico oficial del proyecto (go vet) sin advertencias | `direnv exec . bash -c "cd engine && go vet ./..."` |
| 010-CHK-99 | GLOBAL | Compilacion limpia de binario seb y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify integracion_documentador` (o `010`) reporta **100% de los controles (`010-CHK-01` a `010-CHK-99`) en estado `[PASA]`**.
