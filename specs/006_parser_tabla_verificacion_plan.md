# Plan Técnico — Parser Robusto de Tablas de Verificación y Prevención de Omisiones Silenciosas (Spec 006)

- **Spec Origen:** `specs/006_parser_tabla_verificacion_spec.md`
- **Verificador Asociado:** `specs/006_parser_tabla_verificacion_verificador.md`
- **Estado:** `PLANIFICADO`

## 1. Resumen Arquitectónico y Aislamiento de Lógica

El arnés `seb-harness` delega la certificación de calidad en la ejecución de la tabla Markdown ubicada en `specs/<NNN>_<tarea>_verificador.md`. Actualmente, la función `parseVerifierFile` en `engine/cmd/seb/main.go` utiliza una expresión regular lineal rígida que intenta capturar las 4 columnas de la tabla.

Este enfoque original adolece de tres fallos arquitectónicos fundamentales:

1. **Fragilidad de Identificadores y Espaciado:** Exigía originalmente prefijos `RF-`, y tras la flexibilización parcial en la Spec 005 (`[A-Za-z0-9_,-]+`), continúa descartando cualquier fila con espacios tras coma (como `RNF-01, RNF-02`), algo estándar en Markdown.
2. **Omisión Silenciosa (_Silent Drop_ / Falso Aprobado):** Si una fila inicia con intención de control (`| CHK-XX`) pero presenta una comilla invertida faltante, un pipe de más o una celda vacía, la regex simplemente no coincide y el escáner la descarta sin emitir advertencia alguna. Esto reduce artificialmente el número de comprobaciones ejecutadas y emite veredictos de `APROBADO` sobre suites incompletas (evasión involuntaria de controles / reward-hacking).
3. **Confusión por Operadores de Shell en Comandos:** Comandos que utilizan operadores `||` (fallback en bash) o tuberías `|` dentro de las comillas invertidas pueden desfasar el conteo de pipes de la fila si el parser no aísla adecuadamente los delimitadores de celda.

Para solucionar de raíz estos problemas respetando los principios de Go puro sin CGO (`CGO_ENABLED=0`), latencias `< 10ms` y autonomía offline (`docs/constitution.md`), se implementa la siguiente arquitectura técnica:

1. **Analizador Sintáctico por Estados y Filtrado Contextual:**
   El escáner de líneas mantiene un estado booleano `inFencedBlock` que rastrea bloques de código cercados (delimitados por ```). Cualquier línea dentro de estos bloques es ignorada, protegiendo bloques de ejemplo o documentación.
2. **Detector de Filas Candidatas y Rechazo Bloqueante:**
   Toda línea fuera de bloques cercados cuyo primer carácter no blanco sea `|` seguido inmediatamente o con espacios por `CHK-` es catalogada como _Fila Candidata a Control_. Si la línea no puede ser parseada con éxito en las 4 columnas requeridas (Check ID, Requisito, Descripción, Comando entre backticks), o si el comando extraído está vacío, `parseVerifierFile` no continúa silenciosamente: retorna inmediatamente un error descriptivo con el número de línea y el motivo del fallo, provocando el aborto con `exit code 1` de `seb plan-lock` y `seb verify`.
3. **Extracción y Aislamiento de Comandos Complejos:**
   La extracción del comando se realiza identificando el primer y el último backtick (`` ` ``) presentes en la porción final de la fila. Todo lo que se encuentre contenido entre esas dos comillas invertidas es tratado como el comando de shell ejecutable literal, permitiendo operadores `|`, `||`, `&&`, comillas dobles y comillas simples sin alterar la división de columnas de la tabla.
4. **Normalización de Requisitos e Integridad de Identificadores:**
   La columna de requisitos se limpia extrayendo cada token alfanumérico separado por comas, eliminando espacios en blanco adyacentes (`strings.TrimSpace`) y uniéndolos con comas (`RNF-01,RNF-02`). La cadena normalizada se almacena en `CheckItem.ReqID` para su persistencia en la tabla SQLite `verifications` sin requerir modificaciones en el esquema existente.
