# Plan Técnico — estado_sesion_determinista (Spec 008)

- **Spec Origen:** `specs/008_estado_sesion_determinista_spec.md`
- **Verificador Asociado:** `specs/008_estado_sesion_determinista_verificador.md`
- **Estado:** `PLANIFICADO`

## 1. Resumen Arquitectónico y Aislamiento de Lógica

El diseño resuelve la falsa inferencia de roles desacoplando la identidad de gobernanza de la lectura pasiva de archivos e introduciendo un estado explícito de sesión en el espacio de trabajo local:

1. **Gestión de Rol Determinista (`seb role <set|get|clear>`):**
   Se introduce un nuevo subcomando en el CLI `seb` que administra el archivo plano local `.seb/ACTIVE_ROLE`. Este archivo define unívocamente la fase de gobernanza activa sin sobrecargar SQLite ni romper la sincronización multi-máquina.
2. **Jerarquía Estricta de 4 Niveles en `inferActiveSkill`:**
   La resolución del rol activo abandona la heurística plana y adopta un orden de precedencia inmutable:
   - **Nivel 1:** Variable de entorno `SEB_SKILL` (si no está vacía).
   - **Nivel 2:** Rol persistido en `.seb/ACTIVE_ROLE` (validado contra el conjunto cerrado de roles canónicos).
   - **Nivel 3:** Inferencia en `transcript.jsonl` filtrada estrictamente para habilidades oficiales del arnés (`seb-*`).
   - **Nivel 4:** Fallback seguro al rol `implementador`.
3. **Aislamiento de Habilidades de Dominio en `checkPermission`:**
   Se modifica la cláusula por defecto de `checkPermission`. Cualquier habilidad de dominio o soporte no reconocida (como `java-springboot`) hereda los permisos estándar de `implementador` en lugar de sufrir un bloqueo ciego. Esto permite la creación y edición de código fuente del proyecto mientras preserva el bloqueo absoluto sobre contratos protegidos (`docs/constitution.md`, `specs/*_spec.md`, `specs/*_verificador.md`).
4. **Ciclo de Vida Integrado con `plan-lock`, `verify` e `init`:**
   El sellado de plan con `plan-lock` fuerza atómicamente la transición a `implementador`. La finalización exitosa de `verify` e `init` elimina `.seb/ACTIVE_ROLE` para dejar el repositorio en estado limpio.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta                        | Acción      | Responsabilidad Técnica                                                                                                                                        | Requisitos Cubiertos                 |
| :------------------------------------ | :---------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------- | :----------------------------------- |
| `engine/cmd/seb/main.go`              | `MODIFICAR` | Subcomando `role`, helpers de rol, precedencia en `inferActiveSkill`, herencia de `implementador` en `checkPermission` y hooks en `plan-lock`/`verify`/`init`. | `RF-01` a `RF-09`, `CL-01` a `CL-04` |
| `engine/cmd/seb/role_session_test.go` | `CREAR`     | Suite de pruebas unitarias e integración para CLI de rol, jerarquía de precedencia, aislamiento de dominio y ciclo de vida.                                    | `RF-01` a `RF-09`, `CL-01` a `CL-04` |
| `.gitignore`                          | `MODIFICAR` | Registro de `.seb/ACTIVE_ROLE` para evitar versionado accidental en Git.                                                                                       | `CL-04`, `RNF-02`                    |

## 3. Contratos, Firmas y Modelo de Datos

### Conjunto de Roles Canónicos

```go
var canonicalRoles = map[string]bool{
	"seb-iniciador":     true,
	"seb-especificador": true,
	"seb-clarificador":  true,
	"seb-planificador":  true,
	"seb-verificador":   true,
	"implementador":     true,
}

func isCanonicalRole(role string) bool {
	return canonicalRoles[strings.ToLower(strings.TrimSpace(role))]
}
```

### Firmas de Funciones Principales

```go
// cmdRole despacha los subcomandos set, get y clear
func cmdRole(db *sql.DB, args []string)

// getActiveRolePath devuelve la ruta al archivo .seb/ACTIVE_ROLE
func getActiveRolePath() string

// setActiveRole valida y persiste el rol canónico normalizado
func setActiveRole(role string) error

// getActiveRole lee el rol persistido, aplicando fallback seguro si está vacío o corrupto
func getActiveRole() string

// clearActiveRole elimina .seb/ACTIVE_ROLE si existe
func clearActiveRole() error

// inferActiveSkill resuelve el rol según la jerarquía estricta de 4 niveles
func inferActiveSkill(envSkill, transcriptPath, activeTaskPath, activeRolePath string) string
```

### Reglas de Decisión en `checkPermission`

