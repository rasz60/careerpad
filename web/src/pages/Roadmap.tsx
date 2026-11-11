import { useEffect, useId, useRef, useState, type DragEvent, type ReactNode } from 'react';
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router-dom';
import { Icon, useDialog } from '../components/form';
import { PageHead, useWorkspace } from '../components/Workspace';
import { api, errorText } from '../lib/api';
import { dotDate, monthsBetween, pct, roadmapStatus, seoulToday, STATUS_LABEL, type RoadmapStatus } from '../lib/roadmap';
import { convertPrompt, newRoadmapPrompt, type NewRoadmapAnswers } from '../lib/roadmapPrompt';
import type { RoadmapDraft, RoadmapSummary } from '../lib/types';

const ORDER: RoadmapStatus[] = ['active', 'upcoming', 'ended'];

const EMPTY_TEXT: Record<RoadmapStatus, string> = {
  active: '오늘이 기간 안에 든 로드맵이 여기에 보여요.',
  upcoming: '시작일이 아직 오지 않은 로드맵이 여기에 모여요.',
  ended: '종료일이 지난 로드맵은 기록으로 남아요. 열어서 달성률을 다시 볼 수 있어요.',
};

/** /me/roadmap — 로드맵 목록 (진행 중 · 예정 · 끝남). 진행 중이 하나뿐이면 바로 그 로드맵을 연다 */
export const RoadmapList = () => {
  const { roadmaps } = useWorkspace();
  const [params, setParams] = useSearchParams();
  const [creating, setCreating] = useState(params.get('new') === '1');
  const today = seoulToday();
  const list = roadmaps.data ?? [];
  const active = list.filter((r) => roadmapStatus(r, today) === 'active');

  if (!params.has('all') && !creating && active.length === 1) {
    return <Navigate to={`/me/roadmap/${active[0]!.id}`} replace />;
  }

  const close = () => {
    setCreating(false);
    if (params.has('new')) setParams({ all: '1' }, { replace: true });
  };

  return (
    <>
      <PageHead title="로드맵" sub="기간을 정해 두고 달마다 할 일을 체크해 가는 계획표">
        <button type="button" className="btn primary small" onClick={() => setCreating(true)}>
          로드맵 만들기
        </button>
      </PageHead>

      {roadmaps.error ? (
        <div className="sheet-error">
          <p>{errorText(roadmaps.error)}</p>
          <button type="button" className="btn ghost small" onClick={roadmaps.reload}>
            다시 불러오기
          </button>
        </div>
      ) : null}
      {roadmaps.loading && !roadmaps.data && <div className="skeleton" aria-busy="true" />}

      {roadmaps.data && list.length === 0 && (
        <div className="roadmap-empty">
          <p>Claude와 만든 체크리스트 md 파일을 올리면 달마다 체크하고 달성률을 볼 수 있어요.</p>
          <button type="button" className="btn primary" onClick={() => setCreating(true)}>
            md 파일로 만들기
          </button>
        </div>
      )}

      {roadmaps.data &&
        list.length > 0 &&
        ORDER.map((status) => {
          const group = list.filter((r) => roadmapStatus(r, today) === status);
          return (
            <section key={status} className="roadmap-group">
              <h2>{STATUS_LABEL[status]}</h2>
              {group.length === 0 ? (
                <p className="quiet">{EMPTY_TEXT[status]}</p>
              ) : (
                <ul>
                  {group.map((r) => (
                    <li key={r.id}>
                      <RoadmapCard roadmap={r} status={status} />
                    </li>
                  ))}
                </ul>
              )}
            </section>
          );
        })}

      {creating && <CreateRoadmap onClose={close} />}
    </>
  );
};

const RoadmapCard = ({ roadmap: r, status }: { roadmap: RoadmapSummary; status: RoadmapStatus }) => {
  const p = pct(r.done, r.total);
  return (
    <Link to={`/me/roadmap/${r.id}`} className={`roadmap-card is-${status}`}>
      <span className="roadmap-card-main">
        <strong>{r.title}</strong>
        <span className="muted">
          <span className="num">
            {dotDate(r.starts_on)} – {dotDate(r.ends_on)}
          </span>
          {r.phases.length > 0 && ` · 구간 ${r.phases.length}개`}
          {status === 'active' && r.overdue > 0 && <span className="badge warn">기한 지남 {r.overdue}</span>}
        </span>
        <span className="meter" aria-hidden="true">
          <i style={{ width: `${p}%` }} />
        </span>
      </span>
      <span className="roadmap-card-side">
        <b className="roadmap-pct">{p}%</b>
        <span className="num muted">
          {r.done} / {r.total}
        </span>
      </span>
    </Link>
  );
};

