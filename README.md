# seb-harness

```
proyecto-objetivo/
├── .seb/                          # Motor interno del harness (agnóstico)
│   ├── bin/
│   │   └── seb                    # CLI compilado en Go (ejecutor, auditor, DB manager)
│   ├── seb-audit.db               # SQLite: telemetría, sesiones, ejecuciones, costos y estado
│   └── config.json                # Mapeo de precios OpenRouter, timeouts y permisos
├── .agents/                       # Estándar unificado (compatible con Antigravity y Pi via symlinks)
│   ├── rules/
│   │   └── 00-seb-harness.md      # Regla global que inyecta consciencia del harness y menor privilegio
│   ├── skills/
│   │   ├── seb-iniciador/
│   │   │   └── SKILL.md
│   │   ├── seb-especificador/
│   │   │   └── SKILL.md
│   │   ├── seb-clarificador/
│   │   │   └── SKILL.md
│   │   ├── seb-planificador/
│   │   │   └── SKILL.md
│   │   └── seb-verificador/
│   │       └── SKILL.md
│   └── hooks/                     # Hooks de ciclo de vida del agente (agy / pi)
│       ├── pre-tool.sh            # Guardrail de menor privilegio + inicio de timer
│       └── post-tool.sh           # Registro en SQLite (latencia, tool, args, exit code)
├── .git/hooks/                    # Git hooks (instalados vía `seb init`)
│   ├── pre-commit                 # Bloquea commits si verificador.md de la tarea activa no está en PASA
│   └── post-commit                # Registra el hash del commit en seb-audit.db vinculado a la spec
├── docs/
│   └── constitution.md            # Generado por seb-iniciador (1 vez por proyecto)
├── specs/                         # Artefactos SDD planos y directos (sin burocracia)
│   ├── 001_login_spec.md
│   ├── login_plan.md
│   └── login_verificador.md
├── AGENTS.md                      # Punto de entrada universal de contexto para cualquier agente
└── MEMORY.md                      # Memoria viva: callejones sin salida, riesgos y limitaciones
```


## Matriz de Clientes Soportados y Formato de Hooks

| Cliente de IA | Artefactos de Integración | Formato de Hooks | Configuración |
| :--- | :--- | :--- | :--- |
| **Antigravity** | `.agents/{hooks,rules,skills}`, `AGENTS.md` | `antigravity` (por defecto) | `.agents/hooks.json` |
| **Claude Code** | `.claude/{hooks,rules,skills}` -> `../.agents/*`, `CLAUDE.md` | `claude` (`--format claude`) | `.claude/settings.json` (`$CLAUDE_PROJECT_DIR/.claude/hooks/<script>.sh --format claude`) |
| **Claude Desktop** | Comparte `.claude/{hooks,rules,skills}`, `.claude/settings.json` y `CLAUDE.md` | `claude` (`--format claude`) | `.claude/settings.json` |

### Selección de Formato

- El subcomando `.seb/bin/seb hook <pre-tool|post-tool|stop>` acepta `--format antigravity|claude` (por defecto `antigravity`) o la variable de entorno de respaldo `SEB_HOOK_FORMAT`. El flag `--format` tiene precedencia sobre la variable de entorno.
- Los scripts plantilla en `.agents/hooks/{pre-tool,post-tool,stop}.sh` reenvían todos sus argumentos (`"$@"`) al binario `seb` y respetan `--format claude` incluso en el fallback si el binario no existe.
- `install.sh` crea automáticamente `.claude/{hooks,rules,skills}` (solo si no existen) y fusiona `.claude/settings.json` de forma idempotente y no destructiva. `uninstall.sh` revierte únicamente los enlaces hacia `.agents/` y los registros de `seb-harness`.

### Validación End-to-End Manual (Claude Code)

1. Instalar `seb-harness` en un proyecto consumidor ejecutando `./install.sh /ruta/proyecto`.
2. Verificar que `.claude/settings.json` registra `PreToolUse`, `PostToolUse` y `Stop` apuntando a `$CLAUDE_PROJECT_DIR/.claude/hooks/<script>.sh --format claude`.
3. Abrir el proyecto en Claude Code y solicitar una acción protegida por menor privilegio en rol `implementador` (por ejemplo, editar `docs/constitution.md` o `specs/*_verificador.md`).
4. Confirmar que el hook `PreToolUse` devuelve `permissionDecision: "deny"` con el motivo del guardrail y Claude Code aborta la edición sin modificar el archivo.
