package usecase

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

// 로드맵 md 형식 (업로드 · 내보내기 공통). Claude 가 쓰기 쉽게 마크다운 체크리스트를 그대로 쓴다.
//
//	---
//	title: 커리어 로드맵
//	period: 2026-11-01 ~ 2028-06-30      (YYYY-MM ~ YYYY-MM 도 된다. 비우면 항목의 달로 정한다)
//	lead: 한 줄 설명
//	categories: 자격증, 기본기
//	---
//	## 구간 1 · 정보처리기사 (2026-11 ~ 2027-04)
//	> 목표: 실기 합격
//	### 2026-11
//	- [ ] [자격증] 할 일
//	- [x] [기본기] ★ 끝낸 마일스톤 (완료 2026-11-03)
//	## 매주 습관
//	- 장애 노트 1건
//
// 구간은 달 범위로 항목을 모은다. 어느 구간에도 안 들어가는 달의 항목은 '구간 밖'으로 보인다.

const (
	roadmapMaxBytes      = 200 << 10
	roadmapMaxPhases     = 12
	roadmapMaxCategories = 6
	roadmapMaxItems      = 500
	roadmapMaxHabits     = 10
	roadmapTitleMax      = 80
	roadmapLeadMax       = 200
	roadmapPhaseTitleMax = 60
	roadmapGoalMax       = 120
	roadmapItemMax       = 300
	roadmapHabitMax      = 100
	roadmapCategoryMax   = 20
)

// habitSections 는 습관 목록으로 읽는 ## 제목이다.
var habitSections = []string{"매주 습관", "주간 습관", "습관"}

// ParseMessage 는 md 를 읽다 나온 경고 · 오류다. Line 은 1부터 (0 = 파일 전체).
type ParseMessage struct {
	Line int    `json:"line"`
	Text string `json:"text"`
}

// RoadmapDraft 는 md 를 읽은 결과다. Errors 가 있으면 저장하지 않는다. 경고가 난 줄은 건너뛴다.
type RoadmapDraft struct {
	Roadmap  domain.RoadmapDetail `json:"roadmap"`
	Warnings []ParseMessage       `json:"warnings"`
	Errors   []ParseMessage       `json:"errors"`
}

var (
	reMonthHead  = regexp.MustCompile(`^(\d{4})\s*(?:-|\.|/|년)\s*(\d{1,2})\s*월?\s*(?:까지)?$`)
	rePhaseHead  = regexp.MustCompile(`^(.*?)\s*[(（]\s*(\d{4})[-./](\d{1,2})\s*(?:~|–|—|-)\s*(\d{4})[-./](\d{1,2})\s*[)）]$`)
	reListItem   = regexp.MustCompile(`^[-*+]\s+(?:\[([ xX])\]\s*)?(.*)$`)
	reCategory   = regexp.MustCompile(`^\[([^\]]+)\]\s*`)
	reStar       = regexp.MustCompile(`^(?:★|⭐)\s*`)
	reDoneSuffix = regexp.MustCompile(`\s*[(（]\s*(?:완료|done)\s*:?\s*(\d{4}-\d{2}-\d{2})\s*[)）]$`)
	rePeriod     = regexp.MustCompile(`^(\d{4}-\d{2}(?:-\d{2})?)\s*(?:~|–|—|\s-\s)\s*(\d{4}-\d{2}(?:-\d{2})?)$`)
	reGoal       = regexp.MustCompile(`^>\s*(?:목표\s*[:：]\s*)?`)
)

type mdParser struct {
	draft     RoadmapDraft
	today     string
	periodRaw string
	periodAt  int
	habits    bool // 지금 ## 매주 습관 아래인가
	phase     int  // 지금 구간 (목표 줄을 붙일 곳), -1 = 없음
	month     string
}

func (p *mdParser) warn(line int, format string, args ...any) {
	p.draft.Warnings = append(p.draft.Warnings, ParseMessage{Line: line, Text: fmt.Sprintf(format, args...)})
}

func (p *mdParser) fail(line int, format string, args ...any) {
	p.draft.Errors = append(p.draft.Errors, ParseMessage{Line: line, Text: fmt.Sprintf(format, args...)})
}

