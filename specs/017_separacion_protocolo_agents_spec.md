# Spec 017 — Separación del Protocolo SDD y Bloque Gestionado en AGENTS.md

- **Tarea ID:** `017`
- **Identificador:** `separacion_protocolo_agents`
- **Origen:** Prosa (diseño iterativo con el autor)
- **Estado:** `APROBADA`
- **Afecta-UC:** `UC-brownfield_context_assimilation`, `UC-documentation_functional_generation`, `UC-provisioning_harness_uninstall`

## 1. Contexto y Objetivo (El POR QUÉ)

Actualmente el archivo `AGENTS.md` de los proyectos consumidores mezcla dos responsabilidades incompatibles: el protocolo operativo evolutivo de `seb-harness` y las directivas propias del proyecto destino. Para evitar sobrescribir ediciones locales, el proceso de actualización conserva `AGENTS.md` intacto cuando detecta la firma `seb-harness`, lo que impide propagar automáticamente nuevas reglas del arnés (como la máquina de estados, identificadores prefijados u operaciones one-off) y obligó a introducir parches ad-hoc como `AGENTS.md.patch.md`. Además, inyectar el protocolo SDD completo en cada turno consume contexto innecesario cuando una tarea no requiere el flujo SDD completo, obstaculizando la futura incorporación de un flujo rápido para cambios menores.

El objetivo de esta especificación es separar el protocolo operativo SDD completo hacia un archivo exclusivo gestionado por el arnés dentro de `.agents/` (`.agents/SEB-HARNESS.md`) y transformar `AGENTS.md` en un enrutador liviano con un bloque delimitado actualizable automáticamente, preservando siempre el contenido propio del proyecto, migrando limpiamente proyectos legados y preparando la base para enrutar múltiples modos de trabajo en el futuro.

## 2. Actores y Usuarios

- **Desarrollador / Mantenedor del Proyecto Destino:** Instala, actualiza y desinstala `seb-harness` sobre repositorios existentes (por ejemplo, proyectos consumidores) y puede mantener instrucciones propias en `AGENTS.md`.
- **Agente de IA:** Lee `AGENTS.md` y la regla global al inicio de la sesión y carga el protocolo SDD completo desde `.agents/SEB-HARNESS.md` cuando opera bajo el flujo SDD.
- **Sistema de Aprovisionamiento del Arnés:** Proceso automatizado de instalación, actualización y desinstalación del arnés en proyectos destino y en modo dogfooding (`--self`).

## 3. Historias de Usuario

- **017-HU-01:** Como desarrollador que actualiza `seb-harness` en un proyecto existente, quiero que el protocolo SDD completo se actualice automáticamente en `.agents/SEB-HARNESS.md` sin intervención manual para no perder mejoras ni reglas nuevas del arnés.
- **017-HU-02:** Como desarrollador con directivas propias en `AGENTS.md`, quiero que el arnés actualice únicamente su bloque gestionado dentro de `AGENTS.md` (y lo remueva limpiamente al desinstalar) preservando intactas mis instrucciones locales para evitar conflictos y migraciones manuales.
- **017-HU-03:** Como diseñador del arnés, quiero que `AGENTS.md` y la regla global de sesión sean livianos y deleguen el protocolo SDD completo a `.agents/SEB-HARNESS.md` para habilitar carga progresiva de contexto y preparar la incorporación futura de un flujo rápido para cambios menores.

## 4. Definiciones de Dominio (Opcional)

- **Protocolo Gestionado (`.agents/SEB-HARNESS.md`):** Documento de referencia propiedad exclusiva de `seb-harness` que concentra el protocolo operativo completo del flujo SDD y se sobrescribe íntegramente en cada instalación o actualización.
- **Bloque Gestionado en `AGENTS.md`:** Sección delimitada por un marcador de inicio (`<!-- BEGIN seb-harness:managed -->`) y un marcador de fin (`<!-- END seb-harness:managed -->`) cuyo contenido administra automáticamente el instalador y limpia el desinstalador del arnés.
- **Formato Legado de `AGENTS.md`:** Archivo `AGENTS.md` preexistente en un proyecto destino (o en el repositorio en modo `--self`) que contiene la firma `seb-harness` pero carece de los marcadores de bloque gestionado.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

