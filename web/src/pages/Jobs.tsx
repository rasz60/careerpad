import { useState } from 'react';
import { Navigate, useSearchParams } from 'react-router-dom';
import { Field, Modal, WordsField } from '../components/form';
import { usePage } from '../components/Layout';
import { api, errorText } from '../lib/api';
import { deadlineLabel } from '../lib/format';
import type { JobSearch, JobSearchInput, JobSite, SiteResult } from '../lib/types';
import { useAsync } from '../lib/useAsync';

/** job++ (주인만): 저장한 검색 조건 목록 → 조건을 누르면 사이트별 공고 목록 */
export const Jobs = () => {
  const { page } = usePage();
  if (!page.is_owner) return <Navigate to={`/${page.profile.slug}`} replace />;
  return <JobBoard />;
};

const JobBoard = () => {
  const [params, setParams] = useSearchParams();
  const searches = useAsync(() => api.jobSearches(), []);
  const sites = useAsync(() => api.jobSites(), []);
  const [editing, setEditing] = useState<JobSearch | 'new' | null>(null);

  const list = searches.data ?? [];
  const selected = list.find((s) => s.id === params.get('s')) ?? list[0];
  const select = (id: string) => setParams({ s: id }, { replace: true });

  return (
    <section className="jobs">
      <aside className="job-searches">
        <div className="page-tools">
          <h2>검색 조건</h2>
          <span className="grow" />
          <button type="button" className="btn primary small" onClick={() => setEditing('new')} disabled={!sites.data}>
            + 조건
          </button>
        </div>
        {searches.loading && !searches.data && <p className="muted">불러오는 중…</p>}
        {searches.error ? <p className="error">{errorText(searches.error)}</p> : null}
        {searches.data && list.length === 0 && (
          <p className="empty">원하는 사이트와 키워드로 조건을 만들어 두면, 누를 때마다 사이트별 최신 공고를 모아 보여 줘요.</p>
        )}
        <ul>
          {list.map((s) => (
            <li key={s.id}>
              <button type="button" className={`search-card ${s === selected ? 'is-active' : ''}`} onClick={() => select(s.id)}>
                <strong>{s.name}</strong>
                <span className="chips">
                  {s.keywords.map((k) => (
                    <span className="chip" key={k}>
                      {k}
                    </span>
                  ))}
                </span>
                <small className="muted">
                  {[
                    siteNames(s.sites, sites.data),
                    s.location,
                    s.career_years !== null && `경력 ${s.career_years}년`,
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                </small>
              </button>
            </li>
          ))}
        </ul>
      </aside>

      <div className="job-results">
        {selected && (
          <Results
            key={selected.id}
            search={selected}
            onEdit={() => setEditing(selected)}
          />
        )}
      </div>

      {editing && sites.data && (
        <SearchEditor
          search={editing === 'new' ? null : editing}
          sites={sites.data}
          nextOrder={list.length}
          onClose={() => setEditing(null)}
          onSaved={(saved) => {
            setEditing(null);
            searches.reload();
            if (saved) select(saved.id);
          }}
        />
      )}
    </section>
  );
};

const siteNames = (ids: string[], sites: JobSite[] | undefined): string =>
  ids.map((id) => sites?.find((s) => s.id === id)?.name ?? id).join(', ');

interface ResultsProps {
  search: JobSearch;
  onEdit: () => void;
}

const Results = ({ search, onEdit }: ResultsProps) => {
  const results = useAsync(() => api.jobResults(search.id), [search.id, search.updated_at]);
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  const data = results.data ?? [];
  const total = data.reduce((n, r) => n + r.postings.length, 0);

  const toggle = (id: string) =>
    setHidden((h) => {
      const next = new Set(h);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  return (
    <>
      <div className="page-tools">
        <div>
          <h2>{search.name}</h2>
          <small className="muted">
            {results.loading ? '사이트에서 모으는 중…' : `공고 ${total}건`}
            {search.exclude_words.length > 0 && ` · 제외: ${search.exclude_words.join(', ')}`}
          </small>
        </div>
        <span className="grow" />
        <button type="button" className="btn ghost small" onClick={results.reload} disabled={results.loading}>
          새로고침
        </button>
        <button type="button" className="btn ghost small" onClick={onEdit}>
          조건 편집
        </button>
      </div>

      {results.error ? <p className="error">{errorText(results.error)}</p> : null}

      {data.length > 1 && (
        <div className="site-filter" role="group" aria-label="사이트 보기">
          {data.map((r) => (
            <button key={r.site.id} type="button" aria-pressed={!hidden.has(r.site.id)} onClick={() => toggle(r.site.id)}>
              {r.site.name}
              <span className="count">{r.site.listable ? r.postings.length : '링크'}</span>
            </button>
          ))}
        </div>
      )}

      {results.loading && !results.data && <div className="skeleton" aria-busy="true" />}

      {data
        .filter((r) => !hidden.has(r.site.id))
        .map((r) => (
          <SiteSection key={r.site.id} result={r} />
        ))}
    </>
  );
};

interface SiteSectionProps {
  result: SiteResult;
}

const SiteSection = ({ result }: SiteSectionProps) => {
  const { site, links, postings, error } = result;
  return (
    <section className="site-section">
      <header>
        <h3>{site.name}</h3>
        <span className="muted">{site.listable ? `${postings.length}건` : '목록 없이 사이트에서 직접 보기'}</span>
        <span className="grow" />
        <span className="site-links">
          {links.map((l) => (
            <a key={l.keyword} href={l.url} target="_blank" rel="noopener noreferrer">
              {l.keyword} ↗
            </a>
          ))}
        </span>
      </header>
      {error && <p className="error small">{error}</p>}
      {site.listable && postings.length === 0 && !error && <p className="muted">조건에 맞는 공고가 없어요.</p>}
      {postings.length > 0 && (
        <ul className="postings">
          {postings.map((p) => (
            <li key={p.id}>
              <a href={p.url} target="_blank" rel="noopener noreferrer" className="posting">
                <span className="posting-main">
                  <strong>{p.title}</strong>
                  <span className="sub">{[p.company, p.location, p.career].filter(Boolean).join(' · ')}</span>
                  {p.tags.length > 0 && (
                    <span className="chips">
                      {p.tags.slice(0, 6).map((t) => (
                        <span className="chip small" key={t}>
                          {t}
                        </span>
                      ))}
                    </span>
                  )}
                </span>
                <span className="posting-side">
                  <DeadlineBadge deadline={p.deadline} known={site.id !== 'wanted'} />
                  <small className="muted">{p.keywords.join(', ')}</small>
                </span>
              </a>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
};

interface DeadlineBadgeProps {
  deadline: string;
  /** false 면 사이트가 마감일을 주지 않는다 (원티드 검색 API) — 빈 값이 '상시'가 아니라 '모름' */
  known: boolean;
}

const DeadlineBadge = ({ deadline, known }: DeadlineBadgeProps) => {
  if (!deadline && !known) return null;
  const label = deadlineLabel(deadline);
  return <span className={`badge ${label.startsWith('D-') || label === '오늘 마감' ? 'warn' : 'muted'}`}>{label}</span>;
};

interface SearchEditorProps {
  search: JobSearch | null;
  sites: JobSite[];
  nextOrder: number;
  onClose: () => void;
  onSaved: (saved: JobSearch | null) => void;
}

const SearchEditor = ({ search, sites, nextOrder, onClose, onSaved }: SearchEditorProps) => {
  const [draft, setDraft] = useState<JobSearchInput>(
    search
      ? {
          name: search.name, keywords: search.keywords, sites: search.sites, location: search.location,
          career_years: search.career_years, exclude_words: search.exclude_words, sort_order: search.sort_order,
        }
      : {
          name: '', keywords: [], sites: sites.filter((s) => s.listable).map((s) => s.id), location: '',
          career_years: null, exclude_words: [], sort_order: nextOrder,
        },
  );
  const set = <K extends keyof JobSearchInput>(key: K, value: JobSearchInput[K]) => setDraft((d) => ({ ...d, [key]: value }));
  const toggleSite = (id: string) =>
    set('sites', draft.sites.includes(id) ? draft.sites.filter((s) => s !== id) : [...draft.sites, id]);

  const save = async () => {
    onSaved(search ? await api.updateJobSearch(search.id, draft) : await api.createJobSearch(draft));
  };
  const remove = async () => {
    if (!search || !confirm(`‘${search.name}’ 조건을 지울까요?`)) return;
    await api.deleteJobSearch(search.id);
    onSaved(null);
  };

  return (
    <Modal
      title={search ? '검색 조건 편집' : '새 검색 조건'}
      onClose={onClose}
      onSubmit={save}
      extra={
        search && (
          <button type="button" className="btn danger small" onClick={() => void remove()}>
            지우기
          </button>
        )
      }
    >
      <Field label="조건 이름" value={draft.name} onChange={(v) => set('name', v)} placeholder="Go 백엔드 · 서울" required maxLength={60} />
      <WordsField
        label="키워드"
        value={draft.keywords}
        onChange={(v) => set('keywords', v)}
        placeholder="golang, 백엔드, kotlin"
        hint="키워드마다 따로 검색해서 합쳐요 (최대 10개)"
      />
      <fieldset>
        <legend>사이트</legend>
        <div className="check-grid">
          {sites.map((s) => (
            <label key={s.id} className="check">
              <input type="checkbox" checked={draft.sites.includes(s.id)} onChange={() => toggleSite(s.id)} />
              <span>
                {s.name}
                {!s.listable && <small> · 검색 링크만</small>}
              </span>
            </label>
          ))}
        </div>
      </fieldset>
      <div className="grid-2">
        <Field
          label="근무지"
          value={draft.location}
          onChange={(v) => set('location', v)}
          placeholder="서울"
          hint="근무지에 이 글자가 있는 공고만 (근무지를 안 주는 사이트는 그대로)"
        />
        <Field
          label="내 경력 (년)"
          type="number"
          value={draft.career_years === null ? '' : String(draft.career_years)}
          onChange={(v) => set('career_years', v === '' ? null : Math.max(0, Math.min(40, Number(v))))}
          hint="공고의 경력 범위에 드는 것만. 비우면 전체"
        />
      </div>
      <WordsField
        label="제외어"
        value={draft.exclude_words}
        onChange={(v) => set('exclude_words', v)}
        placeholder="영업, 인턴"
        hint="제목 · 회사에 이 말이 있으면 빼요"
      />
    </Modal>
  );
};
