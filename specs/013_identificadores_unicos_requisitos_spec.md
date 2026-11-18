# Spec 013 — Identificadores Únicos de Requisitos con Prefijo de Spec y Migración Homogénea

- **Tarea ID:** `013`
- **Identificador:** `identificadores_unicos_requisitos`
- **Origen:** Prosa (solicitud del usuario; re-especificación tras la verificación de la Spec 014)
- **Estado:** `APROBADA`
- **Dependencias:** `006_parser_tabla_verificacion`, `011_gobernanza_afecta_uc`, `014_afecta_uc_multiple`
- **Afecta-UC:** `UC-artifacts_sequential_naming`, `UC-verifier_table_parsing`, `UC-documentation_functional_generation`, `UC-session_role_management`, `nuevo`

## 1. Contexto y Objetivo (El POR QUÉ)

Todas las especificaciones de `seb-harness` numeran sus elementos de forma local empezando en `01` (`RF-01`, `HU-01`, `RNF-01`, `CL-01`, `CHK-01`). Cuando esos identificadores se citan fuera de su spec de origen (comentarios en código, tests, casos de uso, `MEMORY.md`, commits), la referencia es ambigua: `RF-05` existe en prácticamente todas las specs.

Esta especificación establece un esquema de identificación globalmente único con el formato `<NNN>-<TIPO>-<NN>` (por ejemplo, `013-RF-05`), donde `<NNN>` es el ID de la spec propietaria, y lo convierte en el **único formato válido**, sin punto de corte. Para homogeneizar los repositorios existentes incorpora una **migración** de artefactos heredados que se ejecuta como **operación one-off de actualización del arnés**, bajo un rol dedicado (`migrador`), con resellado autorizado y auditado de hashes.

`seb-harness` se distribuye a múltiples proyectos consumidores, cada uno con su propia numeración de specs (algunos con su última spec en `002`, otros en `006`, `010`, etc.) y con `.seb/seb.lock` versionado en Git. Este proyecto se desarrolla con el propio arnés (dogfooding). Toda regla de esta spec debe funcionar de forma independiente de la numeración de cada proyecto, y la actualización del arnés no debe dejar a ningún proyecto en un estado sin salida.

## 2. Actores y Usuarios

- **Desarrollador / Usuario Humano:** Actualiza el arnés en sus proyectos, activa el rol `migrador`, ejecuta las operaciones one-off y revisa las referencias que la migración no puede resolver.
- **Rol `migrador`:** Rol de sesión dedicado exclusivamente a ejecutar operaciones one-off de actualización del arnés.
- **Agentes `seb-especificador`, `seb-clarificador`, `seb-planificador`, `seb-documentador`:** Generan y auditan artefactos con identificadores prefijados.
- **Motor CLI (`seb verify`, `seb plan-lock`, `seb role`, ejecución de one-off):** Valida matrices de verificación, gestiona el rol y ejecuta las operaciones one-off.
- **Instalador del arnés:** Actualiza el arnés en proyectos consumidores.

## 3. Historias de Usuario

- **013-HU-01:** Como desarrollador, quiero que cada requisito, historia, caso límite y control tenga un identificador único en todo el repositorio, para citarlo desde código, tests o documentación sin ambigüedad.
- **013-HU-02:** Como desarrollador, quiero que el arnés rechace de forma determinista los identificadores sin prefijo o con prefijo incoherente, para evitar referencias erróneas y verificaciones omitidas silenciosamente.
- **013-HU-03:** Como mantenedor de varios proyectos que usan `seb-harness`, quiero una migración automática y auditada de los artefactos existentes, para que todos los proyectos queden con un formato homogéneo sin edición manual masiva ni violaciones de integridad.
- **013-HU-04:** Como mantenedor, quiero que la migración nunca invente la pertenencia de una referencia ambigua, para no introducir referencias cruzadas incorrectas en contratos sellados.
- **013-HU-05:** Como mantenedor del arnés, quiero un mecanismo formal de operaciones one-off ligadas a la spec que las define, ejecutables solo con un rol dedicado, para aplicar futuras actualizaciones del arnés que requieran modificar contratos sellados sin abrir una vía de reward-hacking.

## 4. Definiciones de Dominio