// ── 만들기 ──────────────────────────────────

type Tab = 'md' | 'ai';

/** 로드맵 만들기 대화상자: md 파일(또는 붙여 넣기) → 서버 미리보기 → 저장. 두 번째 칸은 Claude 부탁 글 만들기 */
const CreateRoadmap = ({ onClose }: { onClose: () => void }) => {
  const { roadmaps } = useWorkspace();
  const navigate = useNavigate();
  const [tab, setTab] = useState<Tab>('md');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const md = useMarkdownPreview();

  const box = useRef<HTMLDivElement>(null);
  const { close, closing, onBackdropMouseDown } = useDialog(box, onClose, busy);

  const save = async () => {
    setBusy(true);
    setError('');
    try {
      const saved = await api.createRoadmap(md.text);
      roadmaps.reload();
      navigate(`/me/roadmap/${saved.id}`);
    } catch (e) {
      setError(errorText(e));
      setBusy(false);
    }
  };

  const canSave = !!md.draft && md.draft.errors.length === 0 && !md.loading && !busy;

  return (
    <div className="modal-backdrop" data-closing={closing || undefined} onMouseDown={onBackdropMouseDown}>
      <div ref={box} className="modal" role="dialog" aria-modal="true" aria-labelledby="roadmap-create-title" tabIndex={-1}>
        <header className="modal-head">
          <h2 id="roadmap-create-title">로드맵 만들기</h2>
          <button type="button" className="icon-btn" onClick={close} disabled={busy} aria-label="닫기">
            <Icon name="close" />
          </button>
        </header>
        <div className="modal-body">
          <div className="seg-tabs" role="tablist" aria-label="만드는 방법">
            <button type="button" role="tab" aria-selected={tab === 'md'} onClick={() => setTab('md')}>
              md 파일로
            </button>
            <button type="button" role="tab" aria-selected={tab === 'ai'} onClick={() => setTab('ai')}>
              Claude로 만들기
            </button>
          </div>
          {tab === 'md' ? <MarkdownPane md={md} /> : <PromptPane onUseMarkdown={() => setTab('md')} />}
        </div>
        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}
        {tab === 'md' && (
          <footer className="modal-foot">
            <span className={`grow small-note ${md.draft?.errors.length ? 'is-error' : ''}`}>{footNote(md.draft)}</span>
            <button type="button" className="btn ghost" onClick={close} disabled={busy}>
              취소
            </button>
            <button type="button" className="btn primary" onClick={() => void save()} disabled={!canSave}>
              {busy ? '저장 중…' : '로드맵 저장'}
            </button>
          </footer>
        )}
      </div>
    </div>
  );
};

const footNote = (d: RoadmapDraft | undefined): string => {
  if (!d) return '';
  if (d.errors.length) return '오류를 고친 뒤 다시 올려 주세요';
  if (d.warnings.length) return '경고가 난 줄은 빼고 저장해요';
  return '';
};

interface MarkdownState {
  text: string;
  fileName: string;
  draft: RoadmapDraft | undefined;
  loading: boolean;
  error: string;
  setText: (text: string, fileName?: string) => void;
}

/** 붙여 넣은 글이나 올린 파일을 서버에서 읽어 본다 (타이핑이 멈추면 한 번) */
const useMarkdownPreview = (): MarkdownState => {
  const [text, setRaw] = useState('');
  const [fileName, setFileName] = useState('');
  const [draft, setDraft] = useState<RoadmapDraft>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!text.trim()) {
      setDraft(undefined);
      setError('');
      return;
    }
    let alive = true;
    setLoading(true);
    const timer = setTimeout(() => {
      api.previewRoadmap(text).then(
        (d) => alive && (setDraft(d), setError(''), setLoading(false)),
        (e: unknown) => alive && (setDraft(undefined), setError(errorText(e)), setLoading(false)),
      );
    }, 350);
    return () => {
      alive = false;
      clearTimeout(timer);
    };
  }, [text]);

  return {
    text,
    fileName,
    draft,
    loading,
    error,
    setText: (t, name = '') => {
      setRaw(t);
      setFileName(name);
    },
  };
};

const MAX_FILE = 200 * 1024;

