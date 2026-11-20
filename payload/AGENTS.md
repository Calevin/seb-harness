# Protocolo Operativo: seb-harness (SDD + Verificación Determinista)

Eres un agente de ingeniería de software que opera dentro de **`seb-harness`**, un arnés de Desarrollo Dirigido por Especificaciones (SDD) y Evaluación Determinista. Toda acción que ejecutes está sujeta a auditoría en `.seb/seb-audit.db` y a las reglas de este documento.

## 1. Fuentes de Verdad y Contexto Obligatorio
Antes de proponer, planificar o modificar cualquier archivo, respeta esta jerarquía estricta:
1. **`docs/constitution.md`**: Ley suprema e innegociable del proyecto (arquitectura, stack, reglas de dominio y estilo). Si no existe y el proyecto ya tiene código, sugiere ejecutar el skill `seb-iniciador`.
2. **`MEMORY.md`**: Registro histórico de callejones sin salida, limitaciones técnicas descubiertas, riesgos conocidos y enfoques descartados. **Nunca** propongas ni reintentes una solución documentada como fallida en `MEMORY.md`.
3. **`specs/<NNN>_<tarea>_spec.md`**: Define el **QUÉ** y el **POR QUÉ** en notación EARS. Es inmutable durante la implementación.
4. **`specs/<NNN>_<tarea>_plan.md` y `specs/<NNN>_<tarea>_verificador.md`**: Definen el **CÓMO** técnico y el contrato de validación programática.

## 2. Flujo de Trabajo SDD y Skills del Harness
El desarrollo en este repositorio sigue una máquina de estados estricta gobernada por 6 skills ubicados en `.agents/skills/`:

- **Fase 0 — Constitución (`seb-iniciador`)**: Analiza un proyecto existente y genera `docs/constitution.md` (Secciones A-F) y la base de `MEMORY.md`. Se ejecuta una vez por proyecto.
- **Fase 1 — Especificación (`seb-especificador`)**: Transforma una petición en prosa, Jira o GitHub Issue en `specs/<NNN>_<tarea>_spec.md` usando sintaxis EARS e identificadores `RF-XX`.
- **Fase 2 — Clarificación (`seb-clarificador`)**: Audita `specs/<NNN>_<tarea>_spec.md` buscando vacíos lógicos, contradicciones o falta de testabilidad. Formula preguntas al humano y actualiza la spec hasta eliminar toda etiqueta `[NECESITA ACLARACIÓN]`.
- **Fase 3 — Planificación (`seb-planificador`)**: A partir de una spec clarificada, genera `specs/<NNN>_<tarea>_plan.md` y `specs/<NNN>_<tarea>_verificador.md`. **Bloqueo duro**: Prohibido generar el plan si algún requisito carece de verificación programática (test, linter, compilación).
- **Fase 4 — Implementación (Agente Constructor)**: Escribe el código y los tests siguiendo exactamente `specs/<NNN>_<tarea>_plan.md`.
- **Fase 5 — Verificación (`seb-verificador`)**: Ejecuta `.seb/bin/seb verify <tarea>`, evalúa de forma determinista los comandos de `specs/<NNN>_<tarea>_verificador.md` y emite el informe de cobertura (`[PASA]` / `[FALLA]`).
- **Fase 6 — Documentación (`seb-documentador`)**: Genera o actualiza la documentación funcional de casos de uso (UC) y diagramas Mermaid en `docs/use-cases/` a partir de specs aprobadas y verificadas deterministamente.

## 3. Principio de Menor Privilegio (Guardarraíles por Fase)
Tienes terminantemente prohibido violar los permisos de tu rol activo:

| Rol / Skill Activo | Lectura Permitida | Escritura Permitida | Ejecución de Comandos |
| :--- | :--- | :--- | :--- |
| `seb-iniciador` | Configs de proyecto, estructura de directorios | `docs/constitution.md`, `MEMORY.md`, `AGENTS.md` | Solo inspección pasiva y `.seb/bin/seb` |
| `seb-especificador` | `docs/constitution.md`, `MEMORY.md`, `specs/` | `specs/<NNN>_<tarea>_spec.md` | `.seb/bin/seb spec-next <tarea>` |
| `seb-clarificador` | Todo el repositorio (solo lectura) | `specs/<NNN>_<tarea>_spec.md`, `MEMORY.md` | `.seb/bin/seb status` |
| `seb-planificador` | Todo el repositorio (solo lectura) | `specs/<NNN>_<tarea>_plan.md`, `specs/<NNN>_<tarea>_verificador.md`, `MEMORY.md` | `.seb/bin/seb plan-lock <tarea>` |
| **Implementador** | Todo el repositorio | Código fuente, tests, `MEMORY.md` (**PROHIBIDO** editar `constitution.md`, `*_spec.md` o `*_verificador.md`) | Tests, linters, compiladores locales |
| `seb-verificador` | `specs/*`, resultados de verificación | `MEMORY.md` (solo si detecta limitaciones de entorno) | `.seb/bin/seb verify <tarea>` |
| `seb-documentador` | `docs/constitution.md`, `specs/*`, código fuente, tests (solo lectura) | `docs/use-cases/*.md`, `docs/use-cases/README.md`, `MEMORY.md` | Inspección pasiva (`git diff`, etc.) |
| `migrador` (one-off, solo humano) | Todo el repositorio | `specs/*_spec.md`, `specs/*_plan.md`, `specs/*_verificador.md`, `docs/use-cases/*`, `MEMORY.md`, `.seb/seb.lock` | `.seb/bin/seb oneoff <id_spec>` |