// ParseRoadmap 은 md 를 로드맵으로 읽는다. today(YYYY-MM-DD)는 날짜 없이 [x] 인 항목의 완료일이다.
func ParseRoadmap(md, today string) RoadmapDraft {
	p := &mdParser{today: today, phase: -1}
	p.draft.Roadmap.Categories = []string{}
	p.draft.Roadmap.Phases = []domain.RoadmapPhase{}
	p.draft.Roadmap.Items = []domain.RoadmapItem{}
	p.draft.Roadmap.Habits = []domain.RoadmapHabit{}
	p.draft.Warnings = []ParseMessage{}
	p.draft.Errors = []ParseMessage{}

	if len(md) > roadmapMaxBytes {
		p.fail(0, "파일이 너무 커요 (%dKB까지)", roadmapMaxBytes>>10)
		return p.draft
	}
	if !utf8.ValidString(md) {
		p.fail(0, "UTF-8 글자로 저장한 파일만 읽을 수 있어요")
		return p.draft
	}
	lines := strings.Split(strings.ReplaceAll(strings.TrimPrefix(md, string(rune(0xFEFF))), "\r\n", "\n"), "\n")
	// Claude 답을 코드 블록째 붙여 넣어도 되게 ``` 줄은 빈 줄로 본다 (형식에 코드 블록은 없다)
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			lines[i] = ""
		}
	}

	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i < len(lines) && strings.TrimSpace(lines[i]) == "---" {
		i = p.frontMatter(lines, i)
	}
	for ; i < len(lines); i++ {
		p.line(i+1, strings.TrimSpace(lines[i]))
	}
	p.finish()
	return p.draft
}

// frontMatter 는 --- 사이의 key: value 를 읽고 다음에 읽을 줄 번호(0부터)를 준다.
func (p *mdParser) frontMatter(lines []string, start int) int {
	r := &p.draft.Roadmap
	for i := start + 1; i < len(lines); i++ {
		s := strings.TrimSpace(lines[i])
		if s == "---" {
			return i + 1
		}
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		key, value, ok := strings.Cut(s, ":")
		if !ok {
			p.warn(i+1, "머리말에서 읽지 못한 줄이라 건너뛰어요: “%s”", clip(s))
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "title":
			r.Title = value
		case "lead":
			r.Lead = value
		case "period":
			p.periodRaw, p.periodAt = value, i+1
		case "categories":
			for c := range strings.FieldsFuncSeq(value, func(r rune) bool { return r == ',' || r == '、' }) {
				p.addCategory(i+1, strings.TrimSpace(c))
			}
		default:
			p.warn(i+1, "모르는 머리말이라 건너뛰어요: %s", strings.TrimSpace(key))
		}
	}
	p.fail(start+1, "머리말(---)이 닫히지 않았어요. 머리말 끝에 --- 줄을 넣어 주세요")
	return len(lines)
}

