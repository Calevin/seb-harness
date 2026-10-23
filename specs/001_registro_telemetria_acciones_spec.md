# Spec 001 — Registro Fiable de Telemetría, Duración y Tokens de Acciones

- **Tarea ID:** `001`
- **Identificador:** `registro_telemetria_acciones`
- **Origen:** Petición en prosa (Auditoría con valores en cero en telemetría local de acciones)
- **Estado:** `APROBADA`

## 1. Contexto y Objetivo (El POR QUÉ)

El arnés cuenta con un subsistema de auditoría para supervisar la actividad de agentes autónomos, pero actualmente registra valores en cero para la duración de las operaciones, los tokens consumidos y los costos económicos. Esto impide conocer el tiempo real invertido en cada fase, imposibilita el control presupuestario de las consultas a modelos de lenguaje y genera registros huérfanos sin vinculación a la tarea activa. El objetivo de esta tarea es garantizar una captura exhaustiva, determinista y precisa del tiempo de ejecución, la vinculación contextual con la tarea activa y el consumo de tokens en todas las acciones del sistema.

## 2. Actores y Usuarios

- **Agente de IA (Constructor / Planificador / Verificador):** Ejecuta herramientas de inspección, modificación de archivos y comandos de terminal bajo la supervisión del arnés.
- **Desarrollador Humano:** Consulta los reportes de observabilidad para evaluar el desempeño, la latencia por tarea y el costo financiero acumulado.
- **Subproceso de Control de Versiones (Hooks):** Notifica eventos del ciclo de vida del repositorio al registro de auditoría.

## 3. Historias de Usuario

- **HU-01:** Como desarrollador, quiero que cada comando del arnés y cada uso de herramienta registre su duración real en milisegundos para identificar cuellos de botella en el flujo de trabajo.
- **HU-02:** Como desarrollador, quiero que el consumo de tokens de entrada y salida sea contabilizado fielmente por tarea para mantener visibilidad de los costos operativos del proyecto.
- **HU-03:** Como desarrollador, quiero que los eventos automáticos del repositorio y del arnés queden asociados a la tarea activa sin requerir parámetros manuales redundantes.

## 4. Definiciones de Dominio

- **Acción:** Operación atómica ejecutada en el espacio de trabajo, que puede ser la invocación de una herramienta por un agente, un comando interno del arnés o un evento de control de versiones.
- **Duración:** Tiempo transcurrido en milisegundos entre el inicio efectivo de una acción y la emisión de su resultado final.
- **Tokens de Entrada (`tokens_in`):** Cantidad de unidades léxicas enviadas al modelo de lenguaje en el contexto de la interacción.
- **Tokens de Salida (`tokens_out`):** Cantidad de unidades léxicas generadas por el modelo de lenguaje como respuesta.
- **Tarea Activa:** Identificador de la especificación funcional actualmente seleccionada para planificación o implementación en el entorno de trabajo.
- **Cascada de Tarea Activa:** Resolución secuencial determinista del identificador de tarea: 1) parámetro explícito de invocación, 2) variable de entorno `SEB_TASK_ID`, 3) puntero en archivo `.seb/ACTIVE_TASK`, 4) última tarea activa en la base de datos de auditoría local, 5) categoría `global`.
- **Ingesta Dual de Métricas:** Capacidad de capturar telemetría tanto atómica en tiempo real (por llamada individual a herramienta) como agregada por sesión (mediante archivo estructurado `usage.jsonl` generado por bucles autónomos de agentes).

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

