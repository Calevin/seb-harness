# MEMORY.md — seb-harness (el proyecto en sí)

> Bitácora viva del repo seb-harness (máx. 150 líneas).
> La plantilla para proyectos destino está en `payload/MEMORY.md`.

## Decisiones vigentes
- Estado compartido entre máquinas en `.seb/seb.lock` y `.seb/ACTIVE_TASK` (texto plano, en Git).
- SQLite (`.seb/seb-audit.db`) es solo telemetría local, ignorada por Git.
- Estructura `payload/` (se instala) vs `engine/` (se compila) vs raíz (el repo mismo).

## Callejones sin salida
- (vacío por ahora)
