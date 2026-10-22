# Handoff Document: Proyecto `seb-harness`

**Fecha de generación:** Octubre 2026
**Propósito:** Este documento sirve como punto de entrada y contexto técnico exhaustivo para cualquier agente de IA que asuma el desarrollo, mantenimiento o uso del proyecto `seb-harness`.

---

## 1. Naturaleza del Proyecto
`seb-harness` es un **Arnés de Desarrollo Dirigido por Especificaciones (SDD)** ligero, agnóstico al stack tecnológico y agnóstico al modelo de IA. Está diseñado para un desarrollador senior (15 años de experiencia, usuario de NixOS) que busca un equilibrio entre calidad y eficiencia sin la burocracia pesada de frameworks como OpenSpec.

**Filosofía Core:**
*   **Separación Generador/Evaluador:** El agente que escribe el código tiene prohibido evaluar su propio trabajo.
*   **Anti-Reward-Hacking:** Los contratos (specs y planes de prueba) se sellan criptográficamente para evitar que el agente relaje los tests si no logra hacerlos pasar.
*   **Evaluación Determinista:** La aprobación de una tarea depende 100% de la salida de comandos en terminal (exit code 0), no de inspección visual de la IA.
*   **Menor Privilegio:** Los skills y hooks limitan estrictamente qué archivos puede leer o modificar el agente en cada fase.

---

## 2. Arquitectura y Topología
El arnés funciona como una plantilla/SDK que se inyecta en proyectos existentes mediante un script de instalación (`install.sh`). 

### Estructura de Archivos del Arnés Base:
```text
seb-harness/
├── .agents/
│   ├── hooks/                  # Scripts bash inyectados en el ciclo del agente y de Git
│   │   ├── pre-tool.sh         # Inicia timer, bloquea comandos destructivos, invoca `seb guard`
│   │   ├── post-tool.sh        # Registra latencia, tokens y coste en SQLite
│   │   ├── git-pre-commit      # Puerta determinista: exige que `seb verify` pase para hacer commit
│   │   └── git-post-commit     # Registra el commit en el audit trail
│   ├── rules/
│   │   └── 00-seb-harness.md   # Regla global inyectada en Antigravity/Pi (Invariantes del harness)
│   └── skills/                 # El flujo SDD en 5 pasos (ver sección 4)
├── .seb/
│   ├── cmd/seb/main.go         # El Motor: Binario Go puro (sin dependencias dinámicas/CGO)
│   ├── seb-audit.db            # (Generado) Telemetría local (tokens, latencia). Ignorado en Git.
│   ├── seb.lock                # (Generado) Sellos SHA-256 de las specs. Trackeado en Git.
│   └── ACTIVE_TASK             # (Generado) Puntero a la tarea actual para hooks. Trackeado en Git.
├── AGENTS.md                   # Contexto universal del proyecto para agentes.
├── MEMORY.md                   # Bitácora viva: restricciones, callejones sin salida (max 150 líneas).
├── install.sh                  # Script que clona/instala el harness en un repo destino.
└── flake.nix                   # Entorno NixOS (Go + sqlite-interactive) para compilar el motor.
```

---

## 3. El Motor (`.seb/bin/seb`)
Desarrollado en Go (`cmd/seb/main.go`). Su objetivo es proveer sensores computacionales y auditoría que consuman $0 en tokens. Comandos principales:
*   `seb init`: Inicializa carpetas y DB.
*   `seb spec-next <tarea>`: Escanea el directorio `specs/` (fuente de verdad) para calcular el siguiente ID `001`, `002` de forma segura en multi-máquina.
*   `seb plan-lock <tarea>`: Calcula SHA-256 de la spec y el verificador, y los guarda en `.seb/seb.lock` (versión Git). Marca `.seb/ACTIVE_TASK`.
*   `seb verify <tarea>`: Comprueba los hashes anti-trampa. Ejecuta los comandos parseados de `*_verificador.md` con un timeout. Trunca salidas largas para proteger la ventana de contexto.
*   `seb guard <rol> <accion> <ruta>`: Valida el Principio de Menor Privilegio en <1ms (ej. bloquea a `implementador` de editar `constitution.md`).
*   `seb audit log / report`: Escribe y lee de SQLite (`seb-audit.db`).

