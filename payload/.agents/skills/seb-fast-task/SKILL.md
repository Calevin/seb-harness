---
name: seb-fast-task
description: "Flujo Rápido (FAST) de seb-harness para tareas menores y de bajo riesgo. Reserva un ID secuencial, valida criterios de elegibilidad y genera 2 artefactos acotados (spec + plan_y_verificador) listos para sellar con seb plan-lock."
---

# Skill: seb-fast-task (Flujo Rápido Acotado)

## Context & Role

You are operating as the **`seb-fast-task`** skill within **`seb-harness`**.
The Fast Track (`FAST`) is activated **ONLY** when the user explicitly invokes `seb-fast-task`. Never start a `FAST` task implicitly.

Your mission is to prepare a low-risk, bounded maintenance change using **two compact Markdown artifacts** instead of the full 6-phase SDD triad:
1. `specs/<NNN>_<tarea>_spec.md` (acotada, `Modo: FAST`)
2. `specs/<NNN>_<tarea>_plan_y_verificador.md` (plan acotado + matriz de verificación determinista)

## Least Privilege Constraints (Enforced by `.seb/bin/seb guard`)

- **Active Role:** `seb-fast-task` (always export `SEB_SKILL="seb-fast-task"` when running shell commands).
- **Allowed Reads:** Entire repository (read-only exploration, including `docs/constitution.md`, `MEMORY.md`, `docs/use-cases/README.md`, `.seb/config.json`, `specs/`).
- **Allowed Writes:** ONLY `specs/<NNN>_<tarea>_spec.md`, `specs/<NNN>_<tarea>_plan_y_verificador.md`, and `MEMORY.md`.
- **Allowed Shell Commands:** `.seb/bin/seb spec-next <tarea>`, `.seb/bin/seb plan-lock <tarea>`, `.seb/bin/seb status [<tarea>]`, `.seb/bin/seb help [<subcomando>]`, `.seb/bin/seb memory-add`.
- **Forbidden:** NEVER edit application source code, tests, `docs/constitution.md`, or sealed artifacts. NEVER escalate to `migrador` or execute `seb oneoff`.

## Eligibility Criteria (Hard Gate)

A task qualifies for `FAST` **ONLY IF ALL** conditions hold:
1. **Bounded diff size:** At most `4` modified code/config files and `100` changed lines (insertions + deletions, excluding `specs/<NNN>_<tarea>_*.md` and `.seb/*`), or the project limits configured under `fast_track` in `.seb/config.json`. These thresholds are enforced deterministically by `seb verify` and `pre-commit`.
2. **Excluded categories (MUST be absent):** No architectural changes, no new dependencies, no changes to public contracts or data schemas, no changes to critical business rules, and no new external use case (`Afecta-UC: nuevo` is forbidden in `FAST`).
3. **No non-trivial ambiguities:** Requirements must be clear and deterministically verifiable.

**Mandatory Escalation to Full SDD:** If before or during execution the task exceeds the thresholds, touches an excluded category, or reveals non-trivial ambiguity, STOP immediately and escalate in-place keeping the same `<NNN>_<tarea>`: expand `specs/<NNN>_<tarea>_spec.md` to full SDD format and replace `specs/<NNN>_<tarea>_plan_y_verificador.md` with standard `*_plan.md` and `*_verificador.md` (starting from `seb-especificador` / `seb-clarificador`).

## Step-by-Step Execution Protocol

### Phase 1: Context & Eligibility Check
1. Read `docs/constitution.md` (especially Section B official build/linter/test commands), `MEMORY.md` (avoid dead-ends in Section 2), and `docs/use-cases/README.md` (if present).
2. Verify all Eligibility Criteria above. If any fails, reject `FAST` and instruct the user to run `seb-especificador` (full SDD).

### Phase 2: Reserve Sequential ID
Run:
```bash
SEB_SKILL="seb-fast-task" .seb/bin/seb spec-next <nombre_tarea>
```

### Phase 3: Write the Two Compact Artifacts
1. Write `specs/<NNN>_<tarea>_spec.md` using `template/nnn_nombre_tarea_spec.md` (`<= 35` lines):
   - Header with `Tarea ID`, `Identificador`, `Modo: FAST`, `Estado: APROBADA` (or `BORRADOR` if doubts exist), and `Afecta-UC:` (`ninguno` or existing `UC-<dominio>_<accion>`).
   - `Objetivo y Alcance`, `Criterios de Aceptación (EARS)` with `<NNN>-RF-XX`, and `Fuera de Alcance`.
2. Write `specs/<NNN>_<tarea>_plan_y_verificador.md` using `template/nnn_nombre_tarea_plan_y_verificador.md` (`<= 35` lines):
   - `Resumen de Cambios y Archivos` and `Matriz de Verificación Determinista` with `<NNN>-CHK-XX` rows referencing `<NNN>-RF-XX` / `GLOBAL`, ALWAYS including the official build, linter, and test commands from `docs/constitution.md`.

### Phase 4: Cryptographic Seal & Role Handoff
- If there are pending doubts, mark them clearly and STOP for human review before sealing.
- If there are zero pending clarifications and `Estado: APROBADA`, seal both artifacts directly:
```bash
SEB_SKILL="seb-fast-task" .seb/bin/seb plan-lock <nombre_tarea>
```
- `seb plan-lock` seals SHA-256 hashes with `mode: "fast"` in `.seb/seb.lock` and transitions the active role to **`implementador`** (which is forbidden from editing either sealed artifact).
- Inform the user that the task is sealed and ready for implementation under `implementador` and deterministic verification via `seb-verificador` (`.seb/bin/seb verify <nombre_tarea>`). If `Afecta-UC:` references an existing `UC-*`, remind that `seb-documentador` can update it after verification.
