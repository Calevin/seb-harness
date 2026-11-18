# Plan Técnico — Identificadores Únicos de Requisitos con Prefijo de Spec y Migración Homogénea (Spec 013)

- **Spec Origen:** `specs/013_identificadores_unicos_requisitos_spec.md`
- **Verificador Asociado:** `specs/013_identificadores_unicos_requisitos_verificador.md`
- **Estado:** `PLANIFICADO`
- **Dependencias verificadas:** `006_parser_tabla_verificacion` (`0528c64`), `011_gobernanza_afecta_uc` (`851cff2`), `014_afecta_uc_multiple` (`59856f8`)
- **Snapshot pre-migración de referencia:** commit `f3f5945` (contiene todos los artefactos históricos `001`-`012` y `014` en formato heredado).

## 1. Resumen Arquitectónico y Aislamiento de Lógica

La tarea tiene cuatro frentes, todos en el motor Go (`engine/cmd/seb/`) salvo el último:

1. **Parser de verificadores estricto** (`013-RF-07` a `013-RF-13`): `parseVerifierFile` pasa a exigir `Check ID` `<NNN>-CHK-*` coherente con la tarea y referencias de requisito prefijadas hacia specs existentes. Se conserva la firma `parseVerifierFile(path string)`: el ID de tarea se deriva del nombre del archivo (`^(\d{3})_`) y la existencia de specs se resuelve en el **directorio del propio verificador** (sin dependencia del cwd). `seb verify` y `seb plan-lock` ya comparten esta función, lo que garantiza `013-RF-13`.
2. **Rol `migrador` y registro de operaciones one-off** (`013-RF-14` a `013-RF-19`): nuevo rol canónico, política de escritura propia en `checkPermission`, subcomando `seb oneoff [<id>] [--dry-run]` y un registro `map[string]OneOff` indexado por ID de spec.
3. **One-off `013`** (`013-RF-20` a `013-RF-29`): migración separada en una **capa pura** (funciones `[]byte -> []byte` sin I/O, testeables con tablas) y una **capa transaccional** (precondición Git, escritura con respaldo y rollback, resellado de `seb.lock`, auditoría SQLite).
4. **Directivas, plantillas, reglas e instalador** (`013-RF-01` a `013-RF-06`, `013-RF-30` a `013-RF-33`): cambios declarativos en `payload/.agents/`, `AGENTS.md` (raíz y `payload/`, idénticos) e `install.sh`.

**Endurecimiento anti-escalada (derivado de `013-RF-17` y `013-RF-33`):** el hook `PreToolUse` deniega a cualquier agente los comandos que ejecuten una one-off con ID, activen el rol `migrador` (`seb role set migrador`) o lo inyecten vía `SEB_SKILL=migrador`. Así, "rol activado por el humano" queda garantizado de forma determinista: un agente puede *listar* one-offs (`seb oneoff` sin ID) pero nunca ejecutarlas.

