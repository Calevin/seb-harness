# UC-documentation_functional_generation: Generacion de Documentacion Funcional de Casos de Uso y Guardarrail de Fase 6

> **Trazabilidad SDD:** Derivado de la spec `specs/010_integracion_documentador_spec.md` (Requisitos: `010-RF-01` a `010-RF-13`, `010-RNF-01`, `010-RNF-02`, `010-RNF-03`).
> **Revision (Spec 014):** `specs/014_afecta_uc_multiple_spec.md` (Requisitos: `014-RF-08` a `014-RF-12`, `014-CL-07`) incorpora el procesamiento de listas en `Afecta-UC:`, la resolucion de `nuevo` en uno o mas casos de uso y la columna "Specs de Evolución" del catalogo.
> **Revision (Spec 013):** `specs/013_identificadores_unicos_requisitos_spec.md` (Requisitos: `013-RF-05`, `013-RF-06`, `013-RF-31`) exige que los casos de uso citen requisitos con el identificador prefijado `<NNN>-<TIPO>-<NN>` de la spec propietaria y que la Fase 2 bloquee specs con identificadores sin prefijo.
> **Revision (Spec 017):** `specs/017_separacion_protocolo_agents_spec.md` (Requisitos: `017-RF-01`, `017-RF-09`, `017-RF-13`, `017-CL-04`) retira la generacion del parche auxiliar legado `AGENTS.md.patch.md` y consolida la documentacion de la Fase 6 (`seb-documentador`) en el protocolo gestionado `.agents/SEB-HARNESS.md`.

## 1. Ficha de Intencion
- **ID:** UC-documentation_functional_generation
- **Actor Principal:** `Developer`, `AIAgent` (Rol `seb-documentador`)
- **Disparador:** Ejecucion del skill `seb-documentador` tras obtener `VEREDICTO: APROBADO` en la Fase 5 (`seb-verificador`), o aprovisionamiento del skill mediante `./install.sh`.
- **Precondiciones:**
  - La tarea objetivo cuenta con verificacion determinista aprobada (`verified: true` en `.seb/seb.lock`).
  - El rol activo de sesion corresponde a `seb-documentador` (vía `SEB_SKILL="seb-documentador"`, `seb role set seb-documentador` o inferencia de lectura de `.agents/skills/seb-documentador/SKILL.md`).
- **Invariantes de Negocio:**
  - Bajo el rol `seb-documentador`, el guardarraíl de menor privilegio (`checkPermission`) autoriza escrituras exclusivamente sobre `docs/use-cases/*`, `docs/use-cases/README.md` y `MEMORY.md`, bloqueando cualquier modificacion en codigo fuente, tests, `docs/constitution.md` o `specs/*` (`010-RF-02`, `010-RF-03`).
  - Todo artefacto generado en `docs/use-cases/UC-<dominio>_<accion>.md` carece por completo de emojis, utiliza `autonumber` en diagramas `sequenceDiagram`, emplea identificadores tecnicos en ingles y redacta las explicaciones en espanol (`010-RF-04`, `010-RF-05`, `010-RF-06`).
  - La propagacion de reglas de la Fase 6 (`seb-documentador`) en proyectos existentes se realiza automaticamente al sobrescribir `.agents/SEB-HARNESS.md` y actualizar el bloque gestionado en `AGENTS.md`; ya no se genera `AGENTS.md.patch.md` (Spec 017 `017-RF-09`, `017-RF-13`, `017-CL-04`, que reemplaza a `010-RF-11`).
  - Cuando `Afecta-UC:` declara una lista (elementos entre backticks separados por coma y espacio, sin orden significativo), se actualiza cada caso de uso `UC-<dominio>_<accion>` declarado (Spec 014 `014-RF-08`).
  - Un elemento `nuevo` (valor simple o dentro de una lista, como maximo una vez) se resuelve en uno o mas casos de uso nuevos cuya cantidad se deriva de los `RF-XX`, cada uno con un identificador inexistente en el catalogo (Spec 014 `014-RF-09`).
  - El catalogo `docs/use-cases/README.md` mantiene la columna "Specs de Evolución" inmediatamente despues de "Spec Originaria"; al evolucionar un caso de uso se agrega la ruta de la spec sin modificar su "Spec Originaria" y sin duplicar rutas (Spec 014 `014-RF-10`, `014-RF-11`, `014-CL-07`).
  - Toda cita de requisitos, historias, casos limite o controles en un caso de uso usa el identificador prefijado de la spec propietaria (ej. `013-RF-05`, nunca `RF-05` a secas), dado que un mismo caso de uso consolida requisitos de varias specs (Spec 013 `013-RF-31`).
  - La spec que alimenta la Fase 6 ya llega auditada por `seb-clarificador`: todos sus identificadores declarados usan el prefijo `<NNN>-` igual a su `Tarea ID`, sin duplicados (Spec 013 `013-RF-05`, `013-RF-06`).

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

    loop "For each element in Afecta-UC list"
        Agent->>Guard: "WriteFile(docs/use-cases/UC-domain_action.md)"
        Guard-->>Agent: "AllowWrite(Target inside docs/use-cases/)"
    end

    Agent->>Guard: "WriteFile(docs/use-cases/README.md)"
    Guard-->>Agent: "AllowWrite(Catalog row and Specs de Evolucion updated)"
    Agent-->>Developer: "DocumentationSummary(Generated & indexed Use Cases)"
