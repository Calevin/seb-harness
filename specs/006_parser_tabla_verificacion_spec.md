# Spec 006 — Parser Robusto de Tablas de Verificación y Prevención de Omisiones Silenciosas

- **Tarea ID:** `006`
- **Identificador:** `parser_tabla_verificacion`
- **Origen:** Reporte de fallo en proyecto consumidor sobre `seb` (`v01a0024-dirty`), donde filas con requisitos no funcionales o globales (`006-RNF-02`, `GLOBAL`) fueron ignoradas silenciosamente por el analizador de expresiones regulares.
- **Estado:** `APROBADA`
- **Dependencias:** `002_prefijo_secuencial_artefactos`, `005_gestion_grafo_dependencias`

## 1. Contexto y Objetivo (El POR QUÉ)

El núcleo de la gobernanza de `seb-harness` reside en la evaluación determinista mediante la matriz de controles definida en `specs/<NNN>_<tarea>_verificador.md`. Esta matriz es parseada y ejecutada por los comandos `seb verify` y `seb plan-lock` para asegurar que el 100% de los requisitos cuenten con validación automatizada (`exit code 0`).

En versiones previas del arnés, el parser de la tabla de verificación empleaba una expresión regular restringida que exigía que la columna de requisitos comenzara exclusivamente con el prefijo `RF-`. Esto provocó un fallo crítico de evasión de pruebas (*reward-hacking* involuntario): filas legítimas como `006-CHK-09 (006-RNF-02)` o `006-CHK-99 (GLOBAL)` fueron omitidas silenciosamente sin emitir advertencias ni errores. El sistema reportó un veredicto erróneo de "APROBADO" sobre un subconjunto parcial de pruebas, dejando requisitos no funcionales, análisis estático y suites integrales sin ejecutar.

Adicionalmente, el parser actual omite de forma silenciosa cualquier línea que sufra desajustes de formato (por ejemplo, espacios tras comas en listas de requisitos `006-RNF-01, 006-RNF-02` o errores tipográficos en los delimitadores de Markdown). En un arnés de aseguramiento determinista, ignorar silenciosamente una fila que declara una comprobación (`CHK-XX`) invalida por completo el contrato de verificación.

El objetivo de esta especificación es redefinir el comportamiento del parser de matrices de verificación para:
1. Reconocer de manera determinista identificadores de requisitos heterogéneos (`RF-XX`, `RNF-XX`, `CL-XX`, `GLOBAL`, nombres de categoría libre y listas compuestas con o sin espacios).
2. Erradicar las omisiones silenciosas, convirtiendo cualquier fila candidata a control (`CHK-XX`) con formato sintáctico defectuoso o identificadores duplicados en un error explícito y bloqueante de ejecución.
3. Asegurar que tanto en la fase de sellado (`plan-lock`) como en la de verificación (`verify`), la totalidad de los controles declarados en el archivo sean contabilizados y auditados sin exclusiones arbitrarias ni fallos por operadores de shell en los comandos.

## 2. Actores y Usuarios

- **Desarrollador Humano:** Diseña tablas de verificación en `specs/*_verificador.md` combinando requisitos funcionales, no funcionales, controles globales y análisis estático.
- **Agente Planificador (`seb-planificador`):** Genera la matriz de controles durante la Fase 3 del arnés y sella el plan mediante `seb plan-lock`.
- **Agente Verificador (`seb-verificador`):** Invoca `seb verify <tarea>` para evaluar los controles programáticos y emitir el veredicto final.
- **Motor del Arnés (`.seb/bin/seb`):** Ejecutable del CLI encargado de analizar sintácticamente las tablas Markdown, ejecutar los comandos asociados y registrar la telemetría en SQLite y `seb.lock`.

## 3. Historias de Usuario

- **006-HU-01:** Como desarrollador o agente de testing, quiero declarar verificaciones asociadas a requisitos no funcionales (`006-RNF-01`), casos límite (`006-CL-01`), suites globales (`GLOBAL`) o listas combinadas (`006-RF-01, 006-RNF-02`), para que `seb verify` ejecute todos los controles sin omitir ninguno.
- **006-HU-02:** Como auditor de calidad, quiero que `seb verify` y `seb plan-lock` fallen inmediatamente con un mensaje de diagnóstico claro si una fila con identificador `CHK-XX` presenta errores sintácticos, comandos mal delimitados o duplicados, evitando que errores de formato encubran pruebas no ejecutadas.
- **006-HU-03:** Como desarrollador que redacta comandos de prueba complejos con pipes `|` o encadenamientos `||`, quiero que el parser extraiga el comando íntegro entre backticks sin confundir los pipes del comando con delimitadores de la tabla Markdown.
- **006-HU-04:** Como usuario del arnés, quiero que el conteo de controles parseados (`Total Checks`) coincida exactamente con la cantidad física de filas de control declaradas en el documento, garantizando cobertura total y fidedigna.

