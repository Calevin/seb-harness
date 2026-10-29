# Spec 008 — Gestión Determinista de Roles de Sesión y Aislamiento de Habilidades

- **Tarea ID:** `008`
- **Identificador:** `estado_sesion_determinista`
- **Origen:** Prosa (Solicitud del usuario tras fallo en inicialización brownfield)
- **Estado:** `APROBADA`

## 1. Contexto y Objetivo (El POR QUÉ)

En flujos asistidos por agentes de inteligencia artificial dentro del arnés, los modelos necesitan inspeccionar de forma pasiva archivos del repositorio, directrices técnicas y habilidades de soporte o dominio (por ejemplo, guías de frameworks o convenciones de tecnologías preexistentes).

Actualmente, el sistema infiere el rol de gobernanza del agente analizando de forma heurística el último archivo de habilidad leído en el historial de la conversación. Esta heurística produce falsos positivos críticos: cuando un agente en fase de inicialización o especificación explora un proyecto con habilidades preexistentes ajenas al arnés, el sistema muta su rol de gobernanza a dicho nombre desconocido. Como consecuencia inmediata, el guardarraíl de menor privilegio le revoca los permisos de escritura sobre todos los archivos del espacio de trabajo (incluyendo la constitución o el código de aplicación), paralizando la operación.

El objetivo de esta tarea es desacoplar de manera estricta la identidad de gobernanza del agente de las lecturas pasivas de archivos. Se debe dotar al arnés de un mecanismo determinista y persistente para la fijación y consulta del rol activo de sesión, establecer una jerarquía de precedencia unívoca donde las variables de entorno primen sobre el estado persistido, restringir la inferencia heurística exclusivamente a las fases canónicas de gobernanza y tratar cualquier habilidad de soporte o dominio no perteneciente al arnés como parte del rol de implementación, garantizando la continuidad operativa y la prevención de bloqueos accidentales.

## 2. Actores y Usuarios

- **Agente de IA (Asistente / Agente Autónomo):** Interactúa con el entorno de trabajo ejecutando acciones y herramientas reguladas por el principio de menor privilegio.
- **Desarrollador (Operador Humano):** Conduce las fases del ciclo de vida SDD e inicializa o supervisa proyectos nuevos y heredados.
- **Interceptor de Seguridad (PreToolUse / Guardarraíl):** Evalúa de forma determinista si la acción solicitada por el agente está autorizada para el rol activo antes de permitir su ejecución.

## 3. Historias de Usuario

- **HU-01:** Como agente que opera en una fase de gobernanza del arnés (por ejemplo, inicialización o especificación), quiero que mi rol y mis permisos se mantengan inmutables aunque lea archivos de documentación o habilidades técnicas ajenas al arnés durante la exploración del repositorio, para completar mi tarea sin bloqueos espurios.
- **HU-02:** Como desarrollador o agente, quiero disponer de instrucciones explícitas para establecer, consultar y limpiar el rol activo de la sesión de trabajo, para que la asignación de permisos sea determinista y verificable sin depender de heurísticas sobre el historial de texto.
- **HU-03:** Como agente implementador que consulta habilidades técnicas de soporte (como guías de frameworks o utilitarios), quiero que el arnés reconozca mis atribuciones de implementación y me permita crear y editar código de aplicación sin restricciones indebidas.

## 4. Definiciones de Dominio

- **Roles Canónicos de Gobernanza:** Conjunto cerrado de fases oficiales definidas por el protocolo del arnés (`seb-iniciador`, `seb-especificador`, `seb-clarificador`, `seb-planificador`, `seb-verificador` e `implementador`).
- **Habilidad de Soporte o Dominio:** Toda habilidad, regla o guía técnica complementaria del proyecto que no constituye una fase de gobernanza del arnés.
- **Estado Determinista de Rol:** Registro local explícito y unívoco en el espacio de trabajo que define la fase activa de gobernanza con precedencia sobre inferencias heurísticas.
- **Contratos Protegidos:** Archivos constitutivos y normativos del arnés (la constitución del proyecto, las especificaciones de requisitos y los contratos de verificación) cuya modificación está estrictamente vedada al rol de implementación para prevenir el reward-hacking.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

