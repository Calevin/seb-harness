# Spec 005 — Gestión de Grafo de Dependencias (DAG), Guardarraíl de Planificación y Protocolo de Suspensión en Clarificación

- **Tarea ID:** `005`
- **Identificador:** `gestion_grafo_dependencias`
- **Origen:** Petición en prosa del usuario Sebastian para evolucionar el arnés incorporando gestión formal de dependencias (DAG), guardarraíl en `plan-lock` y protocolo de bifurcación de alcance y suspensión en `seb-clarificador`.
- **Estado:** `APROBADA`
- **Dependencias:** `002_prefijo_secuencial_artefactos`, `004_captura_telemetria_antigravity`

## 1. Contexto y Objetivo (El POR QUÉ)

El arnés `seb-harness` operaba originalmente bajo la premisa de una secuencia lineal y atómica de tareas (`001`, `002`, `003`...). Sin embargo, en el desarrollo real de software, las funcionalidades no surgen en un orden estrictamente lineal sino que componen un Grafo Acíclico Dirigido (DAG).

Durante la fase de clarificación de requisitos (`seb-clarificador`), el análisis sistemático de casos borde y ambigüedades descubre frecuentemente brechas de capacidad (Capability Gaps): la funcionalidad bajo análisis requiere la existencia de contratos, entidades, servicios o esquemas de persistencia que aún no existen en el sistema (por ejemplo, especificar un filtro sobre una entidad cuyos datos están hardcodeados y carece de ciclo de vida o CRUD).

En el estado actual del arnés:
1. No existe un mecanismo formal en la metadata de las especificaciones para declarar dependencias previas entre tareas (`Depends-On`).
2. El comando `seb plan-lock` no comprueba si las dependencias requeridas por una especificación ya fueron implementadas y verificadas deterministamente antes de permitir el sellado criptográfico del plan técnico.
3. El archivo `.seb/seb.lock` solo registra los hashes de spec y verificador durante `plan-lock`, careciendo de un indicador persistido en Git que certifique si la tarea completó su verificación determinista (`seb verify`).
4. El skill `seb-clarificador` carece de un protocolo estandarizado para gestionar el descubrimiento de dependencias faltantes, lo que genera el riesgo de acoplar alcances masivos (Scope Creep) dentro de una única spec o provocar bloqueos no guiados.

El objetivo de esta especificación es dotar a `seb-harness` de capacidades nativas para declarar dependencias directas entre tareas, registrar en `.seb/seb.lock` el estado de verificación exitosa y el hash de commit Git, validar computacionalmente en tiempo de planificación (`plan-lock`) que los prerrequisitos directos se encuentren cumplidos, y protocolizar en `seb-clarificador` la detección de brechas funcionales mediante una bifurcación de alcance (Scope Fork) asistida, permitiendo suspender formalmente una tarea (`BLOQUEADA_POR_DEPENDENCIA`) con un contrato de desbloqueo conservado históricamente o registrar deudas técnicas en `MEMORY.md` si se opta por reducir el alcance.

## 2. Actores y Usuarios

- **Desarrollador Humano:** Define requerimientos, responde preguntas de clarificación, selecciona caminos de bifurcación de alcance y ejecuta la verificación global.
- **Agente Especificador (`seb-especificador`):** Incorpora el campo declarativo de dependencias en los borradores de especificación.
- **Agente Clarificador (`seb-clarificador`):** Detecta brechas de capacidades durante la auditoría de la spec, formula bifurcaciones de alcance, suspende tareas dependientes cuando corresponde, genera contratos de desbloqueo y audita deudas técnicas en `MEMORY.md`.
- **Motor del Arnés (`.seb/bin/seb`):** Ejecutable en Go que valida deterministamente la integridad de dependencias directas, previene ciclos, bloquea planes con prerrequisitos no verificados y estampa `verified: true` junto al `commit_hash` en `.seb/seb.lock` al superar las pruebas.
- **Agente Planificador (`seb-planificador`):** Genera planes técnicos y verificadores exclusivamente sobre especificaciones aprobadas con todas sus dependencias directas satisfechas.

## 3. Historias de Usuario

