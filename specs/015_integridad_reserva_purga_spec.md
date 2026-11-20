# Spec 015 — Integridad en Reserva Secuencial y Purga de Tareas

- **Tarea ID:** `015`
- **Identificador:** `integridad_reserva_purga`
- **Origen:** Prosa (petición directa)
- **Estado:** `APROBADA`
- **Afecta-UC:** `UC-artifacts_sequential_naming`, `UC-task_abort_and_purge`

## 1. Contexto y Objetivo (El POR QUÉ)

Actualmente, reservar dos o más tareas consecutivas con `seb spec-next` antes de escribir el primer archivo en disco provoca colisiones de identificador secuencial y la sobrescritura silenciosa de la reserva previa en el almacenamiento local, generando desfases entre disco y auditoría, ambigüedades de resolución y falsas dependencias huérfanas. Adicionalmente, purgar una tarea mediante `seb abort <tarea> --purge` puede eliminar por error registros de otras tareas que compartan el mismo nombre simple pero posean un identificador distinto. Esta especificación garantiza la atomicidad e integridad de la reserva secuencial y acota estrictamente la purga a la identidad unívoca de la tarea.

## 2. Actores y Usuarios

- **Agente Especificador / Usuario CLI:** Invoca `seb spec-next` para reservar identificadores secuenciales de nuevas especificaciones (incluyendo ejecuciones consecutivas o encadenadas) y `seb abort --purge` para eliminar tareas abortadas.
- **Motor CLI (`seb`):** Calcula secuencias, materializa el archivo stub inicial en disco, sincroniza el almacenamiento local y ejecuta purgas transaccionales.

## 3. Historias de Usuario

- **015-HU-01:** Como agente especificador o desarrollador, quiero que `seb spec-next` cree de forma atómica el archivo stub inicial en disco y rechace colisiones de ID para poder encadenar reservas de múltiples tareas sin perder registros ni generar dependencias huérfanas.
- **015-HU-02:** Como usuario del CLI, quiero que `seb abort <tarea> --purge` elimine únicamente la tarea resuelta por su identidad unívoca para evitar el borrado colateral de otras tareas con distinto ID que compartan el mismo nombre.

## 4. Definiciones de Dominio (Opcional)

- **Archivo Stub Inicial:** Archivo `specs/<NNN>_<nombre_tarea>_spec.md` recién creado por `seb spec-next` con la cabecera mínima de plantilla (`Tarea ID`, `Identificador`, `Estado: BORRADOR`, `Afecta-UC: [PENDIENTE]`) y el marcador estándar de aclaración pendiente en la sección de dudas abiertas para impedir su planificación prematura.
- **Identidad Unívoca:** Clave canónica de la tarea compuesta por su identificador secuencial `<NNN>` y su identificador `<nombre_tarea>` (ej. `015_integridad_reserva_purga`), que distingue de forma inequívoca a una tarea frente a cualquier otra.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

- **015-RF-01 (Event-Driven):** CUANDO se ejecute `seb spec-next <nombre_tarea>`, EL SISTEMA DEBE calcular el siguiente identificador secuencial de tres dígitos tomando el máximo entre los prefijos numéricos presentes en el directorio `specs/` y las reservas registradas en el almacenamiento local, sumando una unidad (iniciando en `001` cuando no existan especificaciones ni reservas previas).
- **015-RF-02 (Event-Driven):** CUANDO `seb spec-next <nombre_tarea>` reserve un nuevo identificador sin el flag de solo consulta, EL SISTEMA DEBE crear en disco de forma atómica el archivo stub inicial `specs/<NNN>_<nombre_tarea>_spec.md` con cabecera mínima válida (`Tarea ID`, `Identificador`, `Estado: BORRADOR`, `Afecta-UC: [PENDIENTE]`) y marcador de aclaración pendiente en su cuerpo, e imprimir la ruta exacta del archivo en la salida estándar.
- **015-RF-03 (Unwanted Behavior):** SI se intenta reservar o sincronizar un identificador secuencial que ya se encuentra asignado en `specs/` o en el almacenamiento local a un `<nombre_tarea>` diferente, ENTONCES EL SISTEMA DEBE rechazar la operación con un mensaje de error explícito en `stderr`, retornar código de salida `1` y preservar intactos tanto el archivo en disco como el registro previo.
- **015-RF-04 (State-Driven):** MIENTRAS exista una especificación con el mismo `<nombre_tarea>` cuyo contenido corresponda al archivo stub inicial sin redactar, EL SISTEMA DEBE comportarse de manera idempotente al invocar `seb spec-next <nombre_tarea>`, devolviendo la ruta existente sin crear un nuevo identificador ni modificar el archivo en disco.
- **015-RF-05 (Unwanted Behavior):** SI se invoca `seb spec-next <nombre_tarea>` cuando ya existe una especificación con el mismo `<nombre_tarea>` que ya fue editada o completada más allá del stub inicial, ENTONCES EL SISTEMA DEBE rechazar la operación con un mensaje de error explícito en `stderr`, retornar código de salida `1` y no crear un nuevo archivo ni modificar el existente.
- **015-RF-06 (Optional Feature):** DONDE se especifique el modificador `--dry-run` en `seb spec-next <nombre_tarea>`, EL SISTEMA DEBE calcular e imprimir en la salida estándar la ruta propuesta `specs/<NNN>_<nombre_tarea>_spec.md` sin crear ningún archivo en disco ni registrar la reserva en el almacenamiento local.
- **015-RF-07 (Unwanted Behavior):** SI ocurre un fallo de escritura en disco al crear el archivo stub inicial tras iniciar la reserva, ENTONCES EL SISTEMA DEBE revertir (rollback) la reserva en el almacenamiento local, emitir un diagnóstico en `stderr` y retornar código de salida `1`.
- **015-RF-08 (Unwanted Behavior):** SI se invoca `seb spec-next` con un nombre de tarea vacío, inválido tras el saneamiento o con secuencias de recorrido de directorios, ENTONCES EL SISTEMA DEBE rechazar la ejecución con un mensaje de error en `stderr` y código de salida `1` sin crear archivos ni registrar reservas.
- **015-RF-09 (Event-Driven):** CUANDO se ejecute `seb abort <tarea> --purge` (o su alias `seb task abort <tarea> --purge`), EL SISTEMA DEBE resolver primero la identidad unívoca de la tarea objetivo y restringir la eliminación en la tabla principal de tareas del almacenamiento local (y en registros dependientes únicamente cuando exista restricción referencial obligatoria en el esquema) exclusivamente a dicha identidad unívoca (`id` / clave canónica), manteniendo intacta cualquier otra tarea con distinto identificador aunque comparta el mismo nombre simple.
- **015-RF-10 (Unwanted Behavior):** SI la referencia `<tarea>` entregada a `seb abort` es ambigua entre múltiples tareas con distinto identificador o no corresponde a ninguna tarea existente, ENTONCES EL SISTEMA DEBE cancelar la operación con un error explícito en `stderr` y código de salida `1` sin eliminar registros ni artefactos.
- **015-RF-11 (Ubiquitous):** EL SISTEMA DEBE mantener actualizadas las instrucciones del skill `seb-especificador` indicando explícitamente que `seb spec-next <nombre_tarea>` crea el archivo stub inicial en disco y que el agente debe completar o sobrescribir dicho archivo al redactar la especificación.

