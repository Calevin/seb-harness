# Plan de Implementación — Spec 018: Flujo Rápido (`seb-fast-task`) y Artefactos Acotados

- **Tarea ID:** `018`
- **Identificador:** `flujo_rapido`
- **Spec Origen:** `specs/018_flujo_rapido_spec.md`
- **Afecta-UC:** `nuevo`, `UC-brownfield_context_assimilation`, `UC-provisioning_harness_uninstall`, `UC-artifacts_sequential_naming`, `UC-verifier_table_parsing`, `UC-session_role_management`, `UC-telemetry_action_logging`, `UC-cli_observability_introspection`, `UC-task_abort_and_purge`

## 1. Arquitectura y Decisiones Técnicas

1. **Nuevo módulo `engine/cmd/seb/fast_task.go` (núcleo del Flujo Rápido):**
   - Define constantes `taskModeSDD = "SDD"`, `taskModeFAST = "FAST"`, `defaultFastMaxFiles = 4`, `defaultFastMaxLines = 100`.
   - Extiende `EngineConfig` (`.seb/config.json`) con el bloque opcional `fast_track: { "max_files": int, "max_lines": int }`, validando en `loadFastTrackConfig` que valores ausentes apliquen los defaults `(4, 100)` y que valores `<= 0` o malformados fallen con código `1` (`018-RF-06`, `018-CL-05`).
   - Implementa `detectTaskModeAndArtifacts(id, name string)` para distinguir tareas `FAST` (`specs/<NNN>_<tarea>_plan_y_verificador.md`) vs `SDD` (`*_plan.md` + `*_verificador.md`) y rechazar con error `1` cualquier coexistencia ambigua de artefactos mixtos (`018-CL-02`).
   - Implementa `validateFastDiffThresholds` (evaluando archivos modificados y líneas agregadas+eliminadas en el diff de Git excluyendo `specs/<NNN>_<tarea>_*.md`, `.seb/seb.lock` y `.seb/ACTIVE_TASK`) y `validateFastAntiRewardHackingDiff` (rechazando inserciones de patrones de omisión de tests como `t.Skip(`, `@Ignore`, `.skip(`, `.only(`, `markTestSkipped(` en archivos de test) (`018-RF-08`, `018-RF-11`, `018-RF-12`, `018-CL-01`).

2. **Integración modular en el motor Go existente (`main.go`, `status.go`, `help.go`, `hook_claude.go`, `oneoff_013.go`):**
   - **Resolución y parsing (`main.go`, `oneoff_013.go`):** `TaskInfo` incorpora `Mode` y `FastPath`; `resolveTask` utiliza `detectTaskModeAndArtifacts`; `taskIDFromVerifierPath` reconoce tanto `*_verificador.md` como `*_plan_y_verificador.md`, manteniendo la política Anti-Silent-Drop y prefijos `<NNN>-CHK-XX` en `parseVerifierFile` (`018-RF-07`, `018-RF-11`, `018-CL-04`).
   - **Sellado, verificación y aborto (`main.go`):**
     - `TaskLock` en `.seb/seb.lock` añade `Mode string \`json:"mode,omitempty"\`` (retrocompatible con locks previos).
     - `cmdPlanLock` valida ausencia de etiquetas de aclaración pendientes, exige `Estado: APROBADA` en tareas `FAST`, valida la tabla verificadora en `*_plan_y_verificador.md`, sella hashes + `mode: "fast"` en `.seb/seb.lock`, guarda `mode` en SQLite y transiciona `ACTIVE_ROLE` a `implementador` (`018-RF-10`).
     - `cmdVerify` valida hashes SHA-256 de ambos archivos, ejecuta `validateFastDiffThresholds` y `validateFastAntiRewardHackingDiff` cuando la tarea es `FAST`, corre todos los `<NNN>-CHK-XX` y sella `verified: true` + `commit_hash` (`018-RF-11`, `018-RF-12`, `018-CL-06`).
     - `cmdAbort` (`executeTaskPurge` y `executeTaskAbortFormal`) soporta `*_plan_y_verificador.md` junto a `*_spec.md` (`018-RF-17`).
   - **Menor privilegio (`main.go`, `hook_claude.go`):**
     - Registro de `seb-fast-task` en `canonicalRoles` y `ensureDirectories()`.
     - `checkPermission` restringe `seb-fast-task` exclusivamente a escritura en `specs/*_spec.md`, `specs/*_plan_y_verificador.md` y `MEMORY.md`; bloquea `specs/*_plan_y_verificador.md` para `implementador` y roles de dominio (`018-RF-09`, `018-RF-12`).
     - `shellRedirectContractRegex` protege también `_plan_y_verificador.md` en `PreToolUse`; `inferActiveSkill` transiciona a `implementador` cuando existe tarea sellada activa tras `seb-fast-task`.
   - **Auditoría y Observabilidad (`main.go`, `status.go`, `help.go`):**
     - Migración idempotente de SQLite (`migrateDBSchema`) agregando columna `mode TEXT DEFAULT 'SDD'` a `tasks`.
     - `renderAuditReport` incorpora columna `Modo` (`FAST` vs `SDD`) y tabla resumen comparativa por modo (`018-RF-14`, `018-RF-15`, `018-RNF-04`).
     - `status.go` expone `Mode` (`FAST`/`SDD`) y `HasUnifiedPlanVerifier` en vista completa, `--compact` y `--json`, rechaza artefactos mixtos y adapta alertas y `NextStep` (`018-RF-16`, `018-CL-02`).
     - `help.go` documenta el soporte `FAST` en el catálogo y en las fichas de `plan-lock`, `verify`, `status`, `abort`, `guard`, `role` y `audit` (`018-RF-16`).

