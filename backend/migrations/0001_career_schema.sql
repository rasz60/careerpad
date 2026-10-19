-- 0001_career_schema.sql — 커리어++ 스키마 (ae:p 공용 Supabase 프로젝트의 career 스키마)
-- Supabase SQL Editor 에서 한 번 실행한다. 업무일지(public)·운동기록(workout) 스키마는 건드리지 않는다.
--
-- 계정은 Supabase Auth(auth.users). 로그인은 ae:p 포털이 하고, 공개 페이지(/career/<slug>)는 로그인 없이 열린다.
-- 브라우저(anon/authenticated 키)로는 못 읽게 막고 Go 백엔드(postgres 역할)만 접근한다 — 공개 범위는 백엔드가 정한다.
-- 명명 규칙: 테이블/컬럼 모두 snake_case, 시각은 timestamptz, 날짜만 date.

BEGIN;

CREATE SCHEMA IF NOT EXISTS career;

-- ─────────────────────────────── 프로필 (about me) ───────────────────────────────
-- 이력서 본문(경력·학력·기술·자격)은 화면 구성이 자주 바뀔 수 있어 resume(jsonb) 한 칸에 둔다.
-- 목록·검색 대상이 아니라 프로필과 항상 같이 읽히므로 정규화하지 않아도 조회 비용이 같다.

CREATE TABLE career.profiles (
    user_id      uuid PRIMARY KEY REFERENCES auth.users (id) ON DELETE CASCADE, -- 포털에서 계정을 지우면 같이 지워진다
    slug         text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$'), -- 공개 주소 /career/<slug>
    is_public    boolean NOT NULL DEFAULT false,  -- false 면 본인만 본다
    name         text NOT NULL DEFAULT '',
    headline     text NOT NULL DEFAULT '',        -- 한 줄 소개 (예: 백엔드 개발자 · 8년차)
    summary      text NOT NULL DEFAULT '',
    photo_url    text NOT NULL DEFAULT '',
    location     text NOT NULL DEFAULT '',
    public_email text NOT NULL DEFAULT '',        -- 공개 페이지에 그대로 보이는 연락처 (비우면 숨김)
    public_phone text NOT NULL DEFAULT '',
    links        jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(links) = 'array'),
    resume       jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(resume) = 'object'),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- ─────────────────────────────── 프로젝트 ───────────────────────────────

CREATE TABLE career.projects (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES career.profiles (user_id) ON DELETE CASCADE,
    kind         text NOT NULL CHECK (kind IN ('work', 'personal')), -- 업무 / 개인
    title        text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    organization text NOT NULL DEFAULT '',  -- 회사 · 고객사 · 팀
    role         text NOT NULL DEFAULT '',
    started_on   date,
    ended_on     date,                      -- NULL = 진행 중
    summary      text NOT NULL DEFAULT '',
    description  text NOT NULL DEFAULT '',
    tech_stack   text[] NOT NULL DEFAULT '{}',
    links        jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(links) = 'array'),
    is_public    boolean NOT NULL DEFAULT true,
    sort_order   integer NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CHECK (ended_on IS NULL OR started_on IS NULL OR ended_on >= started_on)
);

-- 공개 페이지·편집 화면 모두 "한 사람의 프로젝트를 정렬 순서대로" 읽는다.
CREATE INDEX idx_projects_user_order ON career.projects (user_id, sort_order, started_on DESC);

-- ─────────────────────────────── 채용공고 검색 조건 (job++) ───────────────────────────────

CREATE TABLE career.job_searches (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL REFERENCES career.profiles (user_id) ON DELETE CASCADE,
    name          text NOT NULL CHECK (length(name) BETWEEN 1 AND 60),
    keywords      text[] NOT NULL CHECK (cardinality(keywords) BETWEEN 1 AND 10),
    sites         text[] NOT NULL CHECK (cardinality(sites) >= 1),    -- wanted, jumpit, saramin, jobkorea, linkedin …
    location      text NOT NULL DEFAULT '',                           -- 근무지에 이 글자가 들어간 공고만 (비우면 전체)
    career_years  integer CHECK (career_years BETWEEN 0 AND 40),      -- 내 경력(년). 공고의 경력 범위에 들어가는 것만
    exclude_words text[] NOT NULL DEFAULT '{}',                       -- 제목·회사에 이 말이 있으면 뺀다
    sort_order    integer NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_job_searches_user_order ON career.job_searches (user_id, sort_order, created_at);

-- ─────────────────────────────── 연락 (contact) ───────────────────────────────

-- 연락 메시지를 받을 곳. 공개 페이지에 보이지 않는다.
CREATE TABLE career.contact_settings (
    user_id      uuid PRIMARY KEY REFERENCES career.profiles (user_id) ON DELETE CASCADE,
    notify_email text NOT NULL DEFAULT '',
    notify_phone text NOT NULL DEFAULT '',
    mail_enabled boolean NOT NULL DEFAULT true,
    sms_enabled  boolean NOT NULL DEFAULT false,
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- 방문자가 남긴 메시지. 받은 편지함 + 발송 기록(하루 SMS 한도 계산)을 겸한다.
CREATE TABLE career.messages (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id     uuid NOT NULL REFERENCES career.profiles (user_id) ON DELETE CASCADE,
    sender_name  text NOT NULL CHECK (length(sender_name) BETWEEN 1 AND 60),
    sender_email text NOT NULL DEFAULT '',
    sender_phone text NOT NULL DEFAULT '',
    body         text NOT NULL CHECK (length(body) BETWEEN 1 AND 2000),
    delivered_by text[] NOT NULL DEFAULT '{}',  -- 실제로 알림이 나간 채널 (mail, sms)
    sender_ip    inet,
    is_read      boolean NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- 받은 편지함(최신순)과 "최근 24시간 SMS 건수" 둘 다 이 인덱스로 끝난다.
CREATE INDEX idx_messages_owner_created ON career.messages (owner_id, created_at DESC);

-- ─────────────────────────────── 권한 ───────────────────────────────
-- 브라우저 키로는 접근 불가. 백엔드는 postgres 역할(소유자)이라 RLS 를 지나간다.
-- FORCE ROW LEVEL SECURITY 는 쓰지 않는다 — 소유자까지 정책 검사를 받아 백엔드가 0행만 보게 된다.

ALTER TABLE career.profiles         ENABLE ROW LEVEL SECURITY;
ALTER TABLE career.projects         ENABLE ROW LEVEL SECURITY;
ALTER TABLE career.job_searches     ENABLE ROW LEVEL SECURITY;
ALTER TABLE career.contact_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE career.messages         ENABLE ROW LEVEL SECURITY;

REVOKE ALL ON SCHEMA career FROM anon, authenticated;
REVOKE ALL ON ALL TABLES IN SCHEMA career FROM anon, authenticated;
ALTER DEFAULT PRIVILEGES IN SCHEMA career REVOKE ALL ON TABLES FROM anon, authenticated;

COMMIT;
