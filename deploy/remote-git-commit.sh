#!/usr/bin/env bash
# 在服务器仓库目录执行：提交部署同步后的源码（遵循 .gitignore，不含 bin/dist 等）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

MSG="${1:-deploy: $(date '+%Y-%m-%d %H:%M:%S %z')}"

if ! command -v git >/dev/null 2>&1; then
  echo "ERROR: git 未安装"
  exit 1
fi

if [[ ! -d .git ]]; then
  echo "ERROR: ${ROOT} 不是 git 仓库，请先 git init 或 git clone"
  exit 1
fi

git add -A

if git diff --cached --quiet; then
  echo "git: 无变更，跳过提交"
  exit 0
fi

git commit -m "${MSG}"
echo "git: 已提交 $(git rev-parse --short HEAD)"
git status -sb | head -20
