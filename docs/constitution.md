# Constitución del Proyecto — seb-harness

> **Ley Suprema (`seb-harness`):** Este documento define los principios innegociables del proyecto. Toda especificación (`*_spec.md`), plan (`*_plan.md`), verificador (`*_verificador.md`) e implementación debe cumplirlos estrictamente.

## A. Naturaleza del Proyecto

- **Propósito y Misión:** `seb-harness` es un arnés de Desarrollo Dirigido por Especificaciones (SDD) y Evaluación Determinista agnóstico al stack tecnológico y al modelo de IA. Su propósito es garantizar la máxima calidad técnica eliminando la burocracia pesada, separando la generación de la evaluación y automatizando guardarraíles de menor privilegio y anti-reward-hacking.
- **Usuarios / Consumidores:** Desarrollador senior (entorno NixOS) y agentes autónomos de IA (Antigravity, Pi, etc.) operando en modo pair programming o ejecución autónoma.
- **Tipo de Sistema y Alcance:** CLI portable compilado en Go puro (`engine/` -> `.seb/bin/seb`), templates y skills (`payload/.agents/`) distribuidos mediante script de instalación (`install.sh`), y base de auditoría SQLite local con soporte de sincronización Git multi-máquina.

## B. Stack Tecnológico (Reglas de Implementación)

- **Lenguaje y Runtime:** Go 1.26+ en entorno NixOS (`flake.nix` + `.envrc` con `nix-direnv`).
- **Frameworks y Librerías Core:** Librería estándar de Go + `modernc.org/sqlite` (implementación de SQLite en Go puro, sin CGO).
- **Persistencia e Infraestructura:**
  - **SQLite Local (`.seb/seb-audit.db`):** Base de datos de telemetría pasiva (latencias, tokens, costos USD, logs de herramientas). Estrictamente local, excluida de Git en `.gitignore`.
  - **Estado Git Multi-Máquina (`.seb/seb.lock` y `.seb/ACTIVE_TASK`):** Archivos planos versionados en Git con sellos criptográficos SHA-256 de specs y verificadores, y puntero de tarea activa.
- **Comandos Oficiales de Verificación Programática:**
  - **Compilación / Build:** `direnv exec . bash -c "cd engine && CGO_ENABLED=0 go build -ldflags=\"-s -w\" -o ../.seb/bin/seb ./cmd/seb"`
  - **Tests Unitarios / Integración:** `direnv exec . bash -c "cd engine && go test -v ./..."`
  - **Linter / Análisis Estático:** `direnv exec . bash -c "cd engine && go vet ./..."`
- **Prohibiciones de Stack:**
  - Prohibido habilitar CGO (`CGO_ENABLED=0` mandatorio) para evitar ligaduras dinámicas del sistema.
  - Prohibido incorporar dependencias externas en `engine/` salvo aprobación expresa y justificada en la spec/plan.
  - Prohibido trasladar los sellos criptográficos (`seb.lock`) o la numeración de specs exclusivamente a SQLite; el estado crítico debe preservarse vía Git o escaneo físico del árbol `specs/`.

## C. Reglas de Dominio y Lógica de Negocio

- **Invariantes del Dominio:**
  - **Separación Estricta Generador / Evaluador:** El agente que implementa el código jamás puede evaluar su propio trabajo ni modificar sus criterios de aceptación.
  - **Verificación Determinista:** Ninguna tarea se da por aprobada sin el retorno exitoso (`exit code 0`) ejecutado por `.seb/bin/seb verify <tarea>`. La inspección visual del agente no tiene validez probatoria.
  - **Anti-Reward-Hacking:** Los archivos `docs/constitution.md`, `specs/*_spec.md` y `specs/*_verificador.md` son inmutables durante la fase de implementación.
  - **Autonomía Multi-Máquina:** Un clon limpio del repositorio sin `.seb/seb-audit.db` debe ser capaz de determinar el siguiente ID de spec (`seb spec-next`) y validar tareas existentes (`seb verify`) únicamente con el contenido trackeado en Git.
- **Fronteras de Datos y Consistencia:**
  - Sincronización idempotente: `.seb/bin/seb init` y `.seb/bin/seb spec-next` deben tolerar ejecuciones repetidas sin corromper esquemas ni duplicar secuencias.
  - La numeración secuencial de specs (`001`, `002`...) se deriva escaneando físicamente el directorio `specs/` en disco.
