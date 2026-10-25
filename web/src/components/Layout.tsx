import { useEffect, useState } from 'react';
import { Link, NavLink, Outlet, useLocation, useOutletContext, useParams } from 'react-router-dom';
import { api, ApiError, errorText } from '../lib/api';
import { phoneLabel } from '../lib/format';
import { useSession } from '../lib/session';
import type { Profile, PublicPage } from '../lib/types';
import { useAsync } from '../lib/useAsync';

/** 화면(Outlet)에 넘기는 값 */
export interface PageContext {
  page: PublicPage;
  /** 저장 뒤 다시 불러오기 */
  reload: () => void;
  /** 저장 응답으로 화면만 바로 바꾸기 */
  setPage: (next: PublicPage) => void;
}

export const usePage = (): PageContext => useOutletContext<PageContext>();

let switcherLoaded = false;

/** 포털 앱 전환 버튼 스크립트. 로그인한 사람에게만 (방문자에게는 ae:p 메뉴가 의미 없다) */
const useSwitcher = (enabled: boolean) => {
  useEffect(() => {
    if (!enabled || switcherLoaded || import.meta.env.DEV) return;
    switcherLoaded = true;
    document.head.append(Object.assign(document.createElement('script'), { type: 'module', src: '/shell/switcher.js' }));
  }, [enabled]);
};

/** 주인만 쓰는 관리 화면 (채용공고 · 받은 메시지) */
const MANAGE = ['jobs', 'inbox'];

/** /:slug 아래 공통 틀. 방문자는 한 장짜리 이력서, 주인의 관리 화면은 위쪽 막대 틀 */
export const Layout = () => {
  const { slug = '' } = useParams();
  const { pathname } = useLocation();
  const { ready, session } = useSession();
  const page = useAsync(() => api.publicPage(slug), [slug, session?.user.id]);
  useSwitcher(!!session);

  const profile = page.data?.profile;
  useEffect(() => {
    document.title = profile ? `${profile.name || profile.slug} — 커리어++` : '커리어++';
  }, [profile]);

  if (!ready || (page.loading && !page.data)) return <div className="page-loading" aria-busy="true" />;
  if (page.error || !page.data) {
    const missing = page.error instanceof ApiError && page.error.status === 404;
    return (
      <main className="center-note">
        <h1>{missing ? '페이지를 찾을 수 없어요' : '불러오지 못했어요'}</h1>
        <p className="muted">{missing ? '주소가 바뀌었거나 아직 공개되지 않은 페이지예요.' : errorText(page.error)}</p>
      </main>
    );
  }

  const context: PageContext = { page: page.data, reload: page.reload, setPage: page.setData };
  const manage = MANAGE.includes(pathname.split('/').filter(Boolean).pop() ?? '');
  return manage ? <ManageLayout context={context} signedIn={!!session} /> : <ResumeLayout context={context} signedIn={!!session} />;
};

interface FrameProps {
  context: PageContext;
  signedIn: boolean;
}

/** 공개 이력서: 왼쪽 칸(이름 · 연락처 · 주인 도구) + 본문 한 열 */
const ResumeLayout = ({ context, signedIn }: FrameProps) => {
  const { profile: p, is_owner: owner } = context.page;
  return (
    <div className="resume-shell">
      <aside className="rail">
        <div className="rail-inner">
          {signedIn && (
            <div className="rail-top no-print">
              <aep-switcher mode="inline" className="switcher" />
            </div>
          )}
          <Identity profile={p} />
          <div className="rail-actions">
            <a className="btn primary block" href="#contact">
              메시지 남기기
            </a>
          </div>
          {owner && <OwnerBox profile={p} />}
        </div>
      </aside>
      <main className="resume-main">
        <Outlet context={context} />
        <footer className="foot">
          <span>© {new Date().getFullYear()} {p.name || p.slug}</span>
          <span>커리어++ · ae:p</span>
        </footer>
      </main>
    </div>
  );
};

