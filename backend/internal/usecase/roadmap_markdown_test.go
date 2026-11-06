package usecase

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

const testToday = "2026-11-06"

func readSample(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/roadmap_sample.md")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseRoadmapSample(t *testing.T) {
	d := ParseRoadmap(readSample(t), testToday)
	if len(d.Errors) > 0 || len(d.Warnings) > 0 {
		t.Fatalf("errors = %v, warnings = %v", d.Errors, d.Warnings)
	}
	r := d.Roadmap
	if r.Title != "커리어 로드맵 체크리스트" || r.StartsOn != "2026-11-01" || r.EndsOn != "2028-06-30" {
		t.Errorf("head = %q %s ~ %s", r.Title, r.StartsOn, r.EndsOn)
	}
	if !strings.HasPrefix(r.Lead, "1순위") {
		t.Errorf("lead = %q", r.Lead)
	}
	if want := []string{"자격증", "기본기", "코딩테스트", "경력 심화", "바이브 코딩"}; !reflect.DeepEqual(r.Categories, want) {
		t.Errorf("categories = %v", r.Categories)
	}
	if len(r.Phases) != 4 || r.Phases[0].From != "2026-11" || r.Phases[0].To != "2027-04" || r.Phases[0].Goal != "실기 합격 (2027.04.18)" {
		t.Errorf("phases = %+v", r.Phases)
	}
	if len(r.Items) != 57 {
		t.Errorf("items = %d, want 57", len(r.Items))
	}
	months, keys := map[string]bool{}, 0
	for _, it := range r.Items {
		months[it.Month] = true
		if it.IsKey {
			keys++
		}
	}
	if len(months) != 16 || keys != 4 {
		t.Errorf("months = %d (want 16), keys = %d (want 4)", len(months), keys)
	}
	first := r.Items[0]
	if first.Month != "2026-11" || first.Category != "기본기" || first.Body != "OCI에 gcc · python3 · MySQL 실습 환경 구성" || first.DoneOn != "" {
		t.Errorf("first item = %+v", first)
	}
	// 내용 안의 괄호 · 대괄호 · 화살표는 그대로 남는다
	if got := r.Items[3].Body; got != "C: 배열과 포인터 a[i] == *(a+i)" {
		t.Errorf("item 4 = %q", got)
	}
	if len(r.Habits) != 4 || r.Habits[0].Name != "장애 · 작업 노트 1건 작성" {
		t.Errorf("habits = %+v", r.Habits)
	}
}

func TestParseRoadmapItemMarks(t *testing.T) {
	md := `# 짧은 로드맵
### 2026년 11월
- [x] [자격증] ★ 접수 (완료 2026-11-03)
- [X] ★ [기본기] 정리
- [ ] 분류 없는 할 일
* [ ] 별표 목록도 된다 (완료 2026-11-05)
- 체크 상자 없는 줄도 할 일
### 2026-12 까지
- [ ] [새 분류] 다음 달`
	d := ParseRoadmap(md, testToday)
	if len(d.Errors) > 0 {
		t.Fatalf("errors = %v", d.Errors)
	}
	items := d.Roadmap.Items
	if len(items) != 6 {
		t.Fatalf("items = %d, want 6", len(items))
	}
	if it := items[0]; it.Category != "자격증" || !it.IsKey || it.DoneOn != "2026-11-03" || it.Body != "접수" {
		t.Errorf("item 1 = %+v", it)
	}
	if it := items[1]; it.Category != "기본기" || !it.IsKey || it.DoneOn != testToday {
		t.Errorf("item 2 (날짜 없는 [x] 는 오늘 완료) = %+v", it)
	}
	if it := items[2]; it.Category != "" || it.DoneOn != "" || it.Month != "2026-11" {
		t.Errorf("item 3 = %+v", it)
	}
	if it := items[3]; it.DoneOn != "2026-11-05" || it.Body != "별표 목록도 된다" {
		t.Errorf("item 4 = %+v", it)
	}
	if it := items[5]; it.Month != "2026-12" || it.Category != "새 분류" {
		t.Errorf("item 6 = %+v", it)
	}
	if want := []string{"자격증", "기본기", "새 분류"}; !reflect.DeepEqual(d.Roadmap.Categories, want) {
		t.Errorf("categories = %v (항목의 분류를 자동으로 더한다)", d.Roadmap.Categories)
	}
	// 기간이 없으면 항목의 달로 정한다
	if d.Roadmap.StartsOn != "2026-11-01" || d.Roadmap.EndsOn != "2026-12-31" {
		t.Errorf("period = %s ~ %s", d.Roadmap.StartsOn, d.Roadmap.EndsOn)
	}
}

