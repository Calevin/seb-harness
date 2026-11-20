# Spec 016 — Observabilidad e Introspección del CLI y Estado SDD

- **Tarea ID:** `016`
- **Identificador:** `observabilidad_introspeccion_cli`
- **Origen:** Petición directa en prosa
- **Estado:** `APROBADA`
- **Afecta-UC:** `nuevo`

## 1. Contexto y Objetivo (El POR QUÉ)

Cualquier agente autónomo o desarrollador que opere en un proyecto consumidor de `seb-harness` (donde únicamente dispone del binario `.seb/bin/seb` y los archivos distribuidos desde `payload/`, sin acceso al código fuente del motor) necesita conocer con exactitud el estado real de cada tarea y el contrato operativo de cada subcomando sin tener que inferir ni suponer transiciones.

Actualmente existen dos planos de estado separados que no se explican ni se cruzan en la salida del CLI: el estado editorial en el Markdown `*_spec.md` (`BORRADOR` -> `APROBADA` / `BLOQUEADA_POR_DEPENDENCIA` / `ABORTADA`) y el estado operativo del motor (`SPECIFIED` -> `PLANNED` -> `VERIFIED` / `FAILED_VERIFICATION` / `ABORTED`). Como `seb-clarificador` solo actualiza el Markdown a `APROBADA` sin invocar ningún subcomando CLI, `seb status` continúa reportando `SPECIFIED`, generando ambigüedad. Además, `seb status` ignora argumentos de tarea, depende exclusivamente de SQLite (devolviendo una tabla vacía en clones limpios sin `.seb/seb-audit.db`), no detecta desfases entre SQLite y el disco, no indica el siguiente paso esperado y el CLI carece de ayuda consultable por subcomando. Esta especificación resuelve esa brecha de observabilidad e introspección.

## 2. Actores y Usuarios

- **Agente Autónomo SDD:** Agente de IA operando en un proyecto consumidor bajo cualquiera de las fases del ciclo SDD (`seb-iniciador`, `seb-especificador`, `seb-clarificador`, `seb-planificador`, implementador, `seb-verificador`, `seb-documentador`).
- **Desarrollador Humano:** Ingeniero que supervisa, audita o depura el estado de las tareas y el uso del CLI `.seb/bin/seb` en local o en un clon limpio del repositorio.

## 3. Historias de Usuario

- **016-HU-01:** Como agente autónomo o desarrollador, quiero ejecutar `seb status` (global o filtrado por una tarea) para conocer el estado unificado entre Markdown, `.seb/seb.lock`, dependencias DAG y SQLite, detectar desfases y saber exactamente qué paso sigue.
- **016-HU-02:** Como agente autónomo en un clon limpio sin base de auditoría local, quiero que `seb status` reconstruya el estado real desde `specs/` y `.seb/seb.lock` para no recibir una vista vacía engañosa.
- **016-HU-03:** Como agente autónomo o desarrollador, quiero consultar `seb help <subcomando>` o `--help`/`-h` en cualquier subcomando para conocer bajo demanda qué lee, qué escribe, qué precondiciones exige y qué transición de estado provoca sin sobrecargar el contexto base.
- **016-HU-04:** Como agente autónomo en un proyecto consumidor, quiero contar en `payload/AGENTS.md` y en los `SKILL.md` con el mapa de estados y la indicación explícita de usar `seb status` y `seb help` para no asumir comportamientos incorrectos.

## 4. Definiciones de Dominio (Opcional)

- **Plano Editorial (Markdown):** Estado declarado en la cabecera `- **Estado:**` de `specs/<NNN>_<tarea>_spec.md` (`BORRADOR`, `APROBADA`, `BLOQUEADA_POR_DEPENDENCIA`, `ABORTADA`).
- **Plano Operativo (Motor / SQLite):** Estado registrado en la tabla de tareas de `.seb/seb-audit.db` (`SPECIFIED`, `PLANNED`, `VERIFIED`, `FAILED_VERIFICATION`, `ABORTED`).
- **Plano de Sellado (`.seb/seb.lock`):** Estado versionado en Git que registra hashes SHA-256 de spec y verificador, bandera `verified`, bandera `aborted` y `commit_hash`.
- **Desfase de Estado:** Cualquier contradicción observable entre el plano editorial en disco, el plano de sellado en `.seb/seb.lock` y el plano operativo en `.seb/seb-audit.db`.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

### 5.1. Comando `seb status`

