# Plan Técnico — Aborto Formal y Purga de Tareas (Spec 009)

- **Spec Origen:** `specs/009_aborto_tarea_spec.md`
- **Verificador Asociado:** `specs/009_aborto_tarea_verificador.md`
- **Estado:** `PLANIFICADO`

## 1. Resumen Arquitectónico y Aislamiento de Lógica

El motor Go (`engine/cmd/seb/main.go`) administra el ciclo de vida de las tareas a través de un esquema híbrido y determinista:
1. **Auditoría Local (SQLite):** Registra el estado de la tarea en la tabla `tasks` y la telemetría en `agent_actions` y `verifications`.
2. **Sincronización Git Multi-Máquina:** Versiona candados criptográficos y metadatos críticos en `.seb/seb.lock` y en el directorio `specs/`.

La implementación del comando `abort` (con soporte para `seb abort` y `seb task abort`) se estructura desacoplando las operaciones de resolución, validación de guardarraíles (dependencias en el DAG y tareas ya verificadas) y ejecución de las dos modalidades:
- **Modo Formal (`status = 'ABORTED'`):** Modifica idempotentemente el registro en SQLite, actualiza la cabecera del archivo `specs/<NNN>_<tarea>_spec.md`, registra `"verified": false, "aborted": true` en `.seb/seb.lock` y desvincula `.seb/ACTIVE_TASK` si estaba activa.
- **Modo Purga (`--purge`):** Transacción atómica en SQLite para eliminar filas en `verifications`, `agent_actions` y `tasks`; eliminación de archivos físicos (`_spec.md`, `_plan.md`, `_verificador.md`) usando llamadas nativas de Go (`os.Remove`); remoción de claves en `.seb/seb.lock`; y limpieza de `.seb/ACTIVE_TASK`.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta | Acción (`CREAR` / `MODIFICAR`) | Responsabilidad Técnica | Requisitos Cubiertos (`RF-XX`) |
| :--- | :--- | :--- | :--- |
| `engine/cmd/seb/main.go` | `MODIFICAR` | Agregar subcomandos `abort` y `task abort`, soporte de flags `--purge` y `--force` / `-f`, función `cmdAbort`, campos en `TaskLock`, actualización en `cmdStatus` y rechazo en validador de DAG para tareas abortadas | `RF-01` a `RF-14`, `RNF-01` a `RNF-04` |
| `engine/cmd/seb/task_abort_test.go` | `CREAR` | Batería de pruebas unitarias y de integración en Go para validar aborto formal, purga, guardarraíles, DAG e idempotencia | `RF-01` a `RF-14`, `CL-01` a `CL-04` |

## 3. Contratos, Firmas y Modelo de Datos

### Modelo de Datos en `.seb/seb.lock`
```go
type TaskLock struct {
    SpecHash     string `json:"spec_hash"`
    VerifierHash string `json:"verifier_hash"`
    Verified     bool   `json:"verified,omitempty"`
    CommitHash   string `json:"commit_hash,omitempty"`
    Aborted      bool   `json:"aborted,omitempty"` // Marca formal de tarea abortada
}
```

### Firmas de Funciones
```go
// cmdAbort maneja los subcomandos 'seb abort' y 'seb task abort'
func cmdAbort(db *sql.DB, args []string)

// executeTaskAbortFormal aplica la cancelacion formal reteniendo auditoria
func executeTaskAbortFormal(db *sql.DB, task TaskInfo, lock SebLock) error

// executeTaskPurge elimina atomicamente todos los registros, archivos y candados
func executeTaskPurge(db *sql.DB, task TaskInfo, lock SebLock) error

// checkDependentTasks escanea specs/*_*_spec.md para detectar si otra tarea depende de la tarea objetivo
func checkDependentTasks(targetTask TaskInfo) ([]string, error)
```

## 4. Decisiones Técnicas Justificadas

- **Decisión 1: Modificación de cabecera en spec durante aborto formal:**
  - *Justificación:* Mantiene la visibilidad inmediata para humanos y herramientas de inspección estática al abrir el archivo en `specs/`, dejando claro que la especificación fue cancelada.
  - *Alternativa descartada:* Mover el archivo a un subdirectorio `specs/archive/` o cambiarle la extensión. Se descartó porque rompería herramientas externas de navegación y enlaces relativos documentados en `MEMORY.md`.
- **Decisión 2: Flags soportados y compatibilidad de sintaxis:**
  - *Justificación:* Soportar `seb abort <tarea>` y `seb task abort <tarea>`, así como `--force` y `-f`, permite máxima ergonomía en CLI y compatibilidad con scripts existentes.
- **Decisión 3: Rechazo de tareas abortadas en el evaluador DAG:**
  - *Justificación:* Si una tarea futura lista como prerrequisito una tarea abortada, `plan-lock` debe bloquearla de forma inmediata (`[BLOQUEO SDD]: La tarea prerrequisito '<dep>' fue abortada`), garantizando consistencia matemática en el DAG.

## 5. Secuencia de Tareas Atómicas (TDD: Test en Rojo -> Código -> Test en Verde)

- [ ] **T1. Estructura de datos, despacho en CLI y pruebas de aborto formal (`TestTaskAbortFormal`)** (`RF-01`, `RF-02`, `RF-03`, `RF-04`, `RF-05`, `RNF-03`, `CL-01`, `CL-03`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/task_abort_test.go`
  - _Hecho cuando:_ El comando `CHK-01` de `specs/009_aborto_tarea_verificador.md` devuelve exit code `0`.
- [ ] **T2. Implementación de purga atómica y liberación de prefijo secuencial (`TestTaskAbortPurge`)** (`RF-06`, `RF-07`, `RF-08`, `RF-09`, `RNF-02`, `RNF-04`, `CL-02`, `CL-04`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/task_abort_test.go`
  - _Hecho cuando:_ El comando `CHK-02` de `specs/009_aborto_tarea_verificador.md` devuelve exit code `0`.
- [ ] **T3. Guardarraíles de protección (`--force`), validaciones de DAG, manejo de errores y alias CLI (`TestTaskAbortGuardsAndErrors`)** (`RF-10`, `RF-11`, `RF-12`, `RF-13`, `RF-14`, `RNF-01`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/task_abort_test.go`
  - _Hecho cuando:_ El comando `CHK-03` de `specs/009_aborto_tarea_verificador.md` devuelve exit code `0`.
- [ ] **T4. Verificación global del arnés (linter oficial y suite completa sin regresiones)** (`RNF-01`, `RNF-04`, `GLOBAL`)
  - _Archivos:_ `engine/...`
  - _Hecho cuando:_ Los comandos `CHK-98` y `CHK-99` de `specs/009_aborto_tarea_verificador.md` devuelven exit code `0`.