## 4. Definiciones de Dominio

- **Fila de Control (`CheckItem`):** Registro dentro de una tabla Markdown que especifica un identificador único de control (`CHK-XX`), una referencia de requisitos vinculados, una descripción semántica y un comando de shell ejecutable delimitado por comillas invertidas.
- **Identificador de Requisito Heterogéneo:** Clave o lista de claves en la segunda columna de la tabla que vincula el control con la especificación. Puede incluir identificadores funcionales (`RF-XX`), no funcionales (`RNF-XX`), casos límite (`CL-XX`), categorías globales (`GLOBAL`, `STACK`, `LINTER`) o combinaciones separadas por comas con espacios arbitrarios.
- **Fila Candidata a Control:** Toda línea en el archivo de verificación fuera de bloques de código cercados (```` ``` ````) cuyo primer carácter no blanco sea un pipe Markdown seguido de espacios opcionales y el prefijo `CHK-` (ej. `| CHK-`).
- **Omisión Silenciosa (Silent Drop):** Defecto de diseño donde una línea con intención de control no coincide con una expresión regular estricta y el analizador la descarta sin lanzar excepciones ni advertencias, reduciendo artificialmente el denominador de pruebas.
- **Inmunidad de Bloques Cercados:** Propiedad del parser que descarta líneas ubicadas dentro de bloques de código cercados Markdown (bloques delimitados por tres o más comillas invertidas) para evitar que tablas de ejemplo o documentación disparen falsos errores sintácticos.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

### Compatibilidad con Identificadores de Requisito Heterogéneos
- **006-RF-01 (Ubiquitous):** EL SISTEMA DEBE aceptar en la columna de requisitos de la matriz de verificación cualquier identificador alfanumérico estándar, incluyendo prefijos `RF-`, `RNF-`, `CL-`, identificadores globales (`GLOBAL`, `RF-Stack`, `RF-GLOBAL`), y listas de identificadores separadas por coma con o sin espacios adyacentes.
- **006-RF-02 (Event-Driven):** CUANDO una fila contenga múltiples requisitos separados por coma (por ejemplo, `006-RNF-01, 006-RNF-02` o `006-RF-01,006-CL-01`), EL SISTEMA DEBE normalizar la lista eliminando espacios adyacentes a las comas (ej. `006-RNF-01,006-RNF-02`) y almacenarla íntegramente en el campo `req_id` de la tabla `verifications` de SQLite, preservando la trazabilidad exacta sin requerir migraciones de esquema.

### Detección y Rechazo Determinista de Filas Defectuosas (Anti-Silent-Drop)
- **006-RF-03 (Unwanted Behavior):** SI una línea del archivo verificador es identificada como una fila candidata a control (línea fuera de bloques cercados que inicia con pipe y el prefijo `CHK-`) pero no cumple con la estructura esperada de cuatro columnas (ID, Requisito, Descripción, Comando entre backticks), ENTONCES EL SISTEMA DEBE abortar la operación (`verify` o `plan-lock`) con código de salida `1` y reportar el número de línea y el motivo del fallo sintáctico.
- **006-RF-04 (Event-Driven):** CUANDO una fila de control contenga operadores de shell (como pipes `|`, operadores lógicos `||` o `&&`) o comillas dentro del comando ejecutable, EL SISTEMA DEBE aislar el comando delimitándolo estrictamente por el primer y último backtick de la celda de comando, garantizando que los pipes internos del comando no fragmenten las columnas de la tabla Markdown.
- **006-RF-05 (Unwanted Behavior):** SI el comando ejecutable dentro de una fila de control se encuentra vacío o carece de delimitadores de comillas invertidas válidos, ENTONCES EL SISTEMA DEBE abortar inmediatamente la operación con código de salida `1` indicando la fila afectada.
- **006-RF-06 (Unwanted Behavior):** SI el archivo verificador contiene identificadores de control (`CHK-XX`) duplicados, ENTONCES EL SISTEMA DEBE abortar la operación (`verify` o `plan-lock`) con código de salida `1` reportando el identificador repetido y la línea de colisión.

### Inmunidad Contextual y Filtrado Limpio
- **006-RF-07 (Event-Driven):** CUANDO se procese el archivo de verificación, EL SISTEMA DEBE ignorar líneas que constituyan encabezados de tabla Markdown, separadores de columna (como `| :--- | :--- |`), líneas en blanco, comentarios en prosa y cualquier línea ubicada dentro de bloques de código cercados (```` ``` ````), sin considerarlas filas candidatas a control ni disparar alertas de error sintáctico.