**Secuencia de cierre (DoD de la spec):** el verificador de esta tarea se redacta en formato heredado para poder sellarse con el binario vigente. Hasta que el humano ejecute la one-off `013`, el motor nuevo rechazará todos los verificadores del repositorio (incluido el de esta tarea); por lo tanto la suite completa (`CHK-99`) y `TestParseVerifierNonFunctionalAndCompatibility` **solo pueden pasar después de la migración**. Durante la implementación el agente trabaja con filtros `-run` por tarea.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta | Acción | Responsabilidad Técnica | Requisitos Cubiertos |
| :--- | :--- | :--- | :--- |
| `engine/cmd/seb/verifier_ids.go` | `CREAR` | Regex y validadores de `Check ID` y referencias de requisito; derivación de ID de tarea; existencia de specs por directorio | `013-RF-07`, `013-RF-08`, `013-RF-10`, `013-RF-11`, `013-RF-12`, `013-CL-01`, `013-CL-02`, `013-CL-09` |
| `engine/cmd/seb/main.go` | `MODIFICAR` | `parseVerifierFile` usa los validadores; mensaje de migración ante `CHK-` heredado; rol `migrador` en `canonicalRoles`, mensajes de roles y `checkPermission`; despacho `case "oneoff"`; `printUsage` | `013-RF-07`..`013-RF-14`, `013-RF-17`, `013-RF-19`, `013-CL-03`, `013-CL-07`, `013-CL-08` |
| `engine/cmd/seb/hook_claude.go` | `MODIFICAR` | Regla de denegación anti-escalada a `migrador` en `PreToolUse` | `013-RF-17`, `013-RF-33` |
| `engine/cmd/seb/oneoff.go` | `CREAR` | Tipos `OneOff`, `OneOffContext`, `OneOffReport`; registro; `cmdOneOff` (listado, ejecución, dry-run, validación de rol e ID) | `013-RF-15`, `013-RF-16`, `013-RF-17`, `013-RF-18` |
| `engine/cmd/seb/oneoff_013.go` | `CREAR` | Capa pura de reescritura y capa transaccional de la migración | `013-RF-20`..`013-RF-29`, `013-RNF-01`, `013-CL-04`..`013-CL-06` |
| `engine/cmd/seb/prefixed_ids_test.go` | `CREAR` | `TestPrefixedVerifierParsing*`, `TestPrefixedVerifyAndPlanLockCLI*` | `013-RF-07`..`013-RF-13`, `013-CL-01`..`013-CL-03`, `013-CL-07`..`013-CL-09` |
| `engine/cmd/seb/migrador_role_test.go` | `CREAR` | `TestMigradorRoleAndGuard*` | `013-RF-14`, `013-RF-17`, `013-RF-19`, `013-RF-33` |
| `engine/cmd/seb/oneoff_test.go` | `CREAR` | `TestOneOffRegistryCommand*`, `TestOneOff013RewriteRules*`, `TestOneOff013TransactionalMigration*`, `TestOneOff013RealRepositoryEquivalence` | `013-RF-15`, `013-RF-16`, `013-RF-18`, `013-RF-20`..`013-RF-29`, `013-RNF-01`, `013-RNF-02`, `013-CL-04`..`013-CL-06` |
| `engine/cmd/seb/prefixed_directives_test.go` | `CREAR` | `TestPrefixedIdentifierDirectives*` (literales canónicos + distribución vía `install.sh --no-build`) | `013-RF-01`..`013-RF-06`, `013-RF-31`..`013-RF-33`, `013-RNF-04` |
| `engine/cmd/seb/installer_legacy_ids_test.go` | `CREAR` | `TestInstallLegacyIdentifiersNotice` | `013-RF-30` |
| `engine/cmd/seb/verifier_parser_test.go`, `role_session_test.go`, `dag_dependencies_test.go`, `task_abort_test.go` y demás tests con fixtures de verificador | `MODIFICAR` (acotado, `013-RNF-03`) | Renombrar archivos de fixture a `<NNN>_<x>_verificador.md`, crear las specs referenciadas en el directorio temporal y prefijar filas `CHK-`/`RF-` **sin** eliminar ni relajar aserciones | `013-RNF-03` |
| `install.sh` | `MODIFICAR` | Aviso de identificadores heredados con conteo y comandos | `013-RF-30` |
| `payload/.agents/skills/seb-especificador/SKILL.md` + `template/nnn_nombre_tarea_spec.md` | `MODIFICAR` | Identificadores `<NNN>-*` y sustitución del marcador | `013-RF-01`, `013-RF-02` |
| `payload/.agents/skills/seb-planificador/SKILL.md` + `template/nnn_tarea_verificador.md` + `template/nnn_tarea_plan.md` | `MODIFICAR` | Regla del parser y plantillas con `<NNN>-CHK-*` y referencias prefijadas | `013-RF-03`, `013-RF-04` |
| `payload/.agents/skills/seb-clarificador/SKILL.md` | `MODIFICAR` | Auditoría de prefijo y extensión del Hard Gate | `013-RF-05`, `013-RF-06` |
| `payload/.agents/skills/seb-documentador/SKILL.md` | `MODIFICAR` | Citas con identificador prefijado | `013-RF-31` |
| `payload/.agents/rules/00-seb-harness.md`, `AGENTS.md`, `payload/AGENTS.md` | `MODIFICAR` | Regla de referencias prefijadas (solo texto) y sección de operaciones one-off | `013-RF-32`, `013-RF-33` |

