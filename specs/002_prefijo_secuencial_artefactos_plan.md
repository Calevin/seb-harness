# Plan Técnico — Prefijo Secuencial en Artefactos SDD (Spec 002)

- **Spec Origen:** `specs/002_prefijo_secuencial_artefactos_spec.md`
- **Verificador Asociado:** `specs/002_prefijo_secuencial_artefactos_verificador.md`
- **Estado:** `PLANIFICADO`

## 1. Resumen Arquitectónico y Aislamiento de Lógica

Este refactor homogeneiza la nomenclatura de la tríada de artefactos SDD (`spec`, `plan`, `verificador`) para que los tres compartan de forma obligatoria el prefijo numérico secuencial `<NNN>_`.

Para evitar fricción operativa en el uso diario del CLI (`seb plan-lock`, `seb verify`, `seb status`), se aísla la resolución de tareas en un componente puro en Go (`resolveTask`), que resuelve deterministamente una tarea a partir de tres formas de entrada:
1. Prefijo numérico (`"002"` o normalizado desde `"2"`).
2. Nombre simple de la tarea (`"prefijo_secuencial_artefactos"`).
3. Clave canónica completa (`"002_prefijo_secuencial_artefactos"`).

Si el argumento es ambiguo (por ejemplo, dos tareas con el mismo nombre en distinto ID) o inexistente, el CLI aborta inmediatamente con diagnóstico explícito y código de salida 1.

Se actualiza además la persistencia en Git (`.seb/seb.lock` y `.seb/ACTIVE_TASK`) para indexar con la clave canónica `<NNN>_<tarea>`, y se renuevan los guardarraíles de menor privilegio (`seb guard`) y las plantillas maestras en `payload/`.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta | Acción | Responsabilidad Técnica | Requisitos Cubiertos |
| :--- | :--- | :--- | :--- |
| `engine/cmd/seb/main.go` | `MODIFICAR` | Integrar `resolveTask`, claves canónicas en `seb.lock`/`ACTIVE_TASK` y rutas `<NNN>_` en `plan-lock` y `verify`. | `RF-01` a `RF-08` |
| `engine/cmd/seb/resolver_test.go` | `CREAR` | Tests unitarios para resolución flexible, ambigüedad y formato de rutas. | `RF-01`, `RF-06`, `RF-07` |
| `engine/cmd/seb/guard_test.go` | `CREAR` | Tests unitarios para guardarraíles con rutas prefijadas. | `RF-09`, `RF-10` |
| `specs/001_registro_telemetria_acciones_plan.md` | `CREAR (RENOMBRAR)` | Migración de plan preexistente de tarea 001 con prefijo secuencial. | `RNF-01` |
| `specs/001_registro_telemetria_acciones_verificador.md` | `CREAR (RENOMBRAR)` | Migración de verificador preexistente de tarea 001 con prefijo secuencial. | `RNF-01` |
| `.seb/seb.lock` | `MODIFICAR` | Migrar clave de `registro_telemetria_acciones` a `001_registro_telemetria_acciones`. | `RF-08`, `RNF-01` |
| `payload/.agents/skills/seb-planificador/SKILL.md` | `MODIFICAR` | Actualizar rutas esperadas a `specs/<NNN>_<tarea>_plan.md` y `specs/<NNN>_<tarea>_verificador.md`. | `RNF-02` |
| `payload/.agents/skills/seb-verificador/SKILL.md` | `MODIFICAR` | Actualizar referencias a `specs/<NNN>_<tarea>_verificador.md`. | `RNF-02` |
| `payload/.agents/skills/seb-planificador/template/` | `MODIFICAR` | Renombrar plantillas a `nnn_tarea_plan.md` y `nnn_tarea_verificador.md`. | `RNF-02` |
| `docs/constitution.md` | `MODIFICAR` | Actualizar menciones de convenciones de archivos de specs/plan/verificador en secciones B, C, D y F. | `RNF-02` |
| `payload/AGENTS.md` y `.agents/rules/00-seb-harness.md` | `MODIFICAR` | Sincronizar reglas operativas con la convención de tres artefactos prefijados. | `RNF-02` |

## 3. Contratos, Firmas y Modelo de Datos

