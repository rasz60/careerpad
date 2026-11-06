// "Claude로 만들기" 도우미가 복사해 주는 부탁 글. 서버는 Claude 를 부르지 않는다:
// 사용자가 이 글을 Claude 에 붙여 넣고, 받은 md 를 'md 파일로' 칸에 올린다.
// 형식 설명은 백엔드 usecase/roadmap_markdown.go 의 규칙과 같아야 한다.

export const ROADMAP_FORMAT = `형식 (이 형식만 읽을 수 있어요)
---
title: 로드맵 이름
period: YYYY-MM-DD ~ YYYY-MM-DD
lead: 한 줄 설명
categories: 분류1, 분류2   (6개까지)
---

## 구간 이름 (YYYY-MM ~ YYYY-MM)
> 목표: 이 구간에서 이룰 것

### YYYY-MM
- [ ] [분류] 그 달까지 할 일
- [ ] [분류] ★ 꼭 지켜야 하는 일 (시험 · 지원 같은 마일스톤)
- [x] [분류] 이미 끝낸 일 (완료 YYYY-MM-DD)

## 매주 습관
- 매주 할 일

규칙
- 구간끼리 기간이 겹치지 않게
- ### 달 제목 아래에만 할 일을 쓰기
- 할 일은 한 줄에 하나, 300자 안으로
- 분류 이름에는 쉼표 · 대괄호를 쓰지 않기`;

export interface NewRoadmapAnswers {
  goal: string;
  startsOn: string;
  endsOn: string;
  situation: string;
  categories: string;
}

const orBlank = (v: string, hint: string) => v.trim() || `(${hint})`;

export const newRoadmapPrompt = (a: NewRoadmapAnswers): string => `아래 목표로 커리어 로드맵을 만들어 줘. 결과는 설명 없이 md 한 파일로, 아래 형식을 정확히 지켜서. 코드 블록(\`\`\`)으로 감싸지 말고 md 내용만.

목표: ${orBlank(a.goal, '이루고 싶은 것')}
기간: ${orBlank(a.startsOn, '시작일')} ~ ${orBlank(a.endsOn, '종료일')}
지금 상황: ${orBlank(a.situation, '경력 · 공부할 수 있는 시간 등')}
분류: ${orBlank(a.categories, '알아서 3~5개')}

만들 때
- 구간은 2~6개, 구간마다 목표 한 줄
- 달마다 할 일 2~8개, 바로 체크할 수 있게 구체적으로
- 시험 · 지원처럼 날짜를 꼭 지켜야 하는 일은 ★
- 매주 습관 2~5개

${ROADMAP_FORMAT}`;

export const convertPrompt = (): string => `첨부한 체크리스트를 아래 md 형식으로 바꿔 줘. 설명 없이 md 한 파일만, 코드 블록(\`\`\`)으로 감싸지 말고.

- 구간 · 달 · 분류 · ★ 표시를 그대로 옮기기
- 체크된 항목은 - [x] 로, 완료일이 있으면 (완료 YYYY-MM-DD)
- 매주 습관도 옮기기 (주별 체크 기록은 빼도 됨)
- 내용은 고치지 말고 그대로

${ROADMAP_FORMAT}`;