## 3. Contratos, Firmas y Modelo de Datos

### 3.1 Validación de verificadores (`verifier_ids.go`)

```go
var (
    // Primera celda (texto entre el primer y segundo '|') que contiene "-CHK-" o empieza con "CHK-".
    // La deteccion de candidata se hace sobre la primera celda, no sobre la linea completa.
    legacyCheckIDRegex   = regexp.MustCompile(`^CHK-`)
    prefixedCheckIDRegex = regexp.MustCompile(`^(\d{3})-CHK-[A-Za-z0-9_-]+$`)
    requirementRefRegex  = regexp.MustCompile(`^(\d{3})-(HU|RF|RNF|CL|CHK)-[A-Za-z0-9_-]+$`)
    verifierTaskIDRegex  = regexp.MustCompile(`^(\d{3})_.+_verificador\.md$`)
)

func isCandidateFirstCell(cell string) bool            // strings.Contains(cell, "-CHK-") || strings.HasPrefix(cell, "CHK-")
func taskIDFromVerifierPath(path string) (string, error) // error si el basename no cumple verifierTaskIDRegex
func existingSpecIDs(dir string) map[string]bool        // glob dir/*_*_spec.md -> IDs de 3 digitos
func validateCheckID(id, taskID string) error           // legacy -> errLegacyCheckID; prefijo != taskID o malformado -> error
func validateRequirementRef(ref string, specs map[string]bool) error // "GLOBAL" valido; resto requirementRefRegex + spec existente
```

- `parseVerifierFile(path)` conserva su firma. Flujo por fila: bloque cercado -> ignorar; primera celda candidata -> validación estructural vigente -> `validateCheckID` -> unicidad -> cada referencia normalizada -> `validateRequirementRef`.
- Ante `errLegacyCheckID`, el error devuelto incluye literalmente: `requiere ejecutar la operación one-off 013: .seb/bin/seb role set migrador && .seb/bin/seb oneoff 013` (`013-RF-09`). Al propagarse por `fatal` en `cmdVerify` / `cmdPlanLock` sale con código `1` por `stderr`.

### 3.2 Rol `migrador` (`main.go`)

- `canonicalRoles["migrador"] = true`; los mensajes "Roles canónicos válidos" agregan `, migrador` **al final**, preservando el texto previo.
- `checkPermission("migrador", "write", target)` devuelve `true` solo para: `specs/*_spec.md`, `specs/*_plan.md`, `specs/*_verificador.md`, `docs/use-cases/**`, `MEMORY.md`, `.seb/seb.lock`; `false` para todo lo demás (incluye `docs/constitution.md`, código y tests).
- Resolución de rol para CLI: `resolveCLIRole()` = `SEB_SKILL` (si no vacío) > `.seb/ACTIVE_ROLE` canónico > `implementador`.

### 3.3 Anti-escalada en `PreToolUse` (`hook_claude.go`)

