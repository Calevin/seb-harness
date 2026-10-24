# Spec 004 — Captura Automática de Telemetría, Tokens en Caché y Saneamiento en Antigravity

- **Tarea ID:** `004`
- **Identificador:** `captura_telemetria_antigravity`
- **Origen:** Petición en prosa del usuario Sebastian: "Arreglar registro de tokens_in y tokens_out y otros campos no registrados en la tabla agent_actions de la base sqlite, use antigravity 2.0 (la app de escritorio) y antigravity CLI (agy) y en ningun caso se registran"
- **Estado:** `APROBADA`

## 1. Contexto y Objetivo (El POR QUÉ)

Aunque la especificación inicial de telemetría definió los mecanismos para almacenar duraciones, tokens y costos, en el uso real tanto desde la aplicación de escritorio Antigravity 2.0 como desde la interfaz de línea de comandos (`agy`), las acciones ejecutadas por el agente y el consumo de tokens no se registraban en la base de auditoría local ni en el proyecto principal ni en proyectos externos aprovisionados con el arnés.

Esto ocurría por dos causas estructurales:
1. El entorno de ejecución de Antigravity requiere una declaración formal de eventos de ciclo de vida y contratos estrictos de entrada/salida para invocar interceptores de herramientas y fin de turno, los cuales no estaban aprovisionados ni se distribuían a los repositorios destino.
2. El motor de Antigravity no transmite el conteo de tokens ni la habilidad activa en el flujo de entrada del evento de herramienta, sino que persiste los pasos y tokens en tiempo real dentro del registro de transcripción de la conversación, desglosando además los tokens de entrada nuevos respecto de los tokens leídos desde caché.

El objetivo de esta especificación es establecer un mecanismo determinista, portable e instalable en cualquier repositorio para interceptar cada acción del agente y cada fin de turno en Antigravity 2.0 y `agy`, inferir la habilidad activa, extraer sin duplicaciones los tokens de entrada, salida y lectura de caché desde la transcripción activa de la sesión, y sanear los parámetros registrados para preservar la privacidad del entorno local.

## 2. Actores y Usuarios

- **Motor de Ejecución del Agente (Antigravity 2.0 / `agy` CLI):** Dispara eventos del ciclo de vida antes y después de cada invocación de herramienta, así como al finalizar el turno de ejecución, y actualiza el registro de transcripción de la sesión.
- **Subsistema de Intercepción y Auditoría del Arnés:** Recibe el contexto del evento, infiere el rol activo, autoriza la ejecución según los guardarraíles, cronometra la herramienta, extrae métricas de la transcripción, sanea los argumentos y persiste el registro.
- **Desarrollador Humano:** Instala el arnés en sus proyectos y audita el consumo real de tokens (incluyendo caché), latencias, argumentos saneados y costos por tarea.

## 3. Historias de Usuario

- **HU-01:** Como desarrollador que utiliza Antigravity 2.0 o `agy` CLI, quiero que cada herramienta invocada por el agente y cada respuesta final del modelo quede registrada automáticamente con su identificador de sesión, modelo, habilidad inferida, acción, duración, resultado y estado de salida sin intervención manual.
- **HU-02:** Como desarrollador, quiero que los tokens de entrada, tokens de salida y tokens leídos en caché de cada paso del modelo se extraigan de la transcripción de la sesión y se registren una única vez por paso sin multiplicarse cuando el agente ejecuta varias herramientas en paralelo ni perderse cuando el modelo solo responde texto.
- **HU-03:** Como administrador de seguridad del entorno, quiero que los argumentos de las herramientas se registren con rutas relativas al espacio de trabajo, enmascaramiento de rutas externas y truncamiento determinista de contenidos extensos, para cumplir la prohibición constitucional de no exponer rutas privadas del host ni volcar bloques masivos de código en la auditoría.
- **HU-04:** Como usuario de múltiples repositorios, quiero que el proceso de instalación y desinstalación del arnés configure y revierta automáticamente los interceptores de ciclo de vida en cualquier proyecto destino.

## 4. Definiciones de Dominio