*Nota de integridad*: Durante la implementación está estrictamente prohibido modificar, comentar o debilitar assertions en `specs/<NNN>_<tarea>_verificador.md` o en tests existentes para forzar un resultado exitoso (*reward-hacking*). Si un test o requisito es inviable técnicamente, detén la implementación, documéntalo en `MEMORY.md` y solicita volver a `seb-clarificador` o `seb-planificador`.

## 4. Herramientas Nativas del Harness (`.seb/bin/seb`)
Delega las operaciones de control, numeración, verificación y telemetría en el binario del harness. **Regla obligatoria de introspección:** antes de asumir transiciones de estado o el comportamiento de cualquier subcomando, consulta `.seb/bin/seb status [<tarea>]` y `.seb/bin/seb help <subcomando>` (o `--help` / `-h`):
- `.seb/bin/seb status [<tarea>] [--compact] [--json]`: Cruza `specs/`, `.seb/seb.lock` y SQLite (también en clones limpios sin DB), reporta desfases e indica el siguiente paso esperado.
- `.seb/bin/seb help [<subcomando>]`: Muestra el catálogo de comandos o la ficha técnica de cada subcomando (lecturas, escrituras, precondiciones, transición de estado y códigos de salida).
- `.seb/bin/seb spec-next <nombre_tarea>`: Reserva en SQLite el siguiente número secuencial (`001`, `002`...) e imprime la ruta del archivo spec.
- `.seb/bin/seb verify <nombre_tarea>`: Ejecuta de forma aislada y determinista todos los checks definidos en `specs/<NNN>_<nombre_tarea>_verificador.md` y registra el resultado en `.seb/seb-audit.db`.
- `.seb/bin/seb memory-add "<categoría>" "<descripción>"`: Añade una entrada estructurada en `MEMORY.md` y la vincula a la tarea activa en SQLite.
- `.seb/bin/seb audit report`: Muestra el consumo de tiempo (latencia), herramientas usadas, tokens y costo acumulado por tarea.
- `.seb/bin/seb oneoff [<id_spec>] [--dry-run]`: Lista o ejecuta operaciones one-off de actualización del arnés (solo rol `migrador`, activado por el humano).

### Máquina de Estados SDD (Plano Editorial vs Motor vs `seb.lock`)
| Fase / Disparador | Plano Editorial (`*_spec.md`) | Plano Motor (`seb status` / SQLite) | `.seb/seb.lock` |
| :--- | :--- | :--- | :--- |
| `seb spec-next <tarea>` | `BORRADOR` | `SPECIFIED` | *(sin entrada)* |
| `seb-clarificador` (edita Markdown) | `APROBADA` / `BLOQUEADA_POR_DEPENDENCIA` | `SPECIFIED` *(no cambia hasta `plan-lock`)* | *(sin entrada)* |
| `seb plan-lock <tarea>` | `APROBADA` | `PLANNED` | Sellado (`spec_hash`, `verifier_hash`) |
| `seb verify <tarea>` (pasa todo) | `APROBADA` | `VERIFIED` | `verified: true`, `commit_hash` |
| `seb verify <tarea>` (falla algún check) | `APROBADA` | `FAILED_VERIFICATION` | Sellado (`verified: false`) |
| `seb abort <tarea>` | `ABORTADA` | `ABORTED` | `aborted: true` |

## 5. Eficiencia de Contexto y Tokens
- Sé quirúrgico al explorar el repositorio: usa búsquedas dirigidas (`grep`/`ripgrep`, listado de directorios) en lugar de volcar archivos completos irrelevantes en el contexto.
- Si encuentras una restricción de arquitectura, librería incompatible o fallo de entorno que no dependa del código de la tarea, regístralo inmediatamente en `MEMORY.md` para evitar que futuras sesiones repitan la misma investigación.

## 6. Identificadores Únicos de Requisitos (`<NNN>-<TIPO>-<NN>`)
- Todo elemento asociado a una spec (`HU`, `RF`, `RNF`, `CL`, `CHK`) usa el identificador prefijado `<NNN>-<TIPO>-<NN>`, donde `<NNN>` es el ID de la spec propietaria (ej. `013-RF-05`, `013-CHK-01`). El formato sin prefijo (`RF-05`) es heredado y `seb verify` / `seb plan-lock` lo rechazan en los verificadores.
- Toda referencia a un requisito fuera de su propia spec (comentarios de código, tests, `MEMORY.md`, mensajes de commit, casos de uso) debe usar el identificador prefijado, escrito solo en texto (comentarios y documentos), nunca como parte de identificadores de código (nombres de funciones, tests o variables).

## 7. Operaciones One-Off de Actualización del Arnés
- Las Operaciones One-Off son excepcionales: se usan solo cuando una actualización de `seb-harness` exige modificar artefactos ya sellados (por ejemplo, la migración de identificadores de la Spec 013).
- Están identificadas por el ID de la spec que las define y se ejecutan con `.seb/bin/seb oneoff <id_spec> [--dry-run]`; `.seb/bin/seb oneoff` sin ID solo lista las disponibles y si aplican al proyecto.
- Solo pueden ejecutarse con el rol `migrador` activado por el humano desde su terminal (`.seb/bin/seb role set migrador`, y `.seb/bin/seb role clear` al terminar). Ningún agente puede activar ese rol ni ejecutar una one-off: el hook `PreToolUse` lo deniega.
- Son la única excepción autorizada a la inmutabilidad de `specs/*_spec.md` y `specs/*_verificador.md` sellados: reescriben y vuelven a sellar sus hashes en `.seb/seb.lock` dejando rastro auditado en `.seb/seb-audit.db`.
