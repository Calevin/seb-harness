# Spec 002 — Homogeneización de Nomenclatura con Prefijo Secuencial en Artefactos SDD

- **Tarea ID:** `002`
- **Identificador:** `prefijo_secuencial_artefactos`
- **Origen:** Prosa / Feedback de Sebastian (Refactor de consistencia de nomenclatura)
- **Estado:** `APROBADA`

## 1. Contexto y Objetivo (El POR QUÉ)

Actualmente, el arnés genera el archivo de especificación con un prefijo numérico secuencial (`specs/<NNN>_<tarea>_spec.md`), pero genera el plan técnico y la tabla de verificación sin dicho prefijo (`specs/<tarea>_plan.md` y `specs/<tarea>_verificador.md`).

Esta asimetría introduce desorden visual en el explorador de archivos y en la salida de comandos de terminal (donde las especificaciones quedan agrupadas al inicio por su número y los planes y verificadores quedan dispersos alfabéticamente al final), genera riesgo de colisión de nombres para tareas recurrentes o de nombres genéricos, y debilita la cohesión conceptual de la tríada SDD (`spec`, `plan`, `verificador`).

El objetivo es homogeneizar la nomenclatura para que los tres artefactos de cualquier tarea incorporen de forma consistente el prefijo numérico secuencial de tres dígitos (`<NNN>_`), garantizando que los comandos del arnés resuelvan de forma determinista y ergonómica los archivos de cualquier tarea.

## 2. Actores y Usuarios

- **Desarrollador / Usuario Humano:** Crea especificaciones, revisa artefactos agrupados en disco y ejecuta comandos CLI de verificación y sellado.
- **Agentes de IA (`seb-planificador`, `seb-verificador`, implementador):** Leen, generan y validan los artefactos en disco respetando las convenciones del arnés y los guardarraíles de acceso.
- **CLI del Arnés (`seb`):** Resuelve rutas de archivos, calcula hashes criptográficos y ejecuta las suites de verificación programática.

## 3. Historias de Usuario

- **HU-01:** Como desarrollador que inspecciona el directorio `specs/`, quiero que los archivos de especificación, plan y verificación de una misma tarea compartan el mismo prefijo numérico secuencial, para que aparezcan contiguos y agrupados al ordenar alfabéticamente o listar el directorio.
- **HU-02:** Como desarrollador o agente que opera el CLI, quiero poder referenciar tareas usando indistintamente su identificador de nombre (`<tarea>`), su prefijo (`<NNN>`) o el nombre prefijado canónico (`<NNN>_<tarea>`), sin fricciones ni memorización forzada en terminal.
- **HU-03:** Como usuario del arnés, quiero que la indexación de tareas en `.seb/seb.lock` use la clave canónica completa para evitar colisiones entre tareas con igual nombre en diferentes secuencias.

## 4. Definiciones de Dominio

- **Tríada SDD:** Conjunto de tres artefactos documentales que componen el ciclo de una tarea en el arnés:
  1. Especificación funcional: `<NNN>_<tarea>_spec.md`
  2. Plan técnico de implementación: `<NNN>_<tarea>_plan.md`
  3. Contrato de verificación determinista: `<NNN>_<tarea>_verificador.md`
- **Prefijo Secuencial (`<NNN>`):** Cadena de tres dígitos decimales rellenada con ceros a la izquierda (`001`, `002`, `010`) calculada a partir del mayor índice existente en el directorio `specs/`.
- **Clave Canónica de Tarea:** Cadena con formato `<NNN>_<tarea>` utilizada como identificador unívoco para claves de diccionario en `.seb/seb.lock` y valor en `.seb/ACTIVE_TASK`.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

