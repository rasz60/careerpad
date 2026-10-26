# 커리어++ (careerpad)

ae:p 포털(`https://ae2p.duckdns.org`)의 **커리어++** 앱입니다. 웹은 포털 아래 `/career/` 에서 돕니다.
나만 쓰는 **작업 공간(`/career/me`)** 이 중심이고, 그중 이력서 하나를 **공개 페이지(`/career/<주소>`)** 로 내보내
로그인하지 않은 사람도 링크로 볼 수 있습니다.

| 작업 공간 메뉴 (`/me/…`) | 내용 |
|---|---|
| 오늘 (`/me`) | 다가오는 시험 · 유효기간 D-day, 새 메시지, 저장한 공고 검색, 이력서 채운 정도 |
| 채용공고 (`jobs`) | 사이트 · 키워드 · 근무지 · 경력 · 제외어로 검색 조건 저장 → 누르면 사이트별 공고 목록 |
| 자격증 (`certs`) | 준비 중(시험일 D-day) · 취득(점수 · 자격번호 · 유효기간) 관리. 취득한 것만 공개 이력서에 나간다 |
| 이력서 (`resume`) | 인적 사항 · 소개 · 경력 · 프로젝트 · 학력 · 기술 편집, PDF 저장, 공개 링크 |
| 메시지 (`inbox`) | 공개 이력서의 '메시지 남기기'로 온 연락 + 알림 받을 곳(메일 · SMS) |

공개 페이지(`/:slug`)는 이력서 한 장 + 메시지 남기기 폼입니다. 주인이 열면 위에 '작업 공간으로 돌아가기' 띠가 붙고,
나머지는 방문자와 똑같이 보입니다 (비공개 프로젝트 · 준비 중인 자격은 빠진다). 예전 주소 `/:slug/jobs` · `/:slug/inbox` 는 `/me/…` 로 넘어갑니다.

화면은 회색 책상 위 흰 종이 한 장 + 종이 가장자리의 인덱스 탭입니다. 탭마다 색이 하나씩 있고(오늘 · 채용공고 · 자격증 · 이력서 · 메시지),
그 화면의 주 버튼 · 제목 형광펜 · 강조도 같은 색을 씁니다 (`web/src/styles.css` 의 `[data-tone]`).

## 구조

```
https://ae2p.duckdns.org/career/*      Caddy ─── web/dist           React SPA (base /career/)
https://ae2p.duckdns.org/api/career/*  Caddy ─── 127.0.0.1:8092     Go API (pm2: careerpad-api)
                                                    │
                      공용 Supabase ─ auth.users ───┤ 로그인 토큰 검증 (JWKS 공개키)
                                   └─ career.*   ───┘ 풀러로 직접 SQL (postgres 역할)
```

- **로그인은 포털이 합니다.** 같은 도메인이라 포털의 Supabase 세션을 그대로 씁니다. 공개 페이지는 세션이 없어도 열리고,
  로그인한 주인이 보면 같은 화면에 편집 · Job++ · 받은 메시지가 더해집니다 (`is_owner`).
- **DB 는 공용 Supabase 의 `career` 스키마**입니다. 브라우저 키(anon/authenticated)로는 못 읽게 막고 Go 백엔드만 접근합니다.
  무엇을 공개할지는 백엔드가 정합니다 (비공개 프로필 · 비공개 프로젝트 · 알림 받을 연락처는 주인에게만).
- **이력서 본문은 `profiles.resume`(jsonb) 한 칸**입니다. 화면 구성이 자주 바뀔 수 있어 마이그레이션 없이 항목을 늘리게 했습니다
  (Go `domain.Resume` · TS `Resume` 타입이 형식을 정합니다).
- **Job++ 사이트**
  - 목록을 가져옴: 원티드 · 점핏(사이트 화면이 쓰는 공개 JSON), 사람인(공식 Open API — `SARAMIN_ACCESS_KEY` 가 있을 때)
  - 검색 링크만: 잡코리아 · LinkedIn · 키 없는 사람인
  - 키워드마다 따로 검색해 공고 ID 로 합치고, 근무지 · 경력 · 제외어로 거릅니다. 사이트 · 조건별 결과는 10분 캐시
  - 근무지 · 경력은 **사이트에 검색 조건으로 넘깁니다** (원티드 `locations` · `years`, 점핏 `locationTag` · `career`, 사람인 `loc_mcd`).
    근무지 글자("나주", "서울 강남구", "서울, 경기")는 [regions.go](backend/internal/platform/jobsite/regions.go) 가 지역 코드로 바꾸고,
    표 [regions_data.go](backend/internal/platform/jobsite/regions_data.go) 는 원티드 `/api/v4/filters?country=kr` 의 지역 목록으로 만든 것입니다.
    점핏은 광주 · 전남을 `전남광주` 로 묶어 표시합니다. 근무지를 알 수 없는 공고는 빼지 않고 사이트 칸에 안내를 띄웁니다
  - 공개 JSON 은 사이트가 예고 없이 바꿀 수 있습니다. 깨지면 해당 사이트만 오류로 표시되고 검색 링크는 남습니다
    (`backend/internal/platform/jobsite` 의 어댑터만 고치면 됩니다)
- **Contact 알림**: SMTP 메일 · 솔라피 SMS. 설정이 없는 채널은 꺼지고, 알림이 실패해도 메시지는 받은 메시지함에 남습니다.
  봇 방지 숨은 칸 · IP 당 시간당 횟수 제한 · 하루 SMS 상한이 있습니다.

## 디렉터리

