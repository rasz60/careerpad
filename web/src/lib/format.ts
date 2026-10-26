/** "2024-03" → "2024.03" */
export const monthLabel = (ym: string): string => ym.replace('-', '.');

/** 기간 표시: 2021.03 – 현재 */
export const periodLabel = (start: string, end: string, ongoing = '현재'): string => {
  if (!start && !end) return '';
  return `${start ? monthLabel(start) : '?'} – ${end ? monthLabel(end) : ongoing}`;
};

/** 경력 기간 합계(개월)를 "3년 2개월" 로. 시작이 없으면 셀 수 없다 */
export const durationLabel = (start: string, end: string, now = new Date()): string => {
  if (!start) return '';
  const [sy, sm] = start.split('-').map(Number) as [number, number];
  const [ey, em] = end ? (end.split('-').map(Number) as [number, number]) : [now.getFullYear(), now.getMonth() + 1];
  const months = (ey - sy) * 12 + (em - sm) + 1;
  if (months <= 0) return '';
  const y = Math.floor(months / 12);
  const m = months % 12;
  return [y && `${y}년`, m && `${m}개월`].filter(Boolean).join(' ');
};

/** 01012345678 → 010-1234-5678 */
export const phoneLabel = (digits: string): string =>
  digits.replace(/^(\d{2,3})(\d{3,4})(\d{4})$/, '$1-$2-$3');

/** 마감일까지 남은 날 (D-3, 오늘 마감, 상시) */
export const deadlineLabel = (date: string, today = new Date()): string => {
  if (!date) return '상시';
  const end = new Date(`${date}T23:59:59+09:00`);
  const days = Math.ceil((end.getTime() - today.getTime()) / 86_400_000) - 1;
  if (days < 0) return '마감';
  if (days === 0) return '오늘 마감';
  return `D-${days}`;
};

/** 오늘(서울)부터 그날까지 남은 날. 지났으면 음수, 날짜가 없으면 null */
export const daysUntil = (date: string, today = new Date()): number | null => {
  if (!date) return null;
  const seoul = today.toLocaleDateString('sv-SE', { timeZone: 'Asia/Seoul' });
  return Math.round((Date.parse(date) - Date.parse(seoul)) / 86_400_000);
};

/** "2026-11-15" → "11월 15일 (일)" */
export const dayLabel = (date: string): string =>
  new Date(`${date}T00:00:00+09:00`).toLocaleDateString('ko-KR', { month: 'long', day: 'numeric', weekday: 'short', timeZone: 'Asia/Seoul' });

/** 쉼표·줄바꿈으로 나눈 낱말 목록 (태그 입력) */
export const splitWords = (text: string): string[] =>
  text
    .split(/[,\n]/)
    .map((w) => w.trim())
    .filter(Boolean);

export const dateTimeLabel = (iso: string): string =>
  new Date(iso).toLocaleString('ko-KR', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Asia/Seoul' });