### Integridad de Controles en Sellado y Verificación
- **006-RF-08 (Event-Driven):** CUANDO se ejecute `seb plan-lock <tarea>`, EL SISTEMA DEBE verificar que el archivo de verificación contenga al menos un control ejecutable válido y que ninguna fila candidata haya sido rechazada por error sintáctico o identificador duplicado antes de calcular el hash criptográfico y sellar el plan.
- **006-RF-09 (Event-Driven):** CUANDO se ejecute `seb verify <tarea>`, EL SISTEMA DEBE reportar en el resumen de ejecución el total exacto de controles parseados y exigir que el 100% de dichos controles alcancen estado `[PASA]` para emitir el veredicto final de `APROBADO`.
- **006-RF-10 (State-Driven):** MIENTRAS existan errores de parseo, comandos vacíos o identificadores duplicados en el archivo de verificación, EL SISTEMA DEBE impedir la transición de estado a `VERIFIED` y denegar el desbloqueo del hook `pre-commit`.

## 6. Requisitos No Funcionales (Verificables)

- **006-RNF-01 (Rendimiento de Parseo):** El análisis sintáctico de un archivo verificador de hasta 100 controles debe completarse en menos de 10 milisegundos mediante evaluación computacional en Go puro, sin requerir invocaciones a modelos de lenguaje ni alocaciones excesivas de memoria.
- **RNF-02 (Compatibilidad Retrospectiva):** Todos los archivos de verificación existentes en el repositorio (`specs/001_*` a `specs/005_*`) deben parsearse satisfactoriamente con el nuevo analizador sin requerir modificaciones en su sintaxis actual.
- **006-RNF-03 (Determinismo Offline):** El análisis sintáctico y la validación de errores estructurales deben operar de forma totalmente autónoma en disco y memoria local, garantizando funcionamiento determinista sin dependencias de red.

## 7. Casos Límite (Edge Cases)

- **006-CL-01 (Espacios en listas de requisitos):** Filas con formato `| 006-CHK-01 | 006-RF-01, 006-RF-02, 006-RNF-01 | ...` con espacios variables tras cada coma deben parsearse exitosamente y normalizarse a `006-RF-01,006-RF-02,006-RNF-01`.
- **006-CL-02 (Pipes u operadores en el comando):** Una fila como `| 006-CHK-02 | 006-RF-02 | Verif | `grep "foo" file.txt | wc -l || exit 1` |` debe extraer el comando íntegro `grep "foo" file.txt | wc -l || exit 1` sin desfasar la tabla.
- **006-CL-03 (Inmunidad a bloques de código cercados):** Un bloque explicativo de documentación en Markdown dentro de un bloque cercado ```` ```markdown | 006-CHK-01 | Ejemplo | `ls` | ``` ```` no debe interpretarse como fila ejecutable ni registrarse en la lista de controles.
- **006-CL-04 (Identificador duplicado):** Si existen dos filas con `006-CHK-01`, el sistema debe abortar con error explícito indicando duplicidad.
- **006-CL-05 (Fila CHK con comillas invertidas faltantes):** Una fila como `| 006-CHK-03 | 006-RF-03 | Descripción | exit 0 |` debe ser identificada como un error de sintaxis y provocar un fallo explícito con código `1` en lugar de ser omitida en silencio.
- **006-CL-06 (Archivo sin filas de control):** Si un archivo verificador solo contiene texto introductorio y encabezados de tabla sin filas `CHK-`, la operación debe abortar reportando ausencia de controles ejecutables.

## 8. Fuera de Alcance (Out of Scope)

- Soporte para formatos alternativos de matriz de pruebas que no utilicen tablas estándar de Markdown (ej. archivos YAML o JSON como entrada de verificación).
- Ejecución paralela o concurrente de los comandos de verificación (se mantiene la ejecución secuencial determinista actual).
- Modificación del esquema de la base de datos `verifications` en `.seb/seb-audit.db`.

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos funcionales (`006-RF-01` a `006-RF-10`) y no funcionales (`006-RNF-01` a `006-RNF-03`) cuentan con pruebas programáticas en estado `[PASA]` ejecutadas mediante `.seb/bin/seb verify parser_tabla_verificacion`.
- Inclusión de casos de prueba específicos que reproduzcan exactamente las tres filas reportadas (`006-CHK-09` con `006-RNF-02`, `006-CHK-98` con `006-RNF-01,006-RNF-02` y `006-CHK-99` con `GLOBAL`) y demuestren su ejecución efectiva.
- Inclusión de pruebas que verifiquen el aborto explícito ante filas `CHK-` malformadas (anti-omisión silenciosa) e identificadores duplicados.
- Inclusión de pruebas que certifiquen el parseo correcto de comandos con pipes `|` u operadores `||` entre backticks.
- Cero violaciones a las reglas de `docs/constitution.md`.

## 10. Dudas Abiertas y Puntos por Clarificar

- Ninguna duda pendiente. La matriz de comportamiento, reglas sintácticas, aislamiento de comandos de shell y manejo de casos límite han sido clarificadas y aprobadas en su totalidad.
