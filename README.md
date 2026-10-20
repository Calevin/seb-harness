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