const MarkdownPane = ({ md }: { md: MarkdownState }) => {
  const inputId = useId();
  const [dragging, setDragging] = useState(false);
  const [fileError, setFileError] = useState('');

  const readFile = async (file: File | undefined) => {
    if (!file) return;
    setFileError('');
    if (file.size > MAX_FILE) {
      setFileError('파일이 너무 커요 (200KB까지)');
      return;
    }
    md.setText(await file.text(), file.name);
  };
  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    setDragging(false);
    void readFile(e.dataTransfer.files[0]);
  };

  return (
    <div className="md-pane">
      <label
        htmlFor={inputId}
        className={`drop-zone ${dragging ? 'is-over' : ''}`}
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}
      >
        <svg aria-hidden="true" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
          <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
          <path d="M14 3v5h5" />
          <path d="M12 12v6M9 15l3-3 3 3" />
        </svg>
        <span>
          <strong>{md.fileName || 'md 파일 고르기'}</strong>
          <small>{md.fileName ? '다른 파일을 고르려면 누르거나 여기에 끌어 놓으세요' : '누르거나 여기에 끌어 놓으세요 (.md · .txt, 200KB까지)'}</small>
        </span>
        <input
          id={inputId}
          type="file"
          accept=".md,.markdown,.txt,text/markdown,text/plain"
          className="sr-only"
          onChange={(e) => {
            void readFile(e.target.files?.[0]);
            e.target.value = '';
          }}
        />
      </label>
      {fileError && <p className="error">{fileError}</p>}

      <details className="paste">
        <summary>파일 대신 붙여 넣기</summary>
        <textarea
          rows={7}
          aria-label="md 붙여 넣기"
          value={md.text}
          onChange={(e) => md.setText(e.target.value)}
          placeholder={'---\ntitle: 로드맵 이름\nperiod: 2026-11-01 ~ 2028-06-30\n---\n### 2026-11\n- [ ] [분류] 할 일'}
        />
      </details>

      {md.error && <p className="error">{md.error}</p>}
      {md.loading && !md.draft && <p className="muted">읽는 중…</p>}
      {md.draft && <DraftPreview draft={md.draft} />}
    </div>
  );
};

const DraftPreview = ({ draft }: { draft: RoadmapDraft }) => {
  const r = draft.roadmap;
  const months = new Set(r.items.map((it) => it.month)).size;
  const counts: [string, number][] = [
    ['구간', r.phases.length],
    ['달', months],
    ['할 일', r.items.length],
    ['마일스톤', r.items.filter((it) => it.is_key).length],
    ['습관', r.habits.length],
    ['분류', r.categories.length],
  ];
  const messages = [
    ...draft.errors.map((m) => ({ ...m, kind: '오류' as const })),
    ...draft.warnings.map((m) => ({ ...m, kind: '경고' as const })),
  ];
  const span = r.starts_on && r.ends_on ? monthsBetween(r.starts_on.slice(0, 7), r.ends_on.slice(0, 7)).length : 0;
  return (
    <section className={`draft ${draft.errors.length ? 'has-errors' : ''}`} aria-label="미리보기" aria-live="polite">
      <header>
        <h3>{r.title || '제목 없음'}</h3>
        {r.starts_on && (
          <span className="num muted">
            {dotDate(r.starts_on)} – {dotDate(r.ends_on)} · {span}개월
          </span>
        )}
      </header>
      <dl className="draft-counts">
        {counts.map(([label, n]) => (
          <div key={label}>
            <dt>{label}</dt>
            <dd>{n}</dd>
          </div>
        ))}
      </dl>
      {messages.length > 0 ? (
        <ul className="draft-messages">
          {messages.slice(0, 20).map((m, i) => (
            <li key={i}>
              <span className={`badge ${m.kind === '오류' ? 'danger' : 'warn'}`}>{m.kind}</span>
              <span>
                {m.line > 0 && <b className="num">{m.line}번째 줄 </b>}
                {m.text}
              </span>
            </li>
          ))}
          {messages.length > 20 && <li className="muted">그 밖에 {messages.length - 20}개 더</li>}
        </ul>
      ) : (
        <p className="draft-ok">모든 줄을 읽었어요. 저장하면 바로 체크할 수 있어요.</p>
      )}
    </section>
  );
};

// ── Claude 부탁 글 ───────────────────────────

