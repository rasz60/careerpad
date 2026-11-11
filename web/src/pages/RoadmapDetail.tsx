import { useEffect, useId, useMemo, useRef, useState, type FormEvent } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { Field, Icon, Modal, RowList, Toggle, WordsField } from '../components/form';
import { PageHead, useWorkspace } from '../components/Workspace';
import { api, errorText } from '../lib/api';
import {
  categoryTone,
  dotDate,
  isDone,
  isOverdue,
  isoWeek,
  monthsBetween,
  monthTitle,
  pct,
  phaseIndex,
  recentWeeks,
  roadmapStats,
  seoulToday,
  shortDay,
} from '../lib/roadmap';
import type { RoadmapDetail, RoadmapHabit, RoadmapInput, RoadmapItem, RoadmapItemInput, RoadmapPhase } from '../lib/types';
import { useAsync } from '../lib/useAsync';

type Filter = 'all' | 'todo' | { category: string };

/** /me/roadmap/:id — 로드맵 한 장: 요약 · 구간별 달 묶음 체크 · 매주 습관 · 통계 */
export const RoadmapDetailPage = () => {
  const { id = '' } = useParams();
  const roadmap = useAsync(() => api.roadmap(id), [id]);

  if (roadmap.error) {
    return (
      <div className="sheet-error">
        <p>{errorText(roadmap.error)}</p>
        <Link className="btn ghost small" to="/me/roadmap?all=1">
          모든 로드맵
        </Link>
      </div>
    );
  }
  if (!roadmap.data) return <div className="page-loading" aria-busy="true" />;
  return <RoadmapView key={roadmap.data.id} initial={roadmap.data} />;
};

