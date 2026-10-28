# Plan Técnico — Gestión de Grafo de Dependencias (DAG), Guardarraíl de Planificación y Protocolo de Suspensión en Clarificación (Spec 005)

- **Spec Origen:** `specs/005_gestion_grafo_dependencias_spec.md`
- **Verificador Asociado:** `specs/005_gestion_grafo_dependencias_verificador.md`
- **Estado:** `PLANIFICADO`

## 1. Resumen Arquitectónico y Aislamiento de Lógica

El modelo actual del arnés asume una secuencia lineal de tareas, pero la realidad del desarrollo exige modelar dependencias directas en forma de Grafo Acíclico Dirigido (DAG). Actualmente, `.seb/seb.lock` solo almacena los hashes de spec y verificador durante `plan-lock`, sin registrar si la tarea superó su verificación determinista ni con qué commit de Git se confirmó. Asimismo, el motor no valida dependencias antes de sellar un plan técnico, y `seb-clarificador` carece de un protocolo formal para suspender una spec cuando se descubren brechas de capacidad en el sistema.

Para resolverlo respetando la ley suprema de Go puro sin dependencias dinámicas (`CGO_ENABLED=0`) y asegurando autonomía offline en clones limpios de Git (`docs/constitution.md` Secciones B y C), se implementa la siguiente arquitectura técnica:

1. **Extensión del Candado Criptográfico (`.seb/seb.lock`):**
   Se amplía la estructura `TaskLock` incorporando los campos `verified: bool` y `commit_hash: string`. Cuando `.seb/bin/seb verify <tarea>` concluye con el 100% de los controles en `[PASA]`, actualiza la entrada en `seb.lock` fijando `verified: true` y estampando el hash actual de Git (`git rev-parse --short HEAD`). Esto convierte a `seb.lock` en la fuente de verdad inmutable y versionada en Git para la resolución offline de prerrequisitos.

2. **Parser y Normalizador de Dependencias en la Spec:**
   Se incorpora la función `parseSpecDependencies(specPath string) ([]string, error)` que inspecciona las primeras líneas del archivo buscando la directiva `Dependencias:` o `Depends-On:`. La función extrae los tokens separados por comas, descartando palabras clave como `Ninguna` o vacías, y resuelve cada token contra el catálogo canónico mediante `resolveTask`.

3. **Guardarraíl Computacional en `seb plan-lock`:**
   Antes de calcular los hashes y sellar el plan:
   - Se parsean las dependencias directas declaradas en la spec.
   - Para cada dependencia, se comprueba su existencia en `.seb/seb.lock` y se valida estrictamente que `verified == true` y `commit_hash != ""`.
   - Se valida la ausencia de referencias circulares directas (si la tarea dependiente A declara a B, y la spec de B declara a A).
   - Ante cualquier incumplimiento, `plan-lock` aborta con `exit code 1` emitiendo un diagnóstico claro con prefijo `[BLOQUEO SDD]` y registrando la acción en SQLite si está disponible.

4. **Observabilidad del DAG en `seb status`:**
   Se extiende `cmdStatus` para añadir información de dependencias directas (`Depends-On`), marcando visualmente con alertas aquellas tareas que declaran prerrequisitos aún no verificados.

5. **Formalización Operativa de Habilidades y Reglas (`payload/.agents/`):**
   - En `payload/.agents/skills/seb-clarificador/SKILL.md`: se documenta el protocolo de detección de brechas (Capability Gap), la formulación de la Bifurcación de Alcance (Scope Fork: reducción de alcance vs. desacople en spec base), la transición a `BLOQUEADA_POR_DEPENDENCIA`, la redacción del Contrato de Desbloqueo y su conservación histórica renombrada a `Contrato de Desbloqueo (Satisfecho por Spec XXX)`, junto con el registro obligatorio en `MEMORY.md` (`seb memory-add`) si se opta por datos estáticos/mocks.
   - En `payload/.agents/skills/seb-planificador/SKILL.md` y `payload/.agents/rules/00-seb-harness.md`: se establecen las restricciones operativas del DAG y la inmutabilidad de dependencias.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta | Acción (`CREAR` / `MODIFICAR`) | Responsabilidad Técnica | Requisitos Cubiertos (`RF-XX`) |