- **HU-01:** Como desarrollador que redacta o refina especificaciones, quiero declarar explícitamente en la metadata de la spec las tareas de las cuales depende directamente, para que el sistema tenga visibilidad del grafo de requisitos.
- **HU-02:** Como desarrollador o agente de ingeniería, quiero que `seb plan-lock` verifique de forma determinista y offline que todas las dependencias directas declaradas posean estado `verified: true` y un hash de commit válido en `seb.lock`, impidiendo planificar sobre cimientos no verificados.
- **HU-03:** Como desarrollador, durante la fase de clarificación, quiero que el agente identifique cuando una respuesta revela una brecha de infraestructura o dominio y me ofrezca una bifurcación de alcance formal (reducir a mock o suspender y crear spec base), evitando inflar artificialmente la tarea actual.
- **HU-04:** Como desarrollador, si elijo suspender una tarea bloqueada por dependencias, quiero que el arnés registre su estado formal, redacte un contrato de desbloqueo y me entregue el comando y prompt sugeridos para iniciar de inmediato la tarea previa.
- **HU-05:** Como desarrollador, si elijo reducir el alcance con mocks para avanzar el MVP, quiero que la capacidad postergada quede documentada en la spec y asentada en `MEMORY.md` como deuda técnica para no perderla de vista.
- **HU-06:** Como usuario del arnés, quiero que `seb status` muestre claramente las dependencias directas de cada tarea y alerte si alguna se encuentra en estado insatisfecho o bloqueada.

## 4. Definiciones de Dominio

- **Grafo Acíclico Dirigido (DAG) de Tareas:** Red topológica de especificaciones donde las dependencias representan relaciones de precedencia directa obligatoria (la tarea A debe estar verificada antes de que la tarea B pueda planificarse).
- **Validación Directa de Dependencias:** Estrategia de comprobación que evalúa exclusivamente los prerrequisitos inmediatos declarados en la spec actual, asumiendo transitividad por construcción secuencial.
- **Metadata `Depends-On`:** Campo formal en la cabecera de la especificación que enumera los identificadores numéricos o nombres canónicos de las tareas que constituyen prerrequisitos directos ineludibles.
- **Tarea Resuelta / Cumplida:** Tarea registrada en `.seb/seb.lock` con hashes de spec y verificador válidos, estado booleano `verified: true` y un hash de commit Git (`commit_hash`) persistido por `.seb/bin/seb verify` al completar con éxito el 100% de sus verificaciones deterministas.
- **Brecha de Capacidad (Capability Gap):** Discrepancia detectada durante la clarificación entre lo que la especificación requiere y lo que el repositorio actualmente soporta o expone.
- **Bifurcación de Alcance (Scope Fork):** Punto de decisión formal presentado al humano con dos alternativas excluyentes: acotar el alcance actual mediante datos sintéticos/mocks para mantener la atomicidad, o pausar la tarea actual y extraer el prerrequisito a una nueva especificación independiente.
- **Estado `BLOQUEADA_POR_DEPENDENCIA`:** Estado operativo de una especificación cuya clarificación ha identificado dependencias no resueltas que impiden su aprobación y posterior planificación.
- **Contrato de Desbloqueo Histórico:** Sección documental que define las capacidades mínimas requeridas durante la suspensión y que, al reanudarse tras completarse la tarea base, se conserva bajo el título `Contrato de Desbloqueo (Satisfecho por Spec XXX)` preservando la trazabilidad del desacople.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

### Declaración de Dependencias en la Spec
- **RF-01 (Ubiquitous):** EL SISTEMA DEBE soportar en la cabecera de toda especificación (`specs/<NNN>_<nombre>_spec.md`) el campo de metadata `Dependencias:` o `Depends-On:` aceptando una lista de identificadores numéricos (`001`) o claves canónicas (`001_nombre_tarea`) separados por comas.
- **RF-02 (State-Driven):** MIENTRAS una especificación no posea dependencias funcionales previas, EL SISTEMA DEBE admitir el valor `Ninguna` o la ausencia del campo sin alterar el comportamiento estándar de la tarea.

### Registro de Verificación Determinista en Candado
- **RF-03 (Event-Driven):** CUANDO el comando `.seb/bin/seb verify <tarea>` culmine con el 100% de los controles en estado `[PASA]`, EL SISTEMA DEBE actualizar la entrada correspondiente a la tarea en `.seb/seb.lock` fijando el campo `verified` en `true` y registrando el hash del commit actual de Git (`commit_hash`), asegurando que cualquier clon del repositorio pueda verificar la satisfacción del prerrequisito de forma offline.

