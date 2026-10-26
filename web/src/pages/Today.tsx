import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { PageHead, useWorkspace, type Tone } from '../components/Workspace';
import { api, errorText } from '../lib/api';
import { dateTimeLabel, dayLabel, daysUntil } from '../lib/format';
import type { Profile } from '../lib/types';
import { useAsync } from '../lib/useAsync';

/** 유효기간이 이만큼 남으면 오늘 화면에 올린다 */
const EXPIRY_NOTICE_DAYS = 120;

/** /me — 수첩의 오늘 쪽: 다가오는 시험 · 새 메시지 · 저장한 검색 · 이력서 상태 */
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
