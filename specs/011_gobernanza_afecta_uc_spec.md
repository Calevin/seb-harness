# Spec 011 — Gobernanza del Metadato `Afecta-UC:` en Especificación y Clarificación SDD

- **Tarea ID:** `011`
- **Identificador:** `gobernanza_afecta_uc`
- **Origen:** Solicitud de usuario para resolver la fragmentación de casos de uso y garantizar la estrategia de Documentación Viva (*Living Documentation*) introducida en la Spec 010
- **Estado:** `APROBADA`
- **Dependencias:** `010_integracion_documentador`
- **Afecta-UC:** `UC-documentation_functional_generation`

## 1. Contexto y Objetivo (El POR QUÉ)

En el flujo actual de `seb-harness`, la Spec 010 introdujo la Fase 6 (`seb-documentador`) y el concepto de Documentación Viva (*Living Documentation*), según el cual la evolución de una funcionalidad existente debe actualizar su caso de uso en `docs/use-cases/UC-<dominio>_<accion>.md` y las tareas internas sin impacto funcional externo pueden omitir la generación documental mediante `Afecta-UC: ninguno`.

Sin embargo, ni la Fase 1 (`seb-especificador`) ni la plantilla base de especificaciones incluyen actualmente el metadato `Afecta-UC:`, y la Fase 2 (`seb-clarificador`) tampoco audita su presencia, validez referencial contra `docs/use-cases/README.md` ni su coherencia con los requisitos funcionales `RF-XX`. Como consecuencia, las especificaciones nuevas carecen de este metadato y al llegar a la Fase 6 caen por defecto en la regla de respaldo (crear un archivo UC nuevo), generando una fragmentación 1:1 entre especificaciones y casos de uso en lugar de mantener contratos funcionales vivos y consolidados.

Esta especificación establece la gobernanza declarativa de `Afecta-UC:` en las Fases 1 y 2 para asegurar trazabilidad funcional desde el origen de cada tarea, sin modificar el binario compilado del motor y preservando la compatibilidad con las especificaciones históricas (`001` a `010`).

## 2. Actores y Usuarios

- **Agente Especificador (`seb-especificador`):** Consulta el catálogo de casos de uso existente y declara el metadato `Afecta-UC:` en la cabecera de toda nueva especificación.
- **Agente Clarificador (`seb-clarificador`):** Audita la presencia, validez referencial y coherencia funcional de `Afecta-UC:` antes de aprobar una especificación.
- **Agente Documentador (`seb-documentador`):** Consume el valor validado de `Afecta-UC:` en la Fase 6 para actualizar un caso de uso existente, crear uno nuevo u omitir la documentación.
- **Desarrollador / Usuario Humano:** Responde las aclaraciones levantadas en la Fase 2 cuando el impacto en casos de uso sea ambiguo, inexistente o contradictorio.

## 3. Historias de Usuario

- **HU-01:** Como desarrollador, quiero que `seb-especificador` consulte el catálogo de casos de uso antes de redactar una nueva spec e incluya `Afecta-UC:` en la cabecera, para evitar la proliferación de archivos de casos de uso duplicados cuando una tarea evoluciona una funcionalidad existente.
- **HU-02:** Como desarrollador, quiero que `seb-clarificador` valide que `Afecta-UC:` esté presente, apunte a un caso de uso real (si referencia uno previo) y sea coherente con los `RF-XX`, para impedir que se aprueben especificaciones con metadatos rotos o contradictorios.
- **HU-03:** Como mantenedor del arnés, quiero que esta gobernanza opere exclusivamente mediante directivas de habilidades y plantillas distribuibles, para mantener el motor CLI desacoplado y conservar intactas las specs `001` a `010` ya cerradas.

## 4. Definiciones de Dominio

- **Metadato `Afecta-UC:`:** Campo obligatorio en la cabecera de toda nueva especificación que declara su impacto sobre el catálogo de casos de uso con exactamente uno de tres valores válidos: un identificador preexistente (`UC-<dominio>_<accion>`), el literal `nuevo`, o el literal `ninguno`.
- **Catálogo de Casos de Uso (`docs/use-cases/README.md`):** Índice maestro que registra los identificadores `UC-<dominio>_<accion>` vigentes en el proyecto.
- **Coherencia Funcional de `Afecta-UC:`:** Correspondencia lógica entre el valor declarado en `Afecta-UC:` y el comportamiento descrito en la sección de Requisitos Funcionales (`RF-XX`).

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

### Fase 1: Especificación y Plantilla (`seb-especificador`)
- **RF-01 (Event-Driven):** CUANDO el agente `seb-especificador` inicie la creación de una nueva especificación, EL SISTEMA DEBE instruirle consultar el catálogo `docs/use-cases/README.md` (en caso de existir) antes de redactar la cabecera, para determinar si la petición evoluciona un caso de uso existente, introduce uno nuevo o carece de impacto funcional externo.
- **RF-02 (Ubiquitous):** EL SISTEMA DEBE exigir que la cabecera de toda nueva especificación incluya obligatoriamente el campo `- **Afecta-UC:**` con su valor delimitado entre backticks, correspondiendo exactamente a una de tres opciones: un identificador preexistente con formato `` `UC-<dominio>_<accion>` ``, el valor literal `` `nuevo` ``, o el valor literal `` `ninguno` ``.
- **RF-03 (Ubiquitous):** EL SISTEMA DEBE incluir explícitamente el campo `- **Afecta-UC:**` con formato entre backticks y sus tres opciones válidas en la plantilla base de especificaciones utilizada por `seb-especificador`.
- **RF-04 (Optional Feature):** DONDE el catálogo `docs/use-cases/README.md` no exista todavía en el proyecto, EL SISTEMA DEBE instruir a `seb-especificador` asignar `nuevo` si la especificación introduce comportamiento funcional externo observable, o `ninguno` si se trata de refactorizaciones internas o infraestructura sin impacto funcional externo.

