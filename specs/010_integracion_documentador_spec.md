# Spec 010 — Integración de Skill Documentador Funcional en seb-harness

- **Tarea ID:** `010`
- **Identificador:** `integracion_documentador`
- **Origen:** Solicitud de usuario / Propuesta `doc-funcional-mermaid` adaptada al arnés
- **Estado:** `APROBADA`

## 1. Contexto y Objetivo (El POR QUÉ)

En el flujo actual de `seb-harness`, el ciclo concluye con la Fase 5 (`seb-verificador`), la cual certifica mediante pruebas deterministas que el código implementado cumple con la especificación. Sin embargo, el arnés carece de un paso formal para consolidar y comunicar el comportamiento funcional resultante hacia desarrolladores humanos, stakeholders o futuras sesiones de agentes.

El conocimiento funcional queda fragmentado entre el código fuente y las especificaciones históricas. La integración del skill `seb-documentador` como la Fase 6 del ciclo de vida resuelve este vacío:
1. Estandariza la generación de casos de uso funcionales (enfoque caja negra / *black-box*) utilizando diagramas de secuencia de sistema (SSD) en Mermaid, fichas de intención, catálogos de error e invariantes de negocio, persistidos en `docs/use-cases/UC-<dominio>_<accion>.md`.
2. Incorpora al rol `seb-documentador` dentro de la política de menor privilegio del motor del arnés (`seb guard`), garantizando que el agente documentador no pueda alterar código fuente, pruebas ni especificaciones cerradas.
3. Asegura la distribución automatizada e idempotente del nuevo skill y sus plantillas a través del instalador del arnés (`install.sh`), tanto para proyectos nuevos como para proyectos brownfield que ya utilizan `seb-harness`, asistiendo la actualización de directivas operativas mediante parches sugeridos cuando `AGENTS.md` ya exista.

## 2. Actores y Usuarios

- **Desarrollador / Arquitecto:** Consulta la documentación consolidada de casos de uso para entender el comportamiento funcional del sistema sin inspeccionar el código fuente.
- **Agente de IA (Rol `seb-documentador`):** Transforma los requisitos verificados y el comportamiento real implementado en artefactos de documentación estructurados.
- **Motor del Arnés (`seb guard`):** Evalúa en tiempo real las solicitudes de escritura y ejecución del agente documentador para impedir violaciones de menor privilegio.
- **Instalador del Arnés (`install.sh`):** Despliega y actualiza la definición del skill, reglas y directorios en los proyectos destino.

## 3. Historias de Usuario

- **HU-01:** Como desarrollador, quiero que tras verificar exitosamente una especificación funcional, exista un paso estandarizado para documentar el caso de uso en caja negra con diagramas Mermaid, para mantener viva y sincronizada la documentación del sistema.
- **HU-02:** Como desarrollador, quiero que el motor del arnés bloquee cualquier intento del agente documentador de modificar código fuente o pruebas, para proteger la integridad técnica del repositorio durante la fase de documentación.
- **HU-03:** Como operador del arnés, quiero que al ejecutar la instalación o actualización de `seb-harness` en un proyecto nuevo o existente, el nuevo skill y su estructura de soporte queden configurados automáticamente sin romper personalizaciones previas.
- **HU-04:** Como desarrollador con un proyecto en marcha, quiero que si `install.sh` conserva mi `AGENTS.md` personalizado, me genere un archivo auxiliar con el fragmento exacto de la Fase 6 para que pueda integrarlo fácilmente.

## 4. Definiciones de Dominio

- **Fase 6 (Documentación Funcional):** Etapa del ciclo de vida de `seb-harness` posterior a la verificación determinista exitosa, orientada a consolidar casos de uso funcionales gobernada por directivas de agente sin sobrecargar el CLI Go.
- **Caso de Uso (UC):** Artefacto estructurado nombrado bajo el esquema `UC-<dominio>_<accion>.md` que describe un contrato funcional específico, sus actores, precondiciones, invariantes, catálogo de errores tipificado y diagrama de interacción de sistema (SSD).
- **Guardarraíl del Documentador:** Regla de control de acceso en el motor del arnés que confina la capacidad de escritura del rol `seb-documentador` exclusivamente a la documentación de casos de uso y al archivo de memoria técnica.
- **Documentación Viva (*Living Documentation*):** Estrategia donde la modificación de una funcionalidad existente actualiza el caso de uso preexistente en lugar de crear documentos huérfanos o contradictorios, gobernada por el metadato declarativo `Afecta-UC:` en la cabecera de la spec.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

