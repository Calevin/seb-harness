# Spec 003 — Script de Desinstalación y Reversión del Arnés

- **Tarea ID:** `003`
- **Identificador:** `desinstalacion_arnes`
- **Origen:** Prosa / Petición del usuario Sebastian: "El proyecto requiere un archivo uninstall.sh que revierta todo lo que hace install.sh"
- **Estado:** `APROBADA`

## 1. Contexto y Objetivo (El POR QUÉ)

El script `install.sh` aprovisiona o actualiza `seb-harness` en proyectos destino (copiando componentes de `payload/` a `.agents/`, generando plantillas `AGENTS.md` y `MEMORY.md`, configurando directorios `docs/`, `specs/`, `.seb/`, compilando el motor `seb`, registrando entradas en `.gitignore` y vinculando hooks de Git) o en modo dogfooding (`--self`).

Si un desarrollador o equipo desea retirar el arnés de un repositorio de manera limpia, reversible y determinista, actualmente no dispone de un mecanismo automatizado para revertir dichos cambios. Una desinstalación manual es propensa a errores humanos (borrado accidental de configuraciones o hooks previos, líneas residuales en `.gitignore`, o eliminación inadvertida de especificaciones y modelos de dominio desarrollados por el usuario).

El objetivo es proveer una herramienta de desinstalación (`uninstall.sh`) que deshaga de forma quirúrgica e idempotente todas las modificaciones introducidas por `install.sh`, restaurando respaldos de hooks si existían, removiendo únicamente los artefactos distribuidos por el arnés y preservando por defecto el trabajo del usuario (especificaciones y constitución), salvo instrucción explícita de purga confirmada (`--purge`).

## 2. Actores y Usuarios

- **Desarrollador / Usuario Humano:** Ejecuta `uninstall.sh` para desinstalar el arnés de un proyecto de destino o revertir el modo `--self`.
- **Script de Desinstalación (`uninstall.sh`):** Inspecciona el entorno, valida componentes instalados, revierte configuraciones y elimina artefactos del arnés de forma segura.
- **Entorno Git / Repositorio Destino:** Sistema de control de versiones cuyos hooks y reglas de ignorado son restaurados a su estado previo al arnés.

## 3. Historias de Usuario

- **HU-01:** Como desarrollador que evaluó `seb-harness` en un proyecto y decide removerlo, quiero ejecutar un comando de desinstalación para que elimine los binarios, hooks y componentes del arnés sin alterar los archivos propios de mi repositorio.
- **HU-02:** Como desarrollador precavido, quiero que la desinstalación preserve por defecto mis archivos de especificación (`specs/`) y constitución (`docs/constitution.md`) creados durante el uso del arnés, a menos que indique expresamente una purga total.
- **HU-03:** Como desarrollador que utiliza dogfooding con la opción `--self`, quiero poder revertir el enlace simbólico `.agents` y los hooks de desarrollo sin destruir las fuentes maestras en `payload/` ni `engine/`.
- **HU-04:** Como operador de sistemas o CI, quiero poder simular la desinstalación mediante un modo de prueba (`--dry-run`) para auditar qué archivos y configuraciones serán modificados antes de aplicar los cambios.
- **HU-05:** Como operador de pipelines automatizados, quiero que cualquier intento de purga destructiva en entornos desatendidos aborte de forma segura a menos que se fuerce explícitamente con `--force`.

## 4. Definiciones de Dominio

- **Proyecto Destino (Target Project):** Repositorio externo donde se ejecutó `install.sh` para aprovisionar el arnés.
- **Modo `--self` (Dogfooding):** Modo de operación en el propio repositorio `seb-harness`, donde `.agents` es un enlace simbólico a `payload/.agents`.
- **Componentes Oficiales del Arnés:** Archivos y directorios provistos originalmente por `payload/.agents/` (`skills/`, `rules/`, `hooks/`).
- **Componentes Propios del Proyecto:** Habilidades, reglas o scripts creados por el usuario dentro de `.agents/` que no provienen del arnés.
- **Respaldos de Git Hooks (`*.bak-seb`):** Archivos de hooks preexistentes que `install.sh` renombró con sufijo `.bak-seb` al instalar sus propios enlaces.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

