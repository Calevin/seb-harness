# Verificador Programático — Desinstalación y Reversión del Arnés (Spec 003)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify desinstalacion_arnes` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-01,RF-09,RF-10,RF-11,RF-18 | Validación de argumentos, ayuda y bloqueos de seguridad en uninstall.sh | `direnv exec . bash -c "cd engine && go test -v -run TestUninstallArgsAndSecurity ./cmd/seb"` |
| CHK-02 | RF-02,RF-03 | Remoción de hooks de Git y restauración de respaldos .bak-seb | `direnv exec . bash -c "cd engine && go test -v -run TestUninstallGitHooks ./cmd/seb"` |
| CHK-03 | RF-04,RF-05 | Limpieza quirúrgica de .gitignore y remoción del directorio .seb | `direnv exec . bash -c "cd engine && go test -v -run TestUninstallGitignoreAndSebDir ./cmd/seb"` |
| CHK-04 | RF-06,RF-07 | Remoción de componentes del arnés en .agents y preservación de extensiones de usuario | `direnv exec . bash -c "cd engine && go test -v -run TestUninstallAgentsDirectory ./cmd/seb"` |
| CHK-05 | RF-08 | Reversión de instalación en modo dogfooding (--self) | `direnv exec . bash -c "cd engine && go test -v -run TestUninstallSelfMode ./cmd/seb"` |
| CHK-06 | RF-12 | Simulación pasiva (--dry-run) sin alteraciones en disco | `direnv exec . bash -c "cd engine && go test -v -run TestUninstallDryRun ./cmd/seb"` |
| CHK-07 | RF-13,RF-14,RF-15,RF-16,RF-17 | Modo purga (--purge), validación interactiva, bandera --force y limpieza de docs | `direnv exec . bash -c "cd engine && go test -v -run TestUninstallPurgeAndDocs ./cmd/seb"` |
| CHK-08 | RF-RNF-01,RF-RNF-02 | Idempotencia en ejecuciones repetidas y ejecución con Bash estricto | `direnv exec . bash -c "cd engine && go test -v -run TestUninstallIdempotency ./cmd/seb"` |
| CHK-98 | RF-Stack | Análisis estático oficial de Go (linter sin errores) | `direnv exec . bash -c "cd engine && go vet ./..."` |
| CHK-99 | RF-GLOBAL | Compilación limpia de binario seb y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify desinstalacion_arnes` (o `003`) reporta **100% de los controles (`CHK-01` a `CHK-99`) en estado `[PASA]`**.