- **Tipos de elemento identificables:** `HU`, `RF`, `RNF`, `CL`, `CHK`.
- **Identificador prefijado:** Cadena `<NNN>-<TIPO>-<sufijo>`, donde `<NNN>` son exactamente tres dígitos, `<TIPO>` es un tipo identificable y `<sufijo>` es el identificador local (para `HU`, `RF`, `RNF` y `CL`, numérico de al menos dos dígitos; para `CHK`, letras, dígitos, `_` o `-`, no vacío).
- **Identificador heredado:** Cadena `<TIPO>-<sufijo>` sin prefijo de spec.
- **Artefactos de tarea:** La spec, el plan y el verificador de una tarea.
- **Artefactos documentales:** Los casos de uso del catálogo (`docs/use-cases/*.md`) y `MEMORY.md`.
- **Fila candidata a control:** Fila de tabla Markdown, fuera de bloques de código cercados, cuya primera celda contiene la subcadena `-CHK-` o comienza con `CHK-`.
- **Referencia de requisito:** Cada elemento de la lista separada por comas de la columna `Requisito` de una matriz de verificación.
- **Marcador `GLOBAL`:** Valor especial de la columna `Requisito`, sin prefijo.
- **Spec existente:** Spec para cuyo `<NNN>` existe en disco un archivo de spec en el directorio de specs del proyecto.
- **Regla de pertenencia:** Un identificador heredado dentro de un artefacto de tarea pertenece a esa tarea, salvo que la misma línea mencione el ID de tres dígitos o la clave canónica de otra spec, en cuyo caso es una **referencia ambigua**. En los artefactos documentales, un identificador heredado es ambiguo salvo que la misma línea mencione exactamente una spec (por ID o clave canónica), en cuyo caso pertenece a esa spec.
- **Operación one-off:** Operación de actualización del arnés, no habitual, identificada por el ID de la spec que la define (por ejemplo, la operación one-off `013`), que puede modificar artefactos sellados bajo las reglas de esta spec.
- **Rol `migrador`:** Rol de sesión canónico, distinto de `implementador`, único autorizado para ejecutar operaciones one-off.
- **Proyecto heredado:** Proyecto con al menos un verificador que contiene identificadores heredados.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

### Generación y Plantillas (Fases 1 y 3)
- **013-RF-01 (Ubiquitous):** EL SISTEMA DEBE utilizar identificadores prefijados con el marcador `<NNN>` para todos los elementos `HU`, `RF`, `RNF` y `CL` en la plantilla base de especificaciones distribuida por `seb-especificador`.
- **013-RF-02 (Event-Driven):** CUANDO `seb-especificador` genere una spec, EL SISTEMA DEBE instruirle sustituir el marcador `<NNN>` de cada identificador por el ID de tres dígitos devuelto por `seb spec-next`.
- **013-RF-03 (Ubiquitous):** EL SISTEMA DEBE utilizar identificadores `<NNN>-CHK-<sufijo>` en la columna `Check ID` y referencias prefijadas en la columna `Requisito` de la plantilla base de verificadores distribuida por `seb-planificador`.
- **013-RF-04 (Event-Driven):** CUANDO `seb-planificador` genere el plan y el verificador de una spec, EL SISTEMA DEBE instruirle usar en los `Check ID` el prefijo de la spec origen y en las referencias de requisito el identificador prefijado de la spec propietaria de cada requisito.

### Auditoría en Clarificación (Fase 2)
- **013-RF-05 (Event-Driven):** CUANDO `seb-clarificador` audite una spec, EL SISTEMA DEBE instruirle verificar que todo identificador `HU`, `RF`, `RNF` y `CL` declarado en ella use el formato prefijado con un prefijo igual a su `Tarea ID`.
- **013-RF-06 (Unwanted Behavior):** SI una spec auditada contiene un identificador declarado sin prefijo, con prefijo distinto a su `Tarea ID` o duplicado, ENTONCES EL SISTEMA DEBE instruir a `seb-clarificador` registrarlo como observación y prohibir la transición a `APROBADA` hasta su corrección.

