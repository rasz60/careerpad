import { useEffect, type ReactNode } from 'react';
import { Navigate, NavLink, Outlet, useLocation, useOutletContext } from 'react-router-dom';
import { api, errorText } from '../lib/api';
import { daysUntil } from '../lib/format';
import { useSession } from '../lib/session';
import type { Message, PublicPage } from '../lib/types';
import { useAsync } from '../lib/useAsync';
import { useSwitcher, type PageContext } from './Layout';

/** 메뉴마다 색이 하나씩 (바인더 인덱스 탭). 화면 안의 주 버튼 · 강조도 그 색을 쓴다 */
export type Tone = 'slate' | 'sage' | 'mustard' | 'peri' | 'teal';

const TABS: { to: string; label: string; tone: Tone }[] = [
  { to: '/me', label: '오늘', tone: 'slate' },
  { to: '/me/jobs', label: '채용공고', tone: 'sage' },
  { to: '/me/certs', label: '자격증', tone: 'mustard' },
  { to: '/me/resume', label: '이력서', tone: 'peri' },
  { to: '/me/inbox', label: '메시지', tone: 'teal' },
];

export interface WorkspaceContext extends PageContext {
  messages: Message[] | undefined;
  /** 읽음 · 지우기 뒤 메시지 목록 (탭의 새 메시지 수도 같이 바뀐다) */
  setMessages: (next: Message[]) => void;
}

export const useWorkspace = (): WorkspaceContext => useOutletContext<WorkspaceContext>();

/** 내 공간을 불러온다: 내 프로필의 주소로 공개 페이지 응답을 받으면 비공개 항목까지 온다 */
const loadMine = async (): Promise<PublicPage> => {
  const me = await api.myProfile();
  return api.publicPage(me.slug);
};

/** /me — 나만 쓰는 작업 공간. 왼쪽(좁은 화면은 위쪽) 인덱스 탭 + 흰 종이 한 장 */
export const Workspace = () => {
  const { ready, session } = useSession();
  const { pathname } = useLocation();
  const page = useAsync(() => (session ? loadMine() : Promise.reject(new Error('로그인이 필요해요'))), [session?.user.id]);
  const messages = useAsync(() => (session ? api.messages() : Promise.resolve([])), [session?.user.id]);
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
  const notes: Partial<Record<string, ReactNode>> = {
    '/me/certs': nextExam !== undefined && (nextExam === 0 ? 'D-day' : `D-${nextExam}`),
    '/me/inbox': unread > 0 && unread,
  };

  return (
    <div className="desk" data-tone={tab.tone}>
      <header className="desk-head no-print">
        <aep-switcher mode="inline" className="switcher" />
        <span className="desk-brand">커리어++</span>
        {page.data && <span className="desk-owner">{page.data.profile.name || page.data.profile.slug}</span>}
      </header>
      <div className="binder">
        <nav className="tabs no-print" aria-label="메뉴">
          {TABS.map((t) => (
            <NavLink key={t.to} to={t.to} end={t.to === '/me'} className="tab" data-tone={t.tone}>
              <span>{t.label}</span>
              {notes[t.to] && <span className="tab-note">{notes[t.to]}</span>}
            </NavLink>
          ))}
        </nav>
        <main className="sheet">
          {page.error ? (
            <div className="sheet-error">
              <p>{errorText(page.error)}</p>
              <button type="button" className="btn ghost small" onClick={page.reload}>
                다시 불러오기
              </button>
            </div>
          ) : page.data ? (
            <Outlet
              context={{ page: page.data, reload: page.reload, setPage: page.setData, messages: messages.data, setMessages: messages.setData } satisfies WorkspaceContext}
            />
          ) : (
            <div className="page-loading" aria-busy="true" />
          )}
        </main>
      </div>
    </div>
  );
};

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
