# Spec 012 — Soporte de Integración y Formato de Hooks para Claude (Claude Code y Claude Desktop)

- **Tarea ID:** `012`
- **Identificador:** `soporte_claude`
- **Origen:** Solicitud de usuario / `Tarea soporte de formato.md`
- **Estado:** `APROBADA`
- **Afecta-UC:** `nuevo`

## 1. Contexto y Objetivo (El POR QUÉ)

Actualmente, `seb-harness` se distribuye con scripts de hooks en `.agents/hooks/` (`pre-tool.sh`, `post-tool.sh`, `stop.sh`) que invocan a `.seb/bin/seb hook <evento>`. Tanto el contrato de entrada como el formato de salida de estos hooks están acoplados al protocolo de Antigravity 2.0 / agy CLI (por ejemplo, emitiendo `{"decision":"allow"}`).

Cuando un proyecto consumidor conecta estos hooks y directivas a Claude Code o Claude Desktop mediante enlaces simbólicos en `.claude/` y el registro en `.claude/settings.json`, surge un fallo crítico de gobernanza: Claude Code no interpreta el formato de salida de Antigravity. Ante un bloqueo emitido por `seb`, Claude ignora la denegación y ejecuta la acción de todas formas, anulando las protecciones anti-reward-hacking y de menor privilegio del arnés. Adicionalmente, los nombres de herramientas y el esquema de entrada JSON difieren entre ambos clientes, y la configuración manual de enlaces y `.claude/settings.json` resulta propensa a errores.

Esta especificación define el soporte nativo para Claude Code y Claude Desktop (los cuales comparten exactamente los mismos artefactos de proyecto en `.claude/` y `CLAUDE.md`) en tres dimensiones:
1. Aprovisionamiento automático por defecto, idempotente y no destructivo de enlaces simbólicos (`.claude/hooks`, `.claude/rules`, `.claude/skills` hacia `.agents/`) y registro en `.claude/settings.json`.
2. Soporte explícito del formato de entrada/salida y normalización de herramientas para Claude en el motor de hooks de `seb`, preservando intacto el comportamiento actual de Antigravity por defecto.
3. Reversibilidad quirúrgica durante la desinstalación (`uninstall.sh`), removiendo únicamente los enlaces y registros introducidos por el arnés.

## 2. Actores y Usuarios

- **Desarrollador / Operador del Arnés:** Instala, actualiza, desinstala y opera `seb-harness` en proyectos consumidores que utilizan Claude Code, Claude Desktop o Antigravity.
- **Cliente Claude (Claude Code / Claude Desktop):** Agente de IA que consume reglas, habilidades, `CLAUDE.md` y hooks del proyecto desde `.claude/`, envía eventos JSON por entrada estándar e interpreta las decisiones de permiso o bloqueo.
- **Motor del Arnés (`seb`):** CLI responsable de evaluar permisos, registrar telemetría y emitir decisiones conforme al formato configurado.
- **Instalador y Desinstalador (`install.sh` / `uninstall.sh`):** Procesos automatizados encargados de aprovisionar y revertir enlaces simbólicos, scripts plantilla y configuración sin destruir personalizaciones previas.

## 3. Historias de Usuario

- **012-HU-01:** Como desarrollador que trabaja con Claude Code o Claude Desktop, quiero que el instalador genere por defecto los enlaces `.claude/hooks`, `.claude/rules` y `.claude/skills` hacia `.agents/` y registre los hooks en `.claude/settings.json` solo cuando no existan previamente, para habilitar `seb-harness` sin perder mi configuración personal.
- **012-HU-02:** Como operador del arnés, quiero que `seb hook` acepte un selector explícito de formato para Claude y traduzca los bloqueos y eventos al contrato oficial de Claude Code, para que las reglas de menor privilegio y anti-reward-hacking se apliquen de forma efectiva.
- **012-HU-03:** Como mantenedor de proyectos multi-agente, quiero que el formato por defecto siga siendo Antigravity idéntico byte a byte y que los scripts plantilla reenvíen los argumentos de formato, para evitar regresiones en instalaciones existentes y evitar duplicar scripts.
- **012-HU-04:** Como desarrollador, quiero que al desinstalar el arnés se eliminen únicamente los enlaces y registros en `.claude/` creados por `seb-harness`, para dejar el repositorio limpio sin afectar mis configuraciones previas.

