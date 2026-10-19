import { useEffect, useState } from 'react';
import { Link, NavLink, Outlet, useOutletContext, useParams } from 'react-router-dom';
import { api, ApiError, errorText } from '../lib/api';
import { useSession } from '../lib/session';
import type { PublicPage } from '../lib/types';
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

/** 공개 페이지 틀: 머리말(이름 · 한 줄 소개) + 메뉴 + 본문 */
export const Layout = () => {
  const { slug = '' } = useParams();
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
        <p>{missing ? '주소가 바뀌었거나 아직 공개되지 않은 페이지입니다.' : errorText(page.error)}</p>
      </main>
    );
  }

  const { is_owner: owner } = page.data;
  const p = page.data.profile;
  const context: PageContext = { page: page.data, reload: page.reload, setPage: page.setData };

  return (
    <div className="shell">
      <header className="masthead">
        <div className="masthead-inner">
          {session && <aep-switcher mode="inline" className="switcher" />}
          <Link className="brand" to={`/${p.slug}`}>
            {p.photo_url && <img className="avatar" src={p.photo_url} alt="" />}
            <span>
              <strong>{p.name || p.slug}</strong>
              {p.headline && <small>{p.headline}</small>}
            </span>
          </Link>
          {owner && <OwnerBar slug={p.slug} isPublic={p.is_public} />}
        </div>
        <nav className="tabs" aria-label="메뉴">
          <NavLink to={`/${p.slug}`} end>
            About me
          </NavLink>
          <NavLink to={`/${p.slug}/projects`}>Project</NavLink>
          {owner && <NavLink to={`/${p.slug}/jobs`}>Job++</NavLink>}
          <NavLink to={`/${p.slug}/contact`}>Contact</NavLink>
        </nav>
      </header>
      <main className="content">
        <Outlet context={context} />
      </main>
      <footer className="foot">
        <span>© {new Date().getFullYear()} {p.name || p.slug}</span>
        <span>커리어++ · ae:p</span>
      </footer>
    </div>
  );
};

interface OwnerBarProps {
  slug: string;
  isPublic: boolean;
}

/** 주인에게만: 공개 여부 · 공개 링크 복사 */
const OwnerBar = ({ slug, isPublic }: OwnerBarProps) => {
  const [copied, setCopied] = useState(false);
  const url = `${location.origin}${import.meta.env.BASE_URL}${slug}`;
  const copy = async () => {
    await navigator.clipboard.writeText(url);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };
  return (
    <div className="owner-bar">
      <span className={`badge ${isPublic ? 'ok' : 'muted'}`} title={isPublic ? '링크가 있으면 누구나 볼 수 있어요' : '나만 볼 수 있어요'}>
        {isPublic ? '공개 중' : '비공개'}
      </span>
      <button type="button" className="btn ghost small" onClick={() => void copy()}>
        {copied ? '복사됨' : '링크 복사'}
      </button>
    </div>
  );
};