5. **Control de Unicidad de Check IDs:**
   Se introduce un mapa `seenIDs map[string]int` durante el escaneo para verificar que ningún `Check ID` se repita dentro del mismo archivo verificador. Si se detecta una colisión, se aborta con error detallando el identificador repetido y la línea afectada.

## 2. Mapa de Archivos a Crear o Modificar

| Archivo / Ruta                           | Acción (`CREAR` / `MODIFICAR`) | Responsabilidad Técnica                                                                                                                                                                                                                                                             | Requisitos Cubiertos (`RF-XX`)                            |
| :--------------------------------------- | :----------------------------- | :---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :-------------------------------------------------------- |
| `engine/cmd/seb/main.go`                 | `MODIFICAR`                    | Reemplazar `tableRowRegex` y refactorizar `parseVerifierFile` para incorporar seguimiento de bloques cercados, detección de filas candidatas `CHK-`, aislamiento de comandos entre backticks, normalización de requisitos, validación de unicidad y reporte de errores sintácticos. | `RF-01` a `RF-10`, `RNF-01` a `RNF-03`, `CL-01` a `CL-06` |
| `engine/cmd/seb/verifier_parser_test.go` | `CREAR`                        | Suite determinista de pruebas unitarias y de integración en Go que verifica el soporte de requisitos heterogéneos, anti-silent-drop, aislamiento de operadores de shell, unicidad de IDs, inmunidad contextual y compatibilidad retrospectiva con specs 001-005.                    | `RF-01` a `RF-10`, `RNF-01` a `RNF-03`, `CL-01` a `CL-06` |

## 3. Contratos, Firmas y Modelo de Datos

### Modelo de Estructuras en Go (`engine/cmd/seb/main.go`)

```go
type CheckItem struct {
	ID      string
	ReqID   string
	Desc    string
	Command string
}
```

### Firma y Comportamiento del Parser Refactorizado

```go
// parseVerifierFile analiza sintácticamente el archivo verificador Markdown.
// Retorna la lista de CheckItem válidos o un error explicativo con línea y motivo
// ante cualquier fila CHK- malformada, comando vacío, ID duplicado o ausencia de controles.
func parseVerifierFile(path string) ([]CheckItem, error)
```

### Reglas de Parseo por Línea:

