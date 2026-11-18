# Plan Técnico — Gobernanza del Metadato `Afecta-UC:` en Especificación y Clarificación SDD (Spec 011)

- **Spec Origen:** `specs/011_gobernanza_afecta_uc_spec.md`
- **Verificador Asociado:** `specs/011_gobernanza_afecta_uc_verificador.md`
- **Estado:** `PLANIFICADO`

## 1. Resumen Arquitectónico y Aislamiento de Lógica

Esta tarea implementa la gobernanza declarativa del metadato `Afecta-UC:` en las Fases 1 y 2 del ciclo SDD operando exclusivamente sobre las directivas de habilidades y plantillas distribuibles de `payload/.agents/`, sin modificar el código productivo del motor Go (`engine/cmd/seb/main.go`) ni alterar especificaciones históricas (`001` a `010`).

Los cambios se estructuran en tres frentes sincronizados:

1. **Directiva y Plantilla de Especificación (`seb-especificador`):**
   - Actualizar `payload/.agents/skills/seb-especificador/SKILL.md` para incluir `docs/use-cases/README.md` en `Allowed Reads` y en la Fase 1 (*Context Alignment & Input Ingestion*), instruyendo al agente a consultar el catálogo existente antes de redactar la cabecera.
   - Documentar explícitamente las reglas de asignación de `Afecta-UC:` con valor delimitado entre backticks: `` `UC-<dominio>_<accion>` `` (evolución de caso existente), `` `nuevo` `` (nuevo comportamiento funcional externo o catálogo inexistente con impacto externo) o `` `ninguno` `` (refactorizaciones internas / infraestructura sin impacto funcional externo), así como la división en specs independientes si una petición afecta a múltiples casos de uso (`CL-03`).
   - Actualizar `payload/.agents/skills/seb-especificador/template/nnn_nombre_tarea_spec.md` para incluir `- **Afecta-UC:**` en la cabecera con el formato entre backticks y sus tres opciones válidas.

2. **Directiva de Auditoría y Bloqueo en Clarificación (`seb-clarificador`):**
   - Actualizar `payload/.agents/skills/seb-clarificador/SKILL.md` para incluir `docs/use-cases/README.md` en `Allowed Reads` y añadir en la Fase 1 (*Audit the Spec*) la auditoría obligatoria del campo `Afecta-UC:`: presencia, formato entre backticks (`` `UC-<dominio>_<accion>` ``, `` `nuevo` `` o `` `ninguno` ``), rechazo de valores vacíos o marcadores sin completar, existencia referencial en `docs/use-cases/README.md` (y tratamiento como inexistente si el catálogo no existe), y coherencia lógica respecto a los `RF-XX`.
   - Reforzar en la Fase 2 (*The Interrogation*) que ante cualquier omisión, formato inválido, referencia `UC-*` inexistente o contradicción en `Afecta-UC:`, el agente debe formular la pregunta directamente sin editar la spec previamente y detener el flujo hasta recibir respuesta del usuario.
   - Incorporar en la Fase 5 (*Approval & Handoff — The Hard Gate*) la prohibición explícita de marcar la spec como `APROBADA` si `Afecta-UC:` falta, es inválido, referencia un `UC-*` inexistente o contradice los `RF-XX`.

3. **Suite de Verificación Programática (`engine/cmd/seb/afecta_uc_governance_test.go`):**
   - Incorporar pruebas unitarias Go de solo lectura sobre los artefactos de `payload/.agents/`, `.agents/`, `specs/001..010_*_spec.md` y `.seb/seb.lock` para validar deterministamente todos los requisitos funcionales, la consistencia de distribución (`payload/.agents/` vs `.agents/`) y la compatibilidad hacia atrás con las specs `001` a `010`.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta | Acción (`CREAR` / `MODIFICAR`) | Responsabilidad Técnica | Requisitos Cubiertos (`RF-XX`) |
| :--- | :--- | :--- | :--- |
| `payload/.agents/skills/seb-especificador/SKILL.md` | `MODIFICAR` | Consulta obligatoria de `docs/use-cases/README.md`, reglas de `Afecta-UC:` entre backticks y división de alcance multi-UC | `RF-01`, `RF-02`, `RF-04`, `RF-10`, `CL-03` |
| `payload/.agents/skills/seb-especificador/template/nnn_nombre_tarea_spec.md` | `MODIFICAR` | Incorporar `- **Afecta-UC:**` con formato entre backticks y sus tres valores válidos en la cabecera de la plantilla | `RF-02`, `RF-03`, `RF-10` |
| `payload/.agents/skills/seb-clarificador/SKILL.md` | `MODIFICAR` | Auditoría de presencia, backticks, existencia en `docs/use-cases/README.md`, coherencia con `RF-XX`, interrogación sin edición previa y bloqueo duro en Fase 5 | `RF-05`, `RF-06`, `RF-07`, `RF-08`, `RF-09`, `RF-10`, `CL-01`, `CL-02` |
| `engine/cmd/seb/afecta_uc_governance_test.go` | `CREAR` | Pruebas deterministas de directivas, plantilla, sincronización `payload/.agents` / `.agents` y compatibilidad hacia atrás (`001` a `010`) | `RF-01`..`RF-11`, `RNF-01`, `RNF-02`, `CL-01`..`CL-03` |