### Fase 2: Auditoría y Bloqueo en Clarificación (`seb-clarificador`)
- **RF-05 (Event-Driven):** CUANDO el agente `seb-clarificador` audite una nueva especificación, EL SISTEMA DEBE instruirle verificar la presencia del campo `Afecta-UC:` en la cabecera y validar que su valor esté delimitado entre backticks y corresponda a `` `UC-<dominio>_<accion>` ``, `` `nuevo` `` o `` `ninguno` ``.
- **RF-06 (Event-Driven):** CUANDO el campo `Afecta-UC:` declare un identificador `UC-<dominio>_<accion>`, EL SISTEMA DEBE instruir a `seb-clarificador` comprobar que dicho identificador exista registrado en `docs/use-cases/README.md`.
- **RF-07 (Event-Driven):** CUANDO `seb-clarificador` audite los requisitos `RF-XX` de la especificación, EL SISTEMA DEBE instruirle verificar que no exista contradicción entre el valor de `Afecta-UC:` y el alcance funcional descrito (por ejemplo, declarar `` `ninguno` `` cuando los `RF-XX` definen comportamiento funcional externo nuevo o modificado, o declarar `` `nuevo` `` cuando los `RF-XX` modifican un caso de uso ya catalogado).
- **RF-08 (Unwanted Behavior):** SI una especificación auditada omite el campo `Afecta-UC:`, carece de backticks o contiene un valor fuera de los tres permitidos, referencia un identificador `UC-*` inexistente en `docs/use-cases/README.md`, o presenta contradicción entre `Afecta-UC:` y los `RF-XX`, ENTONCES EL SISTEMA DEBE exigir que `seb-clarificador` formule la observación como pregunta directa en la Fase de Interrogación sin editar la especificación previamente y detenga el flujo hasta recibir la respuesta del usuario.
- **RF-09 (Unwanted Behavior):** SI una especificación carece de una definición válida, referencialmente existente (cuando aplique `UC-*`) y coherente en `Afecta-UC:`, ENTONCES EL SISTEMA DEBE prohibir a `seb-clarificador` cambiar el estado de la especificación a `APROBADA`.

### Alcance, Distribución y Compatibilidad
- **RF-10 (Ubiquitous):** EL SISTEMA DEBE implementar las reglas de consulta, formato, plantilla y auditoría de `Afecta-UC:` exclusivamente en las directivas de habilidades y plantillas distribuibles de `payload/`, sin agregar ni modificar lógica en el binario Go del motor.
- **RF-11 (State-Driven):** MIENTRAS existan especificaciones históricas `001` a `010` cerradas y selladas previamente a esta especificación, EL SISTEMA DEBE preservar su estado `APROBADA` y su validez en el grafo de dependencias sin exigir retroactivamente el campo `Afecta-UC:` en dichos archivos.

## 6. Requisitos No Funcionales (Verificables)

- **RNF-01 (Consistencia de Distribución):** Los archivos de directivas y plantillas modificados en `payload/.agents/` deben mantenerse sincronizados con `.agents/` y ser distribuibles mediante el flujo estándar de instalación sin alterar la firma ni el comportamiento de comandos existentes del CLI `seb`.
- **RNF-02 (Cero Impacto en Binario Go):** La compilación y suite de pruebas unitarias existentes del motor Go deben permanecer intactas y pasar sin regresiones ni cambios requeridos en `engine/`.

## 7. Casos Límite (Edge Cases)

- **CL-01 (Catálogo `docs/use-cases/README.md` ausente con referencia `UC-*`):** Si una especificación declara `Afecta-UC: UC-<dominio>_<accion>` pero `docs/use-cases/README.md` no existe en el repositorio, `seb-clarificador` debe tratar la referencia como inexistente, levantar el punto por aclarar y bloquear la transición a `APROBADA` hasta corregir a `nuevo`, `ninguno` o crear el catálogo previo.
- **CL-02 (Valor vacío o marcador de plantilla sin completar):** Si una especificación conserva un marcador de plantilla sin resolver o un valor vacío en `Afecta-UC:`, `seb-clarificador` debe rechazar la aprobación de la misma forma que ante la omisión del campo.
- **CL-03 (Petición con impacto en múltiples casos de uso):** Si una petición de usuario afecta simultáneamente a más de un `UC-<dominio>_<accion>` independiente, `seb-especificador` debe aplicar la regla de división de alcance y sugerir separar en especificaciones independientes (una por caso de uso afectado), manteniendo la cardinalidad unívoca de `Afecta-UC:`.

## 8. Fuera de Alcance (Out of Scope)

- Modificación del binario Go (`engine/`) para parsear, validar sintácticamente o bloquear comandos CLI (`plan-lock`, `verify`, `guard`) en función del campo `Afecta-UC:`.
- Migración o edición retroactiva de las especificaciones `001` a `010` ya aprobadas y selladas en `.seb/seb.lock`.
- Cambios en la estructura interna de los archivos `docs/use-cases/UC-<dominio>_<accion>.md` definidos en la Spec 010.

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos (`RF-01` a `RF-11` y `RNF-01` a `RNF-02`) cuentan con verificación programática en `[PASA]` ejecutada mediante `.seb/bin/seb verify gobernanza_afecta_uc`.
- Cero violaciones a `docs/constitution.md`.
- Las directivas de `seb-especificador` (incluyendo su plantilla base) y `seb-clarificador` en `payload/.agents/` reflejan de forma inequívoca las reglas de `Afecta-UC:`.

## 10. Dudas Abiertas y Puntos por Clarificar

- Ninguna duda pendiente.
