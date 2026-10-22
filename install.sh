#!/usr/bin/env bash
# install.sh — Instala (o actualiza) seb-harness en un proyecto destino.
#
# Uso:
#   ./install.sh [opciones] [DIRECTORIO_DESTINO]    (por defecto: directorio actual)
#
# Opciones:
#   --self       Dogfooding: prepara ESTE repo para usar su propio harness
#                (symlink .agents -> payload/.agents, motor compilado, hooks git).
#   --no-build   No compilar el motor `seb` (útil si no hay Go/Nix a mano).
#   -h, --help   Muestra esta ayuda.
#
# Re-ejecutarlo sobre un proyecto ya instalado ACTUALIZA el harness:
#   - sobrescribe solo lo que provee payload/ (hooks, rules, skills del harness);
#     otros skills/rules propios del proyecto en .agents/ se conservan;
#   - nunca toca AGENTS.md, MEMORY.md, docs/, specs/, .seb/seb.lock ni .seb/ACTIVE_TASK.

set -euo pipefail

HARNESS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PAYLOAD="$HARNESS_DIR/payload"
ENGINE="$HARNESS_DIR/engine"

info() { printf '\033[1;34m->\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31mxx\033[0m %s\n' "$*" >&2; exit 1; }

usage() { sed -n '2,16p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; }

# --- Argumentos -------------------------------------------------------------
SELF=0
BUILD=1
TARGET=""
while [ $# -gt 0 ]; do
    case "$1" in
        --self)     SELF=1 ;;
        --no-build) BUILD=0 ;;
        -h|--help)  usage; exit 0 ;;
        -*)         die "Opción desconocida: $1 (usá --help)" ;;
        *)          [ -z "$TARGET" ] || die "Solo se admite un directorio destino."
                    TARGET="$1" ;;
    esac
    shift
done

# --- Precondiciones ---------------------------------------------------------
[ -d "$PAYLOAD/.agents" ] || die "No encuentro payload/.agents (¿corriste migrate.sh?)."
[ -d "$ENGINE/cmd/seb" ]  || die "No encuentro engine/cmd/seb (¿corriste migrate.sh?)."

if [ "$SELF" -eq 1 ]; then
    [ -z "$TARGET" ] || die "--self no admite directorio destino."
    TARGET="$HARNESS_DIR"
else
    TARGET="${TARGET:-$PWD}"
    [ -d "$TARGET" ] || die "El directorio destino no existe: $TARGET"
    TARGET="$(cd "$TARGET" && pwd -P)"
    if [ "$TARGET" = "$HARNESS_DIR" ]; then
        die "El destino es el propio repo seb-harness. Para dogfooding usá: ./install.sh --self
Para instalar en otro proyecto: $HARNESS_DIR/install.sh /ruta/a/Brienne"
    fi
fi

# --- 1. Archivos del harness ------------------------------------------------
if [ "$SELF" -eq 1 ]; then
    info "Modo --self: enlazando .agents -> payload/.agents"
    if [ -e "$TARGET/.agents" ] && [ ! -L "$TARGET/.agents" ]; then
        die ".agents existe y no es un symlink. Corré primero migrate.sh."
    fi
    ln -sfn payload/.agents "$TARGET/.agents"