```go
var migradorEscalationRegex = regexp.MustCompile(`(?i)(\bseb\s+oneoff\s+\d)|(\bseb\s+role\s+set\s+migrador\b)|(SEB_SKILL\s*=\s*["']?migrador\b)`)
```

Si `cmdStr` coincide, la decisión es `deny` con razón `[seb-harness ANTI-ESCALADA] Las operaciones one-off y el rol 'migrador' solo pueden ser activados por el humano desde su terminal.`, para cualquier rol inferido.

### 3.4 Registro one-off (`oneoff.go`)

```go
type OneOffContext struct {
    Root   string
    DB     *sql.DB
    DryRun bool
    Stdout io.Writer
    Stderr io.Writer
    FS     fileStore // abstraccion de lectura/escritura inyectable para simular fallos (013-RF-29)
}
type AmbiguousRef struct{ Path string; Line int; Text string }
type FileChange struct{ Path, OldHash, NewHash, TaskID string; Rewritten int }
type OneOffReport struct {
    Changes   []FileChange
    Rewritten int
    Ambiguous []AmbiguousRef
}
type OneOff struct {
    ID         string
    Title      string
    Applicable func(root string) (bool, string, error) // detalle: "N verificadores heredados"
    Run        func(ctx OneOffContext) (OneOffReport, error)
}
var oneOffRegistry = map[string]OneOff{"013": oneOff013}

// seb oneoff                -> listado (ID, titulo, APLICABLE/NO APLICABLE + detalle), exit 0, cualquier rol, sin escrituras
// seb oneoff <id> [--dry-run] -> rol != migrador: GUARDARRAIL + "seb role set migrador", exit 1
//                             -> id no registrado: error + IDs disponibles, exit 1
//                             -> ejecucion; ambiguas > 0: advertencia en stderr, exit 0
func cmdOneOff(db *sql.DB, args []string) int
```

### 3.5 One-off `013` (`oneoff_013.go`)

**Capa pura:**

```go
var (
    legacyProseIDRegex  = regexp.MustCompile(`(^|[^A-Za-z0-9_-])((?:HU|RF|RNF|CL)-\d{2,}|CHK-\d+)\b`)
    specMentionRegex    = regexp.MustCompile(`(^|[^A-Za-z0-9])(\d{3})(_[a-z0-9_]+)?([^A-Za-z0-9]|$)`)
    prefixedDeclRegex   = regexp.MustCompile(`(?m)^\s*-\s*\*\*\d{3}-(HU|RF|RNF|CL)-`)
)

// Menciones de specs existentes en una linea (IDs de 3 digitos que pertenecen a existingSpecs).
func mentionedSpecs(line string, existingSpecs map[string]bool) map[string]bool

// Artefactos de tarea. Ignora bloques cercados. Si kind == spec|plan y specDeclaresPrefixedIDs(spec de la tarea) -> sin cambios (013-CL-06).
// Filas candidatas de verificador: reescribe SOLO celdas 1 y 2 con prefijo ownerID (sin regla de ambiguedad);
// el resto de celdas se preserva byte a byte (013-RF-21).
// Lineas de prosa: si mentionedSpecs(line) contiene un ID distinto de ownerID -> cada ID heredado de la linea es ambiguo;
// si no -> prefijo ownerID.
func migrateTaskArtifact(content []byte, kind artifactKind, ownerID string, ownerSpecPrefixed bool, existingSpecs map[string]bool) (out []byte, rewritten int, ambiguous []AmbiguousRef)

// Artefactos documentales: un ID heredado se prefija solo si la linea menciona exactamente una spec existente; si no, ambiguo.
func migrateDocumentArtifact(content []byte, path string, existingSpecs map[string]bool) (out []byte, rewritten int, ambiguous []AmbiguousRef)

// Sustituye en las celdas de comando literales de hash sellado previo por su hash nuevo (013-RF-27).
func substituteEmbeddedHashes(verifierContent []byte, oldToNew map[string]string) ([]byte, int)
```

**Capa transaccional (`runOneOff013`):**

1. Precondición: `git -C <root> status --porcelain -- specs docs/use-cases MEMORY.md .seb/seb.lock`; error de Git (incluye "no es repositorio") o salida no vacía -> exit `1` sin escribir (`013-RF-28`).
2. Inventario: `specs/<NNN>_*_{spec,plan,verificador}.md` de cada tarea con spec en disco (las purgadas no existen, `013-CL-05`), `docs/use-cases/*.md`, `MEMORY.md`.
3. Cálculo en memoria de todos los contenidos nuevos (capa pura).
4. Hashes embebidos: construir `oldToNew` con los hashes sellados en `seb.lock` de los artefactos modificados; aplicar `substituteEmbeddedHashes` a los verificadores y recalcular hashes; iterar hasta punto fijo con cota `len(verificadores)+1`; sin convergencia -> error (rollback).
5. Si `DryRun`: imprimir reporte y salir sin escrituras ni auditoría (`013-RF-26`).
6. Escritura: respaldo en memoria de cada archivo, escritura vía `fileStore` (temporal + rename); luego `seb.lock` con `SpecHash`/`VerifierHash` actualizados **solo si estaban no vacíos**, preservando `Verified`, `CommitHash`, `Aborted` (`013-RF-23`). Cualquier error -> restaurar todos los respaldos y el `seb.lock` original, exit `1` (`013-RF-29`).
7. Auditoría: por archivo modificado `logInternalAction(db, taskID, "migrador", "oneoff-013:rewrite", path, "<old>-><new>", dur, 0)` y una entrada `oneoff-013:summary` (`013-RF-24`). Sin cambios -> ninguna entrada `:rewrite` (`013-RNF-01`).
8. Reporte en `stdout` (archivos, identificadores reescritos, ambiguas con `path:línea: texto`); si hay ambiguas, advertencia `[seb-harness ADVERTENCIA] N referencias ambiguas requieren revisión manual.` en `stderr` y exit `0` (`013-RF-25`).

### 3.6 Instalador (`install.sh`)

Tras el bloque de compilación (también con `--no-build` y `--self`): contar en `$TARGET/specs/*_verificador.md` los archivos con filas `^\s*\|\s*CHK-`. Si el conteo es mayor que cero, imprimir:

```text
[seb-harness] AVISO: N tareas contienen identificadores heredados (CHK-NN). seb verify las rechazará hasta migrar.
   Ejecutá: .seb/bin/seb role set migrador && .seb/bin/seb oneoff 013 && .seb/bin/seb role clear
```

### 3.7 Literales canónicos de directivas (verificados con `strings.Contains`)

| Archivo | Literal requerido | Requisito |
| :--- | :--- | :--- |
| `template/nnn_nombre_tarea_spec.md` | `**<NNN>-HU-01:**`, `**<NNN>-RF-01 (`, `**<NNN>-RNF-01:**`, `**<NNN>-CL-01 (` | `013-RF-01` |
| `template/nnn_nombre_tarea_spec.md` | **prohibido** `- **RF-01 (` y `- **HU-01:**` | `013-RF-01` |
| `seb-especificador/SKILL.md` | `` replace the `<NNN>` placeholder of every identifier with the three-digit ID returned by `seb spec-next` `` | `013-RF-02` |
| `template/nnn_tarea_verificador.md` | `` | <NNN>-CHK-01 | <NNN>-RF-01 | `` ; **prohibida** toda fila que empiece con `| CHK-` | `013-RF-03` |
| `seb-planificador/SKILL.md` | `` `| <NNN>-CHK-XX | <NNN>-RF-XX | Descripción | `` y `` Check IDs must start with the `<NNN>-CHK-` prefix of the source spec `` y `` reference each requirement with the prefixed identifier of the spec that owns it `` | `013-RF-03`, `013-RF-04` |
| `seb-clarificador/SKILL.md` | `` every declared `HU`, `RF`, `RNF` and `CL` identifier uses the `<NNN>-` prefix matching the spec `Tarea ID` `` y, en el Hard Gate, `` an identifier without the `<NNN>-` prefix, with a prefix different from its `Tarea ID`, or duplicated `` | `013-RF-05`, `013-RF-06` |
| `seb-documentador/SKILL.md` | `` usar el identificador prefijado `<NNN>-<TIPO>-<NN>` de la spec propietaria `` | `013-RF-31` |
| `rules/00-seb-harness.md`, `AGENTS.md`, `payload/AGENTS.md` | `` identificador prefijado `<NNN>-<TIPO>-<NN>` `` y `` solo en texto (comentarios y documentos), nunca como parte de identificadores de código `` | `013-RF-32` |
| `rules/00-seb-harness.md`, `AGENTS.md`, `payload/AGENTS.md` | `Operaciones One-Off`, `` rol `migrador` ``, `` identificadas por el ID de la spec que las define ``, `` única excepción autorizada a la inmutabilidad `` | `013-RF-33` |

Se **preservan** los literales exigidos por tests históricos (por ejemplo `` backed by at least one `RF-XX` `` en `seb-clarificador`); `RF-XX` es un marcador genérico, no un identificador de spec. `AGENTS.md` y `payload/AGENTS.md` deben quedar idénticos byte a byte.

## 4. Decisiones Técnicas Justificadas

- **D1. Conservar la firma `parseVerifierFile(path)` y derivar contexto del path.**
  - _Alternativa descartada:_ cambiar la firma a `(path, taskID, specs)`; obligaría a modificar llamadas en tests históricos más allá de fixtures, lo que `013-RNF-03` no autoriza.
- **D2. Existencia de specs resuelta en el directorio del verificador.**
  - _Alternativa descartada:_ resolver contra `specs/` del cwd; acopla el parser al directorio de ejecución y rompe tests en `t.TempDir()`.
- **D3. Capa pura de reescritura separada de la capa transaccional.**
  - _Alternativa descartada:_ reescritura con I/O directo; impide testear tablas de casos y simular fallos de escritura para el rollback.
- **D4. Sustitución genérica de hashes embebidos en lugar de un caso codificado para la Spec 014.**
  - Solo actúa cuando el comando contiene literalmente un hash sellado previo de un artefacto que la propia migración modificó (que es exactamente el caso de `013-RF-27`); en proyectos consumidores sin ese patrón no tiene efecto. Cada sustitución queda auditada.
  - _Alternativa descartada:_ condicionar por la clave `014_afecta_uc_multiple`; en un proyecto consumidor la `014` es otra spec distinta.
- **D5. Menciones de spec filtradas por specs existentes.** Evita que números como `100` o `150` en prosa marquen líneas como ambiguas cuando no existe esa spec; el error residual siempre es hacia "ambiguo" (seguro), nunca hacia una asignación incorrecta.
- **D6. Repositorio no Git => abortar.** Sin Git no puede garantizarse la precondición de `013-RF-28` ni la recuperación externa; `MEMORY.md` (gotcha) recoge que `seb.lock` debe estar versionado.
- **D7. Anti-escalada en `PreToolUse`.** Implementa de forma determinista "rol `migrador` activado por el humano" (`013-RF-33`); sin ella, un agente podría ejecutar `SEB_SKILL=migrador seb oneoff 013` y reescribir contratos sellados.
- **D8. `013-RNF-02` verificado por equivalencia contra snapshot Git (`f3f5945`) y no re-ejecutando las 13 suites dentro de `seb verify`.** Cada control tiene un timeout de 120 s y `seb verify` anidado limpia `ACTIVE_TASK`/`ACTIVE_ROLE` y reescribe `commit_hash` como efecto secundario. El test compara cantidad de controles y comandos byte a byte (salvo hashes sustituidos, que deben coincidir con los hashes nuevos de la Spec 011). La re-ejecución real de `seb verify` por tarea se realiza en el paso 5 del cierre.

## 5. Secuencia de Tareas Atómicas (TDD: Test en Rojo -> Código -> Test en Verde)

_Regla para el Agente Implementador: Ejecuta UNA tarea a la vez en orden de dependencia. Primero escribe el test y verifica que falle (Rojo), luego escribe el código mínimo hasta pasar (Verde). Usa siempre `direnv exec .`. No ejecutes la one-off `013` ni actives el rol `migrador` (el hook lo denegará)._

- [ ] **T1. Validadores de identificadores y parser estricto** (`013-RF-07`, `013-RF-08`, `013-RF-10`..`013-RF-12`, `013-CL-01`..`013-CL-03`, `013-CL-09`)
  - _Archivos:_ `verifier_ids.go`, `main.go` (`parseVerifierFile`), `prefixed_ids_test.go`.
  - _Hecho cuando:_ `CHK-01` devuelve `0`.
- [ ] **T2. Adaptación acotada de fixtures heredadas** (`013-RNF-03`)
  - _Archivos:_ tests con fixtures de verificador (`verifier_parser_test.go`, `role_session_test.go`, `dag_dependencies_test.go`, `task_abort_test.go`, etc.).
  - _Hecho cuando:_ `CHK-11` devuelve `0` y los tests adaptados pasan con `-run` por archivo (excepto `TestParseVerifierNonFunctionalAndCompatibility`, que depende de la migración).
- [ ] **T3. Integración CLI de `verify` / `plan-lock` y mensaje de migración** (`013-RF-09`, `013-RF-13`, `013-CL-07`, `013-CL-08`)
  - _Archivos:_ `main.go`, `prefixed_ids_test.go` (binario compilado en `t.TempDir()`).
  - _Hecho cuando:_ `CHK-02` devuelve `0`.
- [ ] **T4. Rol `migrador`, política de escritura y anti-escalada** (`013-RF-14`, `013-RF-17`, `013-RF-19`)
  - _Archivos:_ `main.go`, `hook_claude.go`, `migrador_role_test.go`.
  - _Hecho cuando:_ `CHK-03` devuelve `0`.
- [ ] **T5. Registro y subcomando `seb oneoff`** (`013-RF-15`, `013-RF-16`, `013-RF-18`)
  - _Archivos:_ `oneoff.go`, `main.go` (despacho, `printUsage`), `oneoff_test.go`.
  - _Hecho cuando:_ `CHK-04` devuelve `0`.
- [ ] **T6. Capa pura de reescritura de la one-off `013`** (`013-RF-20`..`013-RF-22`, `013-CL-05`, `013-CL-06`)
  - _Archivos:_ `oneoff_013.go`, `oneoff_test.go`.
  - _Hecho cuando:_ `CHK-05` devuelve `0`.
- [ ] **T7. Capa transaccional: precondición Git, resellado, hashes embebidos, auditoría, dry-run, rollback, idempotencia** (`013-RF-23`..`013-RF-29`, `013-RNF-01`, `013-CL-04`)
  - _Archivos:_ `oneoff_013.go`, `oneoff_test.go`.
  - _Hecho cuando:_ `CHK-06` devuelve `0`.
- [ ] **T8. Aviso del instalador** (`013-RF-30`)
  - _Archivos:_ `install.sh`, `installer_legacy_ids_test.go`.
  - _Hecho cuando:_ `CHK-08` devuelve `0`.
- [ ] **T9. Directivas, plantillas, reglas y `AGENTS.md`** (`013-RF-01`..`013-RF-06`, `013-RF-31`..`013-RF-33`, `013-RNF-04`)
  - _Archivos:_ skills y plantillas de `payload/.agents/`, `rules/00-seb-harness.md`, `AGENTS.md`, `payload/AGENTS.md`, `prefixed_directives_test.go`.
  - _Hecho cuando:_ `CHK-09` devuelve `0`.
- [ ] **T10. Test de equivalencia contra el snapshot y compilación** (`013-RNF-02`, `013-RNF-05`)
  - _Archivos:_ `oneoff_test.go` (`TestOneOff013RealRepositoryEquivalence`: extrae `git archive f3f5945 specs docs MEMORY.md .seb/seb.lock` a un temporal, ejecuta la migración allí y compara contra el árbol de trabajo).
  - _Hecho cuando:_ el test pasa ejecutándose sobre el snapshot (la comparación contra el árbol de trabajo pasará tras el paso 3 del cierre), `CHK-97` y `CHK-98` devuelven `0`, y el binario compila.

## 6. Secuencia de Cierre (Excepción One-Off Documentada en la Spec)

0. **Commit SDD previo (humano):** commitear spec, plan, verificador, `MEMORY.md` y `.seb/seb.lock` de esta tarea (commit solo de artefactos SDD, permitido por el hook). Es requisito de la precondición Git de la migración.
1. **Implementación (agente):** T1 a T10. No commitear código antes del paso 3 (el hook exigiría `seb verify 013`, que fallará hasta migrar). Si el implementador actualiza `MEMORY.md`, commitearlo antes del paso 3.
2. **Compilación (agente o humano):** compilar `.seb/bin/seb` con el comando oficial.
3. **Migración (humano, terminal propia):**
   ```bash
   .seb/bin/seb role set migrador
   .seb/bin/seb oneoff 013 --dry-run
   .seb/bin/seb oneoff 013
   .seb/bin/seb role clear
   ```
   Revisar manualmente las referencias ambiguas reportadas.
4. **Verificación (`seb-verificador`):** `.seb/bin/seb verify identificadores_unicos_requisitos` con el verificador ya migrado (`013-CHK-*`).
5. **Re-verificación histórica (`seb-verificador`):** `.seb/bin/seb verify <id>` para `001` a `012` y `014`, una por una.