### Validación de Matrices de Verificación (Motor: `seb verify` y `seb plan-lock`)
- **013-RF-07 (Event-Driven):** CUANDO el motor analice un verificador, EL SISTEMA DEBE tratar como fila candidata a control toda fila cuya primera celda contenga `-CHK-` o comience con `CHK-`, aplicando a cada una la validación estructural vigente (cuatro columnas, comando entre backticks, campos no vacíos, unicidad del `Check ID`).
- **013-RF-08 (Ubiquitous):** EL SISTEMA DEBE aceptar como `Check ID` válido únicamente un identificador que comience con `<NNN>-CHK-`, donde `<NNN>` es el ID de la tarea verificada, seguido de un sufijo no vacío compuesto por letras, dígitos, `_` o `-`.
- **013-RF-09 (Unwanted Behavior):** SI una fila candidata a control declara un `Check ID` heredado (que comienza con `CHK-`), ENTONCES EL SISTEMA DEBE abortar con código de salida `1` y un mensaje en `stderr` que indique la línea, el identificador y el comando exacto para ejecutar la operación one-off `013` con el rol `migrador`.
- **013-RF-10 (Unwanted Behavior):** SI una fila candidata a control declara un `Check ID` cuyo prefijo no son tres dígitos iguales al ID de la tarea verificada, ENTONCES EL SISTEMA DEBE abortar con código de salida `1` y un mensaje en `stderr` que indique la línea y el identificador incoherente.
- **013-RF-11 (Ubiquitous):** EL SISTEMA DEBE aceptar como referencia de requisito válida únicamente el marcador `GLOBAL` o un identificador que comience con `<NNN>-<TIPO>-`, donde `<TIPO>` es cualquier tipo de elemento identificable y `<NNN>` corresponde a una spec existente.
- **013-RF-12 (Unwanted Behavior):** SI una referencia de requisito no cumple `013-RF-11` (identificador heredado, tipo desconocido, prefijo malformado o spec inexistente), ENTONCES EL SISTEMA DEBE abortar con código de salida `1` y un mensaje en `stderr` que indique la línea y la referencia inválida.
- **013-RF-13 (Ubiquitous):** EL SISTEMA DEBE aplicar las reglas `013-RF-07` a `013-RF-12` de forma idéntica en `seb verify` y en `seb plan-lock`, sin depender de ningún número de spec de corte ni de configuración por proyecto.

### Rol `migrador` y Operaciones One-Off
- **013-RF-14 (Ubiquitous):** EL SISTEMA DEBE reconocer `migrador` como rol canónico válido en la gestión de rol de sesión (`seb role set/get/clear`) y en la variable de rol explícita, con la misma precedencia que el resto de roles.
- **013-RF-15 (Event-Driven):** CUANDO el usuario solicite ejecutar una operación one-off indicando el ID de la spec que la define, EL SISTEMA DEBE ejecutar la operación asociada a ese ID.
- **013-RF-16 (Event-Driven):** CUANDO el usuario solicite la operación one-off sin indicar un ID, EL SISTEMA DEBE listar las operaciones one-off disponibles, con el ID de su spec y si son aplicables al proyecto actual, sin modificar ningún archivo.
- **013-RF-17 (Unwanted Behavior):** SI se solicita una operación one-off con un rol activo distinto de `migrador`, incluido el `implementador` asignado por defecto, ENTONCES EL SISTEMA DEBE denegarla con el código de denegación del guardarraíl de menor privilegio y un mensaje que indique cómo activar el rol `migrador`.
- **013-RF-18 (Unwanted Behavior):** SI se solicita una operación one-off con un ID que no corresponde a ninguna operación registrada, ENTONCES EL SISTEMA DEBE abortar con código de salida `1` y listar los IDs disponibles.
- **013-RF-19 (State-Driven):** MIENTRAS el rol activo sea `migrador`, EL SISTEMA DEBE permitir escritura en artefactos de tarea, artefactos documentales y `.seb/seb.lock`, y denegar la escritura en código fuente de la aplicación, tests y `docs/constitution.md`.