## 4. Definiciones de Dominio

- **Contrato de Formato (`antigravity` | `claude`):** Esquema de entrada JSON por entrada estándar, salida JSON por salida estándar y código de retorno esperado por cada cliente de IA en los eventos `PreToolUse`, `PostToolUse` y `Stop`.
- **Selector Explícito de Formato:** Parámetro `--format <antigravity|claude>` en la invocación del hook (o variable de entorno de respaldo `SEB_HOOK_FORMAT`) que determina el serializador y parser a utilizar sin recurrir a heurísticas implícitas.
- **Aprovisionamiento No Destructivo:** Política de instalación ejecutada por defecto que crea enlaces simbólicos o entradas de configuración únicamente cuando el destino o registro no existe, preservando cualquier archivo, directorio o clave preexistente.

## 5. Requisitos Funcionales (Criterios de Aceptación en EARS)

### Aprovisionamiento e Integración en `.claude/`

- **012-RF-01 (Event-Driven):** CUANDO se ejecute el instalador del arnés (`install.sh`) en un proyecto destino, EL SISTEMA DEBE crear por defecto los enlaces simbólicos `.claude/hooks`, `.claude/rules` y `.claude/skills` apuntando a sus directorios equivalentes en `.agents/` únicamente cuando cada destino respectivo no exista previamente.
- **012-RF-02 (Unwanted Behavior):** SI alguno de los destinos `.claude/hooks`, `.claude/rules` o `.claude/skills` ya existe en el proyecto al ejecutar la instalación (como directorio, archivo o enlace simbólico previo), ENTONCES EL SISTEMA DEBE conservar el elemento existente sin sobrescribirlo ni eliminarlo y continuar con el resto del aprovisionamiento.
- **012-RF-03 (Event-Driven):** CUANDO se ejecute el instalador del arnés (`install.sh`), EL SISTEMA DEBE crear `.claude/settings.json` o agregar las entradas faltantes para `PreToolUse`, `PostToolUse` y `Stop` con `command` apuntando a `$CLAUDE_PROJECT_DIR/.claude/hooks/<script>.sh --format claude`, únicamente cuando dicho archivo o registro específico de hook no exista previamente.
- **012-RF-04 (Unwanted Behavior):** SI `.claude/settings.json` ya existe con configuraciones de usuario o hooks previos, ENTONCES EL SISTEMA DEBE fusionar únicamente los registros ausentes preservando íntegramente las claves, permisos y hooks preexistentes sin duplicarlos.
- **012-RF-05 (Ubiquitous):** EL SISTEMA DEBE garantizar que la ejecución repetida del instalador sea idempotente, sin generar enlaces rotos, enlaces anidados ni entradas duplicadas en `.claude/settings.json`.

### Selección Explícita de Formato en Hooks

- **012-RF-06 (Optional Feature):** DONDE se indique el formato `claude` mediante el flag `--format claude` o la variable de entorno `SEB_HOOK_FORMAT=claude`, EL SISTEMA DEBE parsear la entrada y serializar la salida del hook conforme al contrato de Claude Code.
- **012-RF-07 (Ubiquitous):** EL SISTEMA DEBE utilizar el formato `antigravity` por defecto cuando no se proporcione ningún selector explícito, produciendo una salida idéntica byte a byte al comportamiento previo en todos los eventos.
- **012-RF-08 (Unwanted Behavior):** SI se especifica un valor de formato distinto de `antigravity` o `claude`, ENTONCES EL SISTEMA DEBE rechazar la ejecución emitiendo un mensaje de error claro por salida de error estándar y terminando con código de salida distinto de cero.

