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
#   - sobrescribe solo lo que provee payload/ (hooks, rules, skills, SEB-HARNESS.md);
#     otros skills/rules propios del proyecto en .agents/ se conservan;
#   - actualiza unicamente el bloque gestionado en AGENTS.md conservando el contenido propio;
#   - nunca toca MEMORY.md, docs/, specs/, .seb/seb.lock ni .seb/ACTIVE_TASK.

set -euo pipefail

HARNESS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PAYLOAD="$HARNESS_DIR/payload"
ENGINE="$HARNESS_DIR/engine"

info() { printf '\033[1;34m->\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31mxx\033[0m %s\n' "$*" >&2; exit 1; }

MANAGED_BEGIN='<!-- BEGIN seb-harness:managed -->'
MANAGED_END='<!-- END seb-harness:managed -->'

has_seb_signature() {
    local file="$1"
    [ -f "$file" ] && grep -qi "seb-harness" "$file"
}

# Valida los marcadores del bloque gestionado:
#   0 = un par unico y bien ordenado (BEGIN antes de END)
#   2 = sin marcadores presentes
#   1 = marcadores malformados, invertidos o duplicados
validate_managed_markers() {
    local file="$1"
    awk -v begin_m="$MANAGED_BEGIN" -v end_m="$MANAGED_END" '
        {
            line = $0
            sub(/^[[:space:]]+/, "", line)
            sub(/[[:space:]]+$/, "", line)
            if (line == begin_m) {
                begin_count++
                if (begin_line == 0) begin_line = NR
            } else if (line == end_m) {
                end_count++
                if (end_line == 0) end_line = NR
            }
        }
        END {
            if (begin_count == 0 && end_count == 0) exit 2
            if (begin_count == 1 && end_count == 1 && begin_line < end_line) exit 0
            exit 1
        }
    ' "$file"
}

# Reemplaza literalmente el bloque gestionado sin expandir metacaracteres de shell o regex
replace_managed_block() {
    local target_file="$1" template_file="$2"
    local block_tmp out_tmp
    block_tmp="$(mktemp)"
    out_tmp="$(mktemp)"

    awk -v begin_m="$MANAGED_BEGIN" -v end_m="$MANAGED_END" '
        {
            line = $0
            sub(/^[[:space:]]+/, "", line)
            sub(/[[:space:]]+$/, "", line)
            if (line == begin_m) in_block = 1
            if (in_block) print $0
            if (line == end_m) in_block = 0
        }
    ' "$template_file" > "$block_tmp"

    awk -v begin_m="$MANAGED_BEGIN" -v end_m="$MANAGED_END" -v block_file="$block_tmp" '
        {
            line = $0
            sub(/^[[:space:]]+/, "", line)
            sub(/[[:space:]]+$/, "", line)
            if (line == begin_m) {
                in_block = 1
                while ((getline b < block_file) > 0) {
                    print b
                }
                close(block_file)
                next
            }
            if (in_block) {
                if (line == end_m) {
                    in_block = 0
                }
                next
            }
            print $0
        }
    ' "$target_file" > "$out_tmp"

    rm -f "$block_tmp"
    mv "$out_tmp" "$target_file"
}

# Sincroniza AGENTS.md aplicando reglas de bloque gestionado, respaldo legado y respaldo brownfield
sync_agents_md() {
    local target_file="$1" template_file="$2"
    local dir bak status

    if [ ! -e "$target_file" ]; then
        cp "$template_file" "$target_file"
        info "Creado AGENTS.md desde la plantilla."
        return 0
    fi

    status=0
    validate_managed_markers "$target_file" || status=$?

    if [ "$status" -eq 0 ]; then
        replace_managed_block "$target_file" "$template_file"
        info "Actualizado bloque gestionado en AGENTS.md (contenido propio preservado)."
    elif [ "$status" -eq 1 ]; then
        die "Marcadores de bloque gestionado malformados o duplicados en $target_file. Se conserva el archivo intacto."
    else
        if has_seb_signature "$target_file"; then
            dir="$(dirname "$target_file")"
            bak="$dir/AGENTS.legacy-seb.md"
            if [ -e "$bak" ]; then
                warn "Ya existe un respaldo legado $bak; se conserva el respaldo previo."
            else
                mv "$target_file" "$bak"
                info "Detectado AGENTS.md en formato legado. Respaldado en: $bak"
            fi
            cp "$template_file" "$target_file"
            info "Instalado AGENTS.md con bloque gestionado seb-harness."
        else
            backup_pre_seb "$target_file"
            cp "$template_file" "$target_file"
            info "Instalado AGENTS.md oficial de seb-harness."
        fi
    fi
}