const RoadmapView = ({ initial }: { initial: RoadmapDetail }) => {
  const { roadmaps } = useWorkspace();
  const navigate = useNavigate();
  const [d, setD] = useState(initial);
  const [filter, setFilter] = useState<Filter>('all');
  const [collapsed, setCollapsed] = useState<Record<number, boolean>>({});
  const [editingInfo, setEditingInfo] = useState(false);
  const [editingItem, setEditingItem] = useState<RoadmapItem | null>(null);
  const [problem, setProblem] = useState('');
  const today = seoulToday();
  const cur = today.slice(0, 7);
  const week = isoWeek(today);
  const stats = useMemo(() => roadmapStats(d, today), [d, today]);

  // 처음 열 때 이번 달 묶음으로 (첫 달이면 그대로)
  const scrolled = useRef(false);
  useEffect(() => {
    if (scrolled.current) return;
    scrolled.current = true;
    if (cur <= d.starts_on.slice(0, 7)) return;
    const el = document.getElementById(`m-${cur}`);
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    el?.scrollIntoView({ block: 'start', behavior: reduce ? 'auto' : 'smooth' });
  }, [cur, d.starts_on]);

  /**
   * 화면에 먼저 반영하고(apply) 서버에 보낸다. 성공하면 서버 값으로(commit), 실패하면 그 항목만 되돌리고(revert) 이유를 띄운다.
   * 모두 '지금 상태'를 받는 함수라서, 여러 개를 빠르게 체크해도 서로의 변경을 덮어쓰지 않는다.
   */
  const mutate = async <T,>(
    apply: (x: RoadmapDetail) => RoadmapDetail,
    call: () => Promise<T>,
    commit: (x: RoadmapDetail, result: T) => RoadmapDetail,
    revert: (x: RoadmapDetail) => RoadmapDetail,
    countsChanged = true,
  ) => {
    setD(apply);
    setProblem('');
    try {
      const result = await call();
      setD((x) => commit(x, result));
      if (countsChanged) roadmaps.reload();
    } catch (e) {
      setD(revert);
      setProblem(errorText(e));
    }
  };

  const replaceItem = (base: RoadmapDetail, it: RoadmapItem): RoadmapDetail => ({ ...base, items: base.items.map((x) => (x.id === it.id ? it : x)) });
  const replaceHabit = (base: RoadmapDetail, h: RoadmapHabit): RoadmapDetail => ({ ...base, habits: base.habits.map((x) => (x.id === h.id ? h : x)) });

  const toggleItem = (it: RoadmapItem) => {
    const done = !isDone(it);
    void mutate(
      (x) => replaceItem(x, { ...it, done_on: done ? today : '' }),
      () => api.patchRoadmapItem(d.id, it.id, { done }),
      replaceItem,
      (x) => replaceItem(x, it),
    );
  };
  const deleteItem = (it: RoadmapItem) => {
    void mutate(
      (x) => ({ ...x, items: x.items.filter((y) => y.id !== it.id) }),
      () => api.deleteRoadmapItem(d.id, it.id),
      (x) => x,
      (x) => ({ ...x, items: x.items.some((y) => y.id === it.id) ? x.items : [...x.items, it] }),
    );
  };
  const addItem = async (input: RoadmapItemInput) => {
    const saved = await api.addRoadmapItem(d.id, input);
    setD((x) => ({ ...x, items: [...x.items, saved] }));
    roadmaps.reload();
  };
  const saveItem = async (it: RoadmapItem, input: RoadmapItemInput) => {
    const saved = await api.patchRoadmapItem(d.id, it.id, input);
    setD((x) => replaceItem(x, saved));
    roadmaps.reload();
  };
  const toggleWeek = (h: RoadmapHabit) => {
    const on = !h.weeks.includes(week);
    void mutate(
      (x) => replaceHabit(x, { ...h, weeks: on ? [...h.weeks, week].sort() : h.weeks.filter((w) => w !== week) }),
      () => api.checkHabitWeek(d.id, h.id, week, on),
      replaceHabit,
      (x) => replaceHabit(x, h),
      false,
    );
  };
  const addHabit = async (name: string) => {
    const saved = await api.addHabit(d.id, name);
    setD((x) => ({ ...x, habits: [...x.habits, saved] }));
  };
  const deleteHabit = (h: RoadmapHabit) => {
    void mutate(
      (x) => ({ ...x, habits: x.habits.filter((y) => y.id !== h.id) }),
      () => api.deleteHabit(d.id, h.id),
      (x) => x,
      (x) => ({ ...x, habits: x.habits.some((y) => y.id === h.id) ? x.habits : [...x.habits, h].sort((a, b) => a.sort_order - b.sort_order) }),
      false,
    );
  };

  const exportMarkdown = async () => {
    try {
      const text = await api.roadmapMarkdown(d.id);
      const a = document.createElement('a');
      a.href = URL.createObjectURL(new Blob([text], { type: 'text/markdown;charset=utf-8' }));
      a.download = `${d.title.replace(/[\\/:*?"<>|]/g, '_')}.md`;
      a.click();
      URL.revokeObjectURL(a.href);
    } catch (e) {
      setProblem(errorText(e));
    }
  };

  // 구간 · 구간 밖 묶음. 첨부 HTML 과 같은 규칙으로 접는다: 다 끝난 구간과 아직 안 온 구간만 접고,
  // 지나간 구간에 밀린 일이 있으면 펼쳐 둔다 (이번 달이 어느 구간에도 없으면 첫 구간은 펼친다)
  const curPhase = phaseIndex(d.phases, cur);
  const groups = d.phases.map((p, i) => {
    const items = d.items.filter((it) => it.month >= p.from && it.month <= p.to);
    const t = stats.byPhase[i]!;
    const finished = t.total > 0 && t.done === t.total;
    const upcoming = p.from > cur && i !== curPhase && !(curPhase < 0 && i === 0);
    const defaultCollapsed = finished || upcoming;
    return { key: i, phase: p, items, done: t.done, open: !(collapsed[i] ?? defaultCollapsed), months: monthsBetween(p.from, p.to) };
  });
  const loose = d.items.filter((it) => phaseIndex(d.phases, it.month) < 0);
  const looseMonths = monthsBetween(d.starts_on.slice(0, 7), d.ends_on.slice(0, 7)).filter((m) => phaseIndex(d.phases, m) < 0);

  const visible = (it: RoadmapItem) =>
    filter === 'all' ? true : filter === 'todo' ? !isDone(it) : it.category === filter.category;

  const paceText =
    Math.abs(stats.pace) < 3 ? '계획대로 가고 있어요' : stats.pace > 0 ? `계획보다 ${stats.pace}%p 앞서 있어요` : `계획보다 ${-stats.pace}%p 뒤처져 있어요`;
  const monthOf =
    today < d.starts_on ? `${stats.monthCount}개월 · 아직 시작 전` : today > d.ends_on ? `${stats.monthCount}개월 · 끝남` : `${stats.monthCount}개월 중 ${stats.monthIndex}개월째`;

  const itemProps = {
    categories: d.categories,
    cur,
    onToggle: toggleItem,
    onDelete: deleteItem,
    onEdit: setEditingItem,
  };

  return (
    <>
      <Link to="/me/roadmap?all=1" className="back-link no-print">
        모든 로드맵
      </Link>
      <PageHead
        title={d.title}
        sub={
          <>
            <span className="num roadmap-period">
              {dotDate(d.starts_on)} – {dotDate(d.ends_on)}
            </span>
            <span>{monthOf}</span>
            {d.lead && <span className="roadmap-lead">{d.lead}</span>}
          </>
        }
      >
        <button type="button" className="btn ghost small" onClick={() => void exportMarkdown()}>
          md 내보내기
        </button>
        <button type="button" className="btn ghost small" onClick={() => setEditingInfo(true)}>
          정보 고치기
        </button>
      </PageHead>

      <section className="roadmap-summary" aria-label="요약">
        <div>
          <p className="k">전체 진행률</p>
          <p className="v">
            <b className="roadmap-big tone">{stats.all.pct}%</b>
            <span className="num muted">
              {stats.all.done} / {stats.all.total}
            </span>
          </p>
          <span className="meter" aria-hidden="true">
            <i style={{ width: `${stats.all.pct}%` }} />
          </span>
        </div>
        <div>
          <p className="k">이번 달 · {monthTitle(cur)}</p>
          <p className="v">
            <b className="roadmap-big">{stats.thisMonth.done}</b>
            <span className="num muted">/ {stats.thisMonth.total}</span>
          </p>
          <p className="k">{stats.thisMonth.total ? `남은 일 ${stats.thisMonth.total - stats.thisMonth.done}개` : '이번 달 할 일이 없어요'}</p>
        </div>
        <div className={stats.overdue ? 'is-warn' : ''}>
          <p className="k">기한 지난 미완료</p>
          <p className="v">
            <b className="roadmap-big">{stats.overdue}</b>
            <span>개</span>
          </p>
          {stats.overdue > 0 ? (
            <button type="button" className="text-btn" onClick={() => setFilter('todo')}>
              미완료만 보기
            </button>
          ) : (
            <p className="k">밀린 일이 없어요</p>
          )}
        </div>
        <div>
          <p className="k">다음 마일스톤</p>
          <p className="next">{stats.next ? stats.next.body : '마일스톤을 모두 끝냈어요'}</p>
          {stats.next && <p className="k">{monthTitle(stats.next.month)}까지</p>}
        </div>
      </section>

      <div className="roadmap-body">
        <div className="roadmap-main">
          {problem && (
            <p className="error" role="alert">
              {problem}
            </p>
          )}
          <div className="filter-pills" role="group" aria-label="보기 거르기">
            <Pill on={filter === 'all'} onClick={() => setFilter('all')}>
              전체
            </Pill>
            <Pill on={filter === 'todo'} onClick={() => setFilter('todo')}>
              미완료만
            </Pill>
            {d.categories.map((c) => (
              <Pill key={c} on={typeof filter === 'object' && filter.category === c} onClick={() => setFilter({ category: c })}>
                {c}
              </Pill>
            ))}
          </div>

          {groups.map((g) => (
            <section key={g.key} className={`phase ${g.open ? 'is-open' : ''}`}>
              <h2>
                <button type="button" className="phase-toggle" aria-expanded={g.open} onClick={() => setCollapsed((c) => ({ ...c, [g.key]: g.open }))}>
                  <span className="phase-title">
                    <span className="phase-name">{g.phase.title}</span>
                    <span className="phase-meta">
                      <span className="num">
                        {g.phase.from.replace('-', '.')} – {g.phase.to.replace('-', '.')}
                      </span>
                      {g.phase.goal && ` · 목표 ${g.phase.goal}`}
                    </span>
                    <span className="meter" aria-hidden="true">
                      <i style={{ width: `${pct(g.done, g.items.length)}%` }} />
                    </span>
                  </span>
                  <span className="phase-pct">
                    <b>{pct(g.done, g.items.length)}%</b>
                    <span className="num muted">
                      {g.done} / {g.items.length}
                    </span>
                  </span>
                  <Chevron open={g.open} />
                </button>
              </h2>
              {g.open && (
                <div className="phase-body">
                  <MonthGroups items={g.items} visible={visible} {...itemProps} />
                  {g.items.length === 0 && <p className="quiet">이 구간에 할 일이 아직 없어요.</p>}
                  <AddItemForm months={g.months} categories={d.categories} cur={cur} onAdd={addItem} />
                </div>
              )}
            </section>
          ))}

          {(loose.length > 0 || d.phases.length === 0) && (
            <section className="phase is-open">
              <h2 className="phase-plain">{d.phases.length ? '구간 밖' : '할 일'}</h2>
              <div className="phase-body">
                <MonthGroups items={loose} visible={visible} {...itemProps} />
                {looseMonths.length > 0 && <AddItemForm months={looseMonths} categories={d.categories} cur={cur} onAdd={addItem} />}
              </div>
            </section>
          )}
        </div>

        <aside className="roadmap-side" aria-label="습관과 통계">
          <Habits habits={d.habits} today={today} week={week} onToggle={toggleWeek} onAdd={addHabit} onDelete={deleteHabit} />

          <section className="side-block">
            <h2 className="side-title">계획 대비</h2>
            <Bar label="기간 지남" value={stats.elapsed} muted />
            <Bar label="달성" value={stats.all.pct} />
            <p className={`pace ${stats.pace <= -3 ? 'is-behind' : stats.pace >= 3 ? 'is-ahead' : ''}`}>{paceText}</p>
          </section>

          <section className="side-block">
            <h2 className="side-title">분류별 달성률</h2>
            <ul className="bars">
              {stats.byCategory.map((c) => (
                <li key={c.category} data-tone={c.tone}>
                  <div className="bar-head">
                    <span className={`cat-chip ${c.tone ? '' : 'is-plain'}`}>{c.category}</span>
                    <span className="grow" />
                    <span className="num muted">
                      {c.done} / {c.total}
                    </span>
                    <b className="num">{c.pct}%</b>
                  </div>
                  <span className="meter is-tone" aria-hidden="true">
                    <i style={{ width: `${c.pct}%` }} />
                  </span>
                </li>
              ))}
            </ul>
          </section>

          {stats.byPhase.length > 0 && (
            <section className="side-block">
              <h2 className="side-title">구간별 달성률</h2>
              <ul className="bars">
                {stats.byPhase.map((p, i) => (
                  <li key={i}>
                    <div className="bar-head">
                      <span className="bar-name">{p.title}</span>
                      <span className="num muted">
                        {p.done} / {p.total}
                      </span>
                      <b className="num">{p.pct}%</b>
                    </div>
                    <span className="meter" aria-hidden="true">
                      <i style={{ width: `${p.pct}%` }} />
                    </span>
                  </li>
                ))}
              </ul>
            </section>
          )}
        </aside>
      </div>

      {editingItem && (
        <ItemEditor
          item={editingItem}
          roadmap={d}
          onClose={() => setEditingItem(null)}
          onSave={async (input) => {
            await saveItem(editingItem, input);
            setEditingItem(null);
          }}
        />
      )}
      {editingInfo && (
        <InfoEditor
          roadmap={d}
          onClose={() => setEditingInfo(false)}
          onSaved={(next) => {
            // 서버는 목록에서 뺀 분류를 그 할 일들에서도 비운다. 화면도 같이 맞춘다
            setD((x) => ({
              ...x,
              ...next,
              items: x.items.map((it) => (it.category && !next.categories.includes(it.category) ? { ...it, category: '' } : it)),
            }));
            setEditingInfo(false);
            roadmaps.reload();
          }}
          onDeleted={() => {
            roadmaps.reload();
            navigate('/me/roadmap?all=1', { replace: true });
          }}
        />
      )}
    </>
  );
};