### Guardarraíl Determinista en `seb plan-lock`
- **RF-04 (Event-Driven):** CUANDO se ejecute el comando `.seb/bin/seb plan-lock <tarea>`, EL SISTEMA DEBE parsear la metadata de la especificación objetivo y extraer la totalidad de las dependencias directas declaradas.
- **RF-05 (Event-Driven):** CUANDO existan dependencias directas declaradas en la especificación, EL SISTEMA DEBE comprobar que cada tarea identificada figure en `.seb/seb.lock` con estado `verified: true` y registro no vacío de `commit_hash`.
- **RF-06 (Unwanted Behavior):** SI al menos una de las dependencias directas declaradas en la especificación no existe en el repositorio o no posee `verified: true` con su respectivo `commit_hash` en `.seb/seb.lock`, ENTONCES EL SISTEMA DEBE abortar la ejecución de `seb plan-lock` con código de salida `1`, registrar el bloqueo en la auditoría y emitir un mensaje de error detallando la lista de dependencias directas insatisfechas.
- **RF-07 (Unwanted Behavior):** SI el análisis de dependencias directas detecta una referencia circular mutua (por ejemplo, la tarea A declara depender de B y la tarea B declara depender de A), ENTONCES EL SISTEMA DEBE abortar inmediatamente la operación `plan-lock` con código de salida `1` y mostrar la relación circular detectada.

### Protocolo de Clarificación, Bifurcación de Alcance y Desbloqueo
- **RF-08 (Event-Driven):** CUANDO el agente `seb-clarificador` reciba respuestas del desarrollador que evidencien la ausencia de un modelo de datos, servicio, contrato o persistencia requerida por la funcionalidad bajo análisis, EL SISTEMA DEBE formular una Bifurcación de Alcance (Scope Fork) presentando formalmente dos opciones: reducción de alcance mediante datos estáticos o desacople mediante especificación previa.
- **RF-09 (Event-Driven):** CUANDO el desarrollador opte por reducir el alcance mediante datos sintéticos o mocks en lugar de suspender la tarea, EL SISTEMA DEBE registrar la funcionalidad postergada en la sección `## 8. Fuera de Alcance` de la spec activa y añadir una entrada estructurada en `MEMORY.md` bajo la sección `4. Riesgos Conocidos y Deuda Técnica Activa` mediante el comando `.seb/bin/seb memory-add`.
- **RF-10 (Event-Driven):** CUANDO el usuario confirme el desacople de la funcionalidad previa, EL SISTEMA DEBE actualizar el encabezado de la especificación dependiente asignando el estado `BLOQUEADA_POR_DEPENDENCIA` y vinculando la descripción de la tarea bloqueante.
- **RF-11 (Event-Driven):** CUANDO una especificación pase al estado `BLOQUEADA_POR_DEPENDENCIA`, EL SISTEMA DEBE redactar en el archivo de la spec una sección titulada `## Contrato de Desbloqueo Requerido` con los requisitos funcionales mínimos que la tarea base debe implementar.
- **RF-12 (Event-Driven):** CUANDO se complete la suspensión de una especificación por dependencia, EL SISTEMA DEBE emitir en la salida estándar el comando exacto `.seb/bin/seb spec-next <nombre_tarea_base>` y una plantilla de prompt recomendada para iniciar de inmediato el agente especificador sobre la tarea previa.
- **RF-13 (State-Driven):** MIENTRAS una especificación permanezca en estado `BLOQUEADA_POR_DEPENDENCIA` o contenga una sección de `Contrato de Desbloqueo Requerido` no resuelta, EL SISTEMA DEBE impedir que `seb-clarificador` cambie su estado a `APROBADA`.
- **RF-14 (Event-Driven):** CUANDO una especificación previamente bloqueada sea reanudada tras verificarse exitosamente su tarea base, EL SISTEMA DEBE renombrar la sección documental a `## Contrato de Desbloqueo (Satisfecho por Spec XXX)` registrando la tarea que la satisfizo y preservando el rastro histórico de la decisión.

### Observabilidad y Visibilidad del Grafo
- **RF-15 (Event-Driven):** CUANDO se invoque `.seb/bin/seb status`, EL SISTEMA DEBE incluir en la tabla resumen una columna o detalle que explicite las dependencias directas declaradas de cada tarea y alerte si alguna se encuentra insatisfecha o bloqueada.
- **RF-16 (Ubiquitous):** EL SISTEMA DEBE permitir registrar mediante `.seb/bin/seb memory-add` las decisiones de bloqueo y suspensión de tareas, asegurando trazabilidad persistente en `MEMORY.md`.

## 6. Requisitos No Funcionales (Verificables)