func (p *mdParser) line(n int, s string) {
	r := &p.draft.Roadmap
	switch {
	case s == "":
	case strings.HasPrefix(s, "### "):
		p.habits = false
		m := reMonthHead.FindStringSubmatch(strings.TrimSpace(s[4:]))
		if m == nil {
			p.month = ""
			p.fail(n, "달 제목은 ### 2026-11 이나 ### 2026년 11월 처럼 써 주세요")
			return
		}
		mm := fmt.Sprintf("%s-%02d", m[1], atoi(m[2]))
		if !monthPattern.MatchString(mm) {
			p.month = ""
			p.fail(n, "%s 은 없는 달이에요", mm)
			return
		}
		p.month = mm
	case strings.HasPrefix(s, "## "):
		title := strings.TrimSpace(s[3:])
		p.month, p.phase = "", -1
		if slices.Contains(habitSections, title) {
			p.habits = true
			return
		}
		p.habits = false
		m := rePhaseHead.FindStringSubmatch(title)
		if m == nil {
			p.warn(n, "구간 제목에 기간이 없어 구간으로 쓰지 않아요 (예: ## 구간 1 (2026-11 ~ 2027-04))")
			return
		}
		ph := domain.RoadmapPhase{
			Title: strings.TrimSpace(m[1]),
			From:  fmt.Sprintf("%s-%02d", m[2], atoi(m[3])),
			To:    fmt.Sprintf("%s-%02d", m[4], atoi(m[5])),
		}
		switch {
		case ph.Title == "":
			p.fail(n, "구간 이름이 비었어요")
		case utf8.RuneCountInString(ph.Title) > roadmapPhaseTitleMax:
			p.fail(n, "구간 이름은 %d자까지예요", roadmapPhaseTitleMax)
		case !monthPattern.MatchString(ph.From) || !monthPattern.MatchString(ph.To):
			p.fail(n, "구간 기간의 달이 올바르지 않아요")
		case ph.To < ph.From:
			p.fail(n, "구간이 끝나는 달이 시작하는 달보다 빨라요")
		default:
			r.Phases = append(r.Phases, ph)
			p.phase = len(r.Phases) - 1
		}
	case strings.HasPrefix(s, "# "):
		if r.Title == "" {
			r.Title = strings.TrimSpace(s[2:])
			return
		}
		p.warn(n, "제목은 이미 있어서 이 줄은 건너뛰어요")
	case strings.HasPrefix(s, ">"):
		goal := strings.TrimSpace(reGoal.ReplaceAllString(s, ""))
		if p.phase >= 0 && p.month == "" && r.Phases[p.phase].Goal == "" && goal != "" {
			if utf8.RuneCountInString(goal) > roadmapGoalMax {
				p.fail(n, "구간 목표는 %d자까지예요", roadmapGoalMax)
				return
			}
			r.Phases[p.phase].Goal = goal
			return
		}
		p.warn(n, "구간 제목 바로 아래가 아닌 인용 줄이라 건너뛰어요: “%s”", clip(s))
	default:
		m := reListItem.FindStringSubmatch(s)
		if m == nil {
			p.warn(n, "읽지 못한 줄이라 건너뛰어요: “%s”", clip(s))
			return
		}
		if p.habits {
			p.habit(n, m[2])
			return
		}
		p.item(n, m[1], m[2])
	}
}

func (p *mdParser) habit(n int, text string) {
	name := strings.TrimSpace(text)
	switch {
	case name == "":
		p.fail(n, "습관 이름이 비었어요")
	case utf8.RuneCountInString(name) > roadmapHabitMax:
		p.fail(n, "습관 이름은 %d자까지예요", roadmapHabitMax)
	default:
		h := &p.draft.Roadmap.Habits
		*h = append(*h, domain.RoadmapHabit{Name: name, Weeks: []string{}, SortOrder: len(*h)})
	}
}

func (p *mdParser) item(n int, mark, text string) {
	if p.month == "" {
		p.fail(n, "할 일 위에 ### 2026-11 같은 달 제목이 있어야 해요")
		return
	}
	it := domain.RoadmapItem{Month: p.month}
	done := mark == "x" || mark == "X"
	if m := reDoneSuffix.FindStringSubmatch(text); m != nil {
		if _, err := time.Parse(time.DateOnly, m[1]); err != nil {
			p.fail(n, "완료일 %s 은 없는 날짜예요", m[1])
			return
		}
		it.DoneOn, done = m[1], true
		text = text[:len(text)-len(m[0])]
	}
	// [분류] 와 ★ 는 어느 쪽이 먼저 와도 된다
	for range 2 {
		text = strings.TrimSpace(text)
		if m := reCategory.FindStringSubmatch(text); m != nil && it.Category == "" {
			it.Category = strings.TrimSpace(m[1])
			text = text[len(m[0]):]
			continue
		}
		if m := reStar.FindString(text); m != "" {
			it.IsKey = true
			text = text[len(m):]
		}
	}
	it.Body = strings.TrimSpace(text)
	if done && it.DoneOn == "" {
		it.DoneOn = p.today
	}
	switch {
	case it.Body == "":
		p.fail(n, "할 일 내용이 비었어요")
		return
	case utf8.RuneCountInString(it.Body) > roadmapItemMax:
		p.fail(n, "할 일은 %d자까지예요", roadmapItemMax)
		return
	}
	if it.Category != "" && !p.addCategory(n, it.Category) {
		return
	}
	items := &p.draft.Roadmap.Items
	it.SortOrder = len(*items)
	*items = append(*items, it)
}