- **Evento de Ciclo de Vida:** Punto de intercepción provisto por el entorno del agente antes de ejecutar una herramienta (`PreToolUse`), después de completarla (`PostToolUse`) o al finalizar el bucle de ejecución del turno (`Stop`).
- **Transcripción de Sesión:** Registro secuencial de eventos mantenido por el entorno del agente donde se asientan las respuestas del planificador (`PLANNER_RESPONSE`) junto con los tokens de entrada, tokens leídos de caché, tokens de salida y llamadas a herramientas por índice de paso.
- **Índice de Paso (`step_index`):** Identificador numérico secuencial dentro de una sesión de conversación que agrupa la respuesta del modelo y la totalidad de las llamadas a herramientas emitidas en ese mismo turno.
- **Tokens en Caché (`cache_read_tokens`):** Cantidad de unidades léxicas del contexto que fueron recuperadas desde la caché del proveedor del modelo en lugar de procesarse como entrada nueva; se contabilizan como métrica de volumen y quedan excluidas del cálculo monetario de costo (`cost_usd`).
- **Inferencia de Habilidad Activa (`skill`):** Resolución determinista del rol del agente que prioriza la variable de entorno `SEB_SKILL` y, en su ausencia, inspecciona la transcripción de la sesión activa buscando la última lectura de un archivo de definición de habilidad (`.agents/skills/<nombre>/SKILL.md`), asignando `implementador` como valor por defecto si no se detecta ninguna o si la tarea activa ya se encuentra con el plan sellado.
- **Saneamiento de Parámetros:** Proceso determinista de transformación de los argumentos de una herramienta que convierte rutas absolutas dentro del proyecto en rutas relativas limpias, reemplaza el directorio de rutas absolutas externas al espacio de trabajo por el marcador `<EXTERNAL_PATH>/<nombre_base>` y recorta cadenas de texto que superen los 256 caracteres conservando los primeros 256 caracteres seguidos del sufijo `...[TRUNCATED]`.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

- **RF-01 (Event-Driven):** CUANDO el entorno del agente inicie la ejecución de una herramienta, EL SISTEMA DEBE interceptar el evento previo (`PreToolUse`), resolver la habilidad activa (vía entorno o inferencia desde la transcripción), validar los permisos del rol activo, iniciar el cronometraje unívoco de la llamada y emitir en la salida estándar el contrato JSON de decisión (`allow` o `deny`) esperado por el entorno del agente.
- **RF-02 (Event-Driven):** CUANDO el entorno del agente finalice la ejecución de una herramienta, EL SISTEMA DEBE interceptar el evento posterior (`PostToolUse`), registrar en la auditoría local el identificador de conversación como sesión, el nombre del modelo, la habilidad activa inferida, el nombre de la herramienta, los parámetros saneados, la duración en milisegundos, el resumen de resultado y el código de salida, y emitir en la salida estándar un objeto JSON vacío `{}`.
- **RF-03 (Event-Driven):** CUANDO se registre una acción asociada a un paso de conversación que disponga de archivo de transcripción accesible, EL SISTEMA DEBE extraer del paso correspondiente (`step_index`) de la transcripción los valores exactos de tokens de entrada (`tokens_in`), tokens leídos de caché (`cache_read_tokens`) y tokens de salida (`tokens_out`).
- **RF-04 (State-Driven):** MIENTRAS un mismo paso de conversación (`step_index`) dentro de una sesión contenga múltiples llamadas a herramientas, EL SISTEMA DEBE imputar los conteos de tokens (`tokens_in`, `cache_read_tokens` y `tokens_out`) y el costo calculado exclusivamente a la primera acción registrada de dicho paso, asignando cero en tokens y costo a las restantes acciones del mismo paso para evitar la duplicación contable.
- **RF-05 (Ubiquitous):** EL SISTEMA DEBE almacenar de forma diferenciada la cantidad de tokens leídos de caché (`cache_read_tokens`) en el registro de acciones de auditoría, excluir dichos tokens en caché del cálculo monetario (`cost_usd`) y exponer su sumatoria acumulada por tarea en el reporte consolidado de observabilidad.
- **RF-06 (Event-Driven):** CUANDO el sistema registre los parámetros de invocación de una herramienta, EL SISTEMA DEBE sanear la carga útil reemplazando toda ruta absoluta perteneciente al espacio de trabajo por su ruta relativa limpia y reemplazando cualquier ruta absoluta del sistema anfitrión externa al espacio de trabajo por el formato `<EXTERNAL_PATH>/<nombre_base>`.
- **RF-07 (Event-Driven):** CUANDO los parámetros de una herramienta contengan valores de texto que superen los 256 caracteres, EL SISTEMA DEBE truncar dichos valores conservando exactamente los primeros 256 caracteres seguidos del marcador `...[TRUNCATED]` antes de persistirlos en la auditoría.
- **RF-08 (Unwanted Behavior):** SI el evento de finalización de una herramienta reporta un mensaje de fallo o error de ejecución, ENTONCES EL SISTEMA DEBE registrar un código de salida distinto de cero y almacenar el motivo del fallo saneado en el resumen de resultado de la acción.
- **RF-09 (Unwanted Behavior):** SI el archivo de transcripción indicado en el evento no existe, no es legible o aún no contiene las métricas del paso actual, ENTONCES EL SISTEMA DEBE registrar la acción de la herramienta con valores de tokens en cero sin bloquear ni abortar el ciclo de ejecución del agente.
- **RF-10 (Event-Driven):** CUANDO el entorno del agente dispare el evento de fin de turno (`Stop`), EL SISTEMA DEBE inspeccionar la transcripción de la sesión activa y registrar como acción `model_response` cualquier paso del modelo (`PLANNER_RESPONSE`) cuyos tokens aún no hayan sido contabilizados en la sesión (tales como turnos de respuesta en texto sin invocación de herramientas), y emitir en la salida estándar un objeto JSON válido que permita finalizar el turno.
- **RF-11 (State-Driven):** MIENTRAS no exista la variable de entorno `SEB_SKILL` definida explícitamente durante un evento de ciclo de vida, EL SISTEMA DEBE inferir la habilidad activa identificando el último archivo `.agents/skills/<nombre>/SKILL.md` leído en la transcripción de la sesión, o asignar `implementador` si no se leyó ninguna habilidad o si existe una tarea activa con el plan sellado.
- **RF-12 (Event-Driven):** CUANDO se ejecute el proceso de instalación del arnés en un proyecto destino o en modo propio, EL SISTEMA DEBE aprovisionar el archivo declarativo `.agents/hooks.json` junto con los scripts interceptores en el directorio de agentes del proyecto, y CUANDO se ejecute el proceso de desinstalación, EL SISTEMA DEBE remover dicho archivo `.agents/hooks.json` y los interceptores del arnés respetando las reglas de preservación de componentes personalizados del usuario.
- **RF-13 (Ubiquitous):** EL SISTEMA DEBE migrar de forma automática e idempotente la estructura de la base de datos de auditoría local existente para incorporar la columna `cache_read_tokens` con valor por defecto `0` sin pérdida de los registros históricos previos.

