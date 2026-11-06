import type { Roadmap, RoadmapDetail, RoadmapItem, RoadmapPhase } from './types';
import type { Tone } from '../components/Workspace';

// 로드맵 화면의 계산 (오늘 · 이번 달 · 이번 주 · 통계). 날짜는 모두 한국 시간 기준이다.

/** 오늘 (서울) YYYY-MM-DD */
export const seoulToday = (now = new Date()): string => now.toLocaleDateString('sv-SE', { timeZone: 'Asia/Seoul' });

/** "2026-11" → "2026년 11월" */
export const monthTitle = (m: string): string => {
  const [y, mo] = m.split('-');
  return `${y}년 ${Number(mo)}월`;
};

/** "2026-11-03" → "11.03" */
export const shortDay = (d: string): string => d.slice(5).replace('-', '.');

/** "2026-11-01" → "2026.11.01" */
export const dotDate = (d: string): string => d.replaceAll('-', '.');

/** a ~ b 사이의 달 (둘 다 포함) */
export const monthsBetween = (a: string, b: string): string[] => {
  const out: string[] = [];
  let [y, m] = a.split('-').map(Number) as [number, number];
  const [ey, em] = b.split('-').map(Number) as [number, number];
  while ((y < ey || (y === ey && m <= em)) && out.length < 240) {
    out.push(`${y}-${String(m).padStart(2, '0')}`);
    m += 1;
    if (m > 12) {
      m = 1;
      y += 1;
    }
  }
  return out;
};

/** 날짜(YYYY-MM-DD)의 ISO 주 키 "2026-W45" (서버 검증과 같은 규칙) */
export const isoWeek = (date: string): string => {
  const [y, m, d] = date.split('-').map(Number) as [number, number, number];
  const t = new Date(Date.UTC(y, m - 1, d));
  const day = t.getUTCDay() || 7;
  t.setUTCDate(t.getUTCDate() + 4 - day);
  const y0 = Date.UTC(t.getUTCFullYear(), 0, 1);
  const week = Math.ceil(((t.getTime() - y0) / 86_400_000 + 1) / 7);
  return `${t.getUTCFullYear()}-W${String(week).padStart(2, '0')}`;
};

/** 오늘을 끝으로 하는 최근 n주의 키 (오래된 주가 앞) */
export const recentWeeks = (today: string, n = 12): string[] => {
  const base = Date.parse(`${today}T00:00:00Z`);
  return Array.from({ length: n }, (_, i) => isoWeek(new Date(base - (n - 1 - i) * 7 * 86_400_000).toISOString().slice(0, 10)));
};

export type RoadmapStatus = 'active' | 'upcoming' | 'ended';

export const roadmapStatus = (r: Pick<Roadmap, 'starts_on' | 'ends_on'>, today: string): RoadmapStatus =>
  today < r.starts_on ? 'upcoming' : today > r.ends_on ? 'ended' : 'active';

export const STATUS_LABEL: Record<RoadmapStatus, string> = { active: '진행 중', upcoming: '예정', ended: '끝남' };

/** 분류 이름표 색: 분류 순서대로 탭 색 하나씩 (분류는 6개까지) */
const CATEGORY_TONES: Tone[] = ['slate', 'sage', 'mustard', 'peri', 'teal', 'rose'];

export const categoryTone = (categories: string[], category: string): Tone | undefined => {
  const i = categories.indexOf(category);
  return i >= 0 ? CATEGORY_TONES[i] : undefined;
};

export const isDone = (it: RoadmapItem): boolean => it.done_on !== '';

/** 이번 달보다 앞선 달인데 아직인 항목 */
export const isOverdue = (it: RoadmapItem, curMonth: string): boolean => !isDone(it) && it.month < curMonth;

export const pct = (done: number, total: number): number => (total ? Math.round((done / total) * 100) : 0);

/** 그 달이 든 구간 번호 (없으면 -1) */
export const phaseIndex = (phases: RoadmapPhase[], month: string): number =>
  phases.findIndex((p) => month >= p.from && month <= p.to);

export interface Tally {
  done: number;
  total: number;
  pct: number;
}

const tally = (items: RoadmapItem[]): Tally => {
  const done = items.filter(isDone).length;
  return { done, total: items.length, pct: pct(done, items.length) };
};

export interface RoadmapStats {
  all: Tally;
  thisMonth: Tally;
  overdue: number;
  /** 가장 이른 미완료 마일스톤 */
  next: RoadmapItem | undefined;
  byCategory: (Tally & { category: string; tone: Tone | undefined })[];
  byPhase: (Tally & { title: string })[];
  /** 기간 중 오늘까지 지난 비율 (0~100) */
  elapsed: number;
  /** 달성률 - 기간 경과율 (%p). 양수면 계획보다 앞섬 */
  pace: number;
  /** 몇 개월 중 몇 개월째 */
  monthIndex: number;
  monthCount: number;
}

export const roadmapStats = (d: RoadmapDetail, today: string): RoadmapStats => {
  const cur = today.slice(0, 7);
  const all = tally(d.items);
  const start = Date.parse(d.starts_on);
  const end = Date.parse(d.ends_on);
  const days = (end - start) / 86_400_000 + 1;
  const passed = (Date.parse(today) - start) / 86_400_000 + 1;
  const elapsed = Math.max(0, Math.min(100, Math.round((passed / days) * 100)));
  const months = monthsBetween(d.starts_on.slice(0, 7), d.ends_on.slice(0, 7));
  const uncategorized = d.items.some((it) => it.category === '' || !d.categories.includes(it.category));

  return {
    all,
    thisMonth: tally(d.items.filter((it) => it.month === cur)),
    overdue: d.items.filter((it) => isOverdue(it, cur)).length,
    next: d.items.filter((it) => it.is_key && !isDone(it)).sort((a, b) => a.month.localeCompare(b.month) || a.sort_order - b.sort_order)[0],
    byCategory: [
      ...d.categories.map((c) => ({ ...tally(d.items.filter((it) => it.category === c)), category: c, tone: categoryTone(d.categories, c) })),
      ...(uncategorized
        ? [{ ...tally(d.items.filter((it) => !d.categories.includes(it.category))), category: '분류 없음', tone: undefined }]
        : []),
    ],
    byPhase: d.phases.map((p) => ({ ...tally(d.items.filter((it) => it.month >= p.from && it.month <= p.to)), title: p.title })),
    elapsed,
    pace: all.pct - elapsed,
    monthIndex: Math.max(0, Math.min(months.length, months.indexOf(cur) + 1 || (cur < months[0]! ? 0 : months.length))),
    monthCount: months.length,
  };
};