### Operación One-Off `013`: Migración de Identificadores
- **013-RF-20 (Event-Driven):** CUANDO se ejecute la operación one-off `013`, EL SISTEMA DEBE reescribir, en los artefactos de tarea y en los artefactos documentales del proyecto, cada identificador heredado cuya pertenencia sea determinable por la regla de pertenencia, anteponiéndole el ID de la spec propietaria.
- **013-RF-21 (Ubiquitous):** EL SISTEMA DEBE limitar la reescritura de los verificadores a las columnas `Check ID` y `Requisito`, preservando byte a byte la descripción, los comandos y el resto del contenido, salvo lo indicado en `013-RF-27`.
- **013-RF-22 (Unwanted Behavior):** SI la migración encuentra una referencia ambigua, ENTONCES EL SISTEMA DEBE dejarla sin modificar e incluirla en el reporte con archivo, línea y texto, sin asignarle un prefijo por inferencia.
- **013-RF-23 (Event-Driven):** CUANDO la migración modifique un artefacto cuyo hash esté sellado en `.seb/seb.lock`, EL SISTEMA DEBE actualizar el hash sellado correspondiente preservando sin cambios los campos `verified`, `commit_hash`, `aborted` y las dependencias de la tarea, sin ejecutar los verificadores de las tareas.
- **013-RF-24 (Event-Driven):** CUANDO la migración finalice, EL SISTEMA DEBE registrar en la base de auditoría local, asociada al rol `migrador` y a la operación `013`, una entrada por archivo modificado con su hash previo y su hash nuevo, y emitir un reporte con la cantidad de identificadores reescritos y la lista de referencias ambiguas.
- **013-RF-25 (State-Driven):** MIENTRAS el reporte de migración contenga referencias ambiguas, EL SISTEMA DEBE finalizar con código de salida `0` e imprimir una advertencia en `stderr` con la cantidad de referencias pendientes de revisión manual.
- **013-RF-26 (Optional Feature):** DONDE la migración se ejecute en modo simulación, EL SISTEMA DEBE emitir el mismo reporte que en una ejecución real sin modificar ningún archivo ni la base de auditoría.
- **013-RF-27 (Event-Driven):** CUANDO la migración procese en este repositorio el verificador de la Spec 014, EL SISTEMA DEBE actualizar los hashes literales de los artefactos de la Spec 011 embebidos en sus comandos por los hashes resultantes de la migración, como excepción explícita y auditada a `013-RF-21`.
- **013-RF-28 (Unwanted Behavior):** SI algún artefacto de tarea, artefacto documental o `.seb/seb.lock` tiene cambios sin commitear en Git al iniciar la migración, ENTONCES EL SISTEMA DEBE abortar con código de salida `1` sin modificar ningún archivo.
- **013-RF-29 (Unwanted Behavior):** SI la migración falla después de haber comenzado a escribir, ENTONCES EL SISTEMA DEBE restaurar todos los archivos modificados y `.seb/seb.lock` a su contenido previo y salir con código `1`.

### Distribución Multi-Proyecto
- **013-RF-30 (Event-Driven):** CUANDO el instalador actualice el arnés en un proyecto heredado, EL SISTEMA DEBE informar al finalizar que el proyecto contiene identificadores heredados, la cantidad de tareas afectadas y los comandos exactos para activar el rol `migrador` y ejecutar la operación one-off `013`.

### Referencias en Documentación y Gobernanza
- **013-RF-31 (Event-Driven):** CUANDO `seb-documentador` cite requisitos en un caso de uso, EL SISTEMA DEBE instruirle usar el identificador prefijado de la spec propietaria.
- **013-RF-32 (Ubiquitous):** EL SISTEMA DEBE documentar en las directivas distribuidas (`AGENTS.md` y reglas globales) que toda referencia a un requisito fuera de su propia spec (comentarios de código, tests, `MEMORY.md`, commits, casos de uso) debe usar el identificador prefijado, escrito solo en texto y nunca como parte de identificadores de código.
- **013-RF-33 (Ubiquitous):** EL SISTEMA DEBE documentar en las directivas distribuidas (`AGENTS.md` y reglas globales) la existencia de operaciones one-off de actualización del arnés: que son excepcionales, que se identifican por el ID de la spec que las define, que solo pueden ejecutarse con el rol `migrador` activado por el humano, y que son la única excepción autorizada a la inmutabilidad de specs y verificadores sellados.

## 6. Requisitos No Funcionales (Verificables)

- **013-RNF-01 (Idempotencia):** Una segunda ejecución de la operación one-off `013` sobre un proyecto ya migrado debe terminar con código `0`, cero archivos modificados y cero entradas nuevas de auditoría de modificación.
- **013-RNF-02 (Equivalencia funcional post-migración, solo este repositorio):** En este repositorio, para toda tarea verificada antes de la migración, `seb verify` posterior a la migración debe ejecutar la misma cantidad de controles, con los mismos comandos (salvo `013-RF-27`), y devolver el veredicto `APROBADO`.
- **013-RNF-03 (No regresión del motor):** La suite de tests del motor debe pasar completa. Solo se autoriza adaptar fixtures cuyo contenido codifica el formato heredado de identificadores, sin eliminar ni relajar aserciones; toda fixture adaptada debe conservar la misma cantidad de aserciones.
- **013-RNF-04 (Consistencia de distribución):** Los skills, plantillas y reglas modificados en `payload/.agents/` deben ser accesibles vía `.agents/` y distribuibles por el instalador sin pasos manuales adicionales.
- **013-RNF-05 (Sin dependencias nuevas ni red):** La implementación no debe agregar dependencias externas al motor, habilitar CGO ni requerir conectividad de red.

## 7. Casos Límite (Edge Cases)