- **RF-01 (Ubiquitous):** EL SISTEMA DEBE aceptar como argumento opcional la ruta del directorio objetivo a desinstalar, asumiendo el directorio actual si no se proporciona ninguna ruta.
- **RF-02 (Event-Driven):** CUANDO se ejecute la desinstalación en un proyecto objetivo, EL SISTEMA DEBE eliminar los enlaces simbólicos de Git hooks (`pre-commit` y `post-commit`) que apunten a los hooks del arnés.
- **RF-03 (Event-Driven):** CUANDO se remuevan los hooks de Git asociados al arnés, EL SISTEMA DEBE restaurar los archivos de respaldo con sufijo `.bak-seb` a sus nombres originales (`pre-commit` y `post-commit`) en caso de existir.
- **RF-04 (Event-Driven):** CUANDO se ejecute la desinstalación en un proyecto objetivo, EL SISTEMA DEBE eliminar del archivo `.gitignore` del destino exactamente las entradas agregadas por el instalador (`.seb/bin/` y `.seb/seb-audit.db*`), preservando inalterado el resto del archivo.
- **RF-05 (Event-Driven):** CUANDO se ejecute la desinstalación en un proyecto objetivo, EL SISTEMA DEBE eliminar el directorio `.seb/` por completo (incluyendo binarios, base de datos local de auditoría, archivo de versión y cerrojos).
- **RF-06 (Event-Driven):** CUANDO se ejecute la desinstalación en un proyecto objetivo, EL SISTEMA DEBE eliminar dentro de `.agents/` únicamente los archivos y subdirectorios provistos por el arnés (coincidentes con los provistos en `payload/.agents/`), conservando cualquier habilidad o regla personalizada creada por el usuario.
- **RF-07 (Event-Driven):** CUANDO el directorio `.agents/` quede vacío tras remover los componentes provistos por el arnés, EL SISTEMA DEBE eliminar el directorio `.agents/`.
- **RF-08 (Event-Driven):** CUANDO se invoque la desinstalación con la opción `--self`, EL SISTEMA DEBE remover el enlace simbólico `.agents` y los hooks de Git vinculados al desarrollo del arnés, sin tocar el directorio `payload/` ni `engine/`.
- **RF-09 (Unwanted Behavior):** SI se invoca la opción `--self` junto con un argumento posicional de directorio destino, ENTONCES EL SISTEMA DEBE abortar la ejecución con un mensaje de error y código de salida `1`.
- **RF-10 (Unwanted Behavior):** SI se invoca la desinstalación apuntando al propio repositorio `seb-harness` sin especificar la opción `--self`, ENTONCES EL SISTEMA DEBE abortar la ejecución con un mensaje de advertencia y código de salida `1`.
- **RF-11 (Unwanted Behavior):** SI el directorio objetivo especificado no existe, ENTONCES EL SISTEMA DEBE abortar con un mensaje de error y código de salida `1`.
- **RF-12 (Optional Feature):** DONDE se invoque la opción `--dry-run`, EL SISTEMA DEBE mostrar en la salida estándar cada una de las operaciones y rutas que serían modificadas o eliminadas sin efectuar alteraciones reales en el sistema de archivos.
- **RF-13 (Optional Feature):** DONDE se invoque la opción `--purge`, EL SISTEMA DEBE solicitar confirmación interactiva al usuario (`¿Desea eliminar irreversiblemente specs y gobernanza? [y/N]`) antes de eliminar `specs/`, `AGENTS.md`, `MEMORY.md` y `docs/constitution.md`.
- **RF-14 (Optional Feature):** DONDE se invoque la opción `--force` (o `-f`) conjuntamente con `--purge`, EL SISTEMA DEBE ejecutar la purga completa omitiendo la solicitud de confirmación interactiva.
- **RF-15 (Unwanted Behavior):** SI se invoca la opción `--purge` en un entorno no interactivo (donde `stdin` no es una terminal/TTY) sin incluir la opción `--force` (o `-f`), ENTONCES EL SISTEMA DEBE abortar de inmediato con un mensaje explicativo y código de salida `1`.
- **RF-16 (Event-Driven):** CUANDO se ejecute la purga completa y se elimine `docs/constitution.md`, EL SISTEMA DEBE eliminar el directorio `docs/` únicamente si este queda completamente vacío, conservando cualquier otro archivo propio del proyecto existente en su interior.
- **RF-17 (Ubiquitous):** EL SISTEMA DEBE preservar intactos los directorios y archivos de especificación y gobernanza (`specs/`, `docs/`, `AGENTS.md`, `MEMORY.md`) cuando la opción `--purge` no esté presente.
- **RF-18 (Event-Driven):** CUANDO se solicite la ayuda mediante `-h` o `--help`, EL SISTEMA DEBE imprimir el manual de uso y opciones en la salida estándar y finalizar con código de salida `0`.