const PromptPane = ({ onUseMarkdown }: { onUseMarkdown: () => void }) => {
  const [kind, setKind] = useState<'new' | 'convert'>('new');
  const [a, setA] = useState<NewRoadmapAnswers>({ goal: '', startsOn: '', endsOn: '', situation: '', categories: '' });
  const [copied, setCopied] = useState<'' | 'ok' | 'fail'>('');
  const promptRef = useRef<HTMLTextAreaElement>(null);
  const prompt = kind === 'new' ? newRoadmapPrompt(a) : convertPrompt();
  const set = (key: keyof NewRoadmapAnswers) => (v: string) => {
    setA((x) => ({ ...x, [key]: v }));
    setCopied('');
  };

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(prompt);
      setCopied('ok');
    } catch {
      // 클립보드를 못 쓰면(권한 · http) 글을 골라 두어 직접 복사하게 한다
      promptRef.current?.select();
      setCopied('fail');
    }
  };

  return (
    <div className="prompt-pane">
      <p className="quiet">
        적은 내용으로 Claude에 보낼 부탁 글을 만들어 드려요. Claude가 준 md를{' '}
        <button type="button" className="text-btn" onClick={onUseMarkdown}>
          md 파일로
        </button>{' '}
        칸에 넣으면 로드맵이 됩니다.
      </p>
      <div className="radio-cards" role="radiogroup" aria-label="부탁 종류">
        <RadioCard checked={kind === 'new'} onChange={() => setKind('new')} title="새로 만들기" sub="목표와 기간으로 처음부터" />
        <RadioCard checked={kind === 'convert'} onChange={() => setKind('convert')} title="기존 체크리스트 바꾸기" sub="HTML · 백업 JSON · 메모를 이 형식으로" />
      </div>

      {kind === 'new' ? (
        <div className="prompt-fields">
          <PromptField label="이루고 싶은 것" value={a.goal} onChange={set('goal')} placeholder="예: 정보처리기사 · 리눅스마스터를 따고 광주·전남 IT 직군으로 이직" multiline />
          <div className="grid-2">
            <PromptField label="시작일" type="date" value={a.startsOn} onChange={set('startsOn')} />
            <PromptField label="종료일" type="date" value={a.endsOn} onChange={set('endsOn')} />
          </div>
          <PromptField label="지금 상황" value={a.situation} onChange={set('situation')} placeholder="예: 인프라 운영 3년 차, 평일 저녁 1시간 · 주말 반나절 공부 가능" multiline />
          <PromptField label="나눌 분류 (쉼표로, 6개까지)" value={a.categories} onChange={set('categories')} placeholder="예: 자격증, 기본기, 코딩테스트, 경력 심화" />
        </div>
      ) : (
        <p className="note-box">부탁 글을 복사한 뒤, 기존 체크리스트 파일(HTML이나 백업 JSON)을 함께 Claude에 올려 주세요. 체크한 항목과 완료일도 그대로 옮겨 달라고 적혀 있어요.</p>
      )}

      <div className="field">
        <label htmlFor="roadmap-prompt">부탁 글 미리보기</label>
        <textarea id="roadmap-prompt" ref={promptRef} className="prompt-preview" rows={8} readOnly value={prompt} />
      </div>
      <div className="row-end">
        <span className="grow small-note is-ok" role="status">
          {copied === 'ok' ? '복사했어요. Claude에 붙여 넣으세요.' : copied === 'fail' ? '복사하지 못했어요. 글을 골라 두었으니 직접 복사해 주세요.' : ''}
        </span>
        <a className="btn ghost" href="https://claude.ai/new" target="_blank" rel="noopener noreferrer">
          Claude 열기
        </a>
        <button type="button" className="btn primary" onClick={() => void copy()}>
          부탁 글 복사
        </button>
      </div>
    </div>
  );
};

const RadioCard = ({ checked, onChange, title, sub }: { checked: boolean; onChange: () => void; title: string; sub: ReactNode }) => (
  <label className={`radio-card ${checked ? 'is-on' : ''}`}>
    <input type="radio" name="prompt-kind" checked={checked} onChange={onChange} />
    <span>
      <strong>{title}</strong>
      <small>{sub}</small>
    </span>
  </label>
);

interface PromptFieldProps {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  type?: 'text' | 'date';
  multiline?: boolean;
}

const PromptField = ({ label, value, onChange, placeholder, type = 'text', multiline }: PromptFieldProps) => {
  const id = useId();
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      {multiline ? (
        <textarea id={id} rows={2} value={value} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} />
      ) : (
        <input id={id} type={type} value={value} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} />
      )}
    </div>
  );
};
