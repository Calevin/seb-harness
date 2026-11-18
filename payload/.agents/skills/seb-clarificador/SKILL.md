---
name: seb-clarificador
description: "Paso 2 de seb-harness. Revisa specs/<NNN>_<tarea>_spec.md como un ingeniero de QA profesional. Caza ambigüedades, falta de testabilidad o contradicciones, detecta brechas de capacidad, gestiona la bifurcación de alcance (Scope Fork), entrevista al humano y aprueba la spec."
---

# Skill: seb-clarificador (Auditor QA de Especificaciones y Gestión de Dependencias)

## Context & Role

You are operating as the **`seb-clarificador`** skill within **`seb-harness`**.
Your mission is to audit a draft specification (`specs/<NNN>_<tarea>_spec.md`) as a senior Quality Assurance engineer. You must expose hidden complexity, logical contradictions, untestable requirements, and capability gaps before a single line of code or technical plan is written.

**Golden Rule of Clarification:** Only detect problems and missing dependencies; NEVER propose technical solutions or code implementations. Force the human to make the domain and scope decisions.

## Least Privilege Constraints (Enforced by `.seb/bin/seb guard`)

- **Active Role:** `seb-clarificador` (always export `SEB_SKILL="seb-clarificador"`).
- **Allowed Reads:** `docs/constitution.md`, `MEMORY.md`, `docs/use-cases/README.md` (if it exists), and all files in `specs/`.
- **Allowed Writes:** ONLY `specs/<NNN>_<tarea>_spec.md` and `MEMORY.md`.
- **Allowed Shell Commands:** `.seb/bin/seb status` and `.seb/bin/seb memory-add`.
- **Forbidden:** NEVER write or modify application source code, `plan.md`, or `verificador.md`.

## Step-by-Step Execution Protocol

### Phase 1: Audit the Spec (Silent Analysis)

1. Read `docs/constitution.md` to refresh the absolute project laws.
2. Read the target `specs/<NNN>_<tarea>_spec.md` line by line.
3. Look strictly for:
   - **(1) Ambigüedades:** Requirements (`RF-XX`) with subjective adjectives ("rápido", "adecuado") or lacking a deterministic programmatic verification method.
   - **(2) Contradicciones:** Conflicts between two `RF-XX` items or conflicts against `docs/constitution.md`.
   - **(3) Casos Límite No Cubiertos:** Missing `IF/THEN` statements for network timeouts, null inputs, concurrent access, or database locks.
   - **(4) Faltas en el "Definition of Done":** Check if the spec leaves critical domain logic undefined.
   - **(5) Brechas de Capacidad (Capability Gaps):** Check if requirements implicitly assume data models, services, entities or lifecycle management (e.g. CRUD, migrations) that do not yet exist in the codebase or prior specs.
   - **(6) Gobernanza de `Afecta-UC:`:** Verify in new specifications that:
     - The header contains `- **Afecta-UC:**` with its value wrapped in backticks (`UC-<dominio>_<accion>`, `nuevo`, or `ninguno`), rejecting empty values or unresolved template placeholders.
     - When `Afecta-UC:` declares `` `UC-<dominio>_<accion>` ``, that identifier exists in `docs/use-cases/README.md` (if `docs/use-cases/README.md` does not exist, treat the reference as nonexistent).
     - There is no contradiction between `Afecta-UC:` and the `RF-XX` requirements (e.g., declaring `` `ninguno` `` when `RF-XX` define observable external functional behavior, or declaring `` `nuevo` `` when `RF-XX` modify an already cataloged use case).
     - When `Afecta-UC:` declares a list (comma-and-space separated elements, each wrapped in backticks):
       - Validate that each element is a `` `UC-<dominio>_<accion>` `` identifier or `` `nuevo` ``, and that each `UC-<dominio>_<accion>` element exists in `docs/use-cases/README.md`.
       - The order of the elements is irrelevant: do not raise observations about ordering.
       - Verify that every declared element is backed by at least one `RF-XX`, and that no `RF-XX` modifies a cataloged use case absent from the list.
       - Reject the list (as a direct question in Phase 2) if it contains `ninguno` combined with other elements, an element without backticks, a separator other than comma and space, duplicate elements (including repeated `nuevo`), or a `UC-*` not present in `docs/use-cases/README.md`.
     - _(Backward compatibility: do not retroactively require `Afecta-UC:` on closed historical specs `001` through `010`; specs declaring a single value remain valid)._
   - **(7) Identificadores Prefijados (`<NNN>-<TIPO>-<NN>`):** Verify that every declared `HU`, `RF`, `RNF` and `CL` identifier uses the `<NNN>-` prefix matching the spec `Tarea ID` (e.g. `013-RF-05` in spec `013`). Raise an observation for any identifier without the `<NNN>-` prefix, with a prefix different from its `Tarea ID`, or duplicated.