| :--- | :--- | :--- | :--- |
| `engine/cmd/seb/main.go` | `MODIFICAR` | Extender struct `TaskLock`, parsear metadata de dependencias en specs, guardarraíl de dependencias en `cmdPlanLock`, estampado de `verified` y `commit_hash` en `cmdVerify`, y columnas de DAG en `cmdStatus`. | `RF-01` a `RF-07`, `RF-15`, `RNF-01` a `RNF-03`, `CL-01` a `CL-04` |
| `engine/cmd/seb/dag_dependencies_test.go` | `CREAR` | Suite determinista de pruebas unitarias y de integración para parser de dependencias, guardarraíl en `plan-lock`, estampado en `verify`, detección de ciclos circulares, `status` y compatibilidad retroactiva. | `RF-01` a `RF-07`, `RF-15`, `RNF-01` a `RNF-03`, `CL-01` a `CL-04` |
| `payload/.agents/skills/seb-clarificador/SKILL.md` | `MODIFICAR` | Incorporar fases formales para Detección de Brechas de Capacidad, Scope Fork, estado `BLOQUEADA_POR_DEPENDENCIA`, `Contrato de Desbloqueo (Satisfecho por Spec XXX)` y registro de deuda técnica en `MEMORY.md`. | `RF-08` a `RF-14`, `CL-05` |
| `payload/.agents/skills/seb-planificador/SKILL.md` | `MODIFICAR` | Añadir verificación previa de prerrequisitos verificados en `seb.lock` como puerta de entrada a la planificación. | `RF-04` a `RF-07` |
| `payload/.agents/rules/00-seb-harness.md` | `MODIFICAR` | Incorporar la regla de invariantes de precedencia del DAG y prohibición de planificar sobre tareas base insatisfechas. | `RF-04` a `RF-07` |

## 3. Contratos, Firmas y Modelo de Datos

### Modelo de Persistencia en `seb.lock` (Go)

```go
// TaskLock representa el sello criptografico y estado de verificacion en .seb/seb.lock
type TaskLock struct {
	SpecHash     string `json:"spec_hash"`
	VerifierHash string `json:"verifier_hash"`
	Verified     bool   `json:"verified,omitempty"`
	CommitHash   string `json:"commit_hash,omitempty"`
}

type SebLock struct {
	Tasks map[string]TaskLock `json:"tasks"`
}
```

### Firmas de Funciones Principales en Go

```go
// parseSpecDependencies extrae la lista de tareas prerrequisito desde la cabecera de la spec
func parseSpecDependencies(specPath string) ([]string, error)

// validateDirectDependencies comprueba que cada tarea prerrequisito este verificada en seb.lock
func validateDirectDependencies(db *sql.DB, targetTask TaskInfo, deps []string, lock SebLock) error

// detectDirectCircularDependency valida que la tarea previa no declare una dependencia reciproca hacia la tarea actual
func detectDirectCircularDependency(targetCanonical string, depTask TaskInfo) error

// updateTaskLockVerification estampa verified: true y commit_hash en seb.lock al superar verify
func updateTaskLockVerification(canonicalName, commitHash string) error
```

## 4. Decisiones Técnicas Justificadas

- **Decisión 1: `.seb/seb.lock` como única fuente de verdad multi-máquina para la resolución de dependencias.**
  - *Justificación:* Conforme a la Sección B de `docs/constitution.md`, SQLite (`seb-audit.db`) es estrictamente local y no se versiona en Git. Al estampar `verified: true` y `commit_hash` dentro de `.seb/seb.lock`, cualquier clon limpio del repositorio en otra máquina o pipeline de CI puede validar el grafo de dependencias de forma autónoma y determinista sin necesidad de reconstruir la base de datos de telemetría.
  - *Alternativa descartada:* Consultar exclusivamente la tabla `verifications` de SQLite (habría fallado en entornos multi-máquina).