const Pill = ({ on, onClick, children }: { on: boolean; onClick: () => void; children: string }) => (
  <button type="button" className="pill" aria-pressed={on} onClick={onClick}>
    {children}
  </button>
);

const Chevron = ({ open }: { open: boolean }) => (
  <svg className={`chevron ${open ? '' : 'is-closed'}`} aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M6 9l6 6 6-6" />
  </svg>
);

const Bar = ({ label, value, muted }: { label: string; value: number; muted?: boolean }) => (
  <div className="pace-bar">
    <div className="bar-head">
      <span className="muted">{label}</span>
      <span className="grow" />
      <b className={`num ${muted ? '' : 'tone'}`}>{value}%</b>
    </div>
    <span className={`meter thick ${muted ? 'is-muted' : ''}`} aria-hidden="true">
      <i style={{ width: `${value}%` }} />
    </span>
  </div>
);

// ── 달 묶음 · 항목 ───────────────────────────

interface ItemHandlers {
  categories: string[];
  cur: string;
  onToggle: (it: RoadmapItem) => void;
  onDelete: (it: RoadmapItem) => void;
  onEdit: (it: RoadmapItem) => void;
}

const MonthGroups = ({ items, visible, ...h }: ItemHandlers & { items: RoadmapItem[]; visible: (it: RoadmapItem) => boolean }) => {
  const months = [...new Set(items.map((it) => it.month))].sort();
  return (
    <>
      {months.map((m) => {
        const all = items.filter((it) => it.month === m).sort((a, b) => a.sort_order - b.sort_order);
        const done = all.filter(isDone).length;
        const shown = all.filter(visible);
        const badge =
          done === all.length ? ['완료', 'ok'] : m === h.cur ? ['이번 달', 'tone'] : m < h.cur ? ['기한 지남', 'warn'] : null;
        return (
          <div key={m} id={`m-${m}`} className="month">
            <h3>
              <span>{monthTitle(m)}까지</span>
              {badge && <span className={`badge ${badge[1] === 'tone' ? '' : badge[1]}`}>{badge[0]}</span>}
              <span className="num muted">
                {done} / {all.length}
              </span>
            </h3>
            {shown.length === 0 ? (
              <p className="quiet small">이 보기에 맞는 할 일이 없어요.</p>
            ) : (
              <ul>
                {shown.map((it) => (
                  <ItemRow key={it.id} item={it} {...h} />
                ))}
              </ul>
            )}
          </div>
        );
      })}
    </>
  );
};

