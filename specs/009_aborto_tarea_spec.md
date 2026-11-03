# Spec 009 — Aborto Formal y Purga de Tareas en seb-harness

- **Tarea ID:** `009`
- **Identificador:** `aborto_tarea`
- **Origen:** Solicitud de usuario / Alternativa 2 recomendada
- **Estado:** `APROBADA`

## 1. Contexto y Objetivo (El POR QUÉ)

Durante el ciclo de desarrollo asistido por modelos de lenguaje (LLM), es común que un agente sobredimensione una tarea (*scope creep*), alucine requisitos no deseados o entre en ciclos de iteración improductivos, requiriendo que el desarrollador aborte la tarea en curso. Actualmente, el arnés carece de un mecanismo formal para cancelar o eliminar una tarea, lo que obliga al usuario a manipular manualmente bases de datos internas, archivos de candado y artefactos en disco para evitar que la secuencia numérica se altere o que una tarea cancelada bloquee dependencias.

Esta funcionalidad proporciona una vía estandarizada y segura para abortar tareas a través de la interfaz de línea de comandos, ofreciendo dos comportamientos bien diferenciados:
1. **Aborto Formal (por defecto):** Preserva la telemetría, el historial de costos en tokens y la trazabilidad de auditoría, marcando la tarea como abortada e invalidándola en el grafo de dependencias para no bloquear otras tareas. El archivo de especificación en disco se conserva con su estado actualizado, manteniendo la correlación histórica en la numeración.
2. **Purga Total (mediante opción explícita `--purge`):** Erradica todo rastro de la tarea (registros de estado, telemetría y archivos de especificación/plan/verificación), liberando el número de secuencia para arranques limpios si era la última tarea creada.

## 2. Actores y Usuarios

- **Desarrollador / Operador Humano:** Ejecuta el comando desde la terminal cuando decide interrumpir una tarea mal encaminada o limpiar una prueba fallida.
- **Agentes de IA del Arnés:** Consultan el estado de tareas y el grafo de dependencias, reconociendo tareas abortadas para no considerarlas dependencias válidas.
- **Sistema de Auditoría y Observabilidad:** Recibe el estado actualizado de la tarea y preserva (o purga, según la modalidad elegida) los registros de costo y esfuerzo consumidos.

## 3. Historias de Usuario

- **HU-01:** Como desarrollador, quiero abortar formalmente una tarea desbocada para que sus costos de tokens queden auditados pero su estado no bloquee ni confunda a otras tareas ni al grafo del proyecto.
- **HU-02:** Como desarrollador, quiero purgar completamente una tarea fallida con un flag explícito para que no quede ningún rastro en el proyecto y el identificador secuencial pueda ser reutilizado en una spec limpia.
- **HU-03:** Como desarrollador, quiero recibir advertencias claras si intento abortar o purgar una tarea de la que dependen otras tareas ya existentes en el proyecto, requiriendo `--force` para proceder.

## 4. Definiciones de Dominio

- **Aborto Formal:** Transición de estado donde la tarea queda catalogada como `ABORTED`. Se desactiva su vigencia operativa, se impide su uso como prerrequisito satisfecho en el grafo de dependencias, se actualiza el estado en el archivo de especificación y se retienen los registros de auditoría y costo acumulados.
- **Purga Atómica:** Eliminación destructiva y total de todos los metadatos, registros de telemetría y artefactos en disco vinculados a una tarea, devolviendo el repositorio al estado previo a la creación de dicha tarea.
- **Tarea Activa:** Tarea actualmente vinculada a la sesión o al contexto de trabajo local sobre la cual se imputan automáticamente las acciones y métricas.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

### Gestión de Aborto Formal (Comportamiento por Defecto)
- **RF-01 (Event-Driven):** CUANDO el usuario invoque el comando de aborto indicando el identificador numérico, el nombre simple o la clave canónica de una tarea sin opciones destructivas, EL SISTEMA DEBE transicionar el estado de la tarea a `ABORTED` en el almacenamiento local y actualizar la cabecera de la especificación a `Estado: ABORTADA` manteniendo el archivo físico en su ubicación.
- **RF-02 (Event-Driven):** CUANDO una tarea sea transicionada al estado `ABORTED`, EL SISTEMA DEBE conservar íntegros todos los registros de auditoría, consumo de tokens, latencia y acciones previamente imputadas a dicha tarea.
- **RF-03 (State-Driven):** MIENTRAS una tarea se encuentre en estado `ABORTED`, EL SISTEMA DEBE reflejar su estado como `[ABORTADA]` en el listado de estado general del proyecto.
- **RF-04 (Event-Driven):** CUANDO se aborte una tarea que esté configurada como la tarea activa de la sesión, EL SISTEMA DEBE desvincular la tarea activa para que las acciones subsiguientes no queden asociadas a una tarea cancelada.
- **RF-05 (Event-Driven):** CUANDO una tarea que posea registro de candado sea formalmente abortada, EL SISTEMA DEBE actualizar dicho registro en el archivo de candado de versiones persistiendo `"verified": false` y `"aborted": true`, impidiendo que sea tratada como dependencia satisfecha por otras tareas.