/** 이름 · 한 줄 소개 · 연락처 */
const Identity = ({ profile: p }: { profile: Profile }) => (
  <>
    {p.photo_url && <img className="rail-photo" src={p.photo_url} alt={`${p.name} 사진`} />}
    <div className="rail-id">
      <h1 className="rail-name">{p.name || p.slug}</h1>
      {p.headline && <p className="rail-headline">{p.headline}</p>}
      {p.location && <p className="rail-meta">{p.location}</p>}
    </div>
    {(p.public_email || p.public_phone || p.links.length > 0) && (
      <ul className="rail-contacts">
        {p.public_email && (
          <li>
            <span>메일</span>
            <a href={`mailto:${p.public_email}`}>{p.public_email}</a>
          </li>
        )}
        {p.public_phone && (
          <li>
            <span>전화</span>
            <a href={`tel:${p.public_phone}`}>{phoneLabel(p.public_phone)}</a>
          </li>
        )}
        {p.links.map((l) => (
          <li key={l.url}>
            <span>링크</span>
            <a href={l.url} target="_blank" rel="noopener noreferrer">
              {l.label} ↗
            </a>
          </li>
        ))}
      </ul>
    )}
  </>
);

/** 주인에게만: 공개 상태 · 링크 복사 · 편집 · PDF · 관리 화면 */
const OwnerBox = ({ profile: p }: { profile: Profile }) => {
  const [copied, setCopied] = useState(false);
  const unread = useUnread();
  const url = `${location.origin}${import.meta.env.BASE_URL}${p.slug}`;
  const copy = async () => {
    await navigator.clipboard.writeText(url);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };
  return (
    <section className="owner-box no-print" aria-label="내 페이지 관리">
      <div className="owner-box-head">
        <span>내 페이지</span>
        <span className={`badge ${p.is_public ? 'ok' : 'muted'}`} title={p.is_public ? '링크가 있으면 누구나 볼 수 있어요' : '나만 볼 수 있어요'}>
          {p.is_public ? '공개 중' : '비공개'}
        </span>
      </div>
      <div className="owner-links">
        <Link to={`/${p.slug}?edit=1`}>이력서 편집</Link>
        <button type="button" onClick={() => void copy()}>
          {copied ? '링크를 복사했어요' : '공개 링크 복사'}
        </button>
        <button type="button" onClick={() => window.print()}>
          PDF로 저장
        </button>
        <Link to={`/${p.slug}/jobs`}>채용공고</Link>
        <Link to={`/${p.slug}/inbox`}>
          받은 메시지 {unread > 0 && <span className="badge warn">{unread}</span>}
        </Link>
      </div>
    </section>
  );
};

/** 읽지 않은 메시지 수 (주인만) */
const useUnread = (): number => {
  const messages = useAsync(() => api.messages(), []);
  return (messages.data ?? []).filter((m) => !m.is_read).length;
};

/** 주인의 관리 화면: 위쪽 막대(이름 · 이력서로 돌아가기) + 탭 */
const ManageLayout = ({ context, signedIn }: FrameProps) => {
  const { profile: p } = context.page;
  return (
    <div className="manage">
      <header className="manage-bar">
        <div className="manage-bar-inner">
          {signedIn && <aep-switcher mode="inline" className="switcher" />}
          <div className="manage-title">
            <strong>{p.name || p.slug}</strong>
            <span className="muted">관리</span>
          </div>
          <Link className="btn ghost small" to={`/${p.slug}`}>
            내 이력서 보기
          </Link>
        </div>
        <nav className="manage-tabs" aria-label="관리 메뉴">
          <NavLink to={`/${p.slug}/jobs`}>채용공고</NavLink>
          <NavLink to={`/${p.slug}/inbox`}>받은 메시지</NavLink>
        </nav>
      </header>
      <main className="manage-content">
        <Outlet context={context} />
      </main>
    </div>
  );
};
