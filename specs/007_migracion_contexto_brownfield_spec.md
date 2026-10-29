# Spec 007 — Asimilación de Contexto Preexistente en Proyectos Brownfield

- **Tarea ID:** `007`
- **Identificador:** `migracion_contexto_brownfield`
- **Origen:** Prosa (Solicitud del usuario / Sesión de arquitectura)
- **Estado:** `APROBADA`

## 1. Contexto y Objetivo (El POR QUÉ)

En proyectos brownfield existentes donde se introduce `seb-harness`, es habitual encontrar directrices preexistentes para asistentes de inteligencia artificial distribuidas en archivos como `AGENTS.md`, `GEMINI.md`, `CLAUDE.md` o documentación interna de arquitectura. El proceso de instalación original podía conservar sin cambios directrices incompatibles o ignorar archivos de otros asistentes, provocando pérdida de conocimiento o divergencia operativa (*split-brain*) donde distintas herramientas de IA siguen reglas contradictorias.

El objetivo de esta tarea es dotar a `seb-harness` de un mecanismo determinista de respaldo y consolidación: preservar el conocimiento previo mediante respaldos deterministas (`<nombre>.pre-seb.<ext>`), auditar activamente dichos archivos durante la inicialización (`seb-iniciador`) para asimilar sus reglas en `docs/constitution.md` (respetando también constituciones preexistentes del arnés), transformar los archivos de asistentes en stubs canónicos de redirección a la única fuente de verdad (*Single Source of Truth*), y garantizar la reversibilidad completa mediante la restauración de respaldos en caso de desinstalación.

## 2. Actores y Usuarios

- **Desarrollador (Usuario / Operador):** Ejecuta la instalación o desinstalación del arnés en proyectos nuevos o heredados y supervisa la constitución resultante.
- **Agente Iniciador (`seb-iniciador`):** Proceso de ingeniería inversa e inicialización que analiza el repositorio y consolida las reglas de arquitectura y negocio.
- **Agentes de Plataforma (Antigravity, Gemini, Claude, Cursor):** Asistentes que consumen archivos de contexto raíz en el repositorio de trabajo.

## 3. Historias de Usuario

- **HU-01:** Como desarrollador que adopta `seb-harness` en un proyecto con instrucciones previas de IA (`AGENTS.md`, `GEMINI.md`, `CLAUDE.md`), quiero que el instalador respalde automáticamente los archivos ajenos al arnés antes de desplegar sus plantillas, para asegurar que no se pierdan directivas previas.
- **HU-02:** Como agente iniciador de constitución, quiero inspeccionar automáticamente las fuentes de contexto heredadas y los archivos respaldados para incorporar las reglas de dominio, arquitectura y comandos en `docs/constitution.md`.
- **HU-03:** Como desarrollador, quiero que al finalizar la inicialización, los archivos de contexto de asistentes queden configurados como stubs de redirección a `docs/constitution.md` y `AGENTS.md`, previniendo la divergencia de directivas.
- **HU-04:** Como desarrollador, quiero que al desinstalar el arnés mediante `uninstall.sh`, cualquier archivo que haya sido respaldado con `.pre-seb.` sea restaurado a su nombre original.

## 4. Definiciones de Dominio

- **Firma Canónica de seb-harness:** Presencia de la subcadena `seb-harness` evaluada de manera insensible a mayúsculas y minúsculas (`grep -qi "seb-harness"`).
- **Archivo Ajeno / Preexistente:** Archivo de contexto presente en el repositorio destino que no contiene la firma canónica de `seb-harness`.
- **Nomenclatura Canónica de Respaldo:** Formato obligatorio `<nombre>.pre-seb.<ext>` aplicado al archivo original ajeno (por ejemplo: `AGENTS.pre-seb.md`, `GEMINI.pre-seb.md`, `CLAUDE.pre-seb.md`, `docs/constitution.pre-seb.md`).
- **Stub de Redirección:** Archivo liviano de contexto para herramientas externas cuyo contenido indica que el proyecto se gobierna bajo `seb-harness` y enlaza a `docs/constitution.md`, `AGENTS.md` y `MEMORY.md`.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

