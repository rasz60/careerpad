#!/usr/bin/env bash
# ae:p Oracle 서버에서 실행하는 배포 스크립트. GitHub Actions(.github/workflows/deploy.yml)가 SSH 로 부르고,
# 서버에서 직접 실행해도 된다:  bash ~/careerpad/scripts/deploy.sh
# 순서: 최신 main → Go 백엔드 빌드 → 웹 빌드(/career/) → pm2 재시작. 빌드가 실패하면 pm2 와 dist 를 건드리지 않는다.
set -euo pipefail

APP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$APP_DIR"

# 비대화형 SSH 세션은 ~/.bashrc 를 읽지 않으므로 node(nvm)·go 를 직접 잡는다
export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"
if [ -s "$NVM_DIR/nvm.sh" ]; then
  set +u
  # shellcheck disable=SC1091
  . "$NVM_DIR/nvm.sh"
  set -u
fi
export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"

echo "== $(date '+%F %T') 배포 시작 ($(git rev-parse --short HEAD))"
git fetch --quiet origin main
git merge --ff-only origin/main
echo "== 코드: $(git rev-parse --short HEAD)"

# 1) Go 백엔드
(cd backend && go build -trimpath -o bin/careerpad-api.new ./cmd/api)

# 2) 웹 (web/.env 의 VITE_* 가 빌드에 들어간다). 다 만든 뒤에 바꿔 끼워 중간 상태를 안 보이게
(cd web && npm ci --no-audit --no-fund && npx tsc -p . && npx vite build --outDir dist.new --emptyOutDir)
rm -rf web/dist && mv web/dist.new web/dist
mv backend/bin/careerpad-api.new backend/bin/careerpad-api

pm2 startOrReload ecosystem.config.cjs --update-env
pm2 save
echo "== $(date '+%F %T') 배포 완료"
