# UC-documentation_functional_generation: Generacion de Documentacion Funcional de Casos de Uso y Guardarrail de Fase 6

> **Trazabilidad SDD:** Derivado de la spec `specs/010_integracion_documentador_spec.md` (Requisitos: `RF-01` a `RF-13`, `RNF-01`, `RNF-02`, `RNF-03`).

## 1. Ficha de Intencion
- **ID:** UC-documentation_functional_generation
- **Actor Principal:** `Developer`, `AIAgent` (Rol `seb-documentador`)
- **Disparador:** Ejecucion del skill `seb-documentador` tras obtener `VEREDICTO: APROBADO` en la Fase 5 (`seb-verificador`), o aprovisionamiento del skill mediante `./install.sh`.
- **Precondiciones:**
  - La tarea objetivo cuenta con verificacion determinista aprobada (`verified: true` en `.seb/seb.lock`).
  - El rol activo de sesion corresponde a `seb-documentador` (vía `SEB_SKILL="seb-documentador"`, `seb role set seb-documentador` o inferencia de lectura de `.agents/skills/seb-documentador/SKILL.md`).
- **Invariantes de Negocio:**
  - Bajo el rol `seb-documentador`, el guardarraíl de menor privilegio (`checkPermission`) autoriza escrituras exclusivamente sobre `docs/use-cases/*`, `docs/use-cases/README.md` y `MEMORY.md`, bloqueando cualquier modificacion en codigo fuente, tests, `docs/constitution.md` o `specs/*` (`RF-02`, `RF-03`).
  - Todo artefacto generado en `docs/use-cases/UC-<dominio>_<accion>.md` carece por completo de emojis, utiliza `autonumber` en diagramas `sequenceDiagram`, emplea identificadores tecnicos en ingles y redacta las explicaciones en espanol (`RF-04`, `RF-05`, `RF-06`).
  - Si un proyecto existente posee `AGENTS.md` con firma `seb-harness` pero sin mencion a `seb-documentador`, `install.sh` conserva `AGENTS.md` y genera `AGENTS.md.patch.md` (`RF-11`, `CL-03`).

## 2. Diagrama de Interaccion del Sistema (SSD)
```mermaid
sequenceDiagram
    autonumber
    actor Developer as "Developer"
    participant Agent as "DocumenterAgent (seb-documentador)"
    participant Guard as "SebGuardEngine (seb guard / pre-tool)"

    Developer->>Agent: "Invoke /seb-documentador on verified specs"
    Agent->>Guard: "ReadFile(specs/NNN_task_spec.md, engine/cmd/seb/main.go)"
    Guard-->>Agent: "AllowRead()"

    Agent->>Guard: "WriteFile(docs/use-cases/UC-domain_action.md)"
    Guard-->>Agent: "AllowWrite(Target inside docs/use-cases/)"

    Agent->>Guard: "WriteFile(docs/use-cases/README.md)"
    Guard-->>Agent: "AllowWrite(Catalog index updated)"
    Agent-->>Developer: "DocumentationSummary(Generated & indexed Use Cases)"
```

## 3. Arbol de Decisiones
```mermaid
flowchart TD
    Start["Start Phase 6 (seb-documentador)"] --> ReadMeta["Read spec header for 'Afecta-UC:' metadata"]
    ReadMeta --> IsNone{"Is Afecta-UC == ninguno?"}
    IsNone -- "Yes" --> SkipGen["Conclude Phase 6 without generating artifacts (RF-13)"]
    IsNone -- "No" --> CheckExisting{"Points to existing UC-<domain>_<action>.md?"}
    CheckExisting -- "Yes" --> UpdateUC["Update existing docs/use-cases/UC-<domain>_<action>.md (Living Documentation)"]
    CheckExisting -- "No (nuevo or omitted)" --> CreateUC["Create new docs/use-cases/UC-<domain>_<action>.md (CL-04)"]

    UpdateUC --> SyncCatalog["Create or update entry in docs/use-cases/README.md (RF-08)"]
    CreateUC --> SyncCatalog

    SyncCatalog --> GuardCheck{"Agent attempts write outside docs/use-cases/ or MEMORY.md?"}
    GuardCheck -- "Yes" --> BlockWrite["seb guard denies write with exit code 1 (RF-03)"]
    GuardCheck -- "No" --> Complete["Phase 6 completed"]
```

## 4. Caminos Alternativos y Catalogo de Errores
| Error Code | Causa Raiz | Respuesta / Efecto |
| :--- | :--- | :--- |
| `GUARD_DOCUMENTER_WRITE_DENIED` | El rol `seb-documentador` intenta escribir en codigo fuente, tests, `docs/constitution.md` o `specs/*` | Rechazo inmediato con `GUARDARRAIL DE MENOR PRIVILEGIO: El rol 'seb-documentador' NO tiene permiso` y `exit code 1` (`RF-03`) |
| `NO_UC_IMPACT_SKIP` | La especificacion declara `Afecta-UC: ninguno` | Finalizacion limpia de la Fase 6 sin crear archivos en `docs/use-cases/` (`RF-13`) |
| `MISSING_AFECTA_UC_METADATA` | Especificacion redactada sin el campo `Afecta-UC:` | Asuncion por defecto de creacion de nuevo caso de uso bajo `UC-<dominio>_<accion>.md` (`CL-04`) |
| `LEGACY_AGENTS_MD_DETECTED` | `install.sh` detecta `AGENTS.md` con firma `seb-harness` pero sin referencia a `seb-documentador` | Creacion de `AGENTS.md.patch.md` y advertencia en consola sin sobrescribir `AGENTS.md` (`RF-11`) |

## 5. Postcondiciones y Efectos Secundarios
- **Estado Persistido:** Creacion o actualizacion de artefactos `docs/use-cases/UC-<dominio>_<accion>.md` y sincronizacion del catalogo indice `docs/use-cases/README.md`. Durante la desinstalacion estandar (`uninstall.sh`), todos los casos de uso en `docs/use-cases/` se preservan intactos (`RF-12`).
- **Eventos Emitidos:** Ninguno.
- **Auditoria:** Registro de operaciones de lectura/escritura y posibles bloqueos de menor privilegio en `agent_actions` de `.seb/seb-audit.db`.