```

## 3. Arbol de Decisiones
```mermaid
flowchart TD
    Start["Start Phase 6 (seb-documentador)"] --> ReadMeta["Read spec header for 'Afecta-UC:' metadata"]
    ReadMeta --> IsNone{"Is Afecta-UC == ninguno?"}
    IsNone -- "Yes" --> SkipGen["Conclude Phase 6 without generating artifacts (010-RF-13)"]
    IsNone -- "No" --> Split["Split value into elements (single value or comma-and-space list)"]
    Split --> NextElem{"Next element?"}
    NextElem -- "UC-domain_action" --> UpdateUC["Update existing docs/use-cases/UC-domain_action.md (Living Documentation)"]
    NextElem -- "nuevo" --> Derive["Derive count of new use cases from RF-XX (014-RF-09)"]
    Derive --> CreateUC["Create one docs/use-cases/UC-domain_action.md per new use case with unused ID"]
    NextElem -- "No more elements" --> HasColumn{"Catalog has 'Specs de Evolucion' column?"}

    UpdateUC --> AddEvolution["Append spec path to 'Specs de Evolucion' if not present (014-RF-11, 014-CL-07)"]
    AddEvolution --> NextElem
    CreateUC --> AddRow["Add catalog row with Spec Originaria and '-' as evolution"]
    AddRow --> NextElem

    HasColumn -- "No" --> Migrate["Add column after 'Spec Originaria' and fill empty rows with '-' (014-RF-12)"]
    HasColumn -- "Yes" --> GuardCheck
    Migrate --> GuardCheck{"Agent attempts write outside docs/use-cases/ or MEMORY.md?"}
    GuardCheck -- "Yes" --> BlockWrite["seb guard denies write with exit code 1 (010-RF-03)"]
    GuardCheck -- "No" --> Complete["Phase 6 completed"]
```

## 4. Caminos Alternativos y Catalogo de Errores
| Error Code | Causa Raiz | Respuesta / Efecto |
| :--- | :--- | :--- |
| `GUARD_DOCUMENTER_WRITE_DENIED` | El rol `seb-documentador` intenta escribir en codigo fuente, tests, `docs/constitution.md` o `specs/*` | Rechazo inmediato con `GUARDARRAIL DE MENOR PRIVILEGIO: El rol 'seb-documentador' NO tiene permiso` y `exit code 1` (`010-RF-03`) |
| `NO_UC_IMPACT_SKIP` | La especificacion declara `Afecta-UC: ninguno` | Finalizacion limpia de la Fase 6 sin crear archivos en `docs/use-cases/` (`010-RF-13`) |
| `MISSING_AFECTA_UC_METADATA` | Especificacion redactada sin el campo `Afecta-UC:` | Asuncion por defecto de creacion de nuevo caso de uso bajo `UC-<dominio>_<accion>.md` (`010-CL-04`) |
| `LEGACY_AGENTS_MD_DETECTED` | `install.sh` detecta `AGENTS.md` con firma `seb-harness` en formato legado (sin marcadores) | Respaldo automatico en `AGENTS.legacy-seb.md` e instalacion del enrutador gestionado + `.agents/SEB-HARNESS.md`, sin generar `AGENTS.md.patch.md` (`017-RF-06`, `017-RF-09`, `017-RF-13`) |
| `CATALOG_MISSING_EVOLUTION_COLUMN` | `docs/use-cases/README.md` existe sin la columna "Specs de Evolución" | Se agrega la columna tras "Spec Originaria" y se completan con `-` las filas sin evoluciones (Spec 014 `014-RF-12`) |
| `UNPREFIXED_CITATION` | Un caso de uso cita un requisito sin el prefijo de su spec | Debe reemplazarse por `<NNN>-<TIPO>-<NN>`; en specs nuevas el formato heredado no se acepta (Spec 013 `013-RF-31`) |
| `EVOLUTION_ALREADY_REGISTERED` | La ruta de la spec ya figura en "Specs de Evolución" del caso de uso | No se duplica la ruta; la fila queda sin cambios en esa columna (Spec 014 `014-CL-07`) |

## 5. Postcondiciones y Efectos Secundarios
- **Estado Persistido:** Creacion o actualizacion de artefactos `docs/use-cases/UC-<dominio>_<accion>.md` (uno por cada elemento de `Afecta-UC:`, y uno o mas por `nuevo`) y sincronizacion del catalogo indice `docs/use-cases/README.md`, incluida la columna "Specs de Evolución". Durante la desinstalacion estandar (`uninstall.sh`), todos los casos de uso en `docs/use-cases/` se preservan intactos (`010-RF-12`).
- **Eventos Emitidos:** Ninguno.
- **Auditoria:** Registro de operaciones de lectura/escritura y posibles bloqueos de menor privilegio en `agent_actions` de `.seb/seb-audit.db`.
