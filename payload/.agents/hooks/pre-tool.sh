#!/usr/bin/env bash
# .agents/hooks/pre-tool.sh
# Hook PreToolUse para Antigravity (agy) y Pi: Menor Privilegio, Seguridad y Timer

set -uo pipefail

SEB_BIN=".seb/bin/seb"

# Capturar payload stdin si el agente (agy / pi) envía JSON descriptivo de la tool
PAYLOAD=""
if [ ! -t 0 ]; then
  PAYLOAD="$(cat)"
fi

# Extraer Session ID homogéneo
SESSION_ID="${SEB_SESSION_ID:-${AGY_SESSION_ID:-${PI_SESSION_ID:-local-$(date +%Y%m%d)}}}"

# Extraer Tool Call ID si existe para correlación exacta en concurrencia
TOOL_CALL_ID=""
if [ -n "${PAYLOAD}" ] && command -v jq >/dev/null 2>&1; then
  TOOL_CALL_ID="$(echo "${PAYLOAD}" | jq -r '.tool_call_id // .tool_use_id // .id // empty' 2>/dev/null)"
fi

if [ -n "${TOOL_CALL_ID}" ]; then
  TIMER_FILE="/tmp/seb_timer_${SESSION_ID}_${TOOL_CALL_ID}"
else
  TIMER_FILE="/tmp/seb_timer_${SESSION_ID}_call_latest"
fi

# Registrar timestamp de inicio en milisegundos (para post-tool.sh)
date +%s%3N > "${TIMER_FILE}" 2>/dev/null || date +%s000 > "${TIMER_FILE}"

TOOL_NAME="${SEB_TOOL_NAME:-${AGY_TOOL_NAME:-${PI_TOOL_NAME:-unknown}}}"
TARGET_PATH="${SEB_TARGET_PATH:-${AGY_FILE_PATH:-${PI_FILE_PATH:-}}}"
COMMAND_STR="${SEB_COMMAND:-${AGY_COMMAND:-${PI_COMMAND:-}}}"
ACTIVE_ROLE="${SEB_SKILL:-implementador}"

# Si los datos vienen en JSON por stdin y tenemos jq disponible, extraerlos con precisión
if [ -n "${PAYLOAD}" ] && command -v jq >/dev/null 2>&1; then
  [ "${TOOL_NAME}" = "unknown" ] && TOOL_NAME="$(echo "${PAYLOAD}" | jq -r '.tool_name // .tool // .name // "unknown"' 2>/dev/null)"
  [ -z "${TARGET_PATH}" ] && TARGET_PATH="$(echo "${PAYLOAD}" | jq -r '.tool_input.path // .tool_input.file_path // .parameters.path // .path // empty' 2>/dev/null)"
  [ -z "${COMMAND_STR}" ] && COMMAND_STR="$(echo "${PAYLOAD}" | jq -r '.tool_input.command // .parameters.command // .command // empty' 2>/dev/null)"
fi

# 1. Bloqueo estricto de archivos sensibles (Secretos y credenciales)
if [ -n "${TARGET_PATH}" ]; then
  case "${TARGET_PATH}" in
    *.env|*.env.*|*id_rsa*|*id_ed25519*|*.pem|*.key)
      echo "[seb-harness BLOQUEO DE SEGURIDAD] Acceso denegado al archivo sensible: ${TARGET_PATH}" >&2
      exit 2
      ;;
  esac
fi

# 2. Bloqueo de comandos de terminal destructivos o que intenten burlar verificaciones
if [ -n "${COMMAND_STR}" ]; then
  if echo "${COMMAND_STR}" | grep -Eq '(rm[[:space:]]+-rf[[:space:]]+/|git[[:space:]]+commit.*--no-verify|git[[:space:]]+push.*--force|chmod[[:space:]]+777)'; then
    echo "[seb-harness BLOQUEO DE SEGURIDAD] Comando peligroso o evasión de verificación detectada: ${COMMAND_STR}" >&2
    exit 2
  fi

  # Evitar que un comando shell redirija escritura hacia specs/*_verificador.md o constitution.md durante implementación
  if [ "${ACTIVE_ROLE}" = "implementador" ] || [ -z "${ACTIVE_ROLE}" ]; then
    if echo "${COMMAND_STR}" | grep -Eq '(>|tee|sed[[:space:]]+-i).*(constitution\.md|_spec\.md|_verificador\.md)'; then
      echo "[seb-harness ANTI-REWARD-HACKING] Prohibido modificar contratos SDD mediante comandos de shell." >&2
      exit 2
    fi
  fi
fi

# 3. Verificación de Menor Privilegio en escrituras de archivos vía binario Go (`seb guard`)
if [ -n "${TARGET_PATH}" ] && [ -x "${SEB_BIN}" ]; then
  case "${TOOL_NAME}" in
    *write*|*edit*|*patch*|*create*|*replace*|*save*)
      if ! "${SEB_BIN}" guard "${ACTIVE_ROLE}" write "${TARGET_PATH}"; then
        exit 2
      fi
      ;;
  esac
fi

exit 0
