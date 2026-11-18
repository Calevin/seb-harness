#!/usr/bin/env bash
# .agents/hooks/post-tool.sh
# Interceptor PostToolUse para Antigravity 2.0, agy CLI y Claude Code

set -uo pipefail

# Resolver raiz del proyecto (cuando se invoca desde .agents/hooks.json el CWD es .agents/)
HOOK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(cd "${HOOK_DIR}/../.." && pwd -P)"
cd "${PROJECT_ROOT}" || exit 0

SEB_BIN="${PROJECT_ROOT}/.seb/bin/seb"

if [ -x "${SEB_BIN}" ]; then
  "${SEB_BIN}" hook post-tool "$@"
  exit $?
fi

is_claude=0
if [ "${SEB_HOOK_FORMAT:-}" = "claude" ]; then
  is_claude=1
fi
prev=""
for arg in "$@"; do
  if [ "$prev" = "--format" ] && [ "$arg" = "claude" ]; then
    is_claude=1
  elif [ "$arg" = "--format=claude" ]; then
    is_claude=1
  fi
  prev="$arg"
done

if [ "$is_claude" -eq 1 ]; then
  exit 0
fi

printf '{}\n'
exit 0
