#!/usr/bin/env bash
# Restart the remote marketd process without building, syncing, or touching config.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CFG="${DEPLOY_CFG:-${ROOT}/deploy/deploy.local.yaml}"

if [[ ! -f "${CFG}" ]]; then
  echo "缺少 ${CFG}"
  echo "请先: cp deploy/deploy.local.yaml.example deploy/deploy.local.yaml"
  exit 1
fi

get_yaml() {
  local key="$1" default="${2:-}"
  local line v
  line="$(grep -E "^${key}:" "${CFG}" 2>/dev/null | head -1 || true)"
  if [[ -z "${line}" ]]; then
    echo "${default}"
    return
  fi
  v="${line#*:}"
  v="${v#"${v%%[![:space:]]*}"}"
  v="${v%"${v##*[![:space:]]}"}"
  v="${v%\"}"
  v="${v#\"}"
  v="${v%\'}"
  v="${v#\'}"
  echo "${v}"
}

SSH_HOST="$(get_yaml ssh_host)"
SSH_PORT="$(get_yaml ssh_port 22)"
SSH_USER="$(get_yaml ssh_user root)"
SSH_KEY="$(get_yaml ssh_key)"
REMOTE_DIR="$(get_yaml remote_dir /home/lzqqdy/github/marketpulse)"

if [[ -z "${SSH_HOST}" || "${SSH_HOST}" == "1.2.3.4" ]]; then
  echo "请在 ${CFG} 中设置真实的 ssh_host"
  exit 1
fi

SSH_OPTS=(-p "${SSH_PORT}" -o StrictHostKeyChecking=accept-new)
if [[ -n "${SSH_KEY}" ]]; then
  SSH_OPTS+=(-i "${SSH_KEY/#\~/$HOME}")
fi
SSH_TARGET="${SSH_USER}@${SSH_HOST}"

echo "==> 只重启远程后端 ${SSH_TARGET}:${REMOTE_DIR}"
ssh "${SSH_OPTS[@]}" "${SSH_TARGET}" \
  "cd '${REMOTE_DIR}' && chmod +x scripts/restart.sh && scripts/restart.sh"

