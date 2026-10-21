#!/usr/bin/env bash
# ae:p Oracle 서버의 커리어++ 배포 스크립트. **서버에서는 빌드하지 않는다.**
#
# 왜: 서버는 메모리 1GB(+스왑 2GB)다. 운동기록 빌드가 서버를 멈춘 뒤(2026-10-20) 같은 구조로 바꿨다 —
#     커리어++ 도 Go 컴파일만 동시 합계 약 650MB 라 여유 메모리(약 470MB)를 넘는다. GitHub Actions 가 빌드하고 서버는 바꿔 끼우기만 한다.
#
# 두 가지로 불린다:
#   1) GitHub Actions:  tar czf - <묶음> | ssh <서버>
#      authorized_keys 의 command= 가 이 스크립트를 실행하고, 표준입력으로 빌드 묶음(tar.gz)을 받는다.
#      묶음: COMMIT(커밋 해시) · bin/careerpad-api-linux-{amd64,arm64} · web/(Vite 웹 빌드)
#   2) 서버에서 손으로:  bash ~/careerpad/scripts/deploy.sh
#      묶음 없이 지금 바이너리로 pm2 만 다시 띄운다 (.env 를 바꾼 뒤). 새 코드를 올리려면 Actions → deploy → Run workflow.
#
# 안전장치: 전체 시간 제한(DEPLOY_TIMEOUT, 기본 10분) · 동시 배포 잠금 · 기동 확인 실패 시 이전 바이너리로 되돌리기
set -euo pipefail

APP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$APP_DIR"

# ── 전체 시간 제한 ── Actions 가 취소돼 접속이 끊겨도 서버에서 끝없이 돌지 않게, timeout 아래에서 자신을 다시 실행한다.
# timeout 은 자식을 별도 프로세스 그룹으로 띄우므로 시간이 넘으면 이 스크립트가 띄운 프로세스까지 함께 끝낸다.
DEPLOY_TIMEOUT="${DEPLOY_TIMEOUT:-600}"
if [ -z "${DEPLOY_UNDER_TIMEOUT:-}" ]; then
  export DEPLOY_UNDER_TIMEOUT=1
  exec timeout --kill-after=20s "$DEPLOY_TIMEOUT" bash "${BASH_SOURCE[0]}" "$@"
fi

# ── 동시 배포 잠금 ── 앞 배포가 아직 돌고 있으면 기다리지 않고 실패한다 (메모리가 작아 둘이 겹치면 위험)
exec 9>/tmp/careerpad-deploy.lock
if ! flock -n 9; then
  echo "!! 다른 커리어++ 배포가 진행 중입니다. 끝난 뒤 다시 실행하세요." >&2
  exit 1
fi

# 비대화형 SSH 세션은 ~/.bashrc 를 읽지 않으므로 nvm 으로 설치한 pm2 를 직접 잡는다
export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"
if [ -s "$NVM_DIR/nvm.sh" ]; then
  set +u
  # shellcheck disable=SC1091
  . "$NVM_DIR/nvm.sh"
  set -u
fi