- **RF-01 (Event-Driven):** CUANDO se reciba la instrucción de fijar el rol de sesión indicando un rol canónico de gobernanza válido, EL SISTEMA DEBE persistir de forma determinista el rol activo en el espacio de trabajo local y finalizar con código de salida `0`.
- **RF-02 (Unwanted Behavior):** SI se recibe la instrucción de fijar el rol de sesión con un identificador de rol no reconocido o vacío, ENTONCES EL SISTEMA DEBE rechazar la operación con código de salida `1`, emitir un mensaje de diagnóstico que liste los roles canónicos aceptados y mantener inalterado el rol previo.
- **RF-03 (Event-Driven):** CUANDO se reciba la instrucción de consultar el rol de sesión activo, EL SISTEMA DEBE emitir en la salida estándar el identificador del rol persistido, o reportar el rol por defecto (`implementador`) si no se ha establecido ningún rol explícito.
- **RF-04 (Event-Driven):** CUANDO se reciba la instrucción de limpiar o restablecer el rol de sesión activo, EL SISTEMA DEBE eliminar el registro persistido de rol, reanudar el comportamiento por defecto (`implementador`) y finalizar con código de salida `0`.
- **RF-05 (State-Driven):** MIENTRAS se evalúe el rol activo para la aplicación de guardarraíles de seguridad, EL SISTEMA DEBE resolver la precedencia estricta en el siguiente orden jerárquico descendente: primero, la variable de entorno `SEB_SKILL` si está definida y no vacía; segundo, el rol de sesión persistido en el espacio de trabajo local; tercero, la inferencia restrictiva desde la transcripción de la sesión; y cuarto, el rol por defecto (`implementador`).
- **RF-06 (State-Driven):** MIENTRAS no exista variable de entorno ni rol de sesión persistido y se requiera inferir la habilidad desde el historial de la sesión, EL SISTEMA DEBE actualizar el rol candidato únicamente si la habilidad identificada pertenece estrictamente al conjunto de roles canónicos de gobernanza del arnés (`seb-*`), ignorando toda lectura pasiva de habilidades de soporte o dominio.
- **RF-07 (State-Driven):** MIENTRAS el rol evaluado durante una operación de escritura corresponda a una habilidad de soporte o dominio ajena a los roles canónicos del arnés, EL SISTEMA DEBE otorgar los permisos equivalentes al rol de implementación (`implementador`), autorizando la edición de archivos del proyecto y bloqueando la modificación de contratos protegidos.
- **RF-08 (Event-Driven):** CUANDO se evalúe un intento de escritura sobre un contrato protegido por parte de cualquier rol distinto al explícitamente autorizado para dicho artefacto, EL SISTEMA DEBE denegar la operación y emitir un mensaje de bloqueo de seguridad.
- **RF-09 (Event-Driven):** CUANDO se ejecute exitosamente la instrucción de sellado de plan (`plan-lock`), EL SISTEMA DEBE restablecer de forma atómica el rol persistido al rol de implementación (`implementador`).

## 6. Requisitos No Funcionales (Verificables)

- **RNF-01 (Tiempo de Respuesta del Interceptor):** La resolución del rol activo y la validación de permisos en la interceptación de herramientas debe completarse en menos de 5 milisegundos.
- **RNF-02 (Operatividad Local y Fuera de Línea):** La gestión, persistencia y comprobación de roles debe funcionar de manera puramente local y offline, sin depender de servicios remotos ni conexión a redes externas.

## 7. Casos Límite (Edge Cases)

- **CL-01 (Normalización de Caracteres):** Si el identificador de rol recibido contiene espacios en blanco circundantes o letras mayúsculas, el sistema debe sanearlo y convertirlo a minúsculas antes de evaluar su validez.
- **CL-02 (Corrupción o Vacío del Registro Local de Rol):** Si el registro local de rol activo se encuentra vacío o no es accesible por permisos del sistema de archivos, el sistema debe asumir de forma segura el rol por defecto (`implementador`) sin emitir fallos no controlados.
- **CL-03 (Persistencia Idempotente):** Fijar de forma consecutiva el mismo rol canónico válido debe producir el mismo estado final y código de salida `0` sin efectos colaterales.
- **CL-04 (Ámbito y Limpieza en Ciclo de Vida):** El registro de rol persistido debe residir exclusivamente en el espacio de trabajo local (ignorado en el control de versiones) y debe ser eliminado automáticamente al finalizar exitosamente comandos terminales de ciclo de vida (`seb verify` y `seb init`).

## 8. Fuera de Alcance (Out of Scope)

- Modificación de las estructuras de persistencia de telemetría pasiva (métricas de tokens o cálculo de costos).
- Modificación de los esquemas de bloqueo de tareas cerradas ya gestionadas por el candado del plan (`ACTIVE_TASK`).

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos (`RF-01` a `RF-09`) cuentan con verificación programática en `[PASA]` ejecutada mediante `.seb/bin/seb verify estado_sesion_determinista`.
- Cero violaciones a `docs/constitution.md`.
- Cobertura completa de casos límite (`CL-01` a `CL-04`) en la suite de pruebas del motor.

## 10. Dudas Abiertas y Puntos por Clarificar

- Ninguna duda pendiente. Especificación aprobada formalmente tras la resolución de clarificaciones de precedencia estricta, ciclo de vida atómico con plan-lock y ámbito local de persistencia.