```go
// Si el rol no pertenece a los casos explícitos seb-*, aplica política de implementador:
default:
	if target == "docs/constitution.md" ||
		(strings.HasPrefix(target, "specs/") && (strings.HasSuffix(target, "_spec.md") || strings.HasSuffix(target, "_verificador.md"))) {
		return false
	}
	return true
```

## 4. Decisiones Técnicas Justificadas

- **Decisión 1: Archivo plano `.seb/ACTIVE_ROLE` en lugar de columna en SQLite:**
  - _Justificación:_ Alineado con la Constitución (Sección B y C) y `MEMORY.md` (Callejón sin Salida 2026-10-22). El estado que gobierna los interceptores de Git y los guardarraíles debe ser de lectura instantánea (<1ms) y no depender de conexiones a bases de datos ni CGO.
- **Decisión 2: Degradación de skills desconocidos a `implementador` en lugar de denegación total:**
  - _Justificación:_ En proyectos reales con directrices técnicas previas (como Spring Boot o React), los agentes deben poder escribir código de aplicación. Restringir solo contratos protegidos previene el reward-hacking sin paralizar el desarrollo.
- **Decisión 3: Sobrescritura atómica en `plan-lock` hacia `implementador`:**
  - _Justificación:_ Cumple la regla inmutable de SDD: una vez que el plan y el verificador están sellados, ningún agente puede mantener privilegios de planificación (`seb-planificador`).

## 5. Secuencia de Tareas Atómicas (TDD: Test en Rojo -> Código -> Test en Verde)

- [x] **T1. Gestión de Roles CLI y Persistencia Local** (`RF-01`, `RF-02`, `RF-03`, `RF-04`, `CL-01`, `CL-03`)
  - _Archivos:_ `engine/cmd/seb/role_session_test.go`, `engine/cmd/seb/main.go`, `.gitignore`
  - _Acción:_ Crear test `TestRoleCLIManagement` validando `seb role set <rol>`, `seb role get`, `seb role clear`, saneamiento de mayúsculas/espacios, salida `1` ante roles inválidos e idempotencia. Implementar funciones auxiliares y subcomando `cmdRole`.
  - _Hecho cuando:_ El comando `CHK-01` devuelve exit code `0`.

- [x] **T2. Jerarquía de Precedencia y Filtrado Restrictivo de Inferencia** (`RF-05`, `RF-06`, `CL-02`)
  - _Archivos:_ `engine/cmd/seb/role_session_test.go`, `engine/cmd/seb/main.go`
  - _Acción:_ Crear test `TestActiveRolePrecedenceAndHierarchy` probando los 4 niveles de precedencia (`SEB_SKILL` > `ACTIVE_ROLE` > transcript `seb-*` > `implementador`), comprobando que lecturas de `java-springboot/SKILL.md` son ignoradas y que archivos de rol corruptos degradan a `implementador`. Actualizar `inferActiveSkill`.
  - _Hecho cuando:_ El comando `CHK-02` devuelve exit code `0`.

- [x] **T3. Aislamiento de Habilidades de Dominio y Protección de Contratos** (`RF-07`, `RF-08`)
  - _Archivos:_ `engine/cmd/seb/role_session_test.go`, `engine/cmd/seb/main.go`
  - _Acción:_ Crear test `TestRolePermissionsAndDomainSkillIsolation` verificando que un rol de dominio externo (`java-springboot`) puede escribir código de aplicación (`src/Main.java`), pero es bloqueado al intentar escribir `docs/constitution.md`, `specs/*_spec.md` o `specs/*_verificador.md`. Actualizar `checkPermission`.
  - _Hecho cuando:_ El comando `CHK-03` devuelve exit code `0`.

- [x] **T4. Transición Atómica en `plan-lock` y Limpieza en Ciclo de Vida** (`RF-09`, `CL-04`)
  - _Archivos:_ `engine/cmd/seb/role_session_test.go`, `engine/cmd/seb/main.go`
  - _Acción:_ Crear test `TestRoleLifecyclePlanLockAndCleanup` validando que `seb plan-lock` restablece el rol activo a `implementador`, y que `seb verify` y `seb init` limpian el archivo `.seb/ACTIVE_ROLE`. Conectar helpers en `cmdPlanLock`, `cmdVerify` y `cmdInit`.
  - _Hecho cuando:_ El comando `CHK-04` devuelve exit code `0`.

- [x] **T5. Validación Integral de Calidad, Rendimiento y Compilación** (`RNF-01`, `RNF-02`, `RF-Stack`, `RF-GLOBAL`)
  - _Archivos:_ `engine/cmd/seb/main.go`, binario `.seb/bin/seb`
  - _Acción:_ Ejecutar análisis estático con `go vet`, verificar que la resolución de rol ocurre en <5ms, compilar el binario con flags de producción y validar la suite completa de pruebas.
  - _Hecho cuando:_ Los comandos `CHK-98` y `CHK-99` devuelven exit code `0`.