- **Decisión 2: Validación directa de primer nivel en lugar de resolución transitiva profunda.**
  - *Justificación:* En un flujo SDD riguroso, cada tarea valida sus dependencias inmediatas al momento de su propio sellado. Si la tarea B ya fue verificada con éxito, la validez de sus dependencias aguas abajo (tarea A) ya fue certificada deterministamente. Validar directamente los prerrequisitos inmediatos mantiene la complejidad computacional en O(N) con ejecución en `<5ms` y evita sobrecarga innecesaria en el CLI.
  - *Alternativa descartada:* Recorrido recursivo profundo del grafo en cada invocación de `plan-lock`.
- **Decisión 3: Verificación determinista programática de las instrucciones de habilidades (`SKILL.md`).**
  - *Justificación:* Para que las directivas de `seb-clarificador` y `seb-planificador` no queden como recomendaciones abstractas, la suite `dag_dependencies_test.go` inspeccionará los archivos `.agents/skills/*/SKILL.md` verificando la presencia obligatoria de los términos y protocolos requeridos (`BLOQUEADA_POR_DEPENDENCIA`, `Bifurcación de Alcance`, `Contrato de Desbloqueo`).

## 5. Secuencia de Tareas Atómicas (TDD: Test en Rojo -> Código -> Test en Verde)

- [x] **T1. Extensión de `TaskLock` y actualización de `seb verify` para estampar `verified: true` y `commit_hash` en `.seb/seb.lock`** (`RF-03`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/dag_dependencies_test.go`
  - _Detalle:_ Modificar struct `TaskLock`, implementar `updateTaskLockVerification` y llamarla en `cmdVerify` tras superar todos los checks con exit code 0.
- [x] **T2. Parser determinista de metadata de dependencias en especificaciones (`parseSpecDependencies`)** (`RF-01`, `RF-02`, `CL-01`, `RNF-02`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/dag_dependencies_test.go`
  - _Detalle:_ Parsear directivas `Dependencias:` y `Depends-On:`, ignorar `Ninguna`, normalizar identificadores y tolerar specs legacy sin el campo.
- [x] **T3. Guardarraíl de dependencias directas y prevención de ciclos en `cmdPlanLock`** (`RF-04`, `RF-05`, `RF-06`, `RF-07`, `CL-02`, `CL-03`, `CL-04`, `RNF-01`, `RNF-03`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/dag_dependencies_test.go`
  - _Detalle:_ Validar prerrequisitos directos en `seb.lock` (`verified == true`, `commit_hash != ""`), validar dependencias huérfanas o no verificadas, comprobar ciclos mutuos directos y abortar con código 1 ante infracciones.
- [x] **T4. Observabilidad de dependencias y alertas en `seb status`** (`RF-15`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/dag_dependencies_test.go`
  - _Detalle:_ Actualizar tabla de `cmdStatus` para exponer la columna de dependencias directas y resaltar visualmente advertencias de bloqueo.
- [x] **T5. Actualización de directivas en `seb-clarificador`, `seb-planificador` y `00-seb-harness.md`** (`RF-08` a `RF-14`, `RF-16`, `CL-05`)
  - _Archivos:_ `payload/.agents/skills/seb-clarificador/SKILL.md`, `payload/.agents/skills/seb-planificador/SKILL.md`, `payload/.agents/rules/00-seb-harness.md`, `engine/cmd/seb/dag_dependencies_test.go`
  - _Detalle:_ Incorporar protocolo formal de Scope Fork, estado `BLOQUEADA_POR_DEPENDENCIA`, conservación histórica del contrato de desbloqueo, registro de deudas técnicas en `MEMORY.md` y verificar integridad estructural.
- [x] **T6. Verificación determinista global y sellado de la tarea** (`RF-Stack`, `RF-GLOBAL`)
  - _Archivos:_ Todo el repositorio.
  - _Detalle:_ Ejecutar `go vet ./...`, compilar binario limpio con `CGO_ENABLED=0` y validar que todos los controles de `005_gestion_grafo_dependencias_verificador.md` pasen con éxito.
