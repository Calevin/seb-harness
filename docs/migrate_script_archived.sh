#!/usr/bin/env bash
# migrate.sh — Reorganiza el repo seb-harness en payload/ + engine/.
# Ejecutar UNA sola vez, desde cualquier lugar dentro del repo seb-harness.
# Requiere working tree limpio (los archivos sin trackear no cuentan):
# si algo sale mal, `git reset --hard && git clean -fd` te devuelve al estado inicial.

set -euo pipefail

info() { printf '\033[1;34m->\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31mxx\033[0m %s\n' "$*" >&2; exit 1; }

cd "$(git rev-parse --show-toplevel 2>/dev/null)" || die "No estás dentro de un repo git."

# --- Precondiciones --------------------------------------------------------
if [ -L .agents ] || [ -d payload/.agents ]; then
    die "Parece que ya migraste (.agents es symlink o payload/.agents existe)."
fi
[ -d .agents ]  || die "No encuentro .agents/ en la raíz."
[ -d .seb/cmd ] || die "No encuentro .seb/cmd/ (¿es este el repo seb-harness?)."
if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
    die "Hay cambios sin commitear en archivos trackeados. Commiteá o stasheá antes de migrar."
fi

# git mv si el origen está trackeado; mv común si no.
move() {
    local src="$1" dst="$2"
    [ -e "$src" ] || { warn "No existe $src, se omite."; return 0; }
    mkdir -p "$(dirname "$dst")"
    if git ls-files --error-unmatch -- "$src" >/dev/null 2>&1; then
        git mv "$src" "$dst"
    else
        mv "$src" "$dst"
    fi
}

# --- 1. Mover lo instalable a payload/ ------------------------------------
info "Moviendo .agents/, AGENTS.md y MEMORY.md a payload/ ..."
mkdir -p payload
move .agents    payload/.agents
move AGENTS.md  payload/AGENTS.md
move MEMORY.md  payload/MEMORY.md

# --- 2. Mover el código del motor a engine/ -------------------------------
info "Moviendo el código Go a engine/ ..."
mkdir -p engine
move .seb/cmd    engine/cmd
move .seb/go.mod engine/go.mod
move .seb/go.sum engine/go.sum

# --- 3. Documentación ------------------------------------------------------
info "Moviendo el handoff a docs/ ..."
mkdir -p docs specs
move documento_de_transferencia_handoff.md docs/handoff.md

# --- 4. Estado de runtime: que no quede trackeado --------------------------
for f in .seb/bin .seb/seb-audit.db .seb/seb-audit.db-shm .seb/seb-audit.db-wal; do
    if git ls-files --error-unmatch -- "$f" >/dev/null 2>&1; then
        info "Destrackeando $f (es estado local)"
        git rm -r --cached -q -- "$f"
    fi
done

# --- 5. Dogfooding: .agents -> payload/.agents -----------------------------
info "Creando symlink .agents -> payload/.agents ..."
ln -s payload/.agents .agents
git add .agents

# --- 6. AGENTS.md y MEMORY.md PROPIOS de seb-harness -----------------------
if [ ! -e AGENTS.md ]; then
    info "Creando AGENTS.md propio (borrador, completalo con seb-iniciador)..."
    cat > AGENTS.md <<'EOF'
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
EOF
fi

if [ ! -e MEMORY.md ]; then
    info "Creando MEMORY.md propio..."
    cat > MEMORY.md <<'EOF'
# MEMORY.md — seb-harness (el proyecto en sí)

> Bitácora viva del repo seb-harness (máx. 150 líneas).
> La plantilla para proyectos destino está en `payload/MEMORY.md`.

## Decisiones vigentes
- Estado compartido entre máquinas en `.seb/seb.lock` y `.seb/ACTIVE_TASK` (texto plano, en Git).
- SQLite (`.seb/seb-audit.db`) es solo telemetría local, ignorada por Git.
- Estructura `payload/` (se instala) vs `engine/` (se compila) vs raíz (el repo mismo).

## Callejones sin salida
- (vacío por ahora)
EOF
fi

# --- 7. .gitignore ----------------------------------------------------------
touch .gitignore
for line in ".seb/bin/" ".seb/seb-audit.db*"; do
    grep -qxF "$line" .gitignore || echo "$line" >> .gitignore
done
git add .gitignore AGENTS.md MEMORY.md

# --- Resumen ----------------------------------------------------------------
echo
info "Migración lista (todo está en el índice de git, aún sin commit)."
echo
echo "Pendientes manuales:"
echo "  1. flake.nix: revisá referencias al código Go (ahora está en engine/, no en .seb/):"
if [ -f flake.nix ]; then
    grep -n -E '\.seb|cmd/seb|go\.mod' flake.nix | sed 's/^/       /' || true
fi
echo "  2. Probá que compila:  (cd engine && CGO_ENABLED=0 go build ./cmd/seb)"
echo "  3. Commit de la migración:  git commit -m 'refactor: separar payload/ y engine/'"
echo "  4. Reemplazá install.sh por la versión nueva y luego, para dogfooding:"
echo "       ./install.sh --self"
echo "  5. Sobre tu proyecto:  ./install.sh /ruta/a/Brienne"