---

## 4. El Flujo de Skills (Máquina de Estados SDD)
El arnés obliga al agente y al humano a seguir este flujo estricto:
1.  **`seb-iniciador` (Paso 0):** Lee manifiestos (`go.mod`, `package.json`, etc.) y genera `docs/constitution.md` (Secciones A-F: Stack, dominio, comandos de verificación base).
2.  **`seb-especificador` (Paso 1):** Toma la petición humana y genera `specs/<001>_<tarea>_spec.md` usando **sintaxis EARS** (Requisitos Funcionales Atómicos). Usa etiquetas `[NECESITA ACLARACIÓN]` si hay vacíos.
3.  **`seb-clarificador` (Paso 2):** Actúa como QA estricto. Lee la spec, detecta ambigüedades, entrevista al humano, actualiza la spec y la pasa a estado `APROBADA`.
4.  **`seb-planificador` (Paso 3):** Transforma la spec aprobada en `<tarea>_plan.md` (arquitectura, breakdown TDD) y `<tarea>_verificador.md` (tabla Markdown estricta con comandos ejecutables `CHK-XX`). Bloquea si un requisito no es testable programáticamente. Invoca `seb plan-lock`.
5.  **`seb-verificador` (Paso 4):** Se ejecuta tras la codificación. Llama a `seb verify`. Emite el veredicto `APROBADO` o `CAMBIOS NECESARIOS`.

---

## 5. Decisiones de Diseño Críticas (Aviso para Agentes Futuros)

### A. La Trampa del Estado Multi-Máquina (¡Resuelto!)
*Problema original:* Inicialmente, la numeración de specs y el "candado" de seguridad (hashes) se guardaban en SQLite. Esto rompía el flujo al cambiar de PC a Notebook vía Git (`git pull` traía archivos, pero SQLite quedaba vacío).
*Decisión:* 
1. **SQLite (`.seb/seb-audit.db`)** es estrictamente una caché local de telemetría pasiva (costos, tiempos). Se añade a `.gitignore`.
2. **Seguridad y Estado (`.seb/seb.lock` y `.seb/ACTIVE_TASK`)** son archivos de texto plano que mantienen los hashes SHA-256 y la tarea actual. Estos **DEBEN** ser trackeados en Git.
3. `seb spec-next` ya no cuenta filas en SQLite, sino que escanea los archivos físicos en la carpeta `specs/`.

### B. Anti-Reward-Hacking
El Agente Implementador (el que escribe el código de la app) tiene prohibido modificar:
*   `docs/constitution.md`
*   `specs/*_spec.md`
*   `specs/*_verificador.md`
Esto se fuerza mediante el hook `pre-tool.sh`, `seb guard`, y comprobaciones de hashes pre-ejecución en `seb verify`.

### C. Estrategia de Instalación (`install.sh`)
Para mantener el arnés agnóstico, no se usan submódulos de Git (lo cual rompería las rutas de agentes que buscan `.agents/` en la raíz). El script `install.sh` copia los directorios `.agents/` y `.seb/` al proyecto destino, enlaza los git hooks, compila el binario de Go localmente en el destino y ajusta el `.gitignore`.

---

## 6. Estado Actual y Siguientes Pasos
El sistema base está teóricamente completo. Todos los hooks, skills, la base de datos, el archivo Nix y el binario Go han sido redactados.

**Próximos pasos sugeridos para el usuario y el nuevo agente:**
1.  Aplicar `install.sh` sobre un proyecto real (como el de React `Brienne` mencionado en el historial).
2.  Ejecutar el `seb-iniciador` para probar la generación de `constitution.md`.
3.  Probar un ciclo completo (Especificar -> Clarificar -> Planificar -> Implementar -> Verificar) para asegurar que el parsing del binario Go sobre las tablas Markdown funciona correctamente bajo estrés real.
4.  Refinar los prompts de los skills si el modelo específico (Pi, Gemini 3.1, Claude, etc.) demuestra alucinaciones recurrentes con la notación EARS.