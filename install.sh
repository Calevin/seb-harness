#!/usr/bin/env bash
# install.sh
# Instala seb-harness en el directorio actual (proyecto destino)

set -euo pipefail

# Obtiene la ruta absoluta de donde vive este script (el repositorio base de seb-harness)
HARNESS_BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_DIR="$(pwd)"

if [ "$HARNESS_BASE_DIR" = "$TARGET_DIR" ]; then
    echo "Estás ejecutando el instalador dentro del propio repositorio base de seb-harness."
    echo "Por favor, ve al directorio de tu proyecto (ej. cd /ruta/a/Brienne) y ejecuta:"
    echo "$HARNESS_BASE_DIR/install.sh"
    exit 1
fi

echo "Instalando seb-harness en: $TARGET_DIR"

# 1. Copiar los directorios core del agente y el motor en Go
echo "-> Copiando .agents/ y .seb/ ..."
cp -r "$HARNESS_BASE_DIR/.agents" "$TARGET_DIR/"
cp -r "$HARNESS_BASE_DIR/.seb" "$TARGET_DIR/"

# Limpiar posibles compilaciones previas arrastradas de la plantilla
rm -f "$TARGET_DIR/.seb/bin/seb"
rm -f "$TARGET_DIR/.seb/seb-audit.db"*

# 2. Copiar archivos de contexto global solo si no existen (para no sobreescribir)
echo "-> Configurando AGENTS.md y MEMORY.md ..."
[ ! -f "$TARGET_DIR/AGENTS.md" ] && cp "$HARNESS_BASE_DIR/AGENTS.md" "$TARGET_DIR/"
[ ! -f "$TARGET_DIR/MEMORY.md" ] && cp "$HARNESS_BASE_DIR/MEMORY.md" "$TARGET_DIR/"

# 3. Crear directorios SDD de trabajo
mkdir -p "$TARGET_DIR/docs" "$TARGET_DIR/specs"

# 4. Configurar Git Hooks (si es un repositorio Git)
if [ -d "$TARGET_DIR/.git" ]; then
    echo "-> Vinculando Git Hooks de seb-harness ..."
    ln -sf ../../.agents/hooks/git-pre-commit "$TARGET_DIR/.git/hooks/pre-commit"
    ln -sf ../../.agents/hooks/git-post-commit "$TARGET_DIR/.git/hooks/post-commit"
else
    echo "El directorio no es un repositorio git. Recuerda ejecutar 'git init' y volver a vincular los hooks."
fi

# 5. Compilar el binario seb localmente usando el entorno actual (Nix)
if command -v go >/dev/null 2>&1; then
    echo "-> Compilando motor seb (Go)..."
    cd "$TARGET_DIR/.seb"
    export CGO_ENABLED=0
    go mod tidy >/dev/null 2>&1 || go mod init seb-harness/seb
    go build -ldflags="-s -w" -o bin/seb ./cmd/seb/main.go
    cd "$TARGET_DIR"

    # 6. Inicializar la BD de auditoría
    "$TARGET_DIR/.seb/bin/seb" init
else
    echo "'go' no está instalado en este entorno. Ingresa a tu devShell de Nix y compila .seb/cmd/seb/main.go manualmente."
fi

# 6. Actualizar gitignore
if [ -f "$TARGET_DIR/.gitignore" ]; then
    if ! grep -q ".seb/seb-audit.db" "$TARGET_DIR/.gitignore"; then
        echo -e "\n# seb-harness local state\n.seb/bin/\n.seb/seb-audit.db*\n!.seb/seb.lock\n!.seb/ACTIVE_TASK" >> "$TARGET_DIR/.gitignore"
    fi
fi

echo "Instalación completada con éxito. Ya puedes usar seb-harness en tu proyecto."