## 6. Requisitos No Funcionales (Verificables)

- **RNF-01 (Compatibilidad de Superficies):** La captura de eventos y lectura de transcripciones debe operar sin cambios de configuración tanto en Antigravity 2.0 (aplicación de escritorio) como en Antigravity CLI (`agy`).
- **RNF-02 (Privacidad y Seguridad Constitucional):** Ningún registro persistido en la base de auditoría local puede contener rutas absolutas del directorio personal del usuario fuera del espacio de trabajo ni credenciales en texto plano.
- **RNF-03 (Rendimiento del Interceptor):** El procesamiento completo del evento posterior a la herramienta o de cierre de turno debe finalizar en menos de 100 milisegundos para no degradar la fluidez del agente.

## 7. Casos Límite (Edge Cases)

- **CL-01 (Bases de datos preexistentes sin columna de caché):** Si un proyecto ya cuenta con una base de auditoría creada por una versión anterior del arnés, la apertura de la base debe agregar la columna `cache_read_tokens` con valor por defecto `0` de forma transparente e idempotente.
- **CL-02 (Herramientas invocadas en paralelo en el mismo turno):** Si el modelo emite 5 llamadas a herramientas en el mismo `step_index`, las 5 llamadas quedan registradas con sus respectivos nombres, parámetros saneados y duraciones individuales, pero los tokens (`tokens_in`, `cache_read_tokens`, `tokens_out`) se registran solo en la primera y las 4 restantes quedan con `0`, garantizando que la suma de la sesión sea exacta.
- **CL-03 (Herramientas de escritura de archivos grandes):** Cuando el agente invoca herramientas de creación o edición de archivos enviando cientos de líneas de código como argumento, el registro de parámetros solo conserva la ruta relativa y el contenido truncado a 256 caracteres con el sufijo `...[TRUNCATED]`.
- **CL-04 (Turnos de conversación sin uso de herramientas):** Cuando el agente responde directamente al usuario sin invocar herramientas (o en el paso final de cierre de turno), el interceptor de fin de turno (`Stop`) registra una fila `model_response` con los tokens de ese paso, evitando pérdidas contables.

## 8. Fuera de Alcance (Out of Scope)

- Intercepción de tráfico de red HTTPS mediante proxies o certificados autofirmados para inspeccionar paquetes crudos hacia las APIs de Google o terceros.
- Imputación monetaria de los tokens leídos en caché (`cache_read_tokens`) dentro de `cost_usd`.

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos funcionales (`RF-01` a `RF-13`) cuentan con verificación programática en estado `[PASA]` ejecutada mediante `.seb/bin/seb verify 004_captura_telemetria_antigravity`.
- Cero violaciones a `docs/constitution.md`.
- Verificación de que la instalación en proyectos destino copia `.agents/hooks.json` y su desinstalación lo limpia adecuadamente.

## 10. Dudas Abiertas y Puntos por Clarificar

- Ninguna duda pendiente. Especificación auditada y aprobada por `seb-clarificador` tras incorporar las decisiones de dominio del usuario.
