// 월드맵 하단 대화창의 안내 문구. 지금 보는 메뉴와 실제 상태(기한 지난 일 · 시험 · 새 메시지)에서만 만든다.

export interface PlaceFacts {
  /** 지금 있는 메뉴 이름 */
  here: string;
  /** 다음 시험까지 남은 날 (없으면 undefined) */
  nextExam?: number | undefined;
  /** 진행 중인 로드맵의 기한 지난 할 일 수 */
  overdue: number;
  unread: number;
  /** 채용공고 · 메시지를 쓸 수 없는 테스터 계정 */
  isTester: boolean;
}

export interface PlaceHint {
  text: string;
  /** 다음 목적지 (경로, 이름) */
  next?: { to: string; label: string };
}

/** 가장 급한 것부터: 기한 지난 할 일 → 새 메시지 → 다가오는 시험 */
export const placeHint = (f: PlaceFacts): PlaceHint => {
  const facts: string[] = [];
  let next: PlaceHint['next'];
  if (f.overdue > 0) {
    facts.push(`기한 지난 할 일이 ${f.overdue}개 있어요.`);
    next = { to: '/me/roadmap', label: '로드맵' };
  }
  if (!f.isTester && f.unread > 0) {
    facts.push(`새 메시지가 ${f.unread}통 와 있어요.`);
    next ??= { to: '/me/inbox', label: '메시지' };
  }
  if (f.nextExam !== undefined) {
    facts.push(f.nextExam === 0 ? '오늘이 시험일이에요.' : `다음 시험까지 ${f.nextExam}일 남았어요.`);
    next ??= { to: '/me/certs', label: '자격증' };
  }
  if (facts.length === 0) return { text: `${f.here}에 있어요. 가고 싶은 곳을 골라 보세요.` };
  return { text: `${f.here}에 있어요. ${facts.join(' ')}`, next };
};
