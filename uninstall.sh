#!/usr/bin/env bash
# uninstall.sh — Desinstala seb-harness de un proyecto destino o revierte modo --self.
#
# Uso:
#   ./uninstall.sh [opciones] [DIRECTORIO_DESTINO]    (por defecto: directorio actual)
#
# Opciones:
#   --self       Revertir dogfooding en este repositorio (remueve symlink .agents y hooks de git).
#   --dry-run    Simular la desinstalacion mostrando que archivos y hooks se modificarian.
#   --purge      Eliminar adicionalmente specs/, AGENTS.md, MEMORY.md y docs/constitution.md.
#   -f, --force  Omitir confirmacion interactiva al usar --purge (mandatorio en entornos CI / no-TTY).
#   -h, --help   Muestra esta ayuda.
#
# Comportamiento:
#   - Remueve los hooks de Git asociados al arnes y restaura respaldos .bak-seb si existian.
#   - Remueve exclusivamente las lineas de seb-harness en .gitignore.
#   - Remueve el directorio .seb/ por completo.
#   - Remueve de .agents/ solo los componentes provistos por el arnes, conservando los propios.
#   - Conserva specs/, docs/constitution.md, AGENTS.md y MEMORY.md salvo con --purge.

set -euo pipefail

HARNESS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PAYLOAD="$HARNESS_DIR/payload"

info() { printf '\033[1;34m->\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31mxx\033[0m %s\n' "$*" >&2; exit 1; }

usage() { sed -n '2,17p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; }

# --- Argumentos -------------------------------------------------------------
SELF=0
DRY_RUN=0
PURGE=0
FORCE=0
TARGET=""

while [ $# -gt 0 ]; do
    case "$1" in
        --self)       SELF=1 ;;
        --dry-run)    DRY_RUN=1 ;;
        --purge)      PURGE=1 ;;
        -f|--force)   FORCE=1 ;;
        -h|--help)    usage; exit 0 ;;
        -*)           die "Opcion desconocida: $1 (usa --help)" ;;
        *)            [ -z "$TARGET" ] || die "Solo se admite un directorio destino."
                      TARGET="$1" ;;
    esac
    shift
done

# --- Validacion de Directorio Objetivo --------------------------------------
if [ "$SELF" -eq 1 ]; then
    [ -z "$TARGET" ] || die "--self no admite directorio destino."
    TARGET="$HARNESS_DIR"
else
    TARGET="${TARGET:-$PWD}"
    [ -d "$TARGET" ] || die "El directorio destino no existe: $TARGET"
    TARGET="$(cd "$TARGET" && pwd -P)"
    if [ "$TARGET" = "$HARNESS_DIR" ]; then
        die "El destino es el propio repo seb-harness. Para dogfooding usa: ./uninstall.sh --self"
    fi
fi

# --- Confirmacion para Purga ------------------------------------------------
if [ "$PURGE" -eq 1 ]; then
    if [ "$FORCE" -eq 0 ]; then
        if [ ! -t 0 ]; then
            die "La purga (--purge) en entornos no interactivos requiere la bandera --force."
        fi
        printf "Desea eliminar irreversiblemente specs y gobernanza? [y/N]: "
        read -r answer
        case "$answer" in
            [yY]|[yY][eE][sS]|[sS]|[sS][iI]) ;;
            *) info "Operacion cancelada por el usuario."; exit 0 ;;
        esac
    fi
fi

# --- 1. Reversion de Git Hooks ----------------------------------------------
revert_hook() {
    local hooks_dir="$1" name="$2"
    local dest="$hooks_dir/$name"
    local bak="$hooks_dir/$name.bak-seb"

    if [ -L "$dest" ]; then
        local target_link
        target_link="$(readlink "$dest" || true)"
        # Verificar si el enlace apunta a los hooks del arnes
        if [[ "$target_link" == *".agents/hooks/"* ]] || [[ "$target_link" == *"git-"* ]]; then
            if [ "$DRY_RUN" -eq 1 ]; then
                info "[dry-run] Se removeria hook: $dest"
                if [ -e "$bak" ]; then
                    info "[dry-run] Se restauraria hook original: $bak -> $dest"
                fi
            else
                rm -f "$dest"
                if [ -e "$bak" ]; then
                    mv "$bak" "$dest"
                    info "Restaurado hook previo: $name"
                fi
            fi
        fi
    elif [ -e "$bak" ]; then
        # Respaldo huerfano sin enlace activo
        if [ "$DRY_RUN" -eq 1 ]; then
            info "[dry-run] Se restauraria hook desde respaldo: $bak -> $dest"
        else
            mv "$bak" "$dest"
            info "Restaurado hook desde respaldo: $name"
        fi
    fi
}

