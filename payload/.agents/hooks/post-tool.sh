#!/usr/bin/env bash
# .agents/hooks/post-tool.sh
# Interceptor PostToolUse para Antigravity 2.0 y agy CLI

set -uo pipefail

# Resolver raiz del proyecto (cuando se invoca desde .agents/hooks.json el CWD es .agents/)
HOOK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(cd "${HOOK_DIR}/../.." && pwd -P)"
cd "${PROJECT_ROOT}" || exit 0

SEB_BIN="${PROJECT_ROOT}/.seb/bin/seb"

if [ -x "${SEB_BIN}" ]; then
  "${SEB_BIN}" hook post-tool
  exit 0
fi

printf '{}\n'
exit 0
