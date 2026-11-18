# Verificador Programático — Asimilación de Contexto Preexistente en Proyectos Brownfield (Spec 007)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify migracion_contexto_brownfield` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| 007-CHK-01 | 007-RF-01,007-RF-09,007-RNF-01 | Respaldo determinista de AGENTS.md sin firma a AGENTS.pre-seb.md y despliegue del protocolo oficial en install.sh | `direnv exec . bash -c "cd engine && go test -v -run TestInstallBackupAgentsPreSeb ./cmd/seb"` |
| 007-CHK-02 | 007-RF-02,007-RF-07,007-RF-09 | Detección y respaldo de asistentes externos (GEMINI.md, CLAUDE.md) a .pre-seb.md y no sobrescritura de backups existentes | `direnv exec . bash -c "cd engine && go test -v -run TestInstallBackupAssistantContextFiles ./cmd/seb"` |
| 007-CHK-03 | 007-RF-08,007-RNF-01 | Restauración automática de archivos *.pre-seb.* a su denominación original al ejecutar uninstall.sh | `direnv exec . bash -c "cd engine && go test -v -run TestUninstallRestorePreSebBackups ./cmd/seb"` |
| 007-CHK-04 | 007-RF-03,007-RF-04,007-RF-05,007-RF-06 | Verificación de especificación y guardarraíles del skill seb-iniciador (inspección de .pre-seb, respaldo de constitution ajeno y plantilla de stubs) | `direnv exec . bash -c "cd engine && go test -v -run TestIniciadorSkillSpecificationAndStubs ./cmd/seb"` |
| 007-CHK-05 | 007-CL-01,007-CL-02,007-RNF-02 | Casos límite: respaldo de archivos de contexto vacíos y proyectos greenfield sin generación de .pre-seb espurios | `direnv exec . bash -c "cd engine && go test -v -run TestBrownfieldEdgeCasesAndGreenfield ./cmd/seb"` |
| 007-CHK-98 | 007-RF-Stack | Análisis estático oficial de Go (linter sin errores) | `direnv exec . bash -c "cd engine && go vet ./..."` |
| 007-CHK-99 | 007-RF-GLOBAL | Compilación limpia de binario seb y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify migracion_contexto_brownfield` (o `007`) reporta **100% de los controles (`007-CHK-01` a `007-CHK-99`) en estado `[PASA]`**.