const ItemRow = ({ item: it, categories, cur, onToggle, onDelete, onEdit }: ItemHandlers & { item: RoadmapItem }) => {
  const [confirming, setConfirming] = useState(false);
  useEffect(() => {
    if (!confirming) return;
    const t = setTimeout(() => setConfirming(false), 2500);
    return () => clearTimeout(t);
  }, [confirming]);
  const done = isDone(it);
  const tone = categoryTone(categories, it.category);
  return (
    <li className={`item ${done ? 'is-done' : ''} ${isOverdue(it, cur) ? 'is-over' : ''}`}>
      <label>
        <input type="checkbox" checked={done} onChange={() => onToggle(it)} />
        <span className="item-text">
          {it.category && (
            <span className={`cat-chip ${tone ? '' : 'is-plain'}`} data-tone={tone}>
              {it.category}
            </span>
          )}
          {it.is_key && (
            <span className="star" role="img" aria-label="마일스톤">
              ★
            </span>
          )}
          <span className="item-body">{it.body}</span>
          {done && <span className="item-when num">{shortDay(it.done_on)} 완료</span>}
        </span>
      </label>
      <span className="item-tools">
        <button type="button" className="icon-btn" aria-label="할 일 고치기" onClick={() => onEdit(it)}>
          <svg aria-hidden="true" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M4 20h4L19 9l-4-4L4 16z" />
          </svg>
        </button>
        <button
          type="button"
          className={`icon-btn danger ${confirming ? 'is-confirm' : ''}`}
          aria-label={confirming ? '한 번 더 누르면 지워요' : '할 일 지우기'}
          onClick={() => (confirming ? onDelete(it) : setConfirming(true))}
        >
          {confirming ? '지울까요?' : <Icon name="close" size={15} />}
        </button>
      </span>
    </li>
  );
};

