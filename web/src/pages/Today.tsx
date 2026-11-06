import { useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { PageHead, useWorkspace, type Tone } from '../components/Workspace';
import { api, errorText } from '../lib/api';
import { dateTimeLabel, dayLabel, daysUntil } from '../lib/format';
import { categoryTone, isDone, isoWeek, monthTitle, pct, roadmapStatus } from '../lib/roadmap';
import type { Profile, RoadmapDetail, RoadmapItem } from '../lib/types';
import { useAsync } from '../lib/useAsync';

/** 유효기간이 이만큼 남으면 오늘 화면에 올린다 */
const EXPIRY_NOTICE_DAYS = 120;

/** /me — 수첩의 오늘 쪽: 다가오는 시험 · 이번 달 로드맵 · 새 메시지 · 저장한 검색 · 이력서 상태 */
export const Today = () => {
  const { page, messages } = useWorkspace();
  const searches = useAsync(() => api.jobSearches(), []);
  const p = page.profile;
  const today = new Date().toLocaleDateString('sv-SE', { timeZone: 'Asia/Seoul' });
  const unread = (messages ?? []).filter((m) => !m.is_read);

  return (
    <>
      <PageHead title="오늘" sub={dayLabel(today)} />
      <div className="today">
        <Block tone="mustard" title="다가오는 일정" to="/me/certs" more="자격증 보기" className="today-schedule">
          <Schedule profile={p} />
        </Block>

        <Block tone="rose" title="이번 달 로드맵" to="/me/roadmap" more="로드맵 보기">
          <RoadmapThisMonth today={today} />
        </Block>

        <Block tone="teal" title="새 메시지" to="/me/inbox" more="메시지 보기">
          {unread.length === 0 ? (
            <p className="quiet">새 메시지가 없어요.</p>
          ) : (
            <ul className="peek">
              {unread.slice(0, 3).map((m) => (
                <li key={m.id}>
                  <Link to="/me/inbox">
                    <strong>{m.sender_name}</strong>
                    <span className="peek-body">{m.body}</span>
                    <small className="muted">{dateTimeLabel(m.created_at)}</small>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </Block>

        <Block tone="sage" title="저장한 공고 검색" to="/me/jobs" more="채용공고 보기">
          {searches.error ? <p className="error small">{errorText(searches.error)}</p> : null}
          {searches.data && searches.data.length === 0 && (
            <p className="quiet">
              키워드와 사이트로 검색 조건을 만들어 두면 누를 때마다 새 공고를 모아 줘요.{' '}
              <Link to="/me/jobs">조건 만들기</Link>
            </p>
          )}
          <ul className="peek">
            {(searches.data ?? []).slice(0, 4).map((s) => (
              <li key={s.id}>
                <Link to={`/me/jobs?s=${s.id}`}>
                  <strong>{s.name}</strong>
                  <span className="peek-body">{s.keywords.join(', ')}</span>
                </Link>
              </li>
            ))}
          </ul>
        </Block>

        <Block tone="peri" title="이력서" to="/me/resume" more="이력서 보기">
          <ResumeStatus profile={p} projects={page.projects.length} />
        </Block>
      </div>
    </>
  );
};

interface BlockProps {
  tone: Tone;
  title: string;
  to: string;
  more: string;
  className?: string;
  children: ReactNode;
}

/** 오늘 쪽의 한 덩어리. 제목 앞의 색 표시는 어느 탭의 내용인지 알려 준다 */
const Block = ({ tone, title, to, more, className = '', children }: BlockProps) => (
  <section className={`block ${className}`} data-tone={tone}>
    <header className="block-head">
      <h2>{title}</h2>
      <Link to={to} className="block-more">
        {more}
      </Link>
    </header>
    {children}
  </section>
);

interface ScheduleItem {
  key: string;
  date: string;
  days: number;
  title: string;
  what: string;
}

/** 준비 중인 자격의 시험일 + 곧 끝나는 유효기간을 날짜순으로 */
const Schedule = ({ profile }: { profile: Profile }) => {
  const items: ScheduleItem[] = [];
  profile.resume.certificates.forEach((c, i) => {
    const date = c.status === 'preparing' ? c.exam_on : c.expires_on;
    const days = daysUntil(date);
    if (days === null || days < 0) return;
    if (c.status !== 'preparing' && days > EXPIRY_NOTICE_DAYS) return;
    items.push({ key: `${i}`, date, days, title: c.name, what: c.status === 'preparing' ? '시험' : '유효기간 끝' });
  });
  items.sort((a, b) => a.days - b.days);

  if (items.length === 0) {
    return (
      <div className="schedule-empty">
        <p className="quiet">준비 중인 자격증의 시험일을 적어 두면, 여기서 남은 날을 세어 줘요.</p>
        <Link className="btn ghost small" to="/me/certs?new=1">
          준비 중인 자격증 추가
        </Link>
      </div>
    );
  }
  return (
    <ol className="ddays">
      {items.map((it) => (
        <li key={it.key} className={it.days <= 7 ? 'is-soon' : ''}>
          <span className="dday">{it.days === 0 ? 'D-day' : `D-${it.days}`}</span>
          <span className="dday-what">
            <strong>{it.title}</strong>
            <small>
              {dayLabel(it.date)} {it.what}
            </small>
          </span>
        </li>
      ))}
    </ol>
  );
};

/** 이력서 칸마다 채웠는지 */
const ResumeStatus = ({ profile: p, projects }: { profile: Profile; projects: number }) => {
  const r = p.resume;
  const parts: [string, boolean][] = [
    ['소개', !!p.summary],
    ['경력', r.experiences.length > 0],
    ['프로젝트', projects > 0],
    ['학력', r.educations.length > 0],
    ['기술', r.skills.length > 0],
    ['자격', r.certificates.some((c) => c.status !== 'preparing')],
  ];
  const filled = parts.filter(([, ok]) => ok).length;
  return (
    <div className="resume-status">
      <p>
        <span className={`badge ${p.is_public ? 'ok' : 'muted'}`}>{p.is_public ? '링크로 공개 중' : '비공개'}</span>
        <span className="muted">
          {parts.length}칸 중 <b className="big-num">{filled}</b>칸 채움
        </span>
      </p>
      <ul className="fill-list" aria-label="채운 칸">
        {parts.map(([label, ok]) => (
          <li key={label} className={ok ? 'is-filled' : ''}>
            {label}
            <span className="sr-only">{ok ? ' 채움' : ' 비어 있음'}</span>
          </li>
        ))}
      </ul>
      {filled < parts.length && (
        <Link className="text-btn" to="/me/resume?edit=1">
          빈 칸 채우기
        </Link>
      )}
    </div>
  );
};

/** 이번 달에 한 번에 보여 줄 미완료 할 일 수 (로드맵마다) */
const TODAY_ITEMS = 5;

/** 진행 중인 로드맵마다 이번 달 할 일 · 진행률 · 기한 지남 · 이번 주 습관. 여기서 바로 체크할 수 있다 */
const RoadmapThisMonth = ({ today }: { today: string }) => {
  const { roadmaps } = useWorkspace();
  const active = (roadmaps.data ?? []).filter((r) => roadmapStatus(r, today) === 'active');
  const ids = active.map((r) => r.id).join(',');
  const details = useAsync(() => Promise.all(active.map((r) => api.roadmap(r.id))), [ids]);

  if (roadmaps.error) return <p className="error small">{errorText(roadmaps.error)}</p>;
  if (roadmaps.data && active.length === 0) {
    return (
      <p className="quiet">
        Claude와 만든 체크리스트 md를 올려 두면 이번 달 할 일을 여기서 보여 줘요.{' '}
        <Link to="/me/roadmap?new=1">로드맵 만들기</Link>
      </p>
    );
  }
  if (details.error) return <p className="error small">{errorText(details.error)}</p>;
  if (!details.data) return null;

  return (
    <div className="today-roadmaps">
      {details.data.map((d) => (
        <RoadmapMonthCard
          key={d.id}
          roadmap={d}
          today={today}
          onChange={(next) => {
            details.setData(details.data!.map((x) => (x.id === next.id ? next : x)));
            roadmaps.reload();
          }}
        />
      ))}
    </div>
  );
};

const RoadmapMonthCard = ({ roadmap: d, today, onChange }: { roadmap: RoadmapDetail; today: string; onChange: (d: RoadmapDetail) => void }) => {
  const [error, setError] = useState('');
  const cur = today.slice(0, 7);
  const week = isoWeek(today);
  const month = d.items.filter((it) => it.month === cur);
  const done = month.filter(isDone).length;
  const todo = month.filter((it) => !isDone(it)).sort((a, b) => a.sort_order - b.sort_order);
  const overdue = d.items.filter((it) => !isDone(it) && it.month < cur).length;
  const habitsDone = d.habits.filter((h) => h.weeks.includes(week)).length;

  const check = async (it: RoadmapItem) => {
    setError('');
    try {
      const saved = await api.patchRoadmapItem(d.id, it.id, { done: true });
      onChange({ ...d, items: d.items.map((x) => (x.id === saved.id ? saved : x)) });
    } catch (e) {
      setError(errorText(e));
    }
  };

  return (
    <div className="today-roadmap">
      <div className="today-roadmap-head">
        <Link to={`/me/roadmap/${d.id}`}>{d.title}</Link>
        <span className="muted">
          {Number(cur.slice(5))}월 할 일 <b className="big-num">{done}</b>
          <span className="num"> / {month.length}</span>
        </span>
      </div>
      <span className="meter" aria-hidden="true">
        <i style={{ width: `${pct(done, month.length)}%` }} />
      </span>
      {month.length === 0 ? (
        <p className="quiet small">{monthTitle(cur)}에 잡힌 할 일이 없어요.</p>
      ) : todo.length === 0 ? (
        <p className="quiet small">이번 달 할 일을 모두 끝냈어요.</p>
      ) : (
        <ul className="today-todo">
          {todo.slice(0, TODAY_ITEMS).map((it) => {
            const tone = categoryTone(d.categories, it.category);
            return (
              <li key={it.id}>
                <label>
                  <input type="checkbox" checked={false} onChange={() => void check(it)} />
                  <span>
                    {it.category && (
                      <span className={`cat-chip ${tone ? '' : 'is-plain'}`} data-tone={tone}>
                        {it.category}
                      </span>
                    )}
                    {it.is_key && <span className="star">★</span>}
                    {it.body}
                  </span>
                </label>
              </li>
            );
          })}
          {todo.length > TODAY_ITEMS && (
            <li className="muted small">
              <Link to={`/me/roadmap/${d.id}`}>그 밖에 {todo.length - TODAY_ITEMS}개 더</Link>
            </li>
          )}
        </ul>
      )}
      {error && <p className="error small">{error}</p>}
      <p className="today-roadmap-foot muted">
        {overdue > 0 ? <span className="badge warn">기한 지난 일 {overdue}개</span> : '기한 지난 일 없음'}
        {d.habits.length > 0 && (
          <span>
            이번 주 습관 <b className="num">{habitsDone} / {d.habits.length}</b>
          </span>
        )}
      </p>
    </div>
  );
};
