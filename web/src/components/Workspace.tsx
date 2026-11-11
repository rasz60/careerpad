import { useEffect, useState, type ReactNode } from 'react';
import { Navigate, NavLink, Outlet, useLocation, useOutletContext } from 'react-router-dom';
import { api, errorText } from '../lib/api';
import { daysUntil } from '../lib/format';
import { useSession } from '../lib/session';
import { roadmapStatus, seoulToday } from '../lib/roadmap';
import type { Message, PublicPage, RoadmapSummary } from '../lib/types';
import { useAsync, type AsyncState } from '../lib/useAsync';
import { useSwitcher, type PageContext } from './Layout';

/** 메뉴마다 색이 하나씩 (바인더 인덱스 탭). 화면 안의 주 버튼 · 강조도 그 색을 쓴다 */
export type Tone = 'slate' | 'sage' | 'mustard' | 'peri' | 'teal' | 'rose';

/** 공용 테스터 계정(P-0004)에게 숨기는 탭 (서버도 403 으로 막는다) */
const TESTER_HIDDEN = new Set(['/me/jobs', '/me/inbox']);

const TABS: { to: string; label: string; tone: Tone }[] = [
  { to: '/me', label: '오늘', tone: 'slate' },
  { to: '/me/roadmap', label: '로드맵', tone: 'rose' },
  { to: '/me/jobs', label: '채용공고', tone: 'sage' },
  { to: '/me/certs', label: '자격증', tone: 'mustard' },
  { to: '/me/resume', label: '이력서', tone: 'peri' },
  { to: '/me/inbox', label: '메시지', tone: 'teal' },
];

export interface WorkspaceContext extends PageContext {
  messages: Message[] | undefined;
  /** 읽음 · 지우기 뒤 메시지 목록 (탭의 새 메시지 수도 같이 바뀐다) */
  setMessages: (next: Message[]) => void;
  /** 내 로드맵 목록 (탭의 기한 지남 수 · 오늘 화면 · 로드맵 목록이 같이 쓴다). 체크 뒤 reload */
  roadmaps: AsyncState<RoadmapSummary[]>;
  /** 공용 테스터 계정이면 true. 채용공고 · 메시지를 숨긴다 */
  isTester: boolean;
}

export const useWorkspace = (): WorkspaceContext => useOutletContext<WorkspaceContext>();

/** 내 공간을 불러온다: 내 프로필의 주소로 공개 페이지 응답을 받으면 비공개 항목까지 온다 */
const loadMine = async (onTester: (isTester: boolean) => void): Promise<PublicPage> => {
  const me = await api.myProfile();
  onTester(me.is_tester === true);
  return api.publicPage(me.slug);
};

/** /me — 나만 쓰는 작업 공간. 왼쪽(좁은 화면은 위쪽) 인덱스 탭 + 흰 종이 한 장 */
export const Workspace = () => {
  const { ready, session } = useSession();
  const { pathname } = useLocation();
  const [isTester, setIsTester] = useState(false);
  const page = useAsync(() => (session ? loadMine(setIsTester) : Promise.reject(new Error('로그인이 필요해요'))), [session?.user.id]);
  const messages = useAsync(() => (session ? api.messages() : Promise.resolve([])), [session?.user.id]);
  const roadmaps = useAsync(() => (session ? api.roadmaps() : Promise.resolve([])), [session?.user.id]);
  useSwitcher(!!session);

  const tab = [...TABS].reverse().find((t) => pathname === t.to || pathname.startsWith(`${t.to}/`)) ?? TABS[0]!;
  useEffect(() => {
    document.title = `${tab.label} — 커리어++`;
  }, [tab.label]);

  if (!ready) return <div className="page-loading" aria-busy="true" />;
  if (!session) return <Navigate to="/" replace />;

  const unread = (messages.data ?? []).filter((m) => !m.is_read).length;
  const certs = page.data?.profile.resume.certificates ?? [];
  const nextExam = certs
    .filter((c) => c.status === 'preparing')
    .map((c) => daysUntil(c.exam_on))
    .filter((d): d is number => d !== null && d >= 0)
    .sort((a, b) => a - b)[0];
  // 진행 중인 로드맵의 기한 지난 할 일 수 (로드맵을 못 불러와도 탭은 그대로)
  const today = seoulToday();
  const overdue = (roadmaps.data ?? [])
    .filter((r) => roadmapStatus(r, today) === 'active')
    .reduce((n, r) => n + r.overdue, 0);
  const notes: Partial<Record<string, ReactNode>> = {
    '/me/roadmap': overdue > 0 && <span aria-label={`기한 지난 할 일 ${overdue}개`}>{overdue}</span>,
    '/me/certs': nextExam !== undefined && (nextExam === 0 ? 'D-day' : `D-${nextExam}`),
    '/me/inbox': unread > 0 && unread,
  };

  return (
    <div className="desk" data-tone={tab.tone}>
      <a className="skip-link no-print" href="#sheet">
        본문으로 건너뛰기
      </a>
      <header className="desk-head no-print">
        <aep-switcher mode="inline" className="switcher" />
        <span className="desk-brand">커리어++</span>
        {page.data && <span className="desk-owner">{page.data.profile.name || page.data.profile.slug}</span>}
      </header>
      <div className="binder">
        <nav className="tabs no-print" aria-label="메뉴">
          {TABS.filter((t) => !(isTester && TESTER_HIDDEN.has(t.to))).map((t) => (
            <NavLink key={t.to} to={t.to} end={t.to === '/me'} className="tab" data-tone={t.tone}>
              <span>{t.label}</span>
              {notes[t.to] && <span className="tab-note">{notes[t.to]}</span>}
            </NavLink>
          ))}
        </nav>
        <main className="sheet" id="sheet" tabIndex={-1}>
          {page.error ? (
            <div className="sheet-error">
              <p>{errorText(page.error)}</p>
              <button type="button" className="btn ghost small" onClick={page.reload}>
                다시 불러오기
              </button>
            </div>
          ) : page.data ? (
            <Outlet
              context={
                {
                  page: page.data,
                  reload: page.reload,
                  setPage: page.setData,
                  messages: messages.data,
                  setMessages: messages.setData,
                  roadmaps,
                  isTester,
                } satisfies WorkspaceContext
              }
            />
          ) : (
            <div className="page-loading" aria-busy="true" />
          )}
        </main>
      </div>
    </div>
  );
};

/** 테스터 계정이 채용공고 · 메시지 주소로 직접 들어왔을 때 */
export const TesterBlocked = ({ title }: { title: string }) => (
  <>
    <PageHead title={title} sub="테스터 계정에서는 쓸 수 없는 기능이에요." />
    <p className="quiet">초대받은 계정으로 로그인하면 쓸 수 있어요. 로드맵 · 자격증 · 이력서는 테스터 계정에서도 써 볼 수 있어요.</p>
  </>
);

interface PageHeadProps {
  title: string;
  sub?: ReactNode;
  children?: ReactNode;
}

/** 종이 맨 위: 제목(형광펜) · 한 줄 설명 · 오른쪽 도구 */
export const PageHead = ({ title, sub, children }: PageHeadProps) => (
  <header className="page-head">
    <div>
      <h1>
        <span className="marker">{title}</span>
      </h1>
      {sub && <p className="page-sub">{sub}</p>}
    </div>
    {children && <div className="page-actions no-print">{children}</div>}
  </header>
);