interface AddItemFormProps {
  months: string[];
  categories: string[];
  cur: string;
  onAdd: (input: RoadmapItemInput) => Promise<void>;
}

const AddItemForm = ({ months, categories, cur, onAdd }: AddItemFormProps) => {
  const id = useId();
  const [month, setMonth] = useState(months.includes(cur) ? cur : months[0]!);
  const [category, setCategory] = useState(categories[0] ?? '');
  const [body, setBody] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!body.trim()) return;
    setBusy(true);
    setError('');
    try {
      await onAdd({ month, category, body });
      setBody('');
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form className="add-item" onSubmit={submit} aria-label="할 일 추가">
      <label className="sr-only" htmlFor={`${id}-m`}>
        달
      </label>
      <select id={`${id}-m`} value={month} onChange={(e) => setMonth(e.target.value)}>
        {months.map((m) => (
          <option key={m} value={m}>
            {monthTitle(m)}
          </option>
        ))}
      </select>
      {categories.length > 0 && (
        <>
          <label className="sr-only" htmlFor={`${id}-c`}>
            분류
          </label>
          <select id={`${id}-c`} value={category} onChange={(e) => setCategory(e.target.value)}>
            {categories.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
            <option value="">분류 없음</option>
          </select>
        </>
      )}
      <label className="sr-only" htmlFor={`${id}-b`}>
        할 일
      </label>
      <input id={`${id}-b`} value={body} onChange={(e) => setBody(e.target.value)} placeholder="할 일 추가" maxLength={300} autoComplete="off" />
      <button type="submit" className="btn ghost small" disabled={busy || !body.trim()}>
        추가
      </button>
      {error && <p className="error small">{error}</p>}
    </form>
  );
};

