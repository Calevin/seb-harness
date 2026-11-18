# UC-artifacts_sequential_naming: Homogeneizacion de Nomenclatura Secuencial y Resolucion de Tareas

> **Trazabilidad SDD:** Derivado de la spec `specs/002_prefijo_secuencial_artefactos_spec.md` (Requisitos: `RF-01`, `RF-02`, `RF-03`, `RF-04`, `RF-05`, `RF-06`, `RF-07`, `RF-08`, `RF-09`, `RF-10`, `RNF-01`, `RNF-02`).

## 1. Ficha de Intencion
- **ID:** UC-artifacts_sequential_naming
- **Actor Principal:** `Developer`, `AIAgent`
- **Disparador:** Invocacion de `seb spec-next <task_name>`, `seb plan-lock <query>`, `seb verify <query>` o `seb guard <role> write <target_path>`.
- **Precondiciones:**
  - Directorio `specs/` existente en la raiz del espacio de trabajo.
- **Invariantes de Negocio:**
  - La triada documental de toda tarea respeta el patron `specs/<NNN>_<tarea>_spec.md`, `specs/<NNN>_<tarea>_plan.md` y `specs/<NNN>_<tarea>_verificador.md` (`buildArtifactPath`).
  - El archivo `.seb/seb.lock` y el puntero `.seb/ACTIVE_TASK` almacenan exclusivamente la clave canonica `<NNN>_<tarea>` (`TaskInfo.CanonicalName`).
  - El resolvedor (`resolveTask`) acepta indistintamente el numero (`1` o `001`), el nombre simple (`<tarea>`) o la clave canonica (`<NNN>_<tarea>`), exigiendo unicidad estricta.

## 2. Diagrama de Interaccion del Sistema (SSD)
```mermaid
sequenceDiagram
    autonumber
    actor User as "User / Agent"
    participant System as "SebEngineCLI"

    User->>System: "seb spec-next nombre_tarea"
    System-->>User: "ArtifactPath(specs/NNN_nombre_tarea_spec.md)"

    User->>System: "seb plan-lock NNN"
    System-->>User: "LockConfirmation(CanonicalKey NNN_nombre_tarea sealed in .seb/seb.lock)"

    User->>System: "seb verify nombre_tarea"
    System-->>User: "VerificationReport(Evaluated from specs/NNN_nombre_tarea_verificador.md)"
```

## 3. Arbol de Decisiones
```mermaid
flowchart TD
    Input["Task Query Argument (query)"] --> EmptyCheck{"Is query empty?"}
    EmptyCheck -- "Yes" --> ErrEmpty["Return Error: Empty task identifier"]
    EmptyCheck -- "No" --> NumericCheck{"Matches ^\\d+$ ?"}
    NumericCheck -- "Yes" --> PadZero["Format as 3-digit zero-padded ID (e.g. 001)"]
    NumericCheck -- "No" --> ScanCatalog["Scan SQLite tasks + physical specs/*_*_spec.md"]
    PadZero --> ScanCatalog

    ScanCatalog --> FilterMatches["Match against ID, Name, or CanonicalName"]
    FilterMatches --> MatchCount{"How many tasks matched?"}
    MatchCount -- "0" --> ErrNotFound["Abort (exit code 1): Task not found"]
    MatchCount -- "> 1" --> ErrAmbiguous["Abort (exit code 1): Ambiguous identifier, list candidates"]
    MatchCount -- "1" --> Resolved["Return TaskInfo (SpecPath, PlanPath, VerifierPath)"]
```

## 4. Caminos Alternativos y Catalogo de Errores
| Error Code | Causa Raiz | Respuesta / Efecto |
| :--- | :--- | :--- |
| `EMPTY_TASK_QUERY` | Argumento de tarea vacio o compuesto solo por espacios | Aborto con mensaje de error en `stderr` y `exit code 1` |
| `TASK_NOT_FOUND` | El identificador no coincide con ninguna tarea en SQLite ni en `specs/` | Aborto con `[seb-harness ERROR] Error resolviendo tarea: tarea no encontrada` y `exit code 1` (`RF-07`) |
| `AMBIGUOUS_TASK_IDENTIFIER` | Nombre simple repetido en multiples secuencias (ej. `001_foo` y `004_foo`) | Aborto con listado de claves canonicas candidatas y `exit code 1` (`RF-07`, `CL-02`) |
| `GUARD_WRITE_DENIED` | Rol `implementador` intenta escribir en `specs/<NNN>_<tarea>_spec.md` o `_verificador.md` | Rechazo inmediato con `GUARDARRAIL DE MENOR PRIVILEGIO` y `exit code 1` (`RF-10`) |

## 5. Postcondiciones y Efectos Secundarios
- **Estado Persistido:** Creacion o actualizacion de entradas con clave `<NNN>_<tarea>` en `.seb/seb.lock`, escritura de `<NNN>_<tarea>` en `.seb/ACTIVE_TASK` y sincronizacion del registro en la tabla `tasks` de `.seb/seb-audit.db`.
- **Eventos Emitidos:** Ninguno.
- **Auditoria:** Registro de accion `spec-next`, `plan-lock`, `verify` o `guard-blocked:write` en `agent_actions`.