## 6. Requisitos No Funcionales (Verificables)

- **RNF-01 (Idempotencia):** La ejecución reiterada de la desinstalación sobre un directorio donde el arnés ya fue removido o parcialmente removido debe finalizar con código de salida `0` sin producir fallos ni inconsistencias.
- **RNF-02 (Portabilidad y Modo Estricto):** El script de desinstalación debe ejecutarse mediante Bash con directiva `set -euo pipefail`, empleando únicamente utilidades estándar de coreutils y Git sin dependencias externas adicionales.

## 7. Casos Límite (Edge Cases)

- **CL-01 (Hooks modificados o no pertenecientes al arnés):** Si un hook en `.git/hooks/` es un archivo regular o symlink que no apunta a los hooks del arnés, no debe ser eliminado ni sobrescrito.
- **CL-02 (`.gitignore` ausente o sin entradas del arnés):** Si `.gitignore` no existe o no contiene las entradas añadidas por el arnés, el script debe continuar su ejecución normalmente sin arrojar error.
- **CL-03 (`.gitignore` que solo contenía entradas del arnés):** Si al remover las líneas del arnés el archivo `.gitignore` queda con tamaño cero (vacío), debe manejarse limpiamente sin dejar residuos corruptos.
- **CL-04 (Directorio `.agents` que es symlink en destino no `--self`):** Si en un proyecto destino `.agents` es un symlink inesperado, debe reportar un error controlado en lugar de seguirlo o borrar su destino.
- **CL-05 (Purga en CI sin bandera de fuerza):** Invocaciones no interactivas con `--purge` deben fallar de forma temprana (`exit code 1`) protegiendo el historial en caso de scripts mal configurados.
- **CL-06 (Directorio `docs/` con contenido ajeno bajo `--purge`):** Si existen otros documentos (e.g. `docs/api.md`), `constitution.md` se elimina pero `docs/api.md` y la carpeta `docs/` se mantienen intactos.

## 8. Fuera de Alcance (Out of Scope)

- Eliminación de commits, ramas o tags creados en el repositorio Git durante el uso del arnés.
- Desinstalación de herramientas a nivel de sistema operativo (Go, Nix, compiladores locales).
- Modificación de archivos de código fuente creados por el usuario fuera de los artefactos del arnés.

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos funcionales (`RF-01` a `RF-18`) cuentan con verificación programática en estado `[PASA]` ejecutada mediante `.seb/bin/seb verify desinstalacion_arnes`.
- Cero violaciones a `docs/constitution.md`.
- El script cuenta con permisos de ejecución (`chmod +x`).

## 10. Dudas Abiertas y Puntos por Clarificar

Ninguna duda pendiente.