log() { echo "== $(date '+%F %T') $*"; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# ── 빌드 묶음 받기 (Actions) ── 터미널이 아니면 표준입력을 읽는다. 비어 있으면 손으로 부른 것으로 본다
COMMIT=""
if [ ! -t 0 ]; then
  head -c 300000000 > "$WORK/bundle.tgz"   # 300MB 넘으면 잘려서 아래 tar 가 실패한다
  if [ -s "$WORK/bundle.tgz" ]; then
    mkdir "$WORK/bundle"
    tar -xzf "$WORK/bundle.tgz" -C "$WORK/bundle"
    COMMIT="$(tr -d '[:space:]' < "$WORK/bundle/COMMIT")"
    if ! [[ "$COMMIT" =~ ^[0-9a-f]{40}$ ]]; then
      echo "!! 빌드 묶음의 COMMIT 값이 올바르지 않습니다: '$COMMIT'" >&2
      exit 1
    fi
  fi
fi

if [ -z "$COMMIT" ]; then
  log "빌드 묶음 없음 → 지금 바이너리로 다시 띄우기만 합니다 (새 코드는 Actions 로 배포)"
  pm2 startOrReload ecosystem.config.cjs --update-env
  pm2 save
  log "완료"
  exit 0
fi

log "배포 시작 (서버 $(git rev-parse --short HEAD) → 빌드 ${COMMIT:0:7})"

# ── 코드(설정 · 스크립트)를 빌드한 커밋과 정확히 맞춘다 ──
git fetch --quiet origin main
if ! git cat-file -e "$COMMIT^{commit}" 2>/dev/null; then
  echo "!! 빌드 커밋 ${COMMIT:0:7} 을 원격 main 에서 찾을 수 없습니다 (push 되지 않은 커밋?)" >&2
  exit 1
fi
if ! git merge-base --is-ancestor HEAD "$COMMIT"; then
  echo "!! 서버 코드($(git rev-parse --short HEAD))가 빌드 커밋보다 앞서 있습니다. 더 오래된 빌드라 적용하지 않습니다." >&2
  exit 1
fi
git merge --ff-only --quiet "$COMMIT"

# ── 바꿔 끼우기 ── 서버 아키텍처에 맞는 바이너리를 고른다
case "$(uname -m)" in
  x86_64) ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  *) echo "!! 지원하지 않는 아키텍처: $(uname -m)" >&2; exit 1 ;;
esac
BIN="$WORK/bundle/bin/careerpad-api-linux-$ARCH"
[ -s "$BIN" ] || { echo "!! 빌드 묶음에 $ARCH 바이너리가 없습니다" >&2; exit 1; }
[ -s "$WORK/bundle/web/index.html" ] || { echo "!! 빌드 묶음에 웹(web/index.html)이 없습니다" >&2; exit 1; }

mkdir -p backend/bin
install -m 755 "$BIN" backend/bin/careerpad-api.new
rm -rf web/dist.new && cp -r "$WORK/bundle/web" web/dist.new

[ -f backend/bin/careerpad-api ] && cp -p backend/bin/careerpad-api backend/bin/careerpad-api.prev
mv backend/bin/careerpad-api.new backend/bin/careerpad-api
rm -rf web/dist.prev && { [ -d web/dist ] && mv web/dist web/dist.prev || true; }
mv web/dist.new web/dist

pm2 startOrReload ecosystem.config.cjs --update-env

# ── 기동 확인 ── 실패하면 이전 바이너리 · 웹으로 되돌린다
PORT="$(grep -E '^PORT=' .env 2>/dev/null | tail -1 | cut -d= -f2 | tr -d "\"' ")"
PORT="${PORT:-8092}"
for _ in $(seq 1 15); do
  if curl -fsS --max-time 2 "http://127.0.0.1:$PORT/healthz" > /dev/null 2>&1; then
    pm2 save
    log "배포 완료 (${COMMIT:0:7}, linux-$ARCH)"
    exit 0
  fi
  sleep 2
done

echo "!! 새 버전이 30초 안에 /healthz(포트 $PORT)에 응답하지 않습니다." >&2
# 시작하자마자 죽은 이유(설정 누락 · DB 접속 실패 등)만 골라 보여 준다. 공개 저장소 로그에 남으므로 DB 사용자 이름은 가린다
REASON="$(pm2 logs careerpad-api --lines 60 --nostream --raw 2>/dev/null   | grep -E '"server exited"|panic:|address already in use' | tail -3   | sed -E 's/postgres\.[A-Za-z0-9]+/postgres.***/g' | cut -c1-500 || true)"
echo "── 시작 실패 이유 (자세히: 서버에서 pm2 logs careerpad-api) ──" >&2
echo "${REASON:-(로그에서 찾지 못함)}" >&2

if [ -f backend/bin/careerpad-api.prev ]; then
  mv backend/bin/careerpad-api.prev backend/bin/careerpad-api
  if [ -d web/dist.prev ]; then rm -rf web/dist && mv web/dist.prev web/dist; fi
  pm2 startOrReload ecosystem.config.cjs --update-env
  echo "!! 이전 버전으로 되돌렸습니다." >&2
else
  echo "!! 되돌릴 이전 버전이 없습니다(첫 배포). 원인을 고친 뒤 서버에서 bash scripts/deploy.sh 로 다시 띄우세요." >&2
fi
exit 1