## 3. Contratos y Reglas a Incorporar

### Cabecera de Plantilla (`template/nnn_nombre_tarea_spec.md`)

```markdown
- **Tarea ID:** `<NNN>`
- **Identificador:** `<nombre_tarea>`
- **Origen:** [Prosa / Jira: KEY-123 / GitHub Issue: #123]
- **Estado:** `BORRADOR`
- **Afecta-UC:** [`UC-<dominio>_<accion>` | `nuevo` | `ninguno`]
```

### Reglas de Validación en `seb-clarificador/SKILL.md`

- **Presencia y formato:** Verificar que exista `- **Afecta-UC:**` con valor delimitado entre backticks perteneciente exactamente a `` `UC-<dominio>_<accion>` ``, `` `nuevo` `` o `` `ninguno` `` (rechazando valores vacíos o el marcador de plantilla sin resolver).
- **Existencia referencial:** Si declara `` `UC-<dominio>_<accion>` ``, validar que dicho identificador figure en `docs/use-cases/README.md` (si `docs/use-cases/README.md` no existe, la referencia se considera inexistente).
- **Coherencia con `RF-XX`:** Detectar contradicciones entre `Afecta-UC:` y el alcance funcional (por ejemplo, `` `ninguno` `` con requisitos funcionales externos observables, o `` `nuevo` `` cuando el comportamiento ya pertenece a un `UC-*` catalogado).
- **Interrogación sin edición previa y Hard Gate:** Ante cualquier fallo, preguntar en Fase 2 sin editar la spec previamente y bloquear la transición a `APROBADA` en Fase 5.

## 4. Decisiones Técnicas Justificadas

- **Decisión 1: Implementación puramente declarativa en `payload/.agents/` sin alterar `engine/cmd/seb/main.go`:**
  - *Justificación:* Cumple con `RF-10` y `RNF-02`, evitando acoplar el binario Go a reglas de formato de Markdown o parsing de cabeceras de specs históricas (`001` a `010`), preservando la velocidad de `seb guard` (`<1ms`) y la compatibilidad hacia atrás (`RF-11`).
- **Decisión 2: Verificación automatizada mediante tests Go en `engine/cmd/seb/afecta_uc_governance_test.go`:**
  - *Justificación:* Reutiliza el patrón establecido en `skill_definition_test.go` (Spec 010) para verificar de forma determinista y multiplataforma que las directivas, la plantilla, la sincronización con `.agents/` y el estado de las specs `001` a `010` cumplan el contrato sin introducir scripts externos ni dependencias.

## 5. Secuencia de Tareas Atómicas (TDD)

- [ ] **T1. Actualizar directiva y plantilla de `seb-especificador`** (`RF-01`, `RF-02`, `RF-03`, `RF-04`, `RF-10`, `CL-03`)
  - _Archivos:_ `payload/.agents/skills/seb-especificador/SKILL.md`, `payload/.agents/skills/seb-especificador/template/nnn_nombre_tarea_spec.md`
  - _Hecho cuando:_ `CHK-01` de `specs/011_gobernanza_afecta_uc_verificador.md` devuelve exit code `0`.
- [ ] **T2. Actualizar directiva de auditoría y bloqueo de `seb-clarificador`** (`RF-05`, `RF-06`, `RF-07`, `RF-08`, `RF-09`, `RF-10`, `CL-01`, `CL-02`)
  - _Archivos:_ `payload/.agents/skills/seb-clarificador/SKILL.md`
  - _Hecho cuando:_ `CHK-02` de `specs/011_gobernanza_afecta_uc_verificador.md` devuelve exit code `0`.
- [ ] **T3. Implementar suite de verificación de gobernanza y compatibilidad histórica (`001` a `010`)** (`RF-11`, `RNF-01`, `RNF-02`)
  - _Archivos:_ `engine/cmd/seb/afecta_uc_governance_test.go`
  - _Hecho cuando:_ `CHK-03`, `CHK-98` y `CHK-99` de `specs/011_gobernanza_afecta_uc_verificador.md` devuelven exit code `0`.