### Incorporación al Ciclo de Vida y Gobernanza
- **RF-01 (Ubiquitous):** EL SISTEMA DEBE definir formalmente la Fase 6 (`seb-documentador`) en el protocolo operativo del arnés (`AGENTS.md` y `00-seb-harness.md`) como una etapa posterior al veredicto favorable de la verificación determinista delegada en las instrucciones del agente.
- **RF-02 (State-Driven):** MIENTRAS el agente opere bajo el rol `seb-documentador`, EL SISTEMA DEBE permitir la escritura únicamente sobre el directorio de casos de uso (`docs/use-cases/`) y sobre la bitácora técnica (`MEMORY.md`).
- **RF-03 (Unwanted Behavior):** SI el rol `seb-documentador` solicita operaciones de escritura sobre código fuente de la aplicación, suites de pruebas, la constitución del proyecto (`docs/constitution.md`) o archivos de especificaciones y planes (`specs/*`), ENTONCES EL SISTEMA DEBE denegar la operación con código de salida no nulo mediante el guardarraíl de menor privilegio.

### Estructura y Reglas del Artefacto de Caso de Uso
- **RF-04 (Event-Driven):** CUANDO el agente documentador genere un caso de uso, EL SISTEMA DEBE producir un documento estructurado que contenga ficha de intención, diagrama de interacción de sistema (SSD) en formato Mermaid con numeración automática (`autonumber`), árbol de decisiones (cuando existan bifurcaciones complejas), catálogo tipificado de errores y postcondiciones.
- **RF-05 (Ubiquitous):** EL SISTEMA DEBE exigir que los identificadores de participantes, comandos, eventos, DTOs y parámetros en los diagramas y tablas estén en idioma inglés, y que las descripciones explicativas, notas y prosa narrativa estén en idioma español.
- **RF-06 (Ubiquitous):** EL SISTEMA DEBE prohibir de forma absoluta el uso de emojis dentro de cualquier archivo, tabla o diagrama de documentación funcional generado.
- **RF-07 (Event-Driven):** CUANDO la especificación indique un caso de uso preexistente en el metadato `Afecta-UC:`, EL SISTEMA DEBE actualizar el documento correspondiente en `docs/use-cases/UC-<dominio>_<accion>.md` y registrar la revisión en lugar de generar un archivo duplicado.
- **RF-08 (Event-Driven):** CUANDO se genere o actualice un caso de uso, EL SISTEMA DEBE registrar o sincronizar la entrada correspondiente en el índice general de casos de uso (`docs/use-cases/README.md`).

### Distribución, Instalación y Actualización
- **RF-09 (Event-Driven):** CUANDO se ejecute el instalador del arnés sobre un proyecto nuevo, EL SISTEMA DEBE aprovisionar la definición del skill `seb-documentador`, garantizar la creación del directorio `docs/use-cases/` y registrar el rol en las reglas operativas distribuidas.
- **RF-10 (Event-Driven):** CUANDO se ejecute el instalador del arnés en modo de actualización sobre un proyecto donde ya se encuentra instalado `seb-harness`, EL SISTEMA DEBE incorporar o actualizar la definición del skill `seb-documentador` y las reglas del arnés en `.agents/` sin sobrescribir ni eliminar casos de uso preexistentes en `docs/use-cases/`.
- **RF-11 (Event-Driven):** CUANDO el instalador detecte un archivo `AGENTS.md` existente con firma de `seb-harness` que no contenga la referencia a `seb-documentador`, EL SISTEMA DEBE generar el archivo auxiliar `AGENTS.md.patch.md` conteniendo el bloque Markdown de la Fase 6 y la tabla de menor privilegio actualizada, emitiendo una advertencia en consola que invite al desarrollador a incorporar dicho fragmento.
- **RF-12 (Event-Driven):** CUANDO se ejecute la desinstalación estándar del arnés en un proyecto, EL SISTEMA DEBE remover la definición del skill `seb-documentador` preservando intactos todos los archivos y casos de uso ubicados en `docs/use-cases/`.

