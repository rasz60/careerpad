-- 커리어++ 개인 로드맵 (P-0003)
-- Supabase SQL Editor 에서 0001 다음에 한 번 실행한다. 배포(push) 전에 실행해야 로드맵 API 가 동작한다.
-- 0001 과 같은 규칙: career 스키마, profiles(user_id) 참조, RLS 켜기 + 브라우저 키 권한 회수.

BEGIN;

-- 로드맵 한 장. 구간(phases)은 자주 바뀌지 않는 구조라 jsonb 한 칸에 둔다: [{title, from:"YYYY-MM", to:"YYYY-MM", goal}]
CREATE TABLE career.roadmaps (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES career.profiles (user_id) ON DELETE CASCADE,
    title       text NOT NULL CHECK (length(title) BETWEEN 1 AND 80),
    lead        text NOT NULL DEFAULT '' CHECK (length(lead) <= 200),
    starts_on   date NOT NULL,
    ends_on     date NOT NULL,
    categories  text[] NOT NULL DEFAULT '{}' CHECK (cardinality(categories) <= 6),
    phases      jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(phases) = 'array'),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_on >= starts_on)
);

-- 목록: 한 사람의 로드맵을 시작일 최신순으로
CREATE INDEX idx_roadmaps_user_starts ON career.roadmaps (user_id, starts_on DESC);

-- 체크할 항목. due_month 는 "그 달까지" 할 일이다.
CREATE TABLE career.roadmap_items (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    roadmap_id  uuid NOT NULL REFERENCES career.roadmaps (id) ON DELETE CASCADE,
    due_month   text NOT NULL CHECK (due_month ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    category    text NOT NULL DEFAULT '' CHECK (length(category) <= 20),
    body        text NOT NULL CHECK (length(body) BETWEEN 1 AND 300),
    is_key      boolean NOT NULL DEFAULT false,      -- 마일스톤(★)
    done_on     date,                                -- null = 아직
    sort_order  integer NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_roadmap_items_order ON career.roadmap_items (roadmap_id, due_month, sort_order);

-- 매주 습관. weeks 는 체크한 ISO 주 'YYYY-Www' 목록이다.
CREATE TABLE career.roadmap_habits (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    roadmap_id  uuid NOT NULL REFERENCES career.roadmaps (id) ON DELETE CASCADE,
    name        text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    weeks       text[] NOT NULL DEFAULT '{}',
    sort_order  integer NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_roadmap_habits_order ON career.roadmap_habits (roadmap_id, sort_order);

-- ─────────────────────────────── 권한 (0001 과 같다) ───────────────────────────────
ALTER TABLE career.roadmaps       ENABLE ROW LEVEL SECURITY;
ALTER TABLE career.roadmap_items  ENABLE ROW LEVEL SECURITY;
ALTER TABLE career.roadmap_habits ENABLE ROW LEVEL SECURITY;

REVOKE ALL ON career.roadmaps, career.roadmap_items, career.roadmap_habits FROM anon, authenticated;

COMMIT;