### Parseo de Entrada y Normalización de Herramientas (Claude)

- **012-RF-09 (Event-Driven):** CUANDO se invoque un hook en formato `claude` con un payload JSON por entrada estándar (`session_id`, `transcript_path`, `cwd`, `hook_event_name`, `tool_name`, `tool_input` y `tool_response` en post-ejecución), EL SISTEMA DEBE extraer el identificador de sesión, ruta de trabajo, herramienta y parámetros objetivo tolerando la ausencia de campos opcionales.
- **012-RF-10 (Ubiquitous):** EL SISTEMA DEBE normalizar los nombres de herramientas de Claude (`Bash`, `Edit`, `Write`, `Read`, `MultiEdit` y equivalentes) hacia las operaciones internas evaluadas por las reglas de menor privilegio y telemetría de `seb`.
- **012-RF-11 (Unwanted Behavior):** SI la entrada estándar en formato `claude` contiene JSON inválido o vacío, ENTONCES EL SISTEMA DEBE evitar un fallo no controlado y finalizar de forma segura con código de salida `0` y sin salida bloqueante para no corromper la sesión, registrando el diagnóstico cuando corresponda.

### Serialización de Salida por Evento y Scripts Plantilla

- **012-RF-12 (Event-Driven):** CUANDO una evaluación de `PreToolUse` en formato `claude` resulte en permiso (`allow`), EL SISTEMA DEBE finalizar con código de salida `0` y salida vacía en salida estándar (`stdout`), permitiendo continuar el flujo normal de permisos de Claude Code sin omitir sus confirmaciones interactivas.
- **012-RF-13 (Event-Driven):** CUANDO una evaluación de `PreToolUse` en formato `claude` resulte en bloqueo (`deny`), EL SISTEMA DEBE finalizar con código de salida `0` y emitir un JSON válido en `stdout` con `hookSpecificOutput.hookEventName` igual a `"PreToolUse"`, `hookSpecificOutput.permissionDecision` igual a `"deny"` y `hookSpecificOutput.permissionDecisionReason` con el motivo del bloqueo, omitiendo los campos deprecados de nivel superior `decision` y `reason`.
- **012-RF-14 (Event-Driven):** CUANDO una evaluación de `PostToolUse` en formato `claude` finalice sin hallazgos bloqueantes, EL SISTEMA DEBE finalizar con código de salida `0` y salida vacía en `stdout`; y CUANDO requiera inyectar contexto adicional a Claude tras la ejecución, EL SISTEMA DEBE emitir un JSON válido con `hookSpecificOutput.hookEventName` igual a `"PostToolUse"` y `hookSpecificOutput.additionalContext` conteniendo el mensaje.
- **012-RF-15 (Event-Driven):** CUANDO el evento `Stop` en formato `claude` reciba `stop_hook_active` con valor `true` en la entrada JSON, EL SISTEMA DEBE finalizar con código de salida `0` y salida vacía en `stdout` evitando devolver una decisión de bloqueo (`block`) para prevenir bucles infinitos de detención.
- **012-RF-16 (Event-Driven):** CUANDO se invoquen los scripts plantilla de hooks (`.agents/hooks/*.sh`), EL SISTEMA DEBE reenviar los argumentos recibidos (`"$@"`) hacia el binario `seb hook` y, ante ausencia del binario, aplicar un comportamiento de respaldo compatible con el formato solicitado (código de salida `0` sin salida en formato `claude`).

### Reversibilidad en Desinstalación y Documentación

- **012-RF-17 (Event-Driven):** CUANDO se ejecute el desinstalador (`uninstall.sh`), EL SISTEMA DEBE eliminar los enlaces simbólicos `.claude/hooks`, `.claude/rules` y `.claude/skills` únicamente cuando apunten hacia `.agents/`, preservando cualquier directorio o archivo real del usuario.
- **012-RF-18 (Event-Driven):** CUANDO se ejecute el desinstalador (`uninstall.sh`) y exista `.claude/settings.json`, EL SISTEMA DEBE remover únicamente los registros de hooks correspondientes a `seb-harness` (`$CLAUDE_PROJECT_DIR/.claude/hooks/*.sh`), preservando intactas todas las demás claves y hooks del usuario.
- **012-RF-19 (Ubiquitous):** EL SISTEMA DEBE documentar en la guía operativa del repositorio la matriz de clientes soportados (Antigravity, Claude Code y Claude Desktop), el uso del selector de formato y el procedimiento de verificación end-to-end.

