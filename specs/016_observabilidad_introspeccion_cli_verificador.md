# Matriz de Verificación Programática — Spec 016: Observabilidad e Introspección del CLI y Estado SDD

- **Tarea ID:** `016`
- **Identificador:** `observabilidad_introspeccion_cli`
- **Spec Origen:** `specs/016_observabilidad_introspeccion_cli_spec.md`

| Check ID | Requisito | Descripción del Control Determinista | Comando Ejecutable |
| :--- | :--- | :--- | :--- |
| 016-CHK-01 | GLOBAL | Compilación limpia del binario CLI `seb` con `CGO_ENABLED=0` | `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../../.seb/bin/seb ./cmd/seb"` |
| 016-CHK-02 | GLOBAL | Análisis estático `go vet` sin advertencias en `engine/` | `direnv exec . bash -c "cd engine && go vet ./..."` |
| 016-CHK-03 | 016-RF-01 | Vista completa de `seb status` cruzando `specs/`, `seb.lock` y SQLite | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestStatusUnifiedFullView"` |
| 016-CHK-04 | 016-RF-02 | Filtrado por `<tarea>` mediante resolución flexible en `seb status` | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestStatusTaskFilter"` |
| 016-CHK-05 | 016-RF-03 | Salida resumida con flag `--compact` | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestStatusCompact"` |
| 016-CHK-06 | 016-RF-04 | Operación en clon limpio sin `.seb/seb-audit.db` y sin crearla en disco | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestStatusCleanCloneNoDB"` |
| 016-CHK-07 | 016-RF-05 | Presencia de estado editorial, aclaraciones, motor, artefactos, lock, DAG y siguiente paso | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestStatusFullFieldsAndNextStep"` |
| 016-CHK-08 | 016-RF-06 | Detección de desfases entre SQLite, disco y `seb.lock` | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestStatusDriftDetection"` |
| 016-CHK-09 | 016-RF-07 | Rechazo con código `1` y `stderr` ante `<tarea>` inexistente o ambigua | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestStatusInvalidOrAmbiguousTask"` |
| 016-CHK-10 | 016-RF-08 | Salida JSON válida con flag `--json` | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestStatusJSON"` |
| 016-CHK-11 | 016-RF-09 | Catálogo completo en `seb help` cubriendo todos los subcomandos activos | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestHelpCatalog"` |
| 016-CHK-12 | 016-RF-10 | Ayuda individual por `seb help <cmd>`, `--help` y `-h` con salida `0` sin efectos secundarios | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestHelpSubcommandAndFlags"` |
| 016-CHK-13 | 016-RF-11 | Ficha técnica completa (propósito, lecturas, escrituras, precondiciones, transiciones, exit codes) | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestHelpTechnicalSheetFields"` |
| 016-CHK-14 | 016-RF-12 | Rechazo con código `1` en `seb help <subcomando>` inexistente | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestHelpUnknownSubcommand"` |
| 016-CHK-15 | 016-RF-13 | Tabla compacta de máquina de estados en `payload/AGENTS.md` | `grep -q "BORRADOR" payload/AGENTS.md && grep -q "SPECIFIED" payload/AGENTS.md && grep -q "seb.lock" payload/AGENTS.md` |
| 016-CHK-16 | 016-RF-14 | Regla explícita de consulta previa a `seb status` y `seb help` en `payload/AGENTS.md` | `grep -q "seb status" payload/AGENTS.md && grep -q "seb help" payload/AGENTS.md` |
| 016-CHK-17 | 016-RF-15 | Inclusión de `seb status` y `seb help` en los 6 `SKILL.md` de `payload/.agents/skills/` | `bash -c 'for s in seb-iniciador seb-especificador seb-clarificador seb-planificador seb-verificador seb-documentador; do grep -q "seb status" "payload/.agents/skills/$s/SKILL.md" && grep -q "seb help" "payload/.agents/skills/$s/SKILL.md" || exit 1; done'` |
| 016-CHK-18 | 016-RF-16 | Aclaración explícita en `seb-clarificador/SKILL.md` sobre `APROBADA` vs `SPECIFIED` | `grep -q "APROBADA" payload/.agents/skills/seb-clarificador/SKILL.md && grep -q "SPECIFIED" payload/.agents/skills/seb-clarificador/SKILL.md && grep -q "plan-lock" payload/.agents/skills/seb-clarificador/SKILL.md` |
| 016-CHK-19 | 016-RF-17 | Autorización de `seb status`, `seb help` y `--help`/`-h` en `seb guard` y `PreToolUse` para todos los roles | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestGuardAndHookAllowStatusAndHelp"` |
| 016-CHK-20 | 016-RNF-01 | Latencia `< 500 ms` de `seb status` y `seb help` con 100 specs offline | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestStatusAndHelpLatency100Specs"` |
| 016-CHK-21 | 016-RNF-02 | Inmutabilidad total en disco al ejecutar `seb status`, `seb help` y `--help`/`-h` | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestStatusAndHelpReadOnlyImmutability"` |
| 016-CHK-22 | 016-RNF-03 | Salida compacta de `seb` sin argumentos | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestRootUsageCompact"` |
| 016-CHK-23 | 016-CL-01 | Repositorio vacío sin specs retorna código `0` en `seb status` | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestStatusEmptyRepo"` |
| 016-CHK-24 | 016-CL-02 | Spec sin cabecera `Estado:` o valor inválido reportada como `DESCONOCIDO` sin abortar | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestStatusUnknownEditorialState"` |
| 016-CHK-25 | 016-CL-03 | Precedencia de `--help` / `-h` incluso con argumentos inválidos | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestHelpFlagPrecedence"` |
| 016-CHK-26 | 016-CL-04 | Múltiples desfases simultáneos reportados sin omisiones | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestStatusMultipleSimultaneousDrifts"` |
| 016-CHK-27 | 016-CL-05 | Error por `stderr` y código `1` cuando `.seb/seb.lock` es JSON corrupto | `direnv exec . bash -c "cd engine && go test -v ./cmd/seb -run TestStatusCorruptLockFile"` |
| 016-CHK-28 | GLOBAL | Suite completa de tests Go en `engine/` sin regresiones | `direnv exec . bash -c "cd engine && go test -v ./..."` |