- **RNF-01 (Aislamiento y Rendimiento del Guardarraíl):** La comprobación de dependencias directas y sellos en `seb plan-lock` debe completarse en menos de 50 milisegundos mediante evaluación computacional en Go puro, sin requerir invocaciones a modelos de lenguaje ni accesos a red.
- **RNF-02 (Compatibilidad Retrospectiva):** La totalidad de las especificaciones existentes (`001` a `004`) deben permanecer válidas y funcionales sin requerir modificaciones obligatorias en sus cabeceras.
- **RNF-03 (Resiliencia Multi-Máquina):** La comprobación de dependencias en `plan-lock` debe operar de forma totalmente autónoma en un clon limpio de Git basándose exclusivamente en los archivos versionados (`specs/` y `.seb/seb.lock`), garantizando funcionamiento determinista aun si la base SQLite local (`.seb/seb-audit.db`) se encuentra vacía o recién inicializada.

## 7. Casos Límite (Edge Cases)

- **CL-01 (Identificador ambiguo o parcial en dependencia):** Si la metadata declara una dependencia como número simple (`003`) o nombre parcial (`desinstalacion`), el resolvedor del motor Go debe normalizarla contra el catálogo de tareas existente y rechazarla con error explícito si coincide con más de una tarea o si no encuentra ninguna.
- **CL-02 (Dependencia huérfana):** Si una spec declara `Depends-On: 099_inexistente`, el comando `plan-lock` debe reportar que la tarea prerrequisito ni siquiera existe en el directorio `specs/`.
- **CL-03 (Dependencia presente pero no verificada):** Si la tarea previa existe en `specs/` e incluso figura en `seb.lock` por haber sido planificada, pero su campo `verified` es `false` o no contiene `commit_hash`, `plan-lock` sobre la tarea dependiente debe abortar reportando que la tarea base no ha sido verificada.
- **CL-04 (Dependencia circular directa):** Si la tarea A declara depender de B y la tarea B declara depender de A, el motor debe detectar la referencia circular mutua y abortar `plan-lock`.
- **CL-05 (Conservación documental tras desbloqueo):** Cuando la tarea base finalmente pasa su verificación determinista (`[PASA]`), la tarea dependiente puede reanudar su clarificación, conservando la sección renombrada `## Contrato de Desbloqueo (Satisfecho por Spec XXX)` y proceder a `plan-lock` sin pérdida de contexto.

## 8. Fuera de Alcance (Out of Scope)

- Paralelización automática o agendamiento concurrente de agentes para implementar múltiples ramas del DAG en simultáneo.
- Detección automática de dependencias mediante análisis estático de código fuente o inspección de AST en tiempo de clarificación (la detección se basa en el diálogo y las respuestas funcionales del usuario).
- Evaluación recursiva profunda de dependencias transitivas (se delega en la validación directa secuencial).

## 9. Criterios de Finalización (Definition of Done)

- Estructura `.seb/seb.lock` extendida para soportar `verified` (bool) y `commit_hash` (string).
- Comandos del motor Go (`plan-lock`, `verify`, `status`) actualizados con validación y estampado de dependencias directas, acompañados de tests unitarios que cubran dependencias satisfechas, insatisfechas, huérfanas y circulares.
- Habilidades `seb-clarificador` y `seb-planificador` en `payload/.agents/skills/` actualizadas con las instrucciones operativas del protocolo DAG, bifurcación de alcance y conservación documental.
- 100% de los controles en `specs/005_gestion_grafo_dependencias_verificador.md` evaluados con resultado `[PASA]` vía `.seb/bin/seb verify 005`.
- Cero violaciones a `docs/constitution.md` y memoria técnica actualizada en `MEMORY.md`.

## 10. Dudas Abiertas y Puntos por Clarificar

- Ninguna duda pendiente. Especificación formalmente auditada y aprobada por `seb-clarificador` tras incorporar las siguientes decisiones de arquitectura y dominio del usuario:
  1. **Persistencia Multi-Máquina:** Extensión de `.seb/seb.lock` con campos `verified: true` y `commit_hash` actualizados por `.seb/bin/seb verify` al completar exitosamente las validaciones.
  2. **Validación Directa:** Evaluación de dependencias directas de primer nivel sin sobrecarga de resolución transitiva recursiva.
  3. **Conservación Histórica:** Preservación de la sección documental bajo el título `Contrato de Desbloqueo (Satisfecho por Spec XXX)` al reanudar especificaciones previamente suspendidas.
  4. **Deuda Técnica en Scope Fork:** Registro mandatorio en `## 8. Fuera de Alcance` y en `MEMORY.md` (`4. Riesgos Conocidos y Deuda Técnica Activa`) cuando el desarrollador elige acotar el alcance mediante datos sintéticos o mocks.
