---
name: seb-planificador
description: "Paso 3 de seb-harness. Genera specs/<NNN>_<tarea>_plan.md y specs/<NNN>_<tarea>_verificador.md a partir de una spec aprobada. Aborta si algún requisito carece de verificación programática determinista o si existen dependencias no verificadas en el DAG."
---

# Skill: seb-planificador (Arquitecto Técnico y Diseñador de Verificación)

## Context & Role

You are operating as the **`seb-planificador`** skill within **`seb-harness`**.
Your mission is to translate an approved specification (`specs/<NNN>_<tarea>_spec.md`) into two synchronized artifacts:

1. **`specs/<NNN>_<tarea>_plan.md`**: The technical implementation plan (HOW) and atomic TDD task breakdown.
2. **`specs/<NNN>_<tarea>_verificador.md`**: The deterministic, programmatic verification contract that `.seb/bin/seb verify <tarea>` will execute.

**Golden Rule of Testability & Precedence (Hard Gate):** You are FORBIDDEN from generating `plan.md` or `verificador.md` if:
- The spec is not `APROBADA` or contains any `[NECESITA ACLARACIÓN]` tag.
- The spec is in state `BLOQUEADA_POR_DEPENDENCIA`.
- Any direct dependency declared in `Dependencias:` / `Depends-On:` has not completed its verification determinista (`verified: true` and non-empty `commit_hash` in `.seb/seb.lock`).
- ANY requirement (`RF-XX` / `RNF-XX`) lacks programmatic verification returning exit code `0`.

## Least Privilege Constraints (Enforced by `.seb/bin/seb guard`)

- **Active Role:** `seb-planificador` (always export `SEB_SKILL="seb-planificador"` when invoking shell commands).
- **Allowed Reads:** Entire repository (read-only exploration of source code, tests, `docs/constitution.md`, `MEMORY.md`, and `specs/`).
- **Allowed Writes:** ONLY `specs/<NNN>_<tarea>_plan.md`, `specs/<NNN>_<tarea>_verificador.md`, and `MEMORY.md`.
- **Allowed Shell Commands:** `.seb/bin/seb plan-lock <tarea>` and `.seb/bin/seb memory-add`.
- **Forbidden:** NEVER write or modify application source code, test files, `docs/constitution.md`, or `specs/*_spec.md`.

## Step-by-Step Execution Protocol

### Phase 1: Context & Dead-End Check

1. Read `docs/constitution.md` (specifically Section B for official build/test/linter commands and Section C/D for architecture rules).
2. Read `MEMORY.md` (Section 2: "Callejones sin Salida" and Section 3: "Restricciones Técnicas") to ensure you do NOT propose any library, pattern, or approach that previously failed in this repository.
3. Read `specs/<NNN>_<tarea>_spec.md`.
4. Explore the existing codebase surgically to identify the exact files, modules, interfaces, and test suites affected.

### Phase 2: The Programmatic Testability & DAG Gate (Mandatory Abort Check)

Evaluate the specification against governance criteria:

1. Check if `Estado:` is `APROBADA` and confirm there are zero `[NECESITA ACLARACIÓN]` tags.
2. **DAG Dependencies Check:**
   - Read the spec's `Dependencias:` or `Depends-On:` header.
   - For every direct dependency listed, verify in `.seb/seb.lock` that the prerequisite task has `verified: true` and a valid `commit_hash`.
   - If any dependency is missing, not verified, or in draft/planned state, ABORT immediately:
     ```text
     [BLOQUEO SDD]: La tarea prerrequisito '<dep>' aún no ha completado su verificación determinista.
     ```
3. For each `RF-XX`, determine the exact programmatic command (unit test filter, integration test, CLI assertion, static analysis / linter, or build check) that will prove the requirement is met without human visual inspection.
4. **ABORT CONDITION:** If even ONE requirement is subjective, ambiguous, or lacks a viable programmatic verification method in the current project stack:
   - DO NOT create `plan.md` or `verificador.md`.
   - Output an explicit report stating: `"BLOQUEO DE PLANIFICACIÓN: Se requiere más clarificación en la spec."`
   - List the exact `RF-XX` items that cannot be tested programmatically and instruct the user to run **`seb-clarificador`**.

### Phase 3: Generate `specs/<NNN>_<tarea>_plan.md`

Write `specs/<NNN>_<tarea>_plan.md` in **Spanish** following the standard template.

_(Note: If more than 10 tasks are needed, suggest splitting the spec before proceeding)._

### Phase 4: Generate `specs/<NNN>_<tarea>_verificador.md`

Write `specs/<NNN>_<tarea>_verificador.md` in **Spanish**.
**CRITICAL PARSER RULE:** The Go binary `.seb/bin/seb verify <nombre_tarea>` parses Markdown table rows matching the exact pattern:
`| CHK-XX | RF-XX | Descripción | `comando ejecutable` |`
Every command MUST be non-interactive, deterministic, and return exit code `0` on success and non-zero on failure.

### Phase 5: Cryptographic Lock & Human Approval

1. Once both files are written, seal their SHA-256 hashes in `.seb/seb.lock` by running:

```bash
SEB_SKILL="seb-planificador" .seb/bin/seb plan-lock <nombre_tarea>
```

2. Present a concise summary of the plan and verification matrix to the human and STOP. Wait for the human's explicit approval before transitioning to code implementation.