- Si la línea inicia o termina un bloque cercado (` ``` `): alternar estado `inFencedBlock` y continuar.
- Si `inFencedBlock == true`: ignorar la línea.
- Si la línea no coincide con `^\s*\|\s*CHK-`: ignorar (encabezados, separadores `| :--- |`, prosa).
- Si la línea coincide con `^\s*\|\s*CHK-`:
  1. Extraer el primer segmento de ID (`CHK-[A-Za-z0-9_-]+`). Si es inválido, retornar error sintáctico.
  2. Validar que el ID no haya sido visto previamente (`seenIDs`). Si ya existe, retornar error de duplicidad.
  3. Extraer el segmento de requisitos y normalizar (separar por comas, trim, unir sin espacios).
  4. Extraer el segmento de descripción (trim).
  5. Extraer el comando contenido entre el primer y último backtick. Si no hay al menos dos backticks o el comando es vacío, retornar error de comando mal delimitado.
  6. Si faltan columnas o la fila no tiene estructura de tabla válida, retornar error explícito.
- Al finalizar el escaneo, si no se registró ningún `CheckItem`, retornar error de ausencia de controles ejecutables.

## 4. Decisiones Técnicas Justificadas

- **Decisión 1: Sustitución de regex monolítica por parser híbrido (detección de cabecera + extracción de comando por backticks externos).**
  - _Justificación:_ Una expresión regular pura que intente validar tanto el formato Markdown de pipes como comandos arbitrarios de shell falla cuando el comando contiene tuberías `|`, operadores `||` o comillas. Aislar la celda del comando mediante los backticks externos y procesar las columnas precedentes de forma estructurada garantiza robustez total sin importar la complejidad del comando bash.
  - _Alternativa descartada:_ Mantener una sola regex con escapes obligatorios para pipes en comandos (habría obligado a los usuarios a escribir `\|` en bash, rompiendo compatibilidad con verificadores existentes).
- **Decisión 2: Error bloqueante inmediato (Anti-Silent-Drop) en lugar de advertencias no bloqueantes.**
  - _Justificación:_ En un arnés de aseguramiento determinista de calidad, una fila `CHK-` mal escrita indica la intención humana de ejecutar una prueba. Ignorarla o emitir un warning que permita continuar con `exit code 0` es un vector crítico de falsos positivos y reward-hacking. Todo error sintáctico en un control debe detener inmediatamente el sellado y la verificación.
- **Decisión 3: Normalización de requisitos en memoria preservando el esquema SQLite existente.**
  - _Justificación:_ Guardar listas normalizadas como `RNF-01,RNF-02` en el campo `req_id TEXT` de la tabla `verifications` permite trazabilidad directa sin necesidad de migraciones complejas de SQLite ni roturas en los consumidores de telemetría pasiva.

## 5. Secuencia de Tareas Atómicas (TDD: Test en Rojo -> Código -> Test en Verde)

- [x] **T1. Suite de pruebas deterministas para requisitos heterogéneos y normalización de listas** (`RF-01`, `RF-02`, `CL-01`)
  - _Archivos:_ `engine/cmd/seb/verifier_parser_test.go`
  - _Detalle:_ Crear pruebas que reproduzcan las filas del reporte (`CHK-09` con `RNF-02`, `CHK-98` con `RNF-01, RNF-02` con espacios, `CHK-99` con `GLOBAL`, `RF-Stack`), verificando que se extraigan correctamente y los requisitos se normalicen sin espacios adyacentes.
- [x] **T2. Refactorización de `parseVerifierFile` con aislamiento de comandos y normalización** (`RF-01`, `RF-02`, `RF-04`, `CL-01`, `CL-02`)
  - _Archivos:_ `engine/cmd/seb/main.go`
  - _Detalle:_ Implementar la lógica de extracción de columnas, normalización de requisitos y aislamiento de comandos entre backticks externos tolerando pipes y operadores de shell.
- [x] **T3. Detección y rechazo bloqueante de filas candidatas malformadas (Anti-Silent-Drop)** (`RF-03`, `RF-05`, `CL-04`, `CL-05`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/verifier_parser_test.go`
  - _Detalle:_ Implementar la detección de líneas candidatas (`^\s*\|\s*CHK-`) y lanzar errores sintácticos si faltan columnas, faltan backticks o el comando está vacío. Comprobar que `plan-lock` y `verify` aborten con código 1.
- [x] **T4. Validación de unicidad de Check IDs y control de duplicados** (`RF-06`, `CL-04`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/verifier_parser_test.go`
  - _Detalle:_ Incorporar mapa de IDs observados y retornar error si un `CHK-XX` se repite dentro del mismo archivo.
- [x] **T5. Inmunidad contextual a bloques de código cercados y líneas no candidatas** (`RF-07`, `CL-03`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/verifier_parser_test.go`
  - _Detalle:_ Ignorar líneas dentro de bloques cercados (` ``` `), encabezados de tabla, separadores y comentarios sin disparar errores.
- [x] **T6. Validación de integridad en `cmdPlanLock` y `cmdVerify` y ausencia de controles** (`RF-08`, `RF-09`, `RF-10`, `CL-06`)
  - _Archivos:_ `engine/cmd/seb/main.go`, `engine/cmd/seb/verifier_parser_test.go`
  - _Detalle:_ Asegurar que `plan-lock` impida sellar si el verificador tiene errores o carece de controles, y que `verify` exija 100% de controles en `[PASA]`.
- [x] **T7. Validación de requisitos no funcionales, rendimiento y compatibilidad retrospectiva** (`RNF-01`, `RNF-02`, `RNF-03`)
  - _Archivos:_ `engine/cmd/seb/verifier_parser_test.go`
  - _Detalle:_ Test que verifica el parseo exitoso de `specs/001_*` a `specs/005_*` en `< 10ms` y valida que la compilación sin CGO y suite global no presenten regresiones.