### Estructura de Tarea Resuelta (`TaskInfo`):
```go
type TaskInfo struct {
    ID            string // Ej: "002"
    Name          string // Ej: "prefijo_secuencial_artefactos"
    CanonicalName string // Ej: "002_prefijo_secuencial_artefactos"
    SpecPath      string // Ej: "specs/002_prefijo_secuencial_artefactos_spec.md"
    PlanPath      string // Ej: "specs/002_prefijo_secuencial_artefactos_plan.md"
    VerifierPath  string // Ej: "specs/002_prefijo_secuencial_artefactos_verificador.md"
}
```

### Firma de Resolución Pura:
```go
func resolveTask(db *sql.DB, query string) (TaskInfo, error)
```
- Algoritmo:
  1. Si `query` es numérico (`\d+`), normalizar con relleno `fmt.Sprintf("%03d", n)`.
  2. Consultar SQLite y escanear `specs/*_*_spec.md` en disco para recolectar candidatos.
  3. Filtrar candidatos por: `ID == q`, `Name == q` o `CanonicalName == q`.
  4. Si len(candidatos) == 0 -> Retornar error `"tarea no encontrada: %s"`.
  5. Si len(candidatos) > 1 -> Retornar error `"identificador ambiguo: %s coincide con múltiples tareas"`.
  6. Construir y validar rutas canónicas de archivos en disco.

## 4. Decisiones Técnicas Justificadas

- **Decisión 1: Resolución con normalización numérica a 3 dígitos.**
  - _Justificación:_ Permite invocar `seb verify 1` o `seb verify 001` con idéntico resultado, tolerando omisión de ceros por el usuario en terminal.
- **Decisión 2: Clave canónica `<NNN>_<tarea>` en `seb.lock`.**
  - _Justificación:_ Elimina colisiones en el mapa JSON si existen tareas homónimas en distintos hitos del proyecto.
- **Decisión 3: Migración quirúrgica atómica sin subcomandos legacy.**
  - _Justificación:_ Respeta la constitución manteniendo el motor `seb` mínimo y sin dependencias superfluas para migraciones únicas.

## 5. Secuencia de Tareas Atómicas (TDD)

- [ ] **T1. Tests unitarios de resolución y rutas de artefactos (`resolver_test.go`)** (`RF-01`, `RF-06`, `RF-07`)
  - _Archivos:_ `engine/cmd/seb/resolver_test.go`
  - _Hecho cuando:_ Los tests fallen inicialmente (Rojo) porque `resolveTask` aún no está implementada.
- [ ] **T2. Implementación de `resolveTask` y adaptación de rutas en `engine/cmd/seb/main.go`** (`RF-01`, `RF-02`, `RF-03`, `RF-06`, `RF-07`)
  - _Archivos:_ `engine/cmd/seb/main.go`
  - _Hecho cuando:_ `CHK-01` y `CHK-02` devuelvan exit code `0`.
- [ ] **T3. Integración de clave canónica en `plan-lock`, `verify`, `seb.lock` y `ACTIVE_TASK`** (`RF-04`, `RF-05`, `RF-08`)
  - _Archivos:_ `engine/cmd/seb/main.go`
  - _Hecho cuando:_ `CHK-03` devuelva exit code `0`.
- [ ] **T4. Guardarraíles de menor privilegio para rutas prefijadas (`guard_test.go` y `checkPermission`)** (`RF-09`, `RF-10`)
  - _Archivos:_ `engine/cmd/seb/guard_test.go`, `engine/cmd/seb/main.go`
  - _Hecho cuando:_ `CHK-04` devuelva exit code `0`.
- [ ] **T5. Migración quirúrgica de la tarea 001 en disco y en `seb.lock`** (`RNF-01`)
  - _Archivos:_ `specs/001_registro_telemetria_acciones_plan.md`, `specs/001_registro_telemetria_acciones_verificador.md`, `.seb/seb.lock`
  - _Hecho cuando:_ `CHK-05` devuelva exit code `0`.
- [ ] **T6. Actualización y sincronización de documentación, templates y skills en `payload/` y raíz** (`RNF-02`)
  - _Archivos:_ `docs/constitution.md`, `AGENTS.md`, `payload/`, `.agents/`
  - _Hecho cuando:_ `CHK-06` devuelva exit code `0`.
- [ ] **T7. Compilación final y verificación completa del arnés** (`GLOBAL`)
  - _Archivos:_ `.seb/bin/seb`
  - _Hecho cuando:_ `CHK-98` y `CHK-99` devuelvan exit code `0`.
