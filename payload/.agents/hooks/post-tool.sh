#!/usr/bin/env bash
# .agents/hooks/post-tool.sh
# Hook PostToolUse para Antigravity (agy) y Pi: Registro de Observabilidad y Costos en SQLite

set -uo pipefail

SEB_BIN=".seb/bin/seb"
[ ! -x "${SEB_BIN}" ] && exit 0

NOW_MS="$(date +%s%3N 2>/dev/null || date +%s000)"
SESSION_ID="${SEB_SESSION_ID:-${AGY_SESSION_ID:-${PI_SESSION_ID:-local-$(date +%Y%m%d)}}}"
TASK_ID="${SEB_TASK_ID:-}"
AGENT_NAME="${SEB_AGENT:-${AGY_AGENT_NAME:-pi-or-agy}}"
SKILL_NAME="${SEB_SKILL:-implementador}"

PAYLOAD=""
if [ ! -t 0 ]; then
  PAYLOAD="$(cat)"
fi

TOOL_NAME="${SEB_TOOL_NAME:-${AGY_TOOL_NAME:-${PI_TOOL_NAME:-tool_call}}}"
EXIT_CODE="${SEB_EXIT_CODE:-${AGY_EXIT_CODE:-0}}"
TOKENS_IN="${SEB_TOKENS_IN:-${OPENROUTER_TOKENS_IN:-0}}"
TOKENS_OUT="${SEB_TOKENS_OUT:-${OPENROUTER_TOKENS_OUT:-0}}"
COST_USD="${SEB_COST_USD:-${OPENROUTER_COST_USD:-0.0}}"
PARAMS_SUMMARY=""
RESULT_SUMMARY="ok"

if [ -n "${PAYLOAD}" ] && command -v jq >/dev/null 2>&1; then
  TOOL_NAME="$(echo "${PAYLOAD}" | jq -r --arg def "${TOOL_NAME}" '.tool_name // .tool // .name // $def' 2>/dev/null)"
  EXIT_CODE="$(echo "${PAYLOAD}" | jq -r --arg def "${EXIT_CODE}" '.exit_code // .tool_response.exit_code // $def' 2>/dev/null)"
  TOKENS_IN="$(echo "${PAYLOAD}" | jq -r --arg def "${TOKENS_IN}" '.usage.input_tokens // .usage.prompt_tokens // $def' 2>/dev/null)"
  TOKENS_OUT="$(echo "${PAYLOAD}" | jq -r --arg def "${TOKENS_OUT}" '.usage.output_tokens // .usage.completion_tokens // $def' 2>/dev/null)"
  COST_USD="$(echo "${PAYLOAD}" | jq -r --arg def "${COST_USD}" '.usage.cost // .usage.total_cost // $def' 2>/dev/null)"
  PARAMS_SUMMARY="$(echo "${PAYLOAD}" | jq -c '.tool_input // .parameters // {}' 2>/dev/null | cut -c 1-160)"
fi

# Calcular latencia buscando el archivo de timer más reciente de la sesión
DURATION_MS=0
LATEST_TIMER="$(ls -t /tmp/seb_timer_${SESSION_ID}_* 2>/dev/null | head -n 1 || true)"
if [ -n "${LATEST_TIMER}" ] && [ -f "${LATEST_TIMER}" ]; then
  START_MS="$(cat "${LATEST_TIMER}" 2>/dev/null || echo "${NOW_MS}")"
  DURATION_MS=$(( NOW_MS - START_MS ))
  [ "${DURATION_MS}" -lt 0 ] && DURATION_MS=0
  rm -f "${LATEST_TIMER}"
fi

# Si no hay TASK_ID explícito en el entorno, tomar la última tarea activa en SQLite
if [ -z "${TASK_ID}" ] && command -v sqlite3 >/dev/null 2>&1 && [ -f ".seb/seb-audit.db" ]; then
  TASK_ID="$(sqlite3 ".seb/seb-audit.db" "SELECT id FROM tasks WHERE status != 'VERIFIED' ORDER BY id DESC LIMIT 1;" 2>/dev/null || true)"
fi

# Registrar de forma estructurada en SQLite sin exponer secretos
"${SEB_BIN}" audit log \
  -session="${SESSION_ID}" \
  -task="${TASK_ID}" \
  -agent="${AGENT_NAME}" \
  -skill="${SKILL_NAME}" \
  -action="${TOOL_NAME}" \
  -params="${PARAMS_SUMMARY}" \
  -result="${RESULT_SUMMARY}" \
  -duration-ms="${DURATION_MS}" \
  -tokens-in="${TOKENS_IN}" \
  -tokens-out="${TOKENS_OUT}" \
  -cost-usd="${COST_USD}" \
  -exit-code="${EXIT_CODE}" >/dev/null 2>&1 || true

exit 0