backup_pre_seb() {
    local file="$1"
    [ -f "$file" ] || return 0
    if ! has_seb_signature "$file"; then
        local dir base name ext bak
        dir="$(dirname "$file")"
        base="$(basename "$file")"
        if [[ "$base" == *.* ]]; then
            name="${base%.*}"
            ext="${base##*.}"
            bak="$dir/$name.pre-seb.$ext"
        else
            bak="$dir/$base.pre-seb"
        fi

        if [ -e "$bak" ]; then
            warn "Ya existe un respaldo $bak; se conserva el respaldo previo."
        else
            mv "$file" "$bak"
            info "Detectado archivo ajeno a seb-harness. Respaldado: $base -> $(basename "$bak")"
        fi
    fi
}

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
[ -d "$PAYLOAD/.agents" ] || die "No encuentro payload/.agents"
[ -d "$ENGINE/cmd/seb" ]  || die "No encuentro engine/cmd/seb"

if [ "$SELF" -eq 1 ]; then
    [ -z "$TARGET" ] || die "--self no admite directorio destino."
    TARGET="$HARNESS_DIR"
else
    TARGET="${TARGET:-$PWD}"
    [ -d "$TARGET" ] || die "El directorio destino no existe: $TARGET"
    TARGET="$(cd "$TARGET" && pwd -P)"
    if [ "$TARGET" = "$HARNESS_DIR" ]; then
        die "El destino es el propio repo seb-harness. Para dogfooding usá: ./install.sh --self
Para instalar en otro proyecto: $HARNESS_DIR/install.sh /ruta/a/Destino"
    fi
fi

# --- 1. Archivos del harness ------------------------------------------------
if [ "$SELF" -eq 1 ]; then
    info "Modo --self: enlazando .agents -> payload/.agents"
    if [ -e "$TARGET/.agents" ] && [ ! -L "$TARGET/.agents" ]; then
        die ".agents existe y no es un symlink."
    fi
    ln -sfn payload/.agents "$TARGET/.agents"
    sync_agents_md "$TARGET/AGENTS.md" "$PAYLOAD/AGENTS.md"