## 6. Requisitos No Funcionales (Verificables)

- **015-RNF-01:** **Autonomía Multi-Máquina:** En un clon limpio del repositorio sin base de datos local, `seb spec-next` debe derivar deterministamente el siguiente identificador secuencial exclusivamente a partir del árbol `specs/` rastreado en Git.
- **015-RNF-02:** **Atomicidad Operacional:** Tanto la reserva con creación de stub en `spec-next` como la purga en `abort --purge` deben garantizar consistencia entre el sistema de archivos y el almacenamiento local ante errores intermedios, sin dejar reservas huérfanas ni borrados parciales.

## 7. Casos Límite (Edge Cases)

- **015-CL-01 (Ejecuciones consecutivas encadenadas):** Al ejecutar `seb spec-next tarea_a && seb spec-next tarea_b` antes de redactar `tarea_a`, `tarea_a` obtiene `<NNN>` con su archivo stub en disco y `tarea_b` obtiene `<NNN+1>` con su propio archivo stub en disco, coexistiendo ambas reservas en el almacenamiento local sin sobrescritura.
- **015-CL-02 (Re-invocación sobre stub vs. spec redactada):** Invocar `seb spec-next tarea_a` por segunda vez mientras `specs/<NNN>_tarea_a_spec.md` mantiene su contenido stub inicial devuelve la misma ruta sin incrementar la secuencia; invocarlo tras haber editado o completado `specs/<NNN>_tarea_a_spec.md` falla con código `1`.
- **015-CL-03 (Purga ante nombres simples repetidos):** Si coexisten en el almacenamiento local dos tareas con distinto identificador (ej. `003` y `007`) y el mismo nombre simple `mi_tarea`, ejecutar `seb abort 003 --purge` elimina únicamente los registros de `003`, conservando íntegros los de `007`.
- **015-CL-04 (Fallo de permisos en disco durante `spec-next`):** Si el directorio `specs/` carece de permisos de escritura al invocar `seb spec-next`, la operación falla con código `1` y no queda ninguna reserva registrada en el almacenamiento local.
- **015-CL-05 (Invocaciones repetidas de `--dry-run`):** Ejecutar `seb spec-next tarea_a --dry-run` múltiples veces consecutivas devuelve siempre el mismo identificador propuesto sin incrementar la secuencia ni crear archivos o filas en el almacenamiento local.

## 8. Fuera de Alcance (Out of Scope)

- Modificación de las reglas de validación del grafo de dependencias (DAG) en `plan-lock` o `verify`.
- Saneamiento o reparación automática de bases de datos locales previamente corruptas por colisiones históricas, ni limpieza de filas locales huérfanas originadas por cambios entre ramas de Git (`git checkout`).
- Sincronización distribuida en tiempo real entre múltiples máquinas sin pasar por commits en Git.

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos (`015-RF-01` a `015-RF-11`) cuentan con verificación programática en `[PASA]` ejecutada mediante `.seb/bin/seb verify integridad_reserva_purga`.
- Cero violaciones a `docs/constitution.md`.

## 10. Dudas Abiertas y Puntos por Clarificar

- Ninguna duda pendiente.