- **016-RF-01 (Event-Driven):** CUANDO se ejecuta `seb status` sin argumentos ni flag `--compact`, EL SISTEMA DEBE mostrar la vista completa de todas las tareas existentes cruzando las tres fuentes de verdad: archivos en `specs/`, registros en `.seb/seb.lock` y la tabla de tareas en `.seb/seb-audit.db`.
- **016-RF-02 (Event-Driven):** CUANDO se ejecuta `seb status <tarea>` pasando un identificador numérico, nombre simple o clave canónica, EL SISTEMA DEBE resolver la tarea según las reglas estándar de resolución flexible y mostrar la vista completa con todo su detalle únicamente para esa tarea.
- **016-RF-03 (Event-Driven):** CUANDO se ejecuta `seb status --compact` (con o sin `<tarea>`), EL SISTEMA DEBE mostrar únicamente la tabla resumida mínima con las columnas esenciales de identificación y estado.
- **016-RF-04 (Ubiquitous):** EL SISTEMA DEBE permitir que `seb status` funcione en clones limpios donde no exista `.seb/seb-audit.db`, reconstruyendo la lista y el estado de las tareas a partir de los archivos en `specs/` y `.seb/seb.lock` en modo de lectura pura sin crear `.seb/seb-audit.db` en disco.
- **016-RF-05 (Ubiquitous):** EL SISTEMA DEBE mostrar en la vista completa de cada tarea en `seb status`:
  1. Estado editorial leído desde `*_spec.md` (`BORRADOR`, `APROBADA`, `BLOQUEADA_POR_DEPENDENCIA`, `ABORTADA` o indicador explícito si falta o es inválido).
  2. Cantidad de etiquetas de aclaración pendiente (`NECESITA ACLARACIÓN`) en `*_spec.md`.
  3. Estado operativo del motor en `.seb/seb-audit.db` (`SPECIFIED`, `PLANNED`, `VERIFIED`, `FAILED_VERIFICATION`, `ABORTED`, o `NO_REGISTRADA` si no existe registro local).
  4. Presencia de los archivos `*_plan.md` y `*_verificador.md`.
  5. Estado en `.seb/seb.lock` (si está sellada, valor de `verified`, valor de `aborted` y `commit_hash`).
  6. Estado real de sus dependencias directas declaradas en el DAG (cumplidas, pendientes, ausentes o abortadas).
  7. Indicación determinista del **siguiente paso esperado** (skill o comando CLI que corresponde ejecutar a continuación según el estado combinado observado).
- **016-RF-06 (Unwanted Behavior):** SI `seb status` detecta desfase o conflicto entre `.seb/seb-audit.db`, los archivos en `specs/` o `.seb/seb.lock` (por ejemplo: tarea `ABORTADA` en disco o en `.seb/seb.lock` con estado distinto en SQLite, tarea `verified: true` en `.seb/seb.lock` sin reflejo en SQLite, o tarea registrada en SQLite sin archivo `*_spec.md` en disco), ENTONCES EL SISTEMA DEBE emitir una alerta explícita asociada a la tarea detallando la discrepancia detectada.
- **016-RF-07 (Unwanted Behavior):** SI se invoca `seb status <tarea>` con un identificador inexistente o ambiguo, ENTONCES EL SISTEMA DEBE emitir un mensaje de error claro por `stderr` y retornar código de salida `1`.
- **016-RF-08 (Event-Driven):** CUANDO se invoca `seb status --json` (con o sin `<tarea>`), EL SISTEMA DEBE emitir por `stdout` un documento JSON válido con la estructura completa de las tareas consultadas y sus alertas o siguientes pasos esperados.

### 5.2. Sistema de Ayuda Bajo Demanda (`seb help` y `--help` / `-h`)

- **016-RF-09 (Event-Driven):** CUANDO se ejecuta `seb help` sin argumentos, EL SISTEMA DEBE mostrar por `stdout` el catálogo general de todos los subcomandos existentes del binario `seb` (incluyendo `init`, `guard`, `telemetry`, `role`, `oneoff`, `audit`, `memory-add`, `abort`, `spec-next`, `plan-lock`, `verify`, `status`, `help` y cualquier otro subcomando activo) con una breve descripción de cada uno y la instrucción para consultar la ayuda individual.
- **016-RF-10 (Event-Driven):** CUANDO se ejecuta `seb help <subcomando>`, `seb <subcomando> --help` o `seb <subcomando> -h`, EL SISTEMA DEBE mostrar por `stdout` la ficha técnica completa de ese subcomando y finalizar con código de salida `0`, sin ejecutar la operación del subcomando ni mutar estado.
- **016-RF-11 (Ubiquitous):** EL SISTEMA DEBE incluir en la ficha técnica de cada subcomando: propósito, sintaxis y flags soportados, archivos o tablas que lee, archivos o tablas que modifica, precondiciones y guardarraíles aplicables, transición de estado que provoca (cuando aplique) y códigos de salida posibles.
- **016-RF-12 (Unwanted Behavior):** SI se invoca `seb help <subcomando>` con un nombre de subcomando inexistente, ENTONCES EL SISTEMA DEBE informar por `stderr` que el subcomando no existe y retornar código de salida `1`.
- **016-RF-17 (Ubiquitous):** EL SISTEMA DEBE autorizar en `seb guard` la ejecución de `seb status`, `seb help` y `--help` / `-h` para todos los roles SDD (`seb-iniciador`, `seb-especificador`, `seb-clarificador`, `seb-planificador`, `implementador`, `seb-verificador`, `seb-documentador`) sin restricciones de menor privilegio.

