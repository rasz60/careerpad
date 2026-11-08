-- 0003: 테스터 계정 (P-0004) — 기본 데이터 잠금 · 테스터가 새로 만든 데이터 정리 · 막은 기능
--
-- 실행: Supabase SQL Editor 에서 이 파일 전체를 실행한다. 다시 실행해도 된다.
-- 먼저 Database → Extensions 에서 pg_cron 을 켠다 (매일 새벽 정리).
--
-- 테스터 상태는 auth.users.raw_app_meta_data 에 있다. 포털 계정 관리가 바꾸고, 사용자는 바꿀 수 없다.
--   role = 'tester'             테스터 계정
--   tester_editing = true       관리자가 기본 데이터를 넣는 중 → 잠금 · 정리 · 막은 쓰기가 모두 꺼진다
--   tester_baseline_at = 시각    마지막 '잠그기' 시각. 이 시각까지 만든 행 = 기본 데이터(잠금)
--
-- 앱(Go)도 테스터에게 막은 기능을 API 에서 먼저 거절한다. 트리거는 마지막 방어선이다.
-- 오류 코드: AE401 = 기본 데이터를 바꾸려 함, AE402 = 테스터에게 막은 기능 (repository/postgres 가 도메인 에러로 바꾼다)

BEGIN;