```
careerpad/
├─ backend/                         # Go API (표준 net/http, pgx)
│  ├─ cmd/api/main.go               # 진입점 · 의존성 조립
│  ├─ internal/
│  │  ├─ domain/                    # 엔티티 + 리포지토리 · 어댑터 인터페이스
│  │  ├─ usecase/                   # profile, project, job, contact (+ 테스트)
│  │  ├─ repository/postgres/       # SQL (career.* 스키마)
│  │  ├─ platform/                  # auth(Supabase JWT), jobsite(채용 사이트), notify(메일·SMS), ratelimit
│  │  ├─ transport/http/            # 라우터 · 핸들러 · 미들웨어
│  │  └─ config/
│  └─ migrations/0001_career_schema.sql
├─ web/                             # React + Vite + TS
│  └─ src/ pages(Today · Jobs · Certificates · About · Projects · Contact · Home) · components(Workspace · Layout) · lib
├─ scripts/deploy.sh                # 서버 배포 (GitHub Actions 가 호출)
└─ ecosystem.config.cjs             # pm2
```

## API

| 메서드 · 경로 (`/api/career` 뒤) | 인증 | 설명 |
|---|---|---|
| `GET /api/v1/public/{slug}` | 선택 | 프로필 + 프로젝트. 주인이면 비공개 포함 · `is_owner: true` |
| `POST /api/v1/public/{slug}/contact` | 없음 | 연락 남기기 |
| `GET·PUT /api/v1/me/profile` | 필요 | 내 프로필 (처음 GET 때 이메일 아이디로 주소를 정해 만든다) |
| `GET·POST /api/v1/me/projects`, `PUT·DELETE …/{id}` | 필요 | 프로젝트 |
| `GET /api/v1/me/job-sites` | 필요 | 고를 수 있는 사이트 |
| `GET·POST /api/v1/me/job-searches`, `PUT·DELETE …/{id}`, `GET …/{id}/results` | 필요 | 검색 조건 · 사이트별 결과 |
| `GET·PUT /api/v1/me/contact-settings` | 필요 | 알림 받을 곳 |
| `GET /api/v1/me/messages`, `PATCH·DELETE …/{id}` | 필요 | 받은 메시지 |

## 개발

```powershell
# 1회 준비 (PowerShell)
Copy-Item .env.example .env               # SUPABASE_URL, DATABASE_URL(풀러) …
Copy-Item web\.env.example web\.env       # VITE_SUPABASE_URL, VITE_SUPABASE_ANON_KEY (포털과 같은 값)
cd web; npm install; cd ..

# 터미널 1 — 백엔드 (http://127.0.0.1:8092)
. .\scripts\load-env.ps1                  # 앞의 점(.) 필수. backend 폴더에서는 . ..\scripts\load-env.ps1
cd backend; go run ./cmd/api

# 터미널 2 — 웹 (http://localhost:5173/career/) — /api/career 는 vite 가 8092 로 넘긴다
cd web; npm run dev

# 검사
cd backend; go vet ./...; go test ./...
cd web; npm run typecheck
```

bash(서버 · Git Bash)에서는 `set -a; . ./.env; set +a` 로 환경변수를 올린 뒤 `cd backend && go run ./cmd/api`.

개발 서버에는 포털이 없어서 `/career/` 에서 같은 ae:p 계정으로 바로 로그인합니다 (배포본은 포털 로그인).

## 배포 (ae:p Oracle 서버, 운동기록과 같은 방식)

**서버에서는 빌드하지 않습니다.** 서버(메모리 1GB)에서 운동기록을 빌드하다 서버가 멈춘 뒤로, Go 바이너리(linux amd64 · arm64)와 웹은
GitHub Actions 러너가 만들고, 묶음(tar.gz)을 SSH 표준입력으로 보내면 서버의 [scripts/deploy.sh](scripts/deploy.sh) 가 받아서
코드를 빌드 커밋에 맞추고 → 서버 아키텍처에 맞는 바이너리 · 웹을 바꿔 끼우고 → pm2 재시작 → `/healthz` 확인 (30초 안에 응답이 없으면 이전 버전으로 되돌림).
`deploy.sh` 는 전체 10분 제한(`DEPLOY_TIMEOUT`) · 동시 배포 잠금이 있습니다. 서버에 Go 는 필요 없습니다.

1. **DB**: Supabase SQL Editor 에서 [backend/migrations/0001_career_schema.sql](backend/migrations/0001_career_schema.sql) 실행
2. **서버에 받기**: `git clone git@github-careerpad:rasz60/careerpad.git ~/careerpad` (읽기 전용 Deploy key. 공개 저장소면 https 로 받아도 된다)
3. **`.env`**: 루트 `.env`(백엔드)만. 웹 값은 서버가 아니라 GitHub 저장소 **Variables** 에 `VITE_SUPABASE_URL` · `VITE_SUPABASE_ANON_KEY`
4. **Caddy**: 포털 저장소 `deploy/Caddyfile` 의 `/career/` · `/api/career/` 블록 반영 → `sudo systemctl reload caddy`
5. **자동 배포**: Secrets(`DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_SSH_KEY`, `DEPLOY_KNOWN_HOSTS`)는 운동기록과 같은 방법으로
   **이 저장소 전용 키**를 만들어 넣습니다 (`command="bash $HOME/careerpad/scripts/deploy.sh",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty …`)
6. **첫 배포**: GitHub → Actions → deploy → **Run workflow** (이후로는 `main` 에 push 하면 자동)

운영: `.env` 를 바꾼 뒤에는 서버에서 `bash ~/careerpad/scripts/deploy.sh` (빌드 없이 pm2 만 다시 띄운다). 로그는 `pm2 logs careerpad-api`.