- **Aislamiento de Lógica:**
  - Los skills de `.agents/skills/` deben permanecer desacoplados de cualquier modelo específico de LLM, utilizando exclusivamente prompts universales y herramientas estándar (`.seb/bin/seb`, bash, git).

## D. Estructura y Estilo de Código

- **Topología de Directorios:**
  - `engine/`: Código fuente en Go del CLI `seb` (`cmd/seb/main.go`, `go.mod`, `go.sum`).
  - `payload/`: Plantillas maestras y artefactos a distribuir (`.agents/skills/`, `.agents/rules/`, `.agents/hooks/`, `AGENTS.md`, `MEMORY.md`).
  - `.agents/`: Symlink local a `payload/.agents/` para habilitar dogfooding en el propio arnés.
  - `docs/`: Documentación arquitectónica (`constitution.md`, `handoff.md`).
  - `specs/`: Directorio raíz de especificaciones EARS, planes y tablas de verificación programática.
  - `.seb/`: Binario compilado (`bin/seb`), base local SQLite (`seb-audit.db`) y archivos de candado (`seb.lock`, `ACTIVE_TASK`).
  - `install.sh`: Script de aprovisionamiento y actualización para proyectos destino y modo `--self`.
  - `flake.nix`: Definición de entorno Nix para Go y SQLite.
- **Patrones Arquitectónicos:**
  - CLI monolítico en Go con subcomandos modulares y dependencias mínimas.
  - Guardarraíles de menor privilegio ejecutados en `<1ms` mediante subcomando `seb guard`.
- **Convenciones de Nomenclatura e Idioma:**
  - Documentación de gobernanza, specs, planes y commits en español formal y conciso.
  - Código fuente en Go, identificadores y comentarios técnicos en inglés idiomático o español según consistencia del módulo.
- **Prohibiciones de Diseño:**
  - Prohibido acoplar el CLI `seb` a APIs externas que requieran conectividad de red obligatoria en runtime para operaciones de verificación o guard.
  - Prohibido el uso de submódulos de Git para la distribución del arnés (se usa `install.sh` y copia versionada).

## E. Manejo de Errores y Validaciones

- **Estrategia de Errores:**
  - En Go: Retorno explícito `error` y salida controlada por `os.Exit(code)` en subcomandos de CLI.
  - En CLI: Salida de diagnósticos a `stderr` con mensajes claros y prefijados (`[BLOQUEO SDD]`, `[ERROR]`), retornando `exit code 1` o códigos específicos ante denegaciones de guard.
- **Validación de Entradas:**
  - Saneamiento estricto de nombres de tareas (`sanitizeName`) para evitar directory traversal o caracteres no válidos en nombres de archivo.
  - Detección mandatoria de etiquetas pendientes `[NECESITA ACLARACIÓN]` antes de permitir `seb plan-lock`.
- **Observabilidad y Seguridad:**
  - Auditoría pasiva en `agent_actions` y `verifications` dentro de `.seb/seb-audit.db`.
  - Prohibición absoluta de registrar en SQLite o terminal secretos, tokens de API o rutas privadas absolutas que expongan el host fuera del workspace.

## F. Comportamiento del Agente de IA (Reglas SDD — `seb-harness`)

1. **La Spec Manda:** Ninguna funcionalidad se implementa sin un archivo `specs/<NNN>_<tarea>_spec.md` aprobado y sin etiquetas `[NECESITA ACLARACIÓN]`.
2. **Testabilidad Programática Obligatoria:** Ningún plan (`specs/<tarea>_plan.md`) es válido sin su correspondiente `specs/<tarea>_verificador.md` con comandos que devuelvan código de salida `0`.
3. **Anti-Reward-Hacking:** Está terminantemente prohibido modificar `docs/constitution.md`, `specs/*_spec.md` o `specs/*_verificador.md` durante la implementación, así como deshabilitar o relajar tests para forzar que pasen.
4. **Memoria Persistente (`MEMORY.md`):** Antes de planificar o codificar, el agente debe consultar `MEMORY.md` para evitar callejones sin salida ya documentados, y registrar allí cualquier nueva limitación técnica descubierta.
5. **Commits Protegidos:** Todo commit de código debe pasar la puerta determinista `.seb/bin/seb verify <tarea>`.