-- 테스터이고 편집 중이 아니면 true
CREATE OR REPLACE FUNCTION career.tester_active(owner uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = '' AS $$
  SELECT coalesce((
    SELECT u.raw_app_meta_data ->> 'role' = 'tester'
       AND coalesce(u.raw_app_meta_data -> 'tester_editing', 'false'::jsonb) <> 'true'::jsonb
      FROM auth.users u
     WHERE u.id = owner
  ), false)
$$;

-- 기본 데이터 기준 시각. 한 번도 잠그지 않았으면 -infinity
CREATE OR REPLACE FUNCTION career.tester_baseline(owner uuid) RETURNS timestamptz
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = '' AS $$
  SELECT coalesce((
    SELECT (u.raw_app_meta_data ->> 'tester_baseline_at')::timestamptz
      FROM auth.users u
     WHERE u.id = owner
  ), '-infinity'::timestamptz)
$$;

-- 테스터가 바꿀 수 없는 기본 데이터인지
CREATE OR REPLACE FUNCTION career.tester_locked(owner uuid, created timestamptz) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = '' AS $$
  SELECT career.tester_active(owner) AND created IS NOT NULL AND created <= career.tester_baseline(owner)
$$;

-- 잠금 트리거
--   tg_argv[0] = 주인 열 (user_id · owner_id)
--   tg_argv[1] = 부모 테이블. 있으면 주인 열은 부모 id 를 가리키고, 부모의 주인 · 만든 시각을 따른다
--   tg_argv[2] = 'block' 이면 테스터의 이 쓰기를 항상 막는다
CREATE OR REPLACE FUNCTION career.tester_guard() RETURNS trigger
LANGUAGE plpgsql SET search_path = '' AS $$
DECLARE
  r jsonb;
  owner uuid;
  parent_created timestamptz;
  created timestamptz;
BEGIN
  -- 정리 함수가 지우는 중
  IF current_setting('ae.tester_reset', true) = 'on' THEN
    RETURN coalesce(NEW, OLD);
  END IF;
  -- 다른 트리거 안에서 일어난 지우기는 판정하지 않는다
  --   계정 · 부모 행을 지우면 딸린 행이 연쇄로 지워진다 (기본 행이 새 행에 딸리는 일은 없다)
  IF TG_OP = 'DELETE' AND pg_trigger_depth() > 1 THEN
    RETURN OLD;
  END IF;

  IF TG_OP = 'INSERT' THEN r := to_jsonb(NEW); ELSE r := to_jsonb(OLD); END IF;
  IF TG_NARGS >= 2 AND TG_ARGV[1] <> '' THEN
    EXECUTE format('SELECT user_id, created_at FROM %s WHERE id = $1', TG_ARGV[1]::regclass)
       INTO owner, parent_created
      USING (r ->> TG_ARGV[0])::uuid;
  ELSE
    owner := (r ->> TG_ARGV[0])::uuid;
  END IF;
  IF owner IS NULL OR NOT career.tester_active(owner) THEN
    RETURN coalesce(NEW, OLD);
  END IF;

  IF TG_NARGS >= 3 AND TG_ARGV[2] = 'block' THEN
    RAISE EXCEPTION USING ERRCODE = 'AE402', MESSAGE = '테스터 계정에서는 쓸 수 없는 기능이에요';
  END IF;

  -- 새 행은 만든 시각을 지금으로 고정한다 (과거 시각을 넣어 기본 데이터처럼 만드는 것을 막는다)
  IF TG_OP = 'INSERT' THEN
    IF r ? 'created_at' THEN
      NEW := jsonb_populate_record(NEW, jsonb_build_object('created_at', now()));
    END IF;
    RETURN NEW;
  END IF;

  created := coalesce((r ->> 'created_at')::timestamptz, parent_created);
  -- 다른 트리거가 고치는 것은 막지 않되, 기본 행이면 원래 값을 그대로 둔다
  IF TG_OP = 'UPDATE' AND pg_trigger_depth() > 1 THEN
    IF created IS NOT NULL AND created <= career.tester_baseline(owner) THEN
      RETURN OLD;
    END IF;
    RETURN NEW;
  END IF;
  IF created IS NOT NULL AND created <= career.tester_baseline(owner) THEN
    RAISE EXCEPTION USING ERRCODE = 'AE401', MESSAGE = '기본 데이터는 테스터가 바꿀 수 없어요';
  END IF;
  IF TG_OP = 'UPDATE' AND (to_jsonb(NEW) ->> 'created_at') IS DISTINCT FROM (r ->> 'created_at') THEN
    RAISE EXCEPTION USING ERRCODE = 'AE401', MESSAGE = '만든 시각은 바꿀 수 없어요';
  END IF;
  RETURN coalesce(NEW, OLD);
END $$;

-- 사용자 데이터: 잠금 + 만든 시각 고정
DROP TRIGGER IF EXISTS tester_guard ON career.profiles;
CREATE TRIGGER tester_guard BEFORE INSERT OR UPDATE OR DELETE ON career.profiles
  FOR EACH ROW EXECUTE FUNCTION career.tester_guard('user_id');
DROP TRIGGER IF EXISTS tester_guard ON career.projects;
CREATE TRIGGER tester_guard BEFORE INSERT OR UPDATE OR DELETE ON career.projects
  FOR EACH ROW EXECUTE FUNCTION career.tester_guard('user_id');
DROP TRIGGER IF EXISTS tester_guard ON career.job_searches;
CREATE TRIGGER tester_guard BEFORE INSERT OR UPDATE OR DELETE ON career.job_searches
  FOR EACH ROW EXECUTE FUNCTION career.tester_guard('user_id');
DROP TRIGGER IF EXISTS tester_guard ON career.contact_settings;
CREATE TRIGGER tester_guard BEFORE INSERT OR UPDATE OR DELETE ON career.contact_settings
  FOR EACH ROW EXECUTE FUNCTION career.tester_guard('user_id');
DROP TRIGGER IF EXISTS tester_guard ON career.messages;
CREATE TRIGGER tester_guard BEFORE INSERT OR UPDATE OR DELETE ON career.messages
  FOR EACH ROW EXECUTE FUNCTION career.tester_guard('owner_id');
DROP TRIGGER IF EXISTS tester_guard ON career.roadmaps;
CREATE TRIGGER tester_guard BEFORE INSERT OR UPDATE OR DELETE ON career.roadmaps
  FOR EACH ROW EXECUTE FUNCTION career.tester_guard('user_id');
DROP TRIGGER IF EXISTS tester_guard ON career.roadmap_items;
CREATE TRIGGER tester_guard BEFORE INSERT OR UPDATE OR DELETE ON career.roadmap_items
  FOR EACH ROW EXECUTE FUNCTION career.tester_guard('roadmap_id', 'career.roadmaps');
DROP TRIGGER IF EXISTS tester_guard ON career.roadmap_habits;
CREATE TRIGGER tester_guard BEFORE INSERT OR UPDATE OR DELETE ON career.roadmap_habits
  FOR EACH ROW EXECUTE FUNCTION career.tester_guard('roadmap_id', 'career.roadmaps');

-- 항상 막는 쓰기: 채용공고 검색, 메시지 알림 설정, 메시지 (남기기 · 읽음 · 삭제)
DROP TRIGGER IF EXISTS tester_block ON career.job_searches;
CREATE TRIGGER tester_block BEFORE INSERT OR UPDATE OR DELETE ON career.job_searches
  FOR EACH ROW EXECUTE FUNCTION career.tester_guard('user_id', '', 'block');
DROP TRIGGER IF EXISTS tester_block ON career.contact_settings;
CREATE TRIGGER tester_block BEFORE INSERT OR UPDATE ON career.contact_settings
  FOR EACH ROW EXECUTE FUNCTION career.tester_guard('user_id', '', 'block');
DROP TRIGGER IF EXISTS tester_block ON career.messages;
CREATE TRIGGER tester_block BEFORE INSERT OR UPDATE OR DELETE ON career.messages
  FOR EACH ROW EXECUTE FUNCTION career.tester_guard('owner_id', '', 'block');

-- 정리: 기준 시각 이후에 만든 테스터 데이터를 지운다. 지운 행 수를 돌려준다.
-- 프로필은 지우지 않는다 (지우면 프로젝트 · 로드맵을 넣을 곳이 없어진다)
CREATE OR REPLACE FUNCTION career.tester_reset(uid uuid) RETURNS integer
LANGUAGE plpgsql SECURITY DEFINER SET search_path = '' AS $$
DECLARE
  b timestamptz;
  e timestamptz;
  n integer := 0;
  c integer;
  is_tester boolean;
  editing boolean;
  started timestamptz;
BEGIN
  SELECT u.raw_app_meta_data ->> 'role' = 'tester',
         coalesce(u.raw_app_meta_data -> 'tester_editing', 'false'::jsonb) = 'true'::jsonb,
         (u.raw_app_meta_data ->> 'tester_edit_started_at')::timestamptz
    INTO is_tester, editing, started
    FROM auth.users u WHERE u.id = uid;
  IF NOT coalesce(is_tester, false) THEN
    RETURN 0;
  END IF;
  b := career.tester_baseline(uid);
  -- 편집 중: 편집을 시작하기 전에 남은 방문자 데이터만 지운다 (관리자가 넣는 중인 기본 데이터는 남긴다)
  e := CASE WHEN editing THEN coalesce(started, '-infinity'::timestamptz) ELSE 'infinity'::timestamptz END;

  PERFORM set_config('ae.tester_reset', 'on', true);
  -- 기본 로드맵에 새로 넣은 할 일 · 습관
  DELETE FROM career.roadmap_items i USING career.roadmaps r
   WHERE i.roadmap_id = r.id AND r.user_id = uid AND i.created_at > b AND i.created_at < e;
  GET DIAGNOSTICS c = ROW_COUNT; n := n + c;
  DELETE FROM career.roadmap_habits h USING career.roadmaps r
   WHERE h.roadmap_id = r.id AND r.user_id = uid AND h.created_at > b AND h.created_at < e;
  GET DIAGNOSTICS c = ROW_COUNT; n := n + c;
  DELETE FROM career.roadmaps WHERE user_id = uid AND created_at > b AND created_at < e;
  GET DIAGNOSTICS c = ROW_COUNT; n := n + c;
  DELETE FROM career.projects WHERE user_id = uid AND created_at > b AND created_at < e;
  GET DIAGNOSTICS c = ROW_COUNT; n := n + c;
  DELETE FROM career.job_searches WHERE user_id = uid AND created_at > b AND created_at < e;
  GET DIAGNOSTICS c = ROW_COUNT; n := n + c;
  -- 방문자가 테스터 공개 페이지에 남긴 메시지
  DELETE FROM career.messages WHERE owner_id = uid AND created_at > b AND created_at < e;
  GET DIAGNOSTICS c = ROW_COUNT; n := n + c;
  PERFORM set_config('ae.tester_reset', 'off', true);
  RETURN n;
END $$;

-- 테스터 계정 모두 정리 (매일 새벽)
CREATE OR REPLACE FUNCTION career.tester_reset_all() RETURNS integer
LANGUAGE plpgsql SECURITY DEFINER SET search_path = '' AS $$
DECLARE
  uid uuid;
  n integer := 0;
BEGIN
  FOR uid IN SELECT u.id FROM auth.users u WHERE u.raw_app_meta_data ->> 'role' = 'tester' LOOP
    n := n + career.tester_reset(uid);
  END LOOP;
  RETURN n;
END $$;

-- 권한: 이 스키마는 앱 백엔드만 쓴다 (0001 과 같다)
REVOKE ALL ON FUNCTION career.tester_active(uuid), career.tester_baseline(uuid), career.tester_locked(uuid, timestamptz),
  career.tester_guard(), career.tester_reset(uuid), career.tester_reset_all() FROM PUBLIC, anon, authenticated;

COMMIT;

-- 매일 04:00(한국) 정리 = 19:00 UTC. 같은 이름으로 다시 등록하면 덮어쓴다
CREATE EXTENSION IF NOT EXISTS pg_cron;
SELECT cron.schedule('career-tester-reset', '0 19 * * *', 'SELECT career.tester_reset_all()');