### 5.3. Documentación Operativa Distribuida (`payload/`)

- **016-RF-13 (Ubiquitous):** EL SISTEMA DEBE incluir en `payload/AGENTS.md` (y/o reglas distribuidas en `payload/.agents/rules/`) una tabla compacta de la máquina de estados completa que relacione el plano editorial Markdown (`*_spec.md`), el plano operativo del motor (`seb status` / SQLite) y el estado en `.seb/seb.lock`, indicando qué acción o comando dispara cada transición.
- **016-RF-14 (Ubiquitous):** EL SISTEMA DEBE incluir en `payload/AGENTS.md` la regla operativa explícita que instruye a los agentes a consultar `seb status` y `seb help` antes de asumir transiciones de estado o comportamiento del CLI.
- **016-RF-15 (Ubiquitous):** EL SISTEMA DEBE actualizar los archivos `SKILL.md` en `payload/.agents/skills/` para incluir `seb status` y `seb help` como herramientas oficiales de diagnóstico en todas las fases del ciclo SDD.
- **016-RF-16 (Ubiquitous):** EL SISTEMA DEBE explicitar en el `SKILL.md` de `seb-clarificador` que cambiar el estado editorial de una spec a `APROBADA` en el Markdown no cambia el estado operativo `SPECIFIED` en `seb status` hasta que se ejecute `seb plan-lock`.

## 6. Requisitos No Funcionales (Verificables)

- **016-RNF-01 (Ejecución Offline y Latencia):** `seb status` y `seb help` deben ejecutarse 100% offline y responder en menos de `500 ms` en un repositorio con hasta 100 especificaciones.
- **016-RNF-02 (Inmutabilidad en Consulta):** `seb status`, `seb help` y `--help`/`-h` no deben crear, modificar ni eliminar archivos en `specs/`, `.seb/seb.lock`, `.seb/ACTIVE_TASK` ni `.seb/seb-audit.db`.
- **016-RNF-03 (Economía de Contexto):** La salida por defecto de `seb` sin argumentos debe mantenerse compacta; la documentación detallada por subcomando solo debe entregarse bajo demanda mediante `seb help <subcomando>` o `--help`/`-h`.

## 7. Casos Límite (Edge Cases)

- **016-CL-01 (Repositorio sin tareas ni specs):** Cuando `specs/` no existe o está vacío y no hay registros en SQLite ni en `seb.lock`, `seb status` debe informar la ausencia de tareas por `stdout` y retornar código de salida `0`.
- **016-CL-02 (Spec sin cabecera `Estado:` o con valor no reconocido):** `seb status` debe reportar el estado editorial como `DESCONOCIDO` (con alerta de formato) sin interrumpir el procesamiento del resto de tareas.
- **016-CL-03 (Precedencia de `--help` / `-h`):** Si se invoca `seb <subcomando> --help` o `-h` junto con otros argumentos incompletos o inválidos, el sistema debe priorizar la exhibición de la ayuda y retornar código `0` sin validar ni ejecutar el subcomando.
- **016-CL-04 (Múltiples desfases simultáneos):** Si una misma tarea presenta discrepancias entre disco, `seb.lock` y SQLite de forma simultánea, `seb status` debe listar todas las alertas detectadas para esa tarea.
- **016-CL-05 (`.seb/seb.lock` corrupto o ilegible):** SI `.seb/seb.lock` existe en disco pero contiene JSON inválido o ilegible, ENTONCES `seb status` debe abortar emitiendo un mensaje de error claro por `stderr` y retornando código de salida `1`.

## 8. Fuera de Alcance (Out of Scope)

- Auto-reparación o sincronización automática de `.seb/seb-audit.db` al ejecutar `seb status` (el comando es estrictamente de solo lectura/observación).
- Generación automática de páginas man o exportación a formatos externos (HTML, PDF).
- Modificación de las transiciones de estado existentes en `seb spec-next`, `seb plan-lock`, `seb verify` o `seb abort` más allá de exponer su ayuda y reflejar su estado actual.

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos (`016-RF-01` a `016-RF-17`, `016-RNF-01` a `016-RNF-03` y `016-CL-01` a `016-CL-05`) cuentan con verificación programática en `[PASA]` ejecutada mediante `.seb/bin/seb verify observabilidad_introspeccion_cli`.
- Cero violaciones a `docs/constitution.md`.

## 10. Dudas Abiertas y Puntos por Clarificar

- Ninguna duda pendiente.