if git -C "$TARGET" rev-parse --git-dir >/dev/null 2>&1; then
    hooks_dir="$(git -C "$TARGET" rev-parse --path-format=absolute --git-path hooks 2>/dev/null || true)"
    if [ -n "$hooks_dir" ] && [ -d "$hooks_dir" ]; then
        revert_hook "$hooks_dir" pre-commit
        revert_hook "$hooks_dir" post-commit
    fi
fi

# --- 2. Modo --self (Dogfooding) --------------------------------------------
if [ "$SELF" -eq 1 ]; then
    info "Modo --self: desvinculando entorno dogfooding..."
    if [ -L "$TARGET/.agents" ]; then
        if [ "$DRY_RUN" -eq 1 ]; then
            info "[dry-run] Se removeria el symlink $TARGET/.agents"
        else
            rm -f "$TARGET/.agents"
            info "Removido symlink .agents"
        fi
    fi
    info "Listo. Modo --self desinstalado con exito."
    exit 0
fi

# --- 3. Limpieza en Proyecto Destino ----------------------------------------
if [ -L "$TARGET/.agents" ]; then
    die "$TARGET/.agents es un enlace simbolico inesperado en un proyecto destino."
fi

# 3.1 Limpieza quirurgica de .gitignore
if [ -f "$TARGET/.gitignore" ]; then
    if [ "$DRY_RUN" -eq 1 ]; then
        if grep -qxF -e ".seb/bin/" -e ".seb/seb-audit.db*" -e ".seb/ACTIVE_ROLE" "$TARGET/.gitignore" 2>/dev/null; then
            info "[dry-run] Se removerian entradas de seb-harness en $TARGET/.gitignore"
        fi
    else
        temp_gi="$(mktemp)"
        grep -vxF -e ".seb/bin/" -e ".seb/seb-audit.db*" -e ".seb/ACTIVE_ROLE" "$TARGET/.gitignore" > "$temp_gi" || true
        mv "$temp_gi" "$TARGET/.gitignore"
    fi
fi

# 3.2 Eliminacion de .seb/
if [ -d "$TARGET/.seb" ]; then
    if [ "$DRY_RUN" -eq 1 ]; then
        info "[dry-run] Se removeria el directorio $TARGET/.seb"
    else
        rm -rf "$TARGET/.seb"
        info "Removido directorio .seb/"
    fi
fi