### Gestión de Purga Completa (Opción `--purge`)
- **RF-06 (Event-Driven):** CUANDO el usuario invoque el comando de aborto indicando una tarea y la opción `--purge`, EL SISTEMA DEBE eliminar de forma atómica y completa los registros de la tarea, sus validaciones y sus acciones de telemetría en el almacenamiento local de auditoría.
- **RF-07 (Event-Driven):** CUANDO el usuario invoque el comando de aborto con la opción `--purge`, EL SISTEMA DEBE eliminar físicamente los archivos de especificación, plan y matriz de verificación vinculados a la tarea en el árbol de especificaciones.
- **RF-08 (Event-Driven):** CUANDO el usuario invoque el comando de aborto con la opción `--purge`, EL SISTEMA DEBE eliminar la entrada de la tarea en el archivo de candado de versiones del proyecto.
- **RF-09 (Event-Driven):** CUANDO se complete una purga con `--purge` y se eliminen los artefactos físicos de la tarea de mayor número secuencial, EL SISTEMA DEBE permitir que la siguiente invocación de reserva de especificación reasigne de inmediato el número secuencial liberado.

### Manejo de Errores y Validaciones
- **RF-10 (Unwanted Behavior):** SI el identificador de tarea proporcionado no coincide con ninguna tarea registrada ni existente en el proyecto, ENTONCES EL SISTEMA DEBE emitir un mensaje de error a la salida estándar de errores indicando que la tarea no fue encontrada y finalizar con código de salida no nulo.
- **RF-11 (Unwanted Behavior):** SI el argumento de tarea es ambiguo por coincidir con múltiples tareas, ENTONCES EL SISTEMA DEBE listar las coincidencias candidatas en el mensaje de error y finalizar con código de salida no nulo sin modificar ningún estado.
- **RF-12 (Unwanted Behavior):** SI el usuario intenta abortar o purgar una tarea de la que dependen otras tareas registradas en el proyecto y no se especifica la opción `--force` (o `-f`), ENTONCES EL SISTEMA DEBE advertir explícitamente sobre el impacto en las dependencias dependientes y finalizar con código de salida no nulo bloqueando la operación.
- **RF-13 (Unwanted Behavior):** SI el usuario intenta abortar o purgar una tarea que ya fue formalmente verificada y sellada con éxito (`VERIFIED`) y no se especifica la opción `--force` (o `-f`), ENTONCES EL SISTEMA DEBE bloquear la operación emitiendo un mensaje de advertencia y finalizar con código de salida no nulo para prevenir la alteración accidental del historial.

### Interfaz de Comando
- **RF-14 (Ubiquitous):** EL SISTEMA DEBE admitir la sintaxis de invocación tanto en la forma compacta `seb abort <tarea> [--purge] [--force|-f]` como en la forma de gestión `seb task abort <tarea> [--purge] [--force|-f]`.

## 6. Requisitos No Funcionales (Verificables)

- **RNF-01 (Determinismo y Rendimiento):** La ejecución del comando de aborto o purga debe completarse en menos de 500 milisegundos en condiciones operativas normales.
- **RNF-02 (Atomicidad de la Purga):** La eliminación con `--purge` debe ser atómica; si ocurre un fallo al eliminar algún componente o archivo, el sistema debe informar el error sin dejar registros corruptos o huérfanos incoherentes.
- **RNF-03 (Idempotencia y Sin Efectos Colaterales):** Invocar el comando de aborto formal sobre una tarea ya abortada debe resultar exitoso sin alterar la telemetría histórica preexistente.
- **RNF-04 (Compatibilidad Multiplataforma):** La operación no debe depender de herramientas externas del sistema operativo (como comandos shell `rm` o clientes binarios de base de datos) y debe operar exclusivamente mediante capacidades nativas del motor del arnés.

## 7. Casos Límite (Edge Cases)

- **CL-01 (Tarea sin artefactos generados aún):** Tarea que solo existe como registro preliminar pero sin plan ni verificador creados. El sistema debe procesar el aborto o la purga limpiando únicamente los elementos que efectivamente existan, sin arrojar error por archivos ausentes.
- **CL-02 (Tarea con solo archivos en disco sin registro en base local):** Caso típico de clon nuevo o entorno multi-máquina sin base local de auditoría. El sistema debe poder resolver la tarea a partir de sus archivos en disco y ejecutar el aborto o purga de los archivos físicos y candados versionados.
- **CL-03 (Presencia de `.seb/ACTIVE_TASK` coincidente):** El archivo de tarea activa debe ser eliminado o limpiado si su contenido coincide con la tarea que está siendo abortada o purgada.
- **CL-04 (Espacios en blanco o mayúsculas en el identificador):** El identificador proporcionado por el usuario debe ser normalizado y saneado de espacios y mayúsculas antes de resolver la tarea.

## 8. Fuera de Alcance (Out of Scope)

- Revertir automáticamente cambios en el árbol de trabajo de Git (código fuente modificado por el agente). El arnés gestiona su propio estado, metadatos y artefactos de especificación; el control del árbol de código permanece bajo responsabilidad del desarrollador mediante comandos de Git (`git restore` o `git clean`).
- Restaurar automáticamente tareas una vez que han sido purgadas con `--purge` (la purga es definitiva e irreversible a nivel de arnés).

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos (`RF-01` a `RF-14`) cuentan con verificación programática en `[PASA]` ejecutada mediante `.seb/bin/seb verify aborto_tarea`.
- Cero violaciones a las reglas arquitectónicas de `docs/constitution.md`.
- Cobertura completa de pruebas unitarias y de integración en el motor que validen tanto el flujo de aborto por defecto como el modo con `--purge`.

## 10. Dudas Abiertas y Puntos por Clarificar

- Ninguna duda pendiente. Especificación formalmente auditada y aprobada.