func TestParseRoadmapErrorsAndWarnings(t *testing.T) {
	md := `---
title: 오류 확인
period: 2026-11-01 ~ 2027-01-31
---
- [ ] 달 제목 없이 나온 할 일
## 구간 이름만 있음
메모: 읽지 못하는 줄
### 2026-13
### 2026-11
- [x] 날짜 오타 (완료 2026-11-31)
- [ ] [a,b] 쉼표 분류`
	d := ParseRoadmap(md, testToday)
	wantErrLines := []int{5, 8, 10, 11} // 달 없는 할 일 · 없는 달 · 없는 날짜 · 쉼표 분류
	if len(d.Errors) != len(wantErrLines) {
		t.Fatalf("errors = %v", d.Errors)
	}
	for i, line := range wantErrLines {
		if d.Errors[i].Line != line {
			t.Errorf("error %d line = %d, want %d (%s)", i, d.Errors[i].Line, line, d.Errors[i].Text)
		}
	}
	if len(d.Warnings) != 2 || d.Warnings[0].Line != 6 || d.Warnings[1].Line != 7 {
		t.Errorf("warnings = %v, want lines 6 (기간 없는 구간), 7 (읽지 못한 줄)", d.Warnings)
	}
}

func TestParseRoadmapWholeFileChecks(t *testing.T) {
	cases := map[string]string{
		"제목 없음":    "### 2026-11\n- [ ] a",
		"할 일 없음":   "# 제목만",
		"분류 7개":    "---\ntitle: x\ncategories: a, b, c, d, e, f, g\n---\n### 2026-11\n- [ ] a",
		"구간 겹침":    "# x\n## A (2026-11 ~ 2027-02)\n## B (2027-02 ~ 2027-05)\n### 2026-11\n- [ ] a",
		"기간 거꾸로":   "---\ntitle: x\nperiod: 2027-01 ~ 2026-11\n---\n### 2026-11\n- [ ] a",
		"머리말 안 닫힘": "---\ntitle: x\n### 2026-11\n- [ ] a",
	}
	for name, md := range cases {
		if d := ParseRoadmap(md, testToday); len(d.Errors) == 0 {
			t.Errorf("%s: want an error", name)
		}
	}
	big := "# x\n### 2026-11\n" + strings.Repeat("- [ ] a\n", roadmapMaxItems+1)
	if d := ParseRoadmap(big, testToday); len(d.Errors) == 0 {
		t.Error("할 일 한도 초과: want an error")
	}
	if d := ParseRoadmap(strings.Repeat("a", roadmapMaxBytes+1), testToday); len(d.Errors) != 1 || d.Errors[0].Line != 0 {
		t.Errorf("파일 크기 초과: errors = %v", d.Errors)
	}
}

func TestParseRoadmapOutsidePeriodWarns(t *testing.T) {
	md := "---\ntitle: x\nperiod: 2026-11 ~ 2026-12\n---\n### 2027-03\n- [ ] 기간 밖"
	d := ParseRoadmap(md, testToday)
	if len(d.Errors) > 0 || len(d.Warnings) != 1 {
		t.Fatalf("errors = %v, warnings = %v", d.Errors, d.Warnings)
	}
}

