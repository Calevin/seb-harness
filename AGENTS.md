# AGENTS.md — seb-harness (el proyecto en sí)

> Este archivo es el contexto del REPO seb-harness.
> La plantilla que se instala en otros proyectos está en `payload/AGENTS.md`.

## Qué es este repo
Arnés SDD (Spec-Driven Development) agnóstico a stack y modelo. Contiene:
- `payload/`  → lo que `install.sh` copia a los proyectos destino (.agents/, AGENTS.md, MEMORY.md).
- `engine/`   → código fuente Go del binario `seb` (se compila hacia `<destino>/.seb/bin/seb`).
- `install.sh`→ instalador / actualizador.
- `.agents`   → symlink a `payload/.agents` (este repo usa su propio harness).

## Stack
Go (sin CGO), bash, Nix (flake.nix).

## Reglas propias del repo
- Los skills y hooks se editan SIEMPRE en `payload/.agents/` (el symlink `.agents` es solo dogfooding).
- No mezclar plantillas (`payload/*.md`) con los archivos de contexto de este repo (raíz).
- Cambios en `payload/` o `engine/` deben probarse con `./install.sh` sobre un proyecto real.

## Verificación
- Compilar motor: `cd engine && CGO_ENABLED=0 go build ./cmd/seb`
- Tests: `cd engine && go test ./...`

## Memoria
Ver `MEMORY.md` (restricciones y callejones sin salida de seb-harness).