- **RF-01 (Event-Driven):** CUANDO se ejecuta y finaliza una acción de herramienta o comando interno del arnés, EL SISTEMA DEBE calcular y registrar la duración de ejecución con una resolución mínima en milisegundos enteros.
- **RF-02 (Event-Driven):** CUANDO un agente o proceso ejecutor suministra métricas de uso de modelo de lenguaje (ya sea por invocación atómica de herramienta o por ingesta de archivo estructurado `usage.jsonl`), EL SISTEMA DEBE registrar la cantidad exacta de tokens de entrada (`tokens_in`) y tokens de salida (`tokens_out`) asociados a la acción o turno.
- **RF-03 (Event-Driven):** CUANDO se registran tokens consumidos y se proporciona un costo explícito en el evento, EL SISTEMA DEBE persistir dicho valor; SI no se proporciona costo explícito pero se identifican los tokens y el modelo, EL SISTEMA DEBE calcular el costo mediante la tabla de tarifas local (`.seb/config.json`); SI no existe tarifa configurada, EL SISTEMA DEBE persistir el costo como 0.0 en dólares estadounidenses.
- **RF-04 (Event-Driven):** CUANDO se invoca un comando interno del arnés de forma autónoma, EL SISTEMA DEBE cronometrar su propia ejecución interna y persistir dicha duración como el tiempo de la acción.
- **RF-05 (State-Driven):** MIENTRAS se registre cualquier acción o comando sin un identificador de tarea explícito, EL SISTEMA DEBE resolver la tarea activa siguiendo estrictamente la Cascada de Tarea Activa: 1) variable de entorno `SEB_TASK_ID`, 2) archivo de estado `.seb/ACTIVE_TASK`, 3) última tarea activa en la base de auditoría local, o 4) asignar la categoría `global`.
- **RF-06 (Event-Driven):** CUANDO un agente genera un archivo estructurado `usage.jsonl` con métricas acumuladas (turnos, tokens de entrada y salida, resultado), EL SISTEMA DEBE proveer un mecanismo determinista para importar o sincronizar dichas métricas en la base de datos de auditoría vinculándolas a la tarea activa.
- **RF-07 (Unwanted Behavior):** SI una medición de duración arroja un valor negativo o inconsistente por desfase de reloj, ENTONCES EL SISTEMA DEBE normalizar el valor a cero y registrar la acción sin interrumpir el flujo del usuario.
- **RF-08 (Unwanted Behavior):** SI se registra una acción sin tarea activa existente en el entorno tras evaluar toda la cascada de resolución, ENTONCES EL SISTEMA DEBE asignar la categoría global a la acción en lugar de abortar la operación.
- **RF-09 (Ubiquitous):** EL SISTEMA DEBE proveer un resumen consolidado de auditoría donde se totalicen de forma determinista la cantidad de acciones, la latencia total acumulada, los tokens de entrada, los tokens de salida y el costo total agrupados por tarea.

## 6. Requisitos No Funcionales (Verificables)

- **RNF-01:** La persistencia y el cronometraje de cualquier acción individual deben completarse con una latencia añadida inferior a diez milisegundos.
- **RNF-02:** El registro y cálculo de telemetría debe funcionar de forma autónoma y determinista sin requerir conectividad a servicios de red externos.
- **RNF-03:** El sistema debe aceptar datos de tokens provistos bajo convenciones estándar alternativas (`input_tokens`/`output_tokens` y `prompt_tokens`/`completion_tokens`).

## 7. Casos Límite (Edge Cases)

- **CL-01 (Acciones sub-milisegundo):** Acciones cuya ejecución tome menos de un milisegundo deben registrarse con duración cero milisegundos sin generar errores de redondeo.
- **CL-02 (Operaciones sin consumo de modelo de lenguaje):** Acciones que no interactúan con modelos de lenguaje (como comandos de control de versiones o verificaciones estáticas) deben registrar cero en tokens y costo, garantizando la consistencia numérica de las sumatorias.
- **CL-03 (Concurrencia de acciones):** La ejecución concurrente de múltiples herramientas en una misma sesión no debe cruzar ni corromper los temporizadores individuales de cada acción.
- **CL-04 (Ausencia de archivo o tabla de precios):** Si el archivo `.seb/config.json` no existe o no contiene la tarifa del modelo reportado, el costo acumulado se registrará en 0.0 sin abortar el registro de la acción ni la contabilidad de tokens.
- **CL-05 (Importación idempotente de `usage.jsonl`):** Si se solicita la sincronización de un archivo `usage.jsonl` más de una vez para la misma sesión o marca temporal, el sistema no debe duplicar las filas ni las sumatorias de tokens.

## 8. Fuera de Alcance (Out of Scope)

- Estimación predictiva o heurística de tokens mediante conteo aproximado de palabras si el modelo no suministra la cifra real.
- Facturación real, procesamiento de pagos con tarjetas o comunicación con pasarelas financieras externas.

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos (`RF-01` a `RF-09`) cuentan con verificación programática en `[PASA]` ejecutada mediante `.seb/bin/seb verify registro_telemetria_acciones`.
- Cero violaciones a `docs/constitution.md`.
- Verificación de que las exportaciones de datos de auditoría contengan duraciones mayores a cero para comandos ejecutados y conteos válidos de tokens cuando sean provistos.

## 10. Dudas Abiertas y Puntos por Clarificar

- Ninguna duda pendiente. Especificación auditada y aprobada por `seb-clarificador` tras incorporar las decisiones de dominio del usuario.
