import { createClient } from '@supabase/supabase-js';

const url = import.meta.env.VITE_SUPABASE_URL as string | undefined;
const anonKey = import.meta.env.VITE_SUPABASE_ANON_KEY as string | undefined;

if (!url || !anonKey) {
  throw new Error('VITE_SUPABASE_URL / VITE_SUPABASE_ANON_KEY 가 설정되지 않았습니다. web/.env 를 확인하세요.');
}

// 로그인은 ae:p 포털이 한다. 같은 도메인 + 같은 Supabase 프로젝트라 포털의 세션(localStorage)을 그대로 본다.
// 공개 페이지 방문자는 세션이 없어도 된다
export const supabase = createClient(url, anonKey, {
  auth: { persistSession: true, autoRefreshToken: true, detectSessionInUrl: false },
});

/** 포털 로그인으로 보낸다. 로그인 뒤 지금 주소로 돌아온다 */
export const goToPortalLogin = (): void => {
  location.assign(`/login?next=${encodeURIComponent(location.pathname + location.search)}`);
};