// addCategory 는 분류를 목록에 더한다 (이미 있으면 그대로). 이름이 틀리면 오류를 남기고 false.
func (p *mdParser) addCategory(n int, c string) bool {
	if c == "" {
		return true
	}
	if err := categoryName(c); err != "" {
		p.fail(n, "%s", err)
		return false
	}
	cats := &p.draft.Roadmap.Categories
	if !slices.Contains(*cats, c) {
		*cats = append(*cats, c)
	}
	return true
}

// categoryName 은 분류 이름 규칙이다. md 의 [분류] · 머리말 쉼표 목록과 부딪치는 글자를 막는다.
func categoryName(c string) string {
	switch {
	case utf8.RuneCountInString(c) > roadmapCategoryMax:
		return fmt.Sprintf("분류 이름은 %d자까지예요: %s", roadmapCategoryMax, clip(c))
	case strings.ContainsAny(c, ",[]\n"):
		return fmt.Sprintf("분류 이름에는 쉼표 · 대괄호를 쓸 수 없어요: %s", clip(c))
	}
	return ""
}

func (p *mdParser) finish() {
	r := &p.draft.Roadmap
	switch {
	case r.Title == "":
		p.fail(0, "제목이 없어요. 머리말에 title: 을 넣거나 맨 위에 # 제목 줄을 써 주세요")
	case utf8.RuneCountInString(r.Title) > roadmapTitleMax:
		p.fail(0, "제목은 %d자까지예요", roadmapTitleMax)
	}
	if utf8.RuneCountInString(r.Lead) > roadmapLeadMax {
		p.fail(0, "한 줄 설명(lead)은 %d자까지예요", roadmapLeadMax)
	}
	if len(r.Categories) > roadmapMaxCategories {
		p.fail(0, "분류는 %d개까지예요 (지금 %d개: %s)", roadmapMaxCategories, len(r.Categories), strings.Join(r.Categories, ", "))
	}
	if len(r.Phases) > roadmapMaxPhases {
		p.fail(0, "구간은 %d개까지예요", roadmapMaxPhases)
	}
	if a, b, ok := overlappingPhases(r.Phases); ok {
		p.fail(0, "구간 기간이 겹쳐요: %s, %s", r.Phases[a].Title, r.Phases[b].Title)
	}
	if len(r.Items) > roadmapMaxItems {
		p.fail(0, "할 일은 %d개까지예요", roadmapMaxItems)
	}
	if len(r.Habits) > roadmapMaxHabits {
		p.fail(0, "매주 습관은 %d개까지예요", roadmapMaxHabits)
	}

	if p.periodRaw != "" {
		m := rePeriod.FindStringSubmatch(p.periodRaw)
		var ok bool
		if m != nil {
			r.StartsOn, ok = dayOf(m[1], false)
			if ok {
				r.EndsOn, ok = dayOf(m[2], true)
			}
		}
		if !ok {
			p.fail(p.periodAt, "기간은 2026-11-01 ~ 2028-06-30 이나 2026-11 ~ 2028-06 처럼 써 주세요")
			return
		}
	} else if len(r.Items) > 0 {
		first, last := r.Items[0].Month, r.Items[0].Month
		for _, it := range r.Items {
			first, last = min(first, it.Month), max(last, it.Month)
		}
		r.StartsOn, _ = dayOf(first, false)
		r.EndsOn, _ = dayOf(last, true)
	} else {
		p.fail(0, "기간(period)도 할 일도 없어요. 둘 중 하나는 있어야 로드맵을 만들 수 있어요")
		return
	}
	if r.EndsOn < r.StartsOn {
		p.fail(p.periodAt, "기간이 끝나는 날이 시작하는 날보다 빨라요")
		return
	}
	startMonth, endMonth := r.StartsOn[:7], r.EndsOn[:7]
	outside := 0
	for _, it := range r.Items {
		if it.Month < startMonth || it.Month > endMonth {
			outside++
		}
	}
	if outside > 0 {
		p.warn(0, "기간 밖의 달에 있는 할 일이 %d개 있어요. 그대로 넣지만 기간을 다시 확인해 주세요", outside)
	}
}