- **RF-01 (Event-Driven):** CUANDO se ejecute el aprovisionamiento del arnés sobre un proyecto destino con un archivo `AGENTS.md` preexistente que no contenga la subcadena `seb-harness` (insensible a mayúsculas/minúsculas), EL SISTEMA DEBE respaldar dicho archivo bajo el nombre `AGENTS.pre-seb.md` antes de instalar el protocolo operativo oficial.
- **RF-02 (Event-Driven):** CUANDO se ejecute el aprovisionamiento del arnés sobre un proyecto destino donde existan archivos de asistentes de IA ajenos (`GEMINI.md`, `CLAUDE.md`) que no contengan la subcadena `seb-harness` (insensible a mayúsculas/minúsculas), EL SISTEMA DEBE generar un respaldo para cada uno bajo la nomenclatura `<nombre>.pre-seb.<ext>`.
- **RF-03 (State-Driven):** MIENTRAS se ejecute la fase de reconocimiento de `seb-iniciador`, EL SISTEMA DEBE incluir e inspeccionar obligatoriamente los archivos de contexto preexistentes (`AGENTS.md`, `AGENTS.pre-seb.md`, `GEMINI.md`, `GEMINI.pre-seb.md`, `CLAUDE.md`, `CLAUDE.pre-seb.md` y `docs/constitution.pre-seb.md` si están presentes) para extraer reglas de negocio, arquitectura y comandos de verificación.
- **RF-04 (Event-Driven):** CUANDO se ejecute `seb-iniciador` en un proyecto donde ya exista `docs/constitution.md` sin la subcadena `seb-harness` (insensible a mayúsculas/minúsculas), EL SISTEMA DEBE respaldar el archivo original como `docs/constitution.pre-seb.md` y sintetizar la nueva constitución incorporando las directivas previas en las secciones A a F.
- **RF-05 (State-Driven):** MIENTRAS exista un `docs/constitution.md` que contenga la subcadena `seb-harness` (insensible a mayúsculas/minúsculas), EL SISTEMA DEBE preservar el archivo existente y abstenerse de sobrescribirlo destructivamente.
- **RF-06 (Event-Driven):** CUANDO `seb-iniciador` complete la consolidación de `docs/constitution.md`, EL SISTEMA DEBE generar o sustituir los archivos de asistentes detectados (`GEMINI.md`, `CLAUDE.md`) como stubs canónicos de redirección que apunten a `docs/constitution.md` y `AGENTS.md`.
- **RF-07 (Unwanted Behavior):** SI al momento de crear un respaldo ya existe un archivo con la ruta `<nombre>.pre-seb.<ext>`, ENTONCES EL SISTEMA DEBE conservar el respaldo preexistente sin sobrescribirlo y emitir un mensaje de advertencia.
- **RF-08 (Event-Driven):** CUANDO se ejecute la desinstalación del arnés mediante `uninstall.sh`, EL SISTEMA DEBE identificar todos los archivos con patrón `*.pre-seb.*` generados por el arnés y restaurarlos a su denominación original sin el infijo `.pre-seb`.
- **RF-09 (Ubiquitous):** EL SISTEMA DEBE considerar como perteneciente a `seb-harness` a cualquier archivo de contexto si contiene la subcadena `seb-harness` de manera insensible a mayúsculas y minúsculas.

## 6. Requisitos No Funcionales (Verificables)

- **RNF-01 (Idempotencia):** La ejecución repetida de los scripts de instalación y de las rutinas de inicialización en un proyecto ya configurado debe finalizar con código de salida `0` y mantener inalterada la consistencia de los respaldos y stubs.
- **RNF-02 (Aislamiento y Determinismo):** El reconocimiento de firmas, la creación de respaldos, la generación de stubs y la restauración en la desinstalación deben operar de manera puramente local y fuera de línea, sin dependencias de red ni herramientas no deterministas.

## 7. Casos Límite (Edge Cases)

- **CL-01 (Archivos vacíos o en blanco):** Si un archivo de contexto ajeno tiene un tamaño de 0 bytes o contiene solo espacios en blanco, el sistema debe respaldarlo igualmente sin emitir errores de ejecución.
- **CL-02 (Proyectos Greenfield):** En proyectos sin ningún archivo de contexto previo, el proceso de instalación e inicialización debe completarse normalmente sin crear archivos `.pre-seb` espurios.
- **CL-03 (Directivas previas contradictorias):** Si los archivos de contexto previos contienen instrucciones incompatibles con el arnés (por ejemplo, omitir tests o saltar especificaciones), el proceso debe descartar dichas directivas operativas y señalar la discrepancia en `MEMORY.md`.

## 8. Fuera de Alcance (Out of Scope)

- No se contempla la sincronización bidireccional automática continua entre `docs/constitution.md` y archivos externos de configuración de asistentes una vez inicializado el arnés.
- No se contempla la conversión o soporte automático de formatos binarios o herramientas propietarias fuera de archivos de texto Markdown (`.md`).

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos funcionales (`RF-01` a `RF-09`) cuentan con verificación programática en estado `[PASA]` ejecutada mediante `.seb/bin/seb verify migracion_contexto_brownfield`.
- La suite de tests unitarios y de integración de `engine/`, `install.sh` y `uninstall.sh` valida la gestión de respaldos, stubs y restauración.
- Cero violaciones a `docs/constitution.md`.

## 10. Dudas Abiertas y Puntos por Clarificar

- Ninguna duda pendiente.
