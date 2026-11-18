# Verificador Programático — Soporte de Integración y Formato para Claude (Spec 012)

> **Contrato Inmutable (`seb-harness`):** Este archivo es ejecutado automáticamente por `.seb/bin/seb verify soporte_claude` y por el hook `pre-commit` de Git.
> **Prohibición Anti-Reward-Hacking:** El agente implementador tiene terminantemente prohibido editar este archivo o relajar los tests invocados.

## 1. Matriz de Controles Ejecutables (Parseada por `seb verify`)

| Check ID | Requisito | Descripción de la Verificación | Comando Determinista |
| :--- | :--- | :--- | :--- |
| CHK-01 | RF-06,RF-07,RF-08,RNF-02,CL-01 | Selector de formato (--format, SEB_HOOK_FORMAT, precedencia, rechazo de valor invalido) y no regresion byte a byte en Antigravity | `direnv exec . bash -c "cd engine && go test -v -run TestClaudeFormatSelectionAndRegression ./cmd/seb"` |
| CHK-02 | RF-09,RF-10,RF-11,RF-12,RF-13,RF-14,RF-15,CL-05 | Contratos Claude por evento (PreToolUse allow/deny, PostToolUse normal/additionalContext, Stop con stop_hook_active), normalizacion de herramientas y JSON invalido | `direnv exec . bash -c "cd engine && go test -v -run TestClaudeHookContractAndPayloads ./cmd/seb"` |
| CHK-03 | RF-16,CL-04 | Scripts plantilla (.agents/hooks/*.sh) reenvian argumentos y aplican fallback seguro en formato claude ante ausencia de binario | `direnv exec . bash -c "cd engine && go test -v -run TestHookScriptsArgForwardingAndFallback ./cmd/seb"` |
| CHK-04 | RF-01,RF-02,RF-03,RF-04,RF-05,RNF-04,CL-02,CL-03 | Instalador (install.sh): symlinks en .claude/ solo si no existen, fusion idempotente en .claude/settings.json y aborto seguro ante JSON corrupto | `direnv exec . bash -c "cd engine && go test -v -run TestInstallClaudeIntegration ./cmd/seb"` |
| CHK-05 | RF-17,RF-18,CL-03,CL-06 | Desinstalador (uninstall.sh): remocion quirurgica de symlinks hacia .agents/, limpieza de hooks seb en settings.json, dry-run y aborto ante JSON corrupto | `direnv exec . bash -c "cd engine && go test -v -run TestUninstallClaudeIntegration ./cmd/seb"` |
| CHK-06 | RF-19 | Documentacion operativa en README.md incluye matriz de agentes (Antigravity, Claude Code, Claude Desktop), selector --format y prueba end-to-end | `grep -q "Claude Code" README.md && grep -q "Claude Desktop" README.md && grep -q -- "--format claude" README.md && grep -qi "end-to-end" README.md` |
| CHK-07 | RNF-01 | Latencia de evaluacion y serializacion de hook Claude por debajo de 10ms sin red | `direnv exec . bash -c "cd engine && go test -v -run TestClaudeHookLatencyUnder10ms ./cmd/seb"` |
| CHK-98 | RNF-03 | Analisis estatico oficial del proyecto (go vet) sin advertencias | `direnv exec . bash -c "cd engine && go vet ./..."` |
| CHK-99 | GLOBAL | Compilacion limpia de binario seb (CGO_ENABLED=0) y suite completa de tests sin regresiones | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb && go test -v ./..."` |

## 2. Criterio de Aprobación

- La tarea solo se considera implementada cuando `.seb/bin/seb verify soporte_claude` (o `012`) reporta **100% de los controles (`CHK-01` a `CHK-99`) en estado `[PASA]`**.