// 내보내고 다시 읽으면 같은 로드맵이 나와야 한다 (체크 · 완료일 · 구간 밖 항목 포함).
func TestRoadmapMarkdownRoundTrip(t *testing.T) {
	d := ParseRoadmap(readSample(t), testToday)
	r := d.Roadmap
	r.Items[0].DoneOn = "2026-11-03"
	r.Items[5].DoneOn = "2026-11-05"
	// 구간 2 를 지워 2027-05 · 06 항목을 어느 구간에도 안 드는 '구간 밖' 항목으로 만든다
	r.Phases = append(r.Phases[:1:1], r.Phases[2:]...)

	md := RoadmapMarkdown(&r)
	again := ParseRoadmap(md, "2030-01-01")
	if len(again.Errors) > 0 || len(again.Warnings) > 0 {
		t.Fatalf("re-parse: errors = %v, warnings = %v\n%s", again.Errors, again.Warnings, md)
	}
	got := again.Roadmap
	if got.Title != r.Title || got.Lead != r.Lead || got.StartsOn != r.StartsOn || got.EndsOn != r.EndsOn {
		t.Errorf("head = %+v", got.Roadmap)
	}
	if !reflect.DeepEqual(got.Categories, r.Categories) || !reflect.DeepEqual(got.Phases, r.Phases) {
		t.Errorf("categories / phases differ:\n%v %v\n%v %v", got.Categories, got.Phases, r.Categories, r.Phases)
	}
	if len(got.Items) != len(r.Items) {
		t.Fatalf("items = %d, want %d", len(got.Items), len(r.Items))
	}
	byKey := map[string]string{}
	for _, it := range r.Items {
		byKey[it.Month+"|"+it.Body] = it.Category + "|" + it.DoneOn + "|" + boolText(it.IsKey)
	}
	for _, it := range got.Items {
		if want, have := byKey[it.Month+"|"+it.Body], it.Category+"|"+it.DoneOn+"|"+boolText(it.IsKey); want != have {
			t.Errorf("%s %q: got %s, want %s", it.Month, it.Body, have, want)
		}
	}
	if len(got.Habits) != len(r.Habits) || got.Habits[3].Name != r.Habits[3].Name {
		t.Errorf("habits = %+v", got.Habits)
	}
}

func boolText(b bool) string {
	if b {
		return "key"
	}
	return "-"
}

// Claude 답을 코드 블록째 붙여 넣어도 읽는다
func TestParseRoadmapIgnoresCodeFence(t *testing.T) {
	md := "```markdown\n---\ntitle: 펜스\n---\n### 2026-11\n- [ ] 할 일\n```\n"
	d := ParseRoadmap(md, testToday)
	if len(d.Errors) > 0 || len(d.Warnings) > 0 || d.Roadmap.Title != "펜스" || len(d.Roadmap.Items) != 1 {
		t.Fatalf("errors = %v, warnings = %v, roadmap = %+v", d.Errors, d.Warnings, d.Roadmap.Roadmap)
	}
}

// 목록 밖 분류가 남은 할 일이 있어도 내보낸 md 는 다시 읽힌다 (분류 순서도 그대로)
func TestRoadmapMarkdownKeepsOrphanCategory(t *testing.T) {
	d := ParseRoadmap("---\ntitle: x\ncategories: a, b\n---\n### 2026-11\n- [ ] [a] 하나\n- [ ] [c] 둘", testToday).Roadmap
	d.Categories = []string{"b", "a"}
	again := ParseRoadmap(RoadmapMarkdown(&d), testToday)
	if len(again.Errors) > 0 {
		t.Fatalf("errors = %v", again.Errors)
	}
	if !reflect.DeepEqual(again.Roadmap.Categories, []string{"b", "a", "c"}) {
		t.Errorf("categories = %v, want [b a c]", again.Roadmap.Categories)
	}
}