- **017-RF-01 (Ubiquitous):** EL SISTEMA DEBE proveer el protocolo operativo completo del flujo SDD (fuentes de verdad, fases SDD, matriz de menor privilegio, catálogo de herramientas nativas del arnés, regla obligatoria de introspección con `seb status`/`seb help`, máquina de estados, convención de identificadores prefijados y reglas de operaciones one-off) en un archivo exclusivo `.agents/SEB-HARNESS.md` encabezado por un aviso explícito de que es gestionado por `seb-harness` y se sobrescribe en cada actualización.
- **017-RF-02 (Event-Driven):** CUANDO se ejecuta la instalación o actualización del arnés sobre un proyecto destino, EL SISTEMA DEBE crear o sobrescribir íntegramente `.agents/SEB-HARNESS.md` con la versión vigente del protocolo sin requerir intervención manual.
- **017-RF-03 (Ubiquitous):** EL SISTEMA DEBE proveer una plantilla de `AGENTS.md` que actúe como enrutador liviano conteniendo un bloque delimitado por `<!-- BEGIN seb-harness:managed -->` y `<!-- END seb-harness:managed -->` con la firma `seb-harness`, la jerarquía base de fuentes de verdad (`docs/constitution.md` y `MEMORY.md`) y la instrucción explícita de consultar `.agents/SEB-HARNESS.md` para operar el flujo SDD completo, dejando el espacio exterior disponible para instrucciones propias del proyecto.
- **017-RF-04 (Event-Driven):** CUANDO se ejecuta la actualización del arnés sobre un proyecto destino cuyo `AGENTS.md` contiene un par único y bien formado de marcadores `<!-- BEGIN seb-harness:managed -->` y `<!-- END seb-harness:managed -->`, EL SISTEMA DEBE reemplazar exclusivamente el segmento comprendido entre ambos marcadores (inclusive) con la versión vigente del bloque gestionado y conservar de forma exacta todo el contenido del proyecto ubicado antes del marcador de inicio y después del marcador de fin.
- **017-RF-05 (Event-Driven):** CUANDO se ejecuta la instalación del arnés sobre un proyecto destino sin `AGENTS.md` previo o con un `AGENTS.md` sin firma `seb-harness`, EL SISTEMA DEBE instalar la plantilla de `AGENTS.md` con bloque gestionado, manteniendo el respaldo previo `AGENTS.pre-seb.md` cuando exista un archivo ajeno.
- **017-RF-06 (Event-Driven):** CUANDO se ejecuta la instalación o actualización del arnés sobre un proyecto destino cuyo `AGENTS.md` está en formato legado (contiene la firma `seb-harness` pero carece de marcadores de bloque gestionado), EL SISTEMA DEBE respaldar dicho archivo como `AGENTS.legacy-seb.md` (conservando el respaldo previo si ya existiera sin sobrescribirlo), instalar en su lugar la plantilla de enrutador con bloque gestionado y emitir un aviso informativo en consola indicando la ruta del respaldo.
- **017-RF-07 (Unwanted Behavior):** SI el `AGENTS.md` de un proyecto destino (o del propio repositorio en modo `--self`) presenta marcadores de bloque gestionado malformados (marcador de inicio sin fin, marcador de fin sin inicio, orden invertido o múltiples pares de marcadores), ENTONCES EL SISTEMA DEBE preservar intacto `AGENTS.md` sin aplicar modificaciones parciales, emitir un mensaje de error diagnóstico en `stderr` describiendo la anomalía y abortar la ejecución con código de salida `1`.
- **017-RF-08 (Ubiquitous):** EL SISTEMA DEBE reducir la regla global cargada automáticamente en todos los turnos (`.agents/rules/00-seb-harness.md`) exclusivamente a los invariantes universales de gobernanza (lectura de `docs/constitution.md` y `MEMORY.md`, integridad de tests y sujeción al control de menor privilegio y a `.agents/SEB-HARNESS.md` en flujos SDD), eliminando la duplicación de tablas y procedimientos completos ya contenidos en `.agents/SEB-HARNESS.md`.
- **017-RF-09 (Event-Driven):** CUANDO se ejecuta la instalación o actualización del arnés sobre cualquier proyecto destino, EL SISTEMA DEBE omitir la generación del archivo auxiliar legado `AGENTS.md.patch.md`.
- **017-RF-10 (Event-Driven):** CUANDO se ejecuta la desinstalación estándar (sin `--purge`) del arnés sobre un proyecto destino, EL SISTEMA DEBE eliminar el archivo gestionado `.agents/SEB-HARNESS.md` junto con los demás artefactos provistos por el arnés dentro de `.agents/` y remover únicamente el bloque delimitado por `<!-- BEGIN seb-harness:managed -->` y `<!-- END seb-harness:managed -->` (inclusive) en `AGENTS.md`, conservando intacto todo el contenido propio del proyecto fuera de dichos marcadores (respetando `--dry-run` sin modificar disco).
- **017-RF-11 (Event-Driven):** CUANDO se ejecuta la instalación en modo dogfooding (`--self`) sobre el propio repositorio `seb-harness`, EL SISTEMA DEBE actualizar el bloque gestionado del `AGENTS.md` raíz desde la plantilla oficial aplicando las mismas reglas de marcadores, respaldo legado y validación estricta (`017-RF-04`, `017-RF-06`, `017-RF-07`), garantizando paridad entre el repositorio del arnés y los proyectos consumidores.
- **017-RF-12 (Ubiquitous):** EL SISTEMA DEBE mantener los stubs de asistentes (`GEMINI.md` y `CLAUDE.md` generados por `seb-iniciador`) apuntando a `AGENTS.md` como único punto de entrada de enrutamiento (junto a `docs/constitution.md` y `MEMORY.md`), sin acoplarlos directamente a `.agents/SEB-HARNESS.md`.
- **017-RF-13 (Ubiquitous):** EL SISTEMA DEBE reemplazar formalmente los contratos heredados de ubicación y parcheo de `010-RF-11` (generación de `AGENTS.md.patch.md`), `010-RF-13` (reglas extensas de `seb-documentador` en `AGENTS.md` y `00-seb-harness.md`), `013-RF-31`/`013-RF-32` (directivas extensas de identificadores prefijados en `AGENTS.md` y `00-seb-harness.md`) y `016-RF-13`/`016-RF-14` (tabla de máquina de estados y regla de introspección en `AGENTS.md`), consolidando dichas directivas SDD en `.agents/SEB-HARNESS.md` y actualizando los tests de comportamiento correspondientes en la suite Go sin reducir el conteo de aserciones ni introducir omisiones (`t.Skip`).