3. **Artefactos de `payload/` y aprovisionamiento (`install.sh`, `uninstall.sh`):**
   - Crear `payload/.agents/skills/seb-fast-task/SKILL.md` y sus 2 plantillas acotadas (`<= 35` líneas cada una):
     - `payload/.agents/skills/seb-fast-task/template/nnn_nombre_tarea_spec.md`
     - `payload/.agents/skills/seb-fast-task/template/nnn_nombre_tarea_plan_y_verificador.md`
   - Actualizar `payload/AGENTS.md` (bloque gestionado `<= 30` líneas) enrutando entre Flujo SDD Completo (`.agents/SEB-HARNESS.md`) y Flujo Rápido (`seb-fast-task`) (`018-RF-01`, `018-RF-04`, `018-RNF-01`).
   - Actualizar `payload/.agents/rules/00-seb-harness.md` (`<= 35` líneas) y `payload/.agents/SEB-HARNESS.md` (fila `seb-fast-task` en matriz de menor privilegio).
   - Ajustar mensaje diagnóstico en `payload/.agents/hooks/git-pre-commit` y validar aprovisionamiento/desinstalación idempotentes en `install.sh` (`--self` incluido) y `uninstall.sh` (`018-RF-02`, `018-RF-03`, `018-RF-13`, `018-RNF-02`).

## 2. Desglose Atómico de Tareas (TDD)

1. **Tarea 1 — Skill `seb-fast-task`, plantillas acotadas y enrutamiento (`018-RF-01` a `018-RF-05`, `018-RNF-01`, `018-RNF-02`):**
   - Crear `payload/.agents/skills/seb-fast-task/SKILL.md` y las dos plantillas acotadas (`<= 35` líneas c/u).
   - Actualizar `payload/AGENTS.md` (bloque gestionado `<= 30` líneas), `payload/.agents/rules/00-seb-harness.md` (`<= 35` líneas), `payload/.agents/SEB-HARNESS.md` y el `AGENTS.md` raíz vía `./install.sh --self --no-build`.
   - Escribir pruebas en `engine/cmd/seb/fast_task_test.go` (`TestFastTaskSkillTemplatesAndRouting`, `TestFastTaskInstallAndUninstallLifecycle`).