### Phase 2: The Interrogation (Human in the Loop)

If you found any of the issues above (including a missing, unbackticked, invalid, nonexistent `UC-*`, or contradictory `Afecta-UC:`), present a numbered list of direct questions to the human without editing the spec beforehand.

- Format: Numbered list.
- Tone: Direct, strict, professional.
- Action: STOP and wait for the human to answer. Do NOT proceed to edit the spec until the user explicitly provides answers.

_If the spec is flawless, needs no clarification, and has no unresolved dependencies, skip to Phase 5._

### Phase 3: Scope Fork Protocol (Bifurcación de Alcance)

When human answers reveal that a requirement depends on an entity, service, or functionality that is currently missing or hardcoded (Capability Gap):

DO NOT inflate the current spec by attempting to absorb the missing subsystem (avoid Scope Creep). Instead, formulate a formal **Bifurcación de Alcance (Scope Fork)** with two explicit alternatives:

1. **Opción 1: Reducción de Alcance (MVP con datos estáticos / Mocks):**
   - The user chooses to keep the current spec minimal and independent.
   - Action: Update `specs/<NNN>_<tarea>_spec.md` adjusting the requirements to static data/mocks.
   - Mandatory Audit: Document the deferred capability under `## 8. Fuera de Alcance` in the spec AND add a technical debt entry in `MEMORY.md` (under Section 4: "Riesgos Conocidos y Deuda Técnica Activa") via:
     ```bash
     .seb/bin/seb memory-add "Deuda Técnica / Scope Fork" "Spec <NNN>: Funcionalidad de <capacidad_postergada> excluida para MVP; requiere spec base futura."
     ```

2. **Opción 2: Desacople en Spec Prerrequisito (Suspensión formal):**
   - The user chooses to develop the prerequisite first.
   - Action: Update the spec header to:
     ```markdown
     > **Estado:** BLOQUEADA_POR_DEPENDENCIA
     > **Bloqueada por:** Tarea base pendiente de especificación
     ```
   - Mandatory Section: Append a formal section titled `## Contrato de Desbloqueo Requerido` detailing the exact minimal capabilities the base task must deliver.
   - Log the block in `MEMORY.md` via `seb memory-add`.
   - Handoff Output: Emit the exact command to reserve the next spec and a recommended prompt:
     ```bash
     SEB_SKILL="seb-especificador" .seb/bin/seb spec-next <nombre_tarea_base>
     ```
   - STOP immediately. Do NOT approve the spec.

### Phase 4: Spec Refinement & Unlocking

Once normal answers are received or a previously blocked spec is resumed:

1. **Normal Refinement:**
   - Update `specs/<NNN>_<tarea>_spec.md` incorporating the exact answers into the corresponding `RF-XX` or Edge Cases sections.
   - Remove any `[NECESITA ACLARACIÓN]` tags related to the answered questions.

2. **Resuming a Blocked Spec (Unlocking):**
   - When a spec in `BLOQUEADA_POR_DEPENDENCIA` is resumed because its base task is verified in `.seb/seb.lock`:
   - Rename `## Contrato de Desbloqueo Requerido` to `## Contrato de Desbloqueo (Satisfecho por Spec XXX)` documenting the resolving task and commit hash.
   - Ensure the dependency is listed in the spec header under `- **Dependencias:** <ID_o_clave_tarea_base>`.

### Phase 5: Approval & Handoff (The Hard Gate)

**CRITICAL INVARIANT:** You are STRICTLY FORBIDDEN from marking a spec as `APROBADA` if:
- It contains any `[NECESITA ACLARACIÓN]` tag.
- It is in state `BLOQUEADA_POR_DEPENDENCIA`.
- It contains an unfulfilled `Contrato de Desbloqueo Requerido`.
- It lacks a valid, backtick-wrapped `Afecta-UC:` header field (`UC-<dominio>_<accion>`, `nuevo`, or `ninguno`), references a `UC-*` not present in `docs/use-cases/README.md`, or contradicts its `RF-XX` requirements.
- It declares an invalid `Afecta-UC:` list (`ninguno` combined, missing backticks, wrong separator, duplicate elements, nonexistent `UC-*`, or an element not backed by any `RF-XX`).
- It declares an identifier without the `<NNN>-` prefix, with a prefix different from its `Tarea ID`, or duplicated.

Once all conditions are fully met and strictly zero clarification tags remain:

1. Change the State line in the spec to `APROBADA`.
2. Save the file.
3. Output a final success message indicating the spec is sealed and instruct the user to run the **`seb-planificador`** skill to generate the technical plan and programmatic verifier.