// overlappingPhases 는 기간이 겹치는 구간 두 개를 찾는다.
func overlappingPhases(phases []domain.RoadmapPhase) (int, int, bool) {
	for a := range phases {
		for b := a + 1; b < len(phases); b++ {
			if phases[a].From <= phases[b].To && phases[b].From <= phases[a].To {
				return a, b, true
			}
		}
	}
	return 0, 0, false
}

// dayOf 는 "YYYY-MM-DD" 를 그대로, "YYYY-MM" 은 그 달 첫날(end 면 말일)로 바꾼다.
func dayOf(s string, end bool) (string, bool) {
	if len(s) == len("2006-01-02") {
		_, err := time.Parse(time.DateOnly, s)
		return s, err == nil
	}
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return "", false
	}
	if end {
		t = t.AddDate(0, 1, -1)
	}
	return t.Format(time.DateOnly), true
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

// clip 은 경고 문장에 넣을 줄을 줄인다.
func clip(s string) string {
	const limit = 40
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit]) + "…"
}

// RoadmapMarkdown 은 로드맵을 같은 md 형식으로 쓴다. 다시 읽으면(ParseRoadmap) 같은 로드맵이 나온다.
// 습관의 주별 체크 기록은 형식에 없으므로 빠진다.
func RoadmapMarkdown(d *domain.RoadmapDetail) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %s\n", d.Title)
	fmt.Fprintf(&b, "period: %s ~ %s\n", d.StartsOn, d.EndsOn)
	if d.Lead != "" {
		fmt.Fprintf(&b, "lead: %s\n", d.Lead)
	}
	// 분류는 로드맵 목록 순서 그대로 (분류를 빼면 저장소가 그 할 일의 분류도 비우므로 목록 밖 분류는 보통 없다.
	// 있더라도 다시 읽을 때와 같은 순서가 되게 뒤에 붙인다)
	cats := slices.Clone(d.Categories)
	for _, it := range d.Items {
		if it.Category != "" && !slices.Contains(cats, it.Category) {
			cats = append(cats, it.Category)
		}
	}
	if len(cats) > 0 {
		fmt.Fprintf(&b, "categories: %s\n", strings.Join(cats, ", "))
	}
	b.WriteString("---\n")

	items := slices.Clone(d.Items)
	slices.SortStableFunc(items, func(x, y domain.RoadmapItem) int {
		if c := strings.Compare(x.Month, y.Month); c != 0 {
			return c
		}
		return x.SortOrder - y.SortOrder
	})
	inPhase := func(m string) bool {
		return slices.ContainsFunc(d.Phases, func(p domain.RoadmapPhase) bool { return m >= p.From && m <= p.To })
	}
	writeMonths := func(keep func(string) bool) {
		month := ""
		for _, it := range items {
			if !keep(it.Month) {
				continue
			}
			if it.Month != month {
				month = it.Month
				fmt.Fprintf(&b, "\n### %s\n", month)
			}
			b.WriteString(itemLine(it))
		}
	}

	// 어느 구간에도 안 드는 달은 구간 제목보다 앞에 둔다 (구간은 달 범위로만 항목을 모으므로 위치는 상관없다)
	writeMonths(func(m string) bool { return !inPhase(m) })
	for _, p := range d.Phases {
		fmt.Fprintf(&b, "\n## %s (%s ~ %s)\n", p.Title, p.From, p.To)
		if p.Goal != "" {
			fmt.Fprintf(&b, "> 목표: %s\n", p.Goal)
		}
		writeMonths(func(m string) bool { return m >= p.From && m <= p.To })
	}
	if len(d.Habits) > 0 {
		b.WriteString("\n## 매주 습관\n")
		for _, h := range d.Habits {
			fmt.Fprintf(&b, "- %s\n", h.Name)
		}
	}
	return b.String()
}

func itemLine(it domain.RoadmapItem) string {
	var b strings.Builder
	if it.DoneOn != "" {
		b.WriteString("- [x] ")
	} else {
		b.WriteString("- [ ] ")
	}
	if it.Category != "" {
		fmt.Fprintf(&b, "[%s] ", it.Category)
	}
	if it.IsKey {
		b.WriteString("★ ")
	}
	b.WriteString(it.Body)
	if it.DoneOn != "" {
		fmt.Fprintf(&b, " (완료 %s)", it.DoneOn)
	}
	b.WriteString("\n")
	return b.String()
}
