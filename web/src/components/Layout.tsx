import { useEffect } from 'react';
import { Link, Outlet, useOutletContext, useParams } from 'react-router-dom';
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
export const useSwitcher = (enabled: boolean) => {
  useEffect(() => {
    if (!enabled || switcherLoaded || import.meta.env.DEV) return;
    switcherLoaded = true;
    document.head.append(Object.assign(document.createElement('script'), { type: 'module', src: '/shell/switcher.js' }));
  }, [enabled]);
};

/** /:slug — 방문자가 보는 공개 이력서. 책상 위에 놓인 이력서 한 장 */
export const Layout = () => {
  const { slug = '' } = useParams();
  const { ready, session } = useSession();
  const page = useAsync(() => api.publicPage(slug), [slug, session?.user.id]);

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
  const { profile: p, is_owner: owner } = page.data;
  return (
    <div className="desk public" data-tone="peri">
      {owner && (
        <p className="owner-strip no-print">
          <span>방문자에게 보이는 내 이력서예요{p.is_public ? '' : ' (지금은 비공개라 나만 볼 수 있어요)'}.</span>
          <Link to="/me/resume">작업 공간으로 돌아가기</Link>
        </p>
      )}
      <div className="resume-sheet">
        <aside className="rail">
          <div className="rail-inner">
            <Identity profile={p} />
            <a className="btn primary block no-print" href="#contact">
              메시지 남기기
            </a>
          </div>
        </aside>
        <main className="resume-main">
          <Outlet context={context} />
        </main>
      </div>
      <footer className="foot no-print">
        © {new Date().getFullYear()} {p.name || p.slug} — 커리어++ 로 만든 이력서
      </footer>
    </div>
  );
};

/** 이름 · 한 줄 소개 · 연락처 (공개 이력서 왼쪽 칸 · 작업 공간 이력서 머리) */
export const Identity = ({ profile: p }: { profile: Profile }) => (
  <div className="identity">
    {p.photo_url && <img className="id-photo" src={p.photo_url} alt={`${p.name} 사진`} />}
    <div className="id-text">
      <h1 className="id-name">{p.name || p.slug}</h1>
      {p.headline && <p className="id-headline">{p.headline}</p>}
      {p.location && <p className="id-meta">{p.location}</p>}
    </div>
    {(p.public_email || p.public_phone || p.links.length > 0) && (
      <ul className="id-contacts">
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
              {l.label}
            </a>
          </li>
        ))}
      </ul>
    )}
  </div>
);
