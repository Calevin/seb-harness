# Plan de Implementación — Spec 016: Observabilidad e Introspección del CLI y Estado SDD

- **Tarea ID:** `016`
- **Identificador:** `observabilidad_introspeccion_cli`
- **Spec Origen:** `specs/016_observabilidad_introspeccion_cli_spec.md`
- **Afecta-UC:** `nuevo`

## 1. Arquitectura y Decisiones Técnicas

1. **Separación modular en `engine/cmd/seb/`:**
   - **`help.go`**: Registro declarativo de fichas técnicas (`commandHelpSpec`) para todos los subcomandos activos (`init`, `spec-next`, `plan-lock`, `verify`, `status`, `abort`, `task`, `guard`, `role`, `memory-add`, `audit`, `hook`, `oneoff`, `help`), más `cmdHelp`, `hasHelpFlag` y `renderSubcommandHelp`.
   - **`status.go`**: Motor de recolección y cruce multi-fuente (`specs/`, `.seb/seb.lock`, `.seb/seb-audit.db` opcional en solo lectura), resolución flexible offline de `<tarea>`, detector de desfases, cálculo determinista del siguiente paso esperado y renderizadores (`full`, `--compact`, `--json`).
   - **`main.go`**: Intercepción temprana de `help`, `--help` y `-h` antes de `openDB()`; apertura opcional de SQLite para `status` solo si `.seb/seb-audit.db` existe en disco (`os.Stat`), pasando `db == nil` en clones limpios sin crear la base.
   - **`hook_claude.go`**: Excepción segura en `PreToolUse` para comandos de ayuda de solo lectura (`seb help ...`, `seb <cmd> --help`, `seb <cmd> -h`) para evitar falsos bloqueos de `migradorEscalationRegex` al consultar ayuda de `oneoff`.
2. **Documentación operativa distribuida en `payload/`:**
   - **`payload/AGENTS.md`**: Tabla compacta de máquina de estados (Plano Editorial Markdown vs Plano Operativo `seb status`/SQLite vs `.seb/seb.lock`) y regla obligatoria de consultar `seb status` y `seb help` antes de asumir transiciones.
   - **`payload/.agents/skills/*/SKILL.md`**: Incorporación de `seb status` y `seb help` en las 6 habilidades SDD, más aclaración explícita en `seb-clarificador/SKILL.md` sobre `APROBADA` vs `SPECIFIED` hasta `seb plan-lock`.
3. **Verificación y Compatibilidad:**
   - Cumplimiento estricto de `CGO_ENABLED=0` y cero dependencias externas nuevas.
   - Tests unitarios e integración aislados con `t.TempDir()` en `help_test.go` y `status_test.go`, además de validaciones sobre `payload/`.

## 2. Desglose Atómico de Tareas (TDD)

1. **Tarea 1 (`016-RF-09` a `016-RF-12`, `016-RNF-02`, `016-RNF-03`, `016-CL-03`):**
   - Implementar `engine/cmd/seb/help.go` con el catálogo completo de subcomandos, fichas técnicas estructuradas (propósito, sintaxis/flags, lecturas, escrituras, precondiciones/guardarraíles, transición de estado, códigos de salida) y `hasHelpFlag`.
   - Actualizar `main.go` para despachar `seb help [<subcomando>]` y `seb <subcomando> --help|-h` antes de `openDB()`, y mantener `printUsage()` compacto.
   - Crear `engine/cmd/seb/help_test.go` cubriendo catálogo, todas las fichas, precedencia de `--help`/`-h` con argumentos inválidos, no creación de `.seb/seb-audit.db` y error `1` ante subcomando inexistente.

2. **Tarea 2 (`016-RF-01`, `016-RF-04`, `016-RNF-02`, `016-CL-01`, `016-CL-02`, `016-CL-05`):**
   - Implementar en `engine/cmd/seb/status.go` la lectura estricta de `.seb/seb.lock` (`readLockFileStrict`, fallando con error si el JSON es inválido) y el escaneo físico de `specs/*_spec.md` (extrayendo `Estado:`, conteo de `[NECESITA ACLARACIÓN]`, `Depends-On`/`Dependencias` y presencia de `*_plan.md` / `*_verificador.md`).
   - Integrar la lectura opcional de SQLite (`db != nil`) en `main.go` + `status.go` sin crear `.seb/seb-audit.db` cuando no exista.

3. **Tarea 3 (`016-RF-05`, `016-RF-06`, `016-CL-04`):**
   - Implementar en `status.go` el detector de desfases multi-fuente (disco vs `seb.lock` vs SQLite, incluyendo entradas huérfanas en SQLite o estados contradictorios) acumulando todas las alertas por tarea.
   - Implementar la evaluación real de dependencias directas en el DAG y el cálculo determinista del campo `NextStep` (siguiente skill o comando CLI esperado).

4. **Tarea 4 (`016-RF-02`, `016-RF-03`, `016-RF-07`, `016-RF-08`, `016-RNF-01`):**
   - Implementar resolución flexible offline para `seb status <tarea>` (ID, nombre simple o clave canónica, con error `1` ante inexistente o ambiguo).
   - Implementar los tres modos de salida: vista completa por defecto, `--compact` y `--json`.
   - Crear `engine/cmd/seb/status_test.go` cubriendo clon limpio sin DB, desfases múltiples, `seb.lock` corrupto, resolución por `<tarea>`, `--compact`, `--json`, `specs/` vacío y latencia `< 500 ms` con 100 specs.

5. **Tarea 5 (`016-RF-17`):**
   - Verificar y reforzar `seb guard` (`checkPermission`) y `PreToolUse` (`hook_claude.go`) para autorizar `seb status`, `seb help` y `--help`/`-h` en todos los roles SDD sin falsos bloqueos.
   - Añadir pruebas en `engine/cmd/seb/help_test.go` / `guard_test.go` validando todos los roles SDD.

6. **Tarea 6 (`016-RF-13`, `016-RF-14`):**
   - Actualizar `payload/AGENTS.md` incorporando la tabla compacta de la máquina de estados completa (Markdown vs motor/`seb status` vs `seb.lock`) y la regla explícita de consultar `seb status` y `seb help` antes de asumir comportamientos.

7. **Tarea 7 (`016-RF-15`, `016-RF-16`):**
   - Actualizar los 6 `SKILL.md` en `payload/.agents/skills/` para incluir `seb status` y `seb help` como herramientas oficiales de diagnóstico.
   - Explicitar en `payload/.agents/skills/seb-clarificador/SKILL.md` que pasar una spec a `APROBADA` no cambia `SPECIFIED` en `seb status` hasta ejecutar `seb plan-lock`.

8. **Tarea 8 (`GLOBAL`):**
   - Compilar `.seb/bin/seb` con `CGO_ENABLED=0`, ejecutar `go vet ./...`, `go test -v ./...` y validar el contrato completo mediante `.seb/bin/seb verify observabilidad_introspeccion_cli`.