- **013-CL-01 (Referencia cruzada en la columna `Requisito`):** Un verificador puede referenciar requisitos de otra spec existente (por ejemplo, `005-RF-03` desde la spec `013`); solo el `Check ID` está obligado a coincidir con la tarea verificada.
- **013-CL-02 (Sufijos largos o alfanuméricos):** `013-RF-100` y `013-CHK-SMOKE` son válidos.
- **013-CL-03 (Bloques de código cercados):** Filas con `-CHK-` o `CHK-` dentro de bloques de código cercados se ignoran tanto en validación como en migración.
- **013-CL-04 (Proyecto sin specs o ya migrado):** La operación one-off `013` sobre un proyecto sin artefactos o íntegramente prefijado termina con código `0` sin modificaciones.
- **013-CL-05 (Tareas en cualquier estado):** La migración incluye tareas especificadas, con plan sellado, verificadas o abortadas formalmente; para artefactos sin hash sellado solo reescribe el contenido. Las tareas purgadas no tienen artefactos y no se procesan.
- **013-CL-06 (Spec ya en formato prefijado):** Un artefacto cuyos identificadores declarados ya están prefijados no se reescribe; las menciones heredadas ilustrativas que contenga (por ejemplo, los ejemplos de formato heredado de esta misma spec) se preservan.
- **013-CL-07 (Proyecto heredado tras actualizar sin migrar):** Mientras no se ejecute la operación one-off `013`, `seb verify`, `seb plan-lock` y el hook `pre-commit` que depende de `seb verify` fallan en tareas con verificadores heredados, indicando el comando de migración (`013-RF-09`, `013-RF-30`); el comportamiento es intencional.
- **013-CL-08 (Menciones heredadas fuera de verificadores):** La regla de rechazo del motor aplica solo a verificadores; las referencias ambiguas que queden en specs, planes o artefactos documentales no bloquean `seb verify`.
- **013-CL-09 (Spec purgada referenciada):** Una referencia de requisito cuyo `<NNN>` corresponde a una spec purgada es inválida (`013-RF-12`); si el ID fue reasignado a otra spec, la referencia se considera válida por existencia y su corrección semántica recae en la revisión humana.

## 8. Fuera de Alcance (Out of Scope)

- Reescritura de identificadores heredados en código fuente, comentarios de código, nombres de tests y mensajes de commit históricos.
- Validación por parte del motor de la cobertura de requisitos (que cada `RF` tenga al menos un control).
- Prefijado de identificadores no asociados a una spec (`D-NN` en `MEMORY.md`, `UC-<dominio>_<accion>`).
- Ejecución automática de operaciones one-off por parte del instalador sin intervención del usuario.
- Ejecución de los verificadores de las tareas como parte de la migración en proyectos consumidores.
- Modificación de `docs/constitution.md`: la promoción de la excepción de operaciones one-off a la constitución corresponde a `seb-iniciador` en una tarea posterior.

## 9. Criterios de Finalización (Definition of Done)

Secuencia obligatoria de cierre de esta tarea (excepción documentada a la inmutabilidad de contratos sellados, `013-RF-33`):

1. **Planificación:** El verificador de esta spec se redacta en formato heredado (`CHK-NN`, referencias `RF-NN`) para poder sellarse con `seb plan-lock` usando el binario vigente; la propia operación one-off `013` lo convertirá a formato prefijado.
2. **Implementación:** El implementador desarrolla el motor, skills, plantillas y reglas. El implementador **no** ejecuta la operación one-off.
3. **Migración:** El humano activa el rol `migrador` y ejecuta la operación one-off `013` sobre este repositorio, incluyendo esta spec, su verificador y la corrección de `013-RF-27`.
4. **Verificación:** `seb-verificador` ejecuta `.seb/bin/seb verify identificadores_unicos_requisitos` con el verificador ya migrado (`013-CHK-*`) y todos los requisitos (`013-RF-01` a `013-RF-33` y `013-RNF-01` a `013-RNF-05`) quedan en `[PASA]`.
5. **Re-verificación histórica:** Todas las tareas previamente verificadas (`001` a `012` y `014`) vuelven a pasar `seb verify` (`013-RNF-02`).
6. Cero violaciones a `docs/constitution.md`.

## 10. Dudas Abiertas y Puntos por Clarificar

- Ninguna duda pendiente.

## Contrato de Desbloqueo (Satisfecho por Spec 014)

La capacidad de declarar múltiples casos de uso en `Afecta-UC:` fue entregada por `014_afecta_uc_multiple`, verificada y sellada en `.seb/seb.lock` con `commit_hash` `59856f8`.