// ── 매주 습관 ─────────────────────────────

interface HabitsProps {
  habits: RoadmapHabit[];
  today: string;
  week: string;
  onToggle: (h: RoadmapHabit) => void;
  onAdd: (name: string) => Promise<void>;
  onDelete: (h: RoadmapHabit) => void;
}

const MAX_HABITS = 10;

const Habits = ({ habits, today, week, onToggle, onAdd, onDelete }: HabitsProps) => {
  const weeks = recentWeeks(today, 12);
  const [name, setName] = useState('');
  const [error, setError] = useState('');
  const [confirm, setConfirm] = useState('');

  const add = async (e: FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    setError('');
    try {
      await onAdd(name);
      setName('');
    } catch (err) {
      setError(errorText(err));
    }
  };

  return (
    <section className="side-block habits">
      <div className="side-title-row">
        <h2 className="side-title">매주 습관</h2>
        <span className="num muted">{week}</span>
      </div>
      <p className="quiet small">최근 12주. 칸 하나가 한 주예요.</p>
      {habits.length === 0 && <p className="quiet small">매주 할 일을 더해 두면 주마다 체크할 수 있어요.</p>}
      <ul>
        {habits.map((h) => {
          const on = h.weeks.includes(week);
          const recent = weeks.filter((w) => h.weeks.includes(w)).length;
          return (
            <li key={h.id}>
              <div className="habit-row">
                <span className="habit-name">{h.name}</span>
                <button type="button" className={`habit-btn ${on ? 'is-on' : ''}`} aria-pressed={on} onClick={() => onToggle(h)}>
                  {on ? '이번 주 완료' : '이번 주 체크'}
                </button>
              </div>
              <div className="habit-row">
                <span className="weeks" role="img" aria-label={`최근 12주 중 ${recent}주 체크`}>
                  {weeks.map((w) => (
                    <span key={w} className={h.weeks.includes(w) ? 'on' : ''} title={w} />
                  ))}
                </span>
                <span className="muted small">
                  누적 <b className="num">{h.weeks.length}</b>주
                </span>
                <span className="grow" />
                <button
                  type="button"
                  className="text-btn muted-btn"
                  onClick={() => (confirm === h.id ? onDelete(h) : setConfirm(h.id))}
                  onBlur={() => setConfirm('')}
                >
                  {confirm === h.id ? '한 번 더 누르면 지워요' : '지우기'}
                </button>
              </div>
            </li>
          );
        })}
      </ul>
      {habits.length < MAX_HABITS && (
        <form className="add-item" onSubmit={add} aria-label="습관 추가">
          <label className="sr-only" htmlFor="habit-new">
            새 습관
          </label>
          <input id="habit-new" value={name} onChange={(e) => setName(e.target.value)} placeholder="매주 할 일 추가" maxLength={100} autoComplete="off" />
          <button type="submit" className="btn ghost small" disabled={!name.trim()}>
            추가
          </button>
        </form>
      )}
      {error && <p className="error small">{error}</p>}
    </section>
  );
};