## 6. Requisitos No Funcionales (Verificables)

- **017-RNF-01 (Idempotencia):** Ejecutar la actualización del arnés dos o más veces consecutivas sobre el mismo proyecto destino (o en modo `--self`) con marcadores válidos DEBE producir un archivo `AGENTS.md` y un archivo `.agents/SEB-HARNESS.md` idénticos en contenido (cero diferencias en la segunda ejecución).
- **017-RNF-02 (Huella de contexto del enrutador):** El bloque gestionado de la plantilla `AGENTS.md` DEBE tener una extensión máxima de 30 líneas y la regla global `.agents/rules/00-seb-harness.md` DEBE tener una extensión máxima de 35 líneas, garantizando baja sobrecarga de contexto en tareas que no invoquen el flujo SDD completo.
- **017-RNF-03 (Portabilidad POSIX/Unix):** La actualización y remoción del bloque delimitado en `AGENTS.md` DEBE funcionar sin requerir dependencias externas ajenas al entorno estándar ya asumido por los scripts del arnés.

## 7. Casos Límite (Edge Cases)

- **017-CL-01 (Contenido de usuario antes y después del bloque gestionado):** Si el proyecto destino tiene texto propio tanto por encima de `<!-- BEGIN seb-harness:managed -->` como por debajo de `<!-- END seb-harness:managed -->`, incluyendo caracteres especiales de shell o expresiones regulares (`$`, `&`, `\`, `/`, backticks), ambos segmentos DEBEN conservarse sin alteraciones ni expansiones indebidas tanto en actualización como en desinstalación.
- **017-CL-02 (Marcadores malformados o múltiples repeticiones):** Si `AGENTS.md` contiene dos bloques `BEGIN`/`END`, marcadores invertidos o un marcador huérfano, el instalador DEBE abortar con código `1` sin alterar el archivo (`017-RF-07`); durante la desinstalación estándar, si los marcadores están malformados, DEBE preservar `AGENTS.md` intacto y emitir una advertencia diagnóstica.
- **017-CL-03 (Archivo `AGENTS.md` solo con el bloque gestionado):** Si `AGENTS.md` contiene únicamente el bloque gestionado sin texto adicional del usuario, la actualización DEBE reemplazar el bloque limpiamente sin introducir líneas en blanco acumulativas entre ejecuciones sucesivas.
- **017-CL-04 (Re-ejecución en proyectos con `AGENTS.md.patch.md` previo):** La actualización del arnés no DEBE crear nuevos archivos `AGENTS.md.patch.md` en ningún escenario.
- **017-CL-05 (Desinstalación cuando `AGENTS.md` no tiene contenido propio del usuario):** Si al remover el bloque gestionado durante la desinstalación estándar (sin `--purge`) no queda contenido propio del proyecto fuera del bloque (archivo vacío o compuesto únicamente por espacios/líneas en blanco), EL SISTEMA DEBE eliminar el archivo `AGENTS.md` residual (respetando `--dry-run`).

## 8. Fuera de Alcance (Out of Scope)

- Diseño e implementación del flujo rápido para tareas menores (por ejemplo, `.agents/SEB-FAST.md` o nuevos modos en `seb guard`); esta spec solo deja preparada la arquitectura de enrutador y carga progresiva.
- Cambios en la lógica de verificación determinista (`seb verify`), en `seb.lock` o en el formato de identificadores prefijados de specs.
- Re-sellado retroactivo de verificadores históricos cerrados (`010`, `013`, `016`).

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos (`017-RF-01` a `017-RF-13`), requisitos no funcionales (`017-RNF-01` a `017-RNF-03`) y casos límite (`017-CL-01` a `017-CL-05`) cuentan con verificación programática en `[PASA]` ejecutada mediante `.seb/bin/seb verify separacion_protocolo_agents`.
- La suite completa de tests Go (`go test ./...`) y `go vet ./...` pasan sin errores ni omisiones (`t.Skip`).
- Cero violaciones a `docs/constitution.md`.

## 10. Dudas Abiertas y Puntos por Clarificar

Ninguna duda pendiente.