## 6. Requisitos No Funcionales (Verificables)

- **012-RNF-01 (Latencia):** La ejecución de `seb hook` en formato `claude` debe completar la evaluación y serialización en menos de `10ms` sin requerir conectividad de red.
- **012-RNF-02 (No Regresión):** La salida de `seb hook` en modo por defecto (`antigravity`) debe coincidir exactamente byte a byte con la salida previa ante entradas equivalentes de `allow`, `deny`, `PostToolUse` y `Stop`.
- **012-RNF-03 (Dependencias):** La solución no debe introducir dependencias externas nuevas en el motor Go ni herramientas externas no estándar en los scripts de instalación y hooks.
- **012-RNF-04 (Portabilidad):** Los comandos registrados en `.claude/settings.json` deben utilizar la variable `$CLAUDE_PROJECT_DIR/.claude/hooks/<script>.sh` sin incrustar rutas absolutas del equipo local.

## 7. Casos Límite (Edge Cases)

- **012-CL-01 (Precedencia de selector de formato):** Si se especifican simultáneamente el flag `--format` y la variable de entorno `SEB_HOOK_FORMAT`, el flag `--format` tiene prioridad sobre la variable de entorno.
- **012-CL-02 (`.claude/settings.json` parcial o con otros hooks):** Si `.claude/settings.json` existe sin objeto `hooks`, o con hooks personalizados en los mismos eventos, se preservan todas las entradas existentes y solo se añaden los registros faltantes de `seb-harness`.
- **012-CL-03 (`.claude/settings.json` con JSON corrupto):** Si `.claude/settings.json` existe y no es un JSON sintácticamente válido, el instalador y el desinstalador deben abortar su modificación reportando el error sin alterar el archivo existente.
- **012-CL-04 (Binario `.seb/bin/seb` ausente con `--format claude`):** El script plantilla debe terminar con código de salida `0` sin emitir texto ni JSON malformado que cause un bloqueo falso en Claude.
- **012-CL-05 (Recursión en `Stop`):** Cuando `stop_hook_active` sea `true`, el hook `Stop` nunca debe emitir `{"decision":"block"}`, independientemente de otras condiciones de evaluación.
- **012-CL-06 (Limpieza de `.claude/` en desinstalación):** Si tras remover los symlinks y registros de `seb` en `uninstall.sh` el directorio `.claude/` queda vacío (o `.claude/settings.json` queda sin configuración útil de usuario), el desinstalador debe manejar la limpieza sin dejar artefactos rotos y respetando el modo `--dry-run`.

## 8. Fuera de Alcance (Out of Scope)

- Emisión de decisiones `"ask"` o `"defer"` en `PreToolUse` para Claude Code.
- Integración de formato de hooks o aprovisionamiento automático para otros clientes de IA distintos de Antigravity, Claude Code y Claude Desktop.
- Implementación de un servidor MCP específico para Claude Desktop.
- Alteración de la lógica de permisos de los 6 skills SDD más allá del mapeo normalizado de nombres de herramientas de Claude.

## 9. Criterios de Finalización (Definition of Done)

- Todos los requisitos (`012-RF-01` a `012-RF-19`, `012-RNF-01` a `012-RNF-04` y `012-CL-01` a `012-CL-06`) cuentan con verificación programática en `[PASA]` ejecutada mediante `.seb/bin/seb verify soporte_claude`.
- Cero violaciones a `docs/constitution.md`.

## 10. Dudas Abiertas y Puntos por Clarificar

Ninguna duda pendiente.