// ── 고치기 대화상자 ───────────────────────────

const ItemEditor = ({
  item,
  roadmap,
  onClose,
  onSave,
}: {
  item: RoadmapItem;
  roadmap: RoadmapDetail;
  onClose: () => void;
  onSave: (input: RoadmapItemInput) => Promise<void>;
}) => {
  const id = useId();
  const [month, setMonth] = useState(item.month);
  const [category, setCategory] = useState(item.category);
  const [body, setBody] = useState(item.body);
  const [isKey, setIsKey] = useState(item.is_key);
  const months = monthsBetween(roadmap.starts_on.slice(0, 7), roadmap.ends_on.slice(0, 7));
  if (!months.includes(item.month)) months.push(item.month);
  const categories = roadmap.categories.includes(item.category) || !item.category ? roadmap.categories : [...roadmap.categories, item.category];

  return (
    <Modal title="할 일 고치기" onClose={onClose} onSubmit={() => onSave({ month, category, body, is_key: isKey })}>
      <Field label="할 일" value={body} onChange={setBody} required maxLength={300} />
      <div className="grid-2">
        <div className="field">
          <label htmlFor={`${id}-m`}>달</label>
          <select id={`${id}-m`} value={month} onChange={(e) => setMonth(e.target.value)}>
            {months.sort().map((m) => (
              <option key={m} value={m}>
                {monthTitle(m)}까지
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label htmlFor={`${id}-c`}>분류</label>
          <select id={`${id}-c`} value={category} onChange={(e) => setCategory(e.target.value)}>
            {categories.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
            <option value="">분류 없음</option>
          </select>
        </div>
      </div>
      <Toggle label="마일스톤 (★)" checked={isKey} onChange={setIsKey} hint="시험 · 지원처럼 꼭 지켜야 하는 일. 요약의 '다음 마일스톤'에 나와요" />
    </Modal>
  );
};

const InfoEditor = ({
  roadmap,
  onClose,
  onSaved,
  onDeleted,
}: {
  roadmap: RoadmapDetail;
  onClose: () => void;
  onSaved: (r: RoadmapDetail) => void;
  onDeleted: () => void;
}) => {
  const [draft, setDraft] = useState<RoadmapInput>({
    title: roadmap.title,
    lead: roadmap.lead,
    starts_on: roadmap.starts_on,
    ends_on: roadmap.ends_on,
    categories: roadmap.categories,
    phases: roadmap.phases,
  });
  const set = <K extends keyof RoadmapInput>(key: K, value: RoadmapInput[K]) => setDraft((x) => ({ ...x, [key]: value }));
  const [removeError, setRemoveError] = useState('');

  const remove = async () => {
    if (!confirm(`‘${roadmap.title}’ 로드맵을 지울까요? 할 일 ${roadmap.items.length}개와 습관 기록도 함께 지워져요.`)) return;
    setRemoveError('');
    try {
      await api.deleteRoadmap(roadmap.id);
      onDeleted();
    } catch (e) {
      setRemoveError(`지우지 못했어요: ${errorText(e)}`);
    }
  };

  return (
    <Modal
      title="로드맵 정보 고치기"
      onClose={onClose}
      onSubmit={async () => onSaved({ ...roadmap, ...(await api.updateRoadmap(roadmap.id, draft)) })}
      extra={
        <button type="button" className="btn danger small" onClick={() => void remove()}>
          로드맵 지우기
        </button>
      }
    >
      {removeError && (
        <p className="error" role="alert">
          {removeError}
        </p>
      )}
      <Field label="제목" value={draft.title} onChange={(v) => set('title', v)} required maxLength={80} />
      <Field label="한 줄 설명" value={draft.lead} onChange={(v) => set('lead', v)} maxLength={200} />
      <div className="grid-2">
        <Field label="시작일" type="date" value={draft.starts_on} onChange={(v) => set('starts_on', v)} required />
        <Field label="종료일" type="date" value={draft.ends_on} onChange={(v) => set('ends_on', v)} required />
      </div>
      <WordsField
        label="분류"
        value={draft.categories}
        onChange={(v) => set('categories', v.slice(0, 6))}
        placeholder="자격증, 기본기, 코딩테스트"
        hint="6개까지. 순서대로 이름표 색이 정해져요. 분류를 빼면 그 분류의 할 일은 '분류 없음'이 돼요"
      />
      <RowList<RoadmapPhase>
        title="구간"
        rows={draft.phases}
        onChange={(rows) => set('phases', rows)}
        empty={() => ({ title: '', from: draft.starts_on.slice(0, 7), to: draft.ends_on.slice(0, 7), goal: '' })}
        addLabel="구간 추가"
        render={(p, update) => (
          <>
            <Field label="구간 이름" value={p.title} onChange={(v) => update({ title: v })} required maxLength={60} />
            <div className="grid-2">
              <Field label="시작 달" type="month" value={p.from} onChange={(v) => update({ from: v })} required />
              <Field label="끝 달" type="month" value={p.to} onChange={(v) => update({ to: v })} required />
            </div>
            <Field label="목표" value={p.goal} onChange={(v) => update({ goal: v })} maxLength={120} />
          </>
        )}
      />
    </Modal>
  );
};