else
    info "Instalando seb-harness en: $TARGET"
    if [ -L "$TARGET/.agents" ]; then
        die "$TARGET/.agents es un symlink; no se sobrescribe (¿es otro repo con dogfooding?)."
    fi

    # Solo se reemplaza lo que provee el payload; lo demás de .agents/ se respeta.
    mkdir -p "$TARGET/.agents"
    for group in "$PAYLOAD/.agents"/*/; do
        gname="$(basename "$group")"
        mkdir -p "$TARGET/.agents/$gname"
        for item in "$group"*; do
            [ -e "$item" ] || continue
            rm -rf "$TARGET/.agents/$gname/$(basename "$item")"
            cp -R "$item" "$TARGET/.agents/$gname/"
        done
    done

    # Archivos de contexto: plantillas solo si no existen.
    for f in AGENTS.md MEMORY.md; do
        if [ -e "$TARGET/$f" ]; then
            info "$f ya existe: se conserva."
        else
            cp "$PAYLOAD/$f" "$TARGET/$f"
            info "Creado $f desde la plantilla."
        fi
    done
fi

chmod +x "$TARGET"/.agents/hooks/* 2>/dev/null || true
mkdir -p "$TARGET/docs" "$TARGET/specs" "$TARGET/.seb"

if [ "$SELF" -eq 0 ]; then
    version="$(git -C "$HARNESS_DIR" describe --tags --always --dirty 2>/dev/null || echo unknown)"
    printf '%s\n' "$version" > "$TARGET/.seb/VERSION"
fi

# --- 2. Git hooks -------------------------------------------------------------
link_hook() {
    local hooks_dir="$1" name="$2" src="$3" dest
    dest="$hooks_dir/$name"
    if [ -L "$dest" ] && [ "$(readlink "$dest")" = "$src" ]; then
        return 0
    fi
    if [ -e "$dest" ] || [ -L "$dest" ]; then
        warn "Ya había un hook $name; se respalda como $name.bak-seb"
        mv "$dest" "$dest.bak-seb"
    fi
    ln -s "$src" "$dest"
}

if git -C "$TARGET" rev-parse --git-dir >/dev/null 2>&1; then
    info "Vinculando git hooks ..."
    hooks_dir="$(git -C "$TARGET" rev-parse --path-format=absolute --git-path hooks)"
    mkdir -p "$hooks_dir"
    link_hook "$hooks_dir" pre-commit  "$TARGET/.agents/hooks/git-pre-commit"
    link_hook "$hooks_dir" post-commit "$TARGET/.agents/hooks/git-post-commit"
    if [ -n "$(git -C "$TARGET" config --get core.hooksPath || true)" ]; then
        warn "core.hooksPath está configurado: Git no usará $hooks_dir. Enlazá los hooks ahí a mano."
    fi
else
    warn "El destino no es un repo git. Cuando hagas 'git init', volvé a correr install.sh."
fi

# --- 3. .gitignore --------------------------------------------------------------
touch "$TARGET/.gitignore"
for line in ".seb/bin/" ".seb/seb-audit.db*"; do
    grep -qxF -- "$line" "$TARGET/.gitignore" || printf '%s\n' "$line" >> "$TARGET/.gitignore"
done

# --- 4. Motor seb (Go, sin CGO) -------------------------------------------------
BUILD_FAILED=0
build_engine() {
    local out="$TARGET/.seb/bin/seb"
    mkdir -p "$TARGET/.seb/bin"
    if command -v go >/dev/null 2>&1; then
        ( cd "$ENGINE" && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$out" ./cmd/seb )
    elif command -v nix >/dev/null 2>&1 && [ -f "$HARNESS_DIR/flake.nix" ]; then
        info "'go' no está en el PATH: usando 'nix develop' de seb-harness."
        nix develop "$HARNESS_DIR" --command bash -c \
            'cd "$1" && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$2" ./cmd/seb' _ "$ENGINE" "$out"
    else
        warn "No hay 'go' ni 'nix'. Compilá a mano: cd engine && go build -o $out ./cmd/seb"
        return 1
    fi
}

if [ "$BUILD" -eq 1 ]; then
    info "Compilando motor seb ..."
    if build_engine; then
        ( cd "$TARGET" && .seb/bin/seb init )
    else
        BUILD_FAILED=1
        warn "El motor no se compiló; el harness no funcionará hasta que lo hagas."
    fi
else
    info "Omitiendo compilación (--no-build)."
fi

# --- Resumen --------------------------------------------------------------------
echo
if [ "$BUILD_FAILED" -eq 1 ]; then
    warn "Instalación incompleta: faltó compilar el motor."
    exit 1
fi
info "Listo. seb-harness instalado en $TARGET"
if [ "$SELF" -eq 0 ]; then
    echo "   Versión: $(cat "$TARGET/.seb/VERSION")"
    echo "   Siguiente paso: pedile al agente que corra el skill seb-iniciador (genera docs/constitution.md)."
    echo "   Commiteá: .agents/ AGENTS.md MEMORY.md .seb/VERSION docs/ specs/ (y .seb/seb.lock cuando exista)."
fi