else
    info "Instalando seb-harness en: $TARGET"
    if [ -L "$TARGET/.agents" ]; then
        die "$TARGET/.agents es un symlink; no se sobrescribe (¿es otro repo con dogfooding?)."
    fi

    # Solo se reemplaza lo que provee el payload; lo demás de .agents/ se respeta.
    mkdir -p "$TARGET/.agents"
    for entry in "$PAYLOAD/.agents"/*; do
        [ -e "$entry" ] || continue
        if [ -d "$entry" ]; then
            gname="$(basename "$entry")"
            mkdir -p "$TARGET/.agents/$gname"
            for item in "$entry"/*; do
                [ -e "$item" ] || continue
                rm -rf "$TARGET/.agents/$gname/$(basename "$item")"
                cp -R "$item" "$TARGET/.agents/$gname/"
            done
        elif [ -f "$entry" ]; then
            cp -f "$entry" "$TARGET/.agents/$(basename "$entry")"
        fi
    done

    # Archivos de contexto: gestion determinista de bloque gestionado y directivas previas (Specs 007, 017)
    sync_agents_md "$TARGET/AGENTS.md" "$PAYLOAD/AGENTS.md"

    if [ -e "$TARGET/MEMORY.md" ]; then
        info "MEMORY.md ya existe: se conserva."
    else
        cp "$PAYLOAD/MEMORY.md" "$TARGET/MEMORY.md"
        info "Creado MEMORY.md desde la plantilla."
    fi

    # Respaldar archivos de contexto de otros asistentes si no pertenecen al arnes
    for assistant_file in "GEMINI.md" "CLAUDE.md"; do
        if [ -f "$TARGET/$assistant_file" ]; then
            backup_pre_seb "$TARGET/$assistant_file"
        fi
    done
fi

chmod +x "$TARGET"/.agents/hooks/* 2>/dev/null || true
mkdir -p "$TARGET/docs" "$TARGET/docs/use-cases" "$TARGET/specs" "$TARGET/.seb"

# --- 1.5 Integracion Claude (Claude Code y Claude Desktop) ---------------------
setup_claude_integration() {
    mkdir -p "$TARGET/.claude"
    for sub in hooks rules skills; do
        local claude_path="$TARGET/.claude/$sub"
        if [ -e "$claude_path" ] || [ -L "$claude_path" ]; then
            info ".claude/$sub ya existe: se conserva."
        else
            ln -s "../.agents/$sub" "$claude_path"
            info "Creado enlace simbolico .claude/$sub -> ../.agents/$sub"
        fi
    done

    local settings_file="$TARGET/.claude/settings.json"
    if command -v jq >/dev/null 2>&1; then
        local base_json="{}"
        if [ -f "$settings_file" ]; then
            if [ -s "$settings_file" ]; then
                jq -e 'type == "object"' "$settings_file" >/dev/null 2>&1 || die "Error procesando $settings_file (JSON invalido o no es un objeto)."
                base_json="$(cat "$settings_file")"
            fi
        fi

        local tmp_json
        tmp_json="$(mktemp)"
        printf '%s\n' "$base_json" | jq '
            .hooks = (.hooks // {}) |
            def ensure_hook($ev; $script; $with_matcher):
                .hooks[$ev] = (.hooks[$ev] // []) |
                if (.hooks[$ev] | any(.hooks[]? | (.command // "") | contains(".claude/hooks/" + $script))) then
                    .
                else
                    .hooks[$ev] += [
                        (if $with_matcher then
                            {"matcher": "", "hooks": [{"type": "command", "command": ("$CLAUDE_PROJECT_DIR/.claude/hooks/" + $script + " --format claude")}]}
                        else
                            {"hooks": [{"type": "command", "command": ("$CLAUDE_PROJECT_DIR/.claude/hooks/" + $script + " --format claude")}]}
                        end)
                    ]
                end;
            ensure_hook("PreToolUse"; "pre-tool.sh"; true) |
            ensure_hook("PostToolUse"; "post-tool.sh"; true) |
            ensure_hook("Stop"; "stop.sh"; false)
        ' > "$tmp_json" || { rm -f "$tmp_json"; die "Error fusionando $settings_file."; }
        mv "$tmp_json" "$settings_file"
    elif command -v python3 >/dev/null 2>&1; then
        python3 - "$settings_file" <<'PYEOF' || die "Error procesando $settings_file (verifique que el JSON existente sea valido)."
import json
import os
import sys

settings_path = sys.argv[1]
data = {}
if os.path.exists(settings_path):
    with open(settings_path, "r", encoding="utf-8") as f:
        content = f.read().strip()
        if content:
            data = json.loads(content)
            if not isinstance(data, dict):
                raise ValueError("settings.json debe ser un objeto JSON")

hooks = data.get("hooks")
if hooks is None:
    hooks = {}
    data["hooks"] = hooks
elif not isinstance(hooks, dict):
    raise ValueError("La clave 'hooks' en settings.json debe ser un objeto")

desired = [
    ("PreToolUse", "pre-tool.sh", True),
    ("PostToolUse", "post-tool.sh", True),
    ("Stop", "stop.sh", False),
]

changed = False
for event_name, script_name, include_matcher in desired:
    event_list = hooks.get(event_name)
    if event_list is None:
        event_list = []
        hooks[event_name] = event_list
    elif not isinstance(event_list, list):
        raise ValueError(f"La clave hooks.{event_name} debe ser una lista")

    already_present = False
    for entry in event_list:
        if not isinstance(entry, dict):
            continue
        inner_hooks = entry.get("hooks", [])
        if not isinstance(inner_hooks, list):
            continue
        for h in inner_hooks:
            if isinstance(h, dict) and f".claude/hooks/{script_name}" in str(h.get("command", "")):
                already_present = True
                break
        if already_present:
            break

    if not already_present:
        new_entry = {
            "hooks": [
                {
                    "type": "command",
                    "command": f"$CLAUDE_PROJECT_DIR/.claude/hooks/{script_name} --format claude",
                }
            ]
        }
        if include_matcher:
            new_entry["matcher"] = ""
        event_list.append(new_entry)
        changed = True

if changed or not os.path.exists(settings_path):
    with open(settings_path, "w", encoding="utf-8") as f:
        json.dump(data, f, indent=2, ensure_ascii=False)
        f.write("\n")
PYEOF
    else
        warn "jq/python3 no disponibles: no se pudo generar/fusionar $settings_file automaticamente."
        return 0
    fi
    info "Configuracion de hooks Claude verificada en .claude/settings.json"
}

setup_claude_integration

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
for line in ".seb/bin/" ".seb/seb-audit.db*" ".seb/ACTIVE_ROLE"; do
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

# --- Aviso de identificadores heredados (Spec 013) -------------------------------
# Cuenta verificadores con filas de control sin prefijo de spec (CHK-NN) fuera de bloques cercados.
LEGACY_TASKS=0
if [ -d "$TARGET/specs" ]; then
    for vf in "$TARGET"/specs/*_verificador.md; do
        [ -f "$vf" ] || continue
        if awk '/^[[:space:]]*```/ { fence = !fence; next }
                !fence && /^[[:space:]]*\|[[:space:]]*CHK-/ { found = 1; exit }
                END { exit !found }' "$vf"; then
            LEGACY_TASKS=$((LEGACY_TASKS + 1))
        fi
    done
fi
if [ "$LEGACY_TASKS" -gt 0 ]; then
    echo
    warn "AVISO: $LEGACY_TASKS tareas contienen identificadores heredados (CHK-NN). seb verify las rechazará hasta migrar."
    echo "   Ejecutá: .seb/bin/seb role set migrador && .seb/bin/seb oneoff 013 && .seb/bin/seb role clear"
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