# 3.3 Limpieza quirurgica de .agents/
if [ -d "$TARGET/.agents" ]; then
    if [ -d "$PAYLOAD/.agents" ]; then
        for entry in "$PAYLOAD/.agents"/*; do
            [ -e "$entry" ] || continue
            if [ -d "$entry" ]; then
                gname="$(basename "$entry")"
                for item in "$entry"/*; do
                    [ -e "$item" ] || continue
                    iname="$(basename "$item")"
                    target_item="$TARGET/.agents/$gname/$iname"
                    if [ -e "$target_item" ] || [ -L "$target_item" ]; then
                        if [ "$DRY_RUN" -eq 1 ]; then
                            info "[dry-run] Se removeria $target_item"
                        else
                            rm -rf "$target_item"
                        fi
                    fi
                done
                if [ "$DRY_RUN" -eq 0 ]; then
                    rmdir "$TARGET/.agents/$gname" 2>/dev/null || true
                fi
            elif [ -f "$entry" ]; then
                fname="$(basename "$entry")"
                target_file="$TARGET/.agents/$fname"
                if [ -e "$target_file" ] || [ -L "$target_file" ]; then
                    if [ "$DRY_RUN" -eq 1 ]; then
                        info "[dry-run] Se removeria $target_file"
                    else
                        rm -f "$target_file"
                    fi
                fi
            fi
        done
    fi
    if [ "$DRY_RUN" -eq 0 ]; then
        rmdir "$TARGET/.agents" 2>/dev/null || true
    fi
fi

# 3.3.5 Limpieza quirurgica de integracion Claude (.claude/)
if [ -d "$TARGET/.claude" ]; then
    for sub in hooks rules skills; do
        claude_path="$TARGET/.claude/$sub"
        if [ -L "$claude_path" ]; then
            link_target="$(readlink "$claude_path" || true)"
            if [ "$link_target" = "../.agents/$sub" ] || [ "$link_target" = ".agents/$sub" ] || [ "$link_target" = "$TARGET/.agents/$sub" ]; then
                if [ "$DRY_RUN" -eq 1 ]; then
                    info "[dry-run] Se removeria el symlink $claude_path"
                else
                    rm -f "$claude_path"
                    info "Removido symlink .claude/$sub"
                fi
            fi
        fi
    done

    settings_file="$TARGET/.claude/settings.json"
    if [ -f "$settings_file" ] && [ -s "$settings_file" ]; then
        if command -v jq >/dev/null 2>&1; then
            jq -e 'type == "object"' "$settings_file" >/dev/null 2>&1 || die "Error procesando $settings_file (JSON invalido o no es un objeto)."
            if [ "$DRY_RUN" -eq 1 ]; then
                info "[dry-run] Se limpiarian hooks de seb-harness en $settings_file"
            else
                tmp_json="$(mktemp)"
                jq '
                    if (.hooks | type) == "object" then
                        .hooks |= with_entries(
                            if (.key == "PreToolUse" or .key == "PostToolUse" or .key == "Stop") and (.value | type) == "array" then
                                .value |= map(
                                    select(((.hooks // []) | any(.command? // "" | test("\\.claude/hooks/(pre-tool|post-tool|stop)\\.sh"))) | not)
                                )
                            else
                                .
                            end
                        ) |
                        .hooks |= with_entries(select(((.key == "PreToolUse" or .key == "PostToolUse" or .key == "Stop") and (.value == [])) | not)) |
                        if .hooks == {} then del(.hooks) else . end
                    else
                        .
                    end
                ' "$settings_file" > "$tmp_json" || { rm -f "$tmp_json"; die "Error limpiando $settings_file."; }
                if [ "$(jq -c '.' "$tmp_json")" = "{}" ]; then
                    rm -f "$tmp_json" "$settings_file"
                    info "Removido $settings_file (sin configuracion restante)."
                else
                    mv "$tmp_json" "$settings_file"
                    info "Removidos hooks de seb-harness en .claude/settings.json"
                fi
            fi
        fi
    fi

    if [ "$DRY_RUN" -eq 0 ]; then
        rmdir "$TARGET/.claude" 2>/dev/null || true
    fi
fi

# 3.4 Purga opcional de artefactos de gobernanza y specs
if [ "$PURGE" -eq 1 ]; then
    if [ "$DRY_RUN" -eq 1 ]; then
        info "[dry-run] Se removerian specs/, AGENTS.md, MEMORY.md y docs/constitution.md"
    else
        rm -rf "$TARGET/specs"
        rm -f "$TARGET/AGENTS.md" "$TARGET/MEMORY.md" "$TARGET/docs/constitution.md"
        rmdir "$TARGET/docs" 2>/dev/null || true
        info "Purgados artefactos de especificacion y gobernanza."
    fi
fi

# 3.5 Restauracion automatica de respaldos *.pre-seb.* (Spec 007)
while IFS= read -r -d '' bak_file; do
    [ -f "$bak_file" ] || continue
    dir="$(dirname "$bak_file")"
    base="$(basename "$bak_file")"
    if [[ "$base" == *".pre-seb."* ]]; then
        orig_base="${base/.pre-seb./.}"
    else
        orig_base="${base%.pre-seb}"
    fi
    orig_file="$dir/$orig_base"

    if [ "$DRY_RUN" -eq 1 ]; then
        info "[dry-run] Se restauraria archivo original: $bak_file -> $orig_file"
    else
        mkdir -p "$dir"
        mv -f "$bak_file" "$orig_file"
        info "Restaurado archivo original desde respaldo: $orig_base"
    fi
done < <(find "$TARGET" -maxdepth 2 -type f \( -name "*.pre-seb.*" -o -name "*.pre-seb" \) -print0 2>/dev/null || true)

# --- Resumen ----------------------------------------------------------------
if [ "$DRY_RUN" -eq 1 ]; then
    info "Simulacion completada. No se realizaron cambios en $TARGET"
else
    info "Desinstalacion de seb-harness completada con exito en: $TARGET"
fi