### Flexibilidad de Alcance
- **RF-13 (Optional Feature):** DONDE una especificación señale en su cabecera `Afecta-UC: ninguno` por corresponder a refactorizaciones internas, tareas de infraestructura o componentes sin impacto funcional externo, EL SISTEMA DEBE permitir dar por concluido el ciclo de la tarea en la Fase 5 sin requerir la generación de artefactos en `docs/use-cases/`.

## 6. Requisitos No Funcionales (Verificables)

- **RNF-01 (Latencia del Guardarraíl):** La evaluación de permisos de acceso para el rol `seb-documentador` mediante `seb guard check-write` debe completarse en menos de 5 milisegundos.
- **RNF-02 (Agnosticismo Tecnológico):** La definición del skill y los artefactos de casos de uso generados deben ser plenamente independientes del stack tecnológico del proyecto consumidor, requiriendo únicamente soporte estándar de Markdown y visualización Mermaid.
- **RNF-03 (Idempotencia de Instalación):** La ejecución repetida del instalador con el nuevo skill no debe generar efectos secundarios, duplicación de líneas en archivos de configuración ni advertencias de ejecución innecesarias.

## 7. Casos Límite (Edge Cases)

- **CL-01 (Inexistencia del directorio destino):** Si el directorio `docs/use-cases/` no existe al momento de documentar, el instalador o el skill debe asegurar su creación previa sin fallos de ruta.
- **CL-02 (Proyectos sin especificación previa de tipo caso de uso):** Si el proyecto no cuenta con casos de uso previos, el skill debe inicializar el archivo `docs/use-cases/README.md` con la estructura de catálogo antes de vincular el primer caso.
- **CL-03 (Actualización en repositorios con directivas pre-existentes):** Si un proyecto posee un archivo `AGENTS.md` personalizado con firma previa de `seb-harness`, el instalador no debe sobrescribirlo, sino generar `AGENTS.md.patch.md` para facilitar la migración manual.
- **CL-04 (Omisión del metadato Afecta-UC):** Si una spec antigua o redactada sin el campo `Afecta-UC:` es procesada por el documentador, el sistema debe asumir por defecto la creación de un nuevo caso de uso bajo la convención canónica `UC-<dominio>_<accion>.md`.

## 8. Fuera de Alcance (Out of Scope)

- Modificación del binario Go para incorporar subcomandos dedicados de documentación (ej. `seb doc status`), manteniendo el motor desacoplado y delegando la coordinación de la Fase 6 en las instrucciones operativas de los agentes.
- Implementación de herramientas de renderizado estático local de imágenes PNG o SVG a partir de diagramas Mermaid (el renderizado permanece delegado a visores Markdown o plataformas de Git).
- Generación automática de diagramas de secuencia de arquitectura interna (*white-box*) por defecto, limitando la Fase 6 a la frontera de caja negra funcional del sistema.

## 9. Criterios de Finalización (Definition of Done)

- Matriz de verificación determinista en `specs/010_integracion_documentador_verificador.md` con el 100% de controles en estado `[PASA]`.
- Subcomando `seb guard check-write` validado con pruebas unitarias para autorizar `docs/use-cases/*` y bloquear código de aplicación bajo el rol `seb-documentador`.
- Definición del skill `seb-documentador` integrada en `payload/.agents/skills/seb-documentador/SKILL.md` y sincronizada en `.agents/skills/`.
- Reglas operativas actualizadas en `payload/AGENTS.md` y `payload/.agents/rules/00-seb-harness.md` reflejando la Fase 6.
- Script de instalación (`install.sh`) actualizado para crear `docs/use-cases/` y emitir `AGENTS.md.patch.md` al detectar proyectos brownfield con firma previa sin Fase 6.

## 10. Dudas Abiertas y Puntos por Clarificar

- Ninguna duda pendiente. Los cuatro puntos de clarificación fueron resueltos e incorporados en la especificación. Especificación formalmente auditada y aprobada.