- **RF-01 (Ubiquitous):** EL SISTEMA DEBE exigir que los tres artefactos de una tarea sigan estrictamente el patrón de nomenclatura `specs/<NNN>_<tarea>_<tipo>.md`, donde `<tipo>` corresponde a `spec`, `plan` o `verificador`.
- **RF-02 (Event-Driven):** CUANDO el agente o usuario genere el plan técnico de una tarea, EL SISTEMA DEBE nombrar el archivo resultante como `specs/<NNN>_<tarea>_plan.md`.
- **RF-03 (Event-Driven):** CUANDO el agente o usuario genere la tabla de verificación determinista de una tarea, EL SISTEMA DEBE nombrar el archivo resultante como `specs/<NNN>_<tarea>_verificador.md`.
- **RF-04 (Event-Driven):** CUANDO se ejecute la operación de sellado de plan (`plan-lock`), EL SISTEMA DEBE localizar y verificar la presencia de `specs/<NNN>_<tarea>_verificador.md` y `specs/<NNN>_<tarea>_spec.md` antes de computar y registrar sus firmas criptográficas.
- **RF-05 (Event-Driven):** CUANDO se ejecute la verificación determinista (`verify`), EL SISTEMA DEBE evaluar los controles contenidos en `specs/<NNN>_<tarea>_verificador.md`.
- **RF-06 (Event-Driven):** CUANDO se invoque un comando del CLI (`plan-lock`, `verify`, `status`) pasando como argumento el identificador simple (`<tarea>`), el prefijo numérico (`<NNN>`) o el nombre canónico (`<NNN>_<tarea>`), EL SISTEMA DEBE resolver de forma determinista la tarea y sus archivos asociados.
- **RF-07 (Unwanted Behavior):** SI un comando del CLI recibe un argumento que coincide con más de una tarea o con ninguna tarea registrada, ENTONCES EL SISTEMA DEBE abortar la ejecución con un mensaje de error explícito y código de salida distinto de cero.
- **RF-08 (Event-Driven):** CUANDO se registre o verifique el sellado criptográfico en `.seb/seb.lock` o la tarea activa en `.seb/ACTIVE_TASK`, EL SISTEMA DEBE utilizar la clave canónica completa `<NNN>_<tarea>`.
- **RF-09 (Event-Driven):** CUANDO los guardarraíles de menor privilegio (`guard`) validen permisos de escritura para el rol de planificador, EL SISTEMA DEBE autorizar la escritura de archivos en `specs/` que coincidan con `specs/<NNN>_<tarea>_plan.md` y `specs/<NNN>_<tarea>_verificador.md`.
- **RF-10 (Event-Driven):** CUANDO los guardarraíles de menor privilegio (`guard`) validen permisos de escritura para el rol de implementador, EL SISTEMA DEBE rechazar cualquier intento de escritura sobre `specs/<NNN>_<tarea>_verificador.md` y `specs/<NNN>_<tarea>_spec.md`.

## 6. Requisitos No Funcionales (Verificables)

- **RNF-01 (Migración Quirúrgica de Tarea 001):** La tarea `001` preexistente (`registro_telemetria_acciones`) debe migrarse renombrando sus archivos en disco a `001_registro_telemetria_acciones_plan.md` y `001_registro_telemetria_acciones_verificador.md`, actualizando la clave en `.seb/seb.lock` y preservando su estado de verificación exitoso sin añadir comandos legacy al binario del arnés.
- **RNF-02 (Alineación Documental y Plantillas):** La totalidad de las plantillas maestras (`payload/`), las reglas de gobernanza (`AGENTS.md`, `00-seb-harness.md`), la constitución (`docs/constitution.md`) y los skills (`seb-planificador`, `seb-verificador`) deben reflejar de forma idéntica la nueva convención `<NNN>_<tarea>_plan.md` y `<NNN>_<tarea>_verificador.md`.

## 7. Casos Límite (Edge Cases)

- **CL-01 (Identificador numérico con diferente relleno):** Si el usuario pasa un número como `1` o `2` en lugar de `001` o `002`, el sistema debe normalizarlo a tres dígitos con ceros a la izquierda antes de resolver la tarea.
- **CL-02 (Colisión por nombre simple repetido en secuencias distintas):** Si dos tareas en `specs/` tienen el mismo nombre semántico pero distinto prefijo secuencial (por ejemplo `001_foo` y `004_foo`), invocar el CLI solo con `foo` debe abortar informando ambigüedad y requiriendo el identificador canónico o numérico.

## 8. Fuera de Alcance (Out of Scope)

- Subcomandos de migración automática (`seb migrate-specs`) o capas de emulación en el CLI para formatos obsoletos.
- Modificación de la sintaxis EARS de los requisitos o de la estructura interna tabular de los verificadores.
- Alteración de los algoritmos de hashing criptográfico SHA-256.

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos funcionales (`RF-01` a `RF-10`) cuentan con controles de verificación determinista en estado `[PASA]`.
- La tarea `001` preexistente queda renombrada a la nueva convención y `.seb/bin/seb verify 001` (así como `seb verify registro_telemetria_acciones` y `seb verify 001_registro_telemetria_acciones`) retorna éxito (`exit code 0`).
- No existen discrepancias entre la documentación en `docs/constitution.md`, las reglas y los skills distribuidos en `payload/`.

## 10. Dudas Abiertas y Puntos por Clarificar

- Ninguna duda pendiente. Decisiones de dominio consensuadas con el usuario el 2026-10-23:
  1. Resolución flexible habilitada en el CLI (ID, nombre simple, nombre canónico) con rechazo ante ambigüedades.
  2. Uso mandatorio de clave canónica `<NNN>_<tarea>` en `.seb/seb.lock` y `.seb/ACTIVE_TASK`.
  3. Migración directa de la tarea 001 sin código legacy en el ejecutable `seb`.