2. **Tarea 2 — Módulo `fast_task.go`: configuración, detección de modo y umbrales (`018-RF-06`, `018-RF-08`, `018-CL-01`, `018-CL-02`, `018-CL-05`):**
   - Implementar `engine/cmd/seb/fast_task.go` con `FastTrackConfig`, `loadFastTrackConfig`, `detectTaskModeAndArtifacts`, `parseSpecMode`, `validateFastDiffThresholds` y `validateFastAntiRewardHackingDiff`.
   - Escribir pruebas unitarias de configuración por defecto/personalizada/inválida, detección `FAST`/`SDD`/ambigüedad y validación determinista de umbrales de archivos/líneas y anti-reward-hacking (`TestFastTaskConfigAndModeDetection`, `TestFastTaskDiffThresholdsAndAntiRewardHacking`).

3. **Tarea 3 — Guardarraíles de menor privilegio y hooks (`018-RF-09`, `018-RF-12`, `018-RNF-03`):**
   - Registrar `seb-fast-task` en `canonicalRoles`, `ensureDirectories()`, `checkPermission`, `inferActiveSkill` y `shellRedirectContractRegex`.
   - Bloquear escritura sobre `specs/*_plan_y_verificador.md` para `implementador` y roles de dominio; verificar latencia `< 10 ms` en `seb guard`.
   - Escribir pruebas en `fast_task_test.go` (`TestFastTaskGuardPermissionsAndLatency`).

4. **Tarea 4 — Parser unificado, `plan-lock` y `verify` para tareas `FAST` (`018-RF-07`, `018-RF-10`, `018-RF-11`, `018-RF-12`, `018-CL-03`, `018-CL-04`, `018-CL-06`):**
   - Extender `taskIDFromVerifierPath`, `TaskLock`, `TaskInfo`, `resolveTask`, `cmdPlanLock` y `cmdVerify` para soportar `*_plan_y_verificador.md`, sellado con `mode: "fast"`, validación determinista de umbrales y anti-reward-hacking en `verify`, y funcionamiento en clon limpio sin SQLite.
   - Escribir pruebas end-to-end en `fast_task_test.go` (`TestFastTaskPlanLockAndVerifyLifecycle`, `TestFastTaskAntiSilentDropAndTampering`).

5. **Tarea 5 — Integración en `pre-commit` y `seb abort` (`018-RF-13`, `018-RF-17`, `018-CL-01`, `018-CL-03`):**
   - Actualizar `payload/.agents/hooks/git-pre-commit` y `executeTaskPurge`/`executeTaskAbortFormal` para manejar `*_plan_y_verificador.md`.
   - Escribir pruebas de integración Git (`git commit` con hook `pre-commit` pasando y rechazando por umbral excedido o test fallido) y de `seb abort` / `seb abort --purge` (`TestFastTaskPreCommitGate`, `TestFastTaskAbortAndPurge`).

6. **Tarea 6 — Auditoría diferenciada en SQLite y `seb audit report` (`018-RF-14`, `018-RF-15`, `018-RNF-04`):**
   - Migrar esquema de tabla `tasks` agregando `mode TEXT DEFAULT 'SDD'`, actualizar `cmdPlanLock`/`cmdVerify` y enriquecer `renderAuditReport` para distinguir `FAST` vs `SDD` con métricas agregadas sin exponer secretos ni rutas externas.
   - Escribir pruebas en `fast_task_test.go` (`TestFastTaskAuditReportDistinction`).

7. **Tarea 7 — Observabilidad en `seb status` y `seb help` (`018-RF-16`, `018-CL-02`, `018-CL-06`):**
   - Actualizar `status.go` (campos `Mode` y `HasUnifiedPlanVerifier`, alertas de artefactos mixtos, `NextStep` para `FAST`, renderizados full/`--compact`/`--json`) y `help.go` (documentación de tareas `FAST` y `*_plan_y_verificador.md`).
   - Escribir pruebas en `fast_task_test.go` (`TestFastTaskStatusAndHelpObservability`).

8. **Tarea 8 — Compilación, análisis estático y regresión completa (`GLOBAL`):**
   - Compilar `.seb/bin/seb` con `CGO_ENABLED=0 -ldflags="-s -w"`, correr `go vet ./...`, `go test -count=1 -v ./...` y ejecutar `.seb/bin/seb verify flujo_rapido`.
