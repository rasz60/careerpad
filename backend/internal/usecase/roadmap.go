package usecase

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

// koreaTime 은 로드맵의 '오늘 · 이번 달 · 이번 주'를 정하는 시간대다 (서버가 UTC 여도 한국 날짜로).
var koreaTime = time.FixedZone("KST", 9*60*60)

var weekPattern = regexp.MustCompile(`^(\d{4})-W(\d{2})$`)

// Roadmap 은 개인 로드맵(md 가져오기 · 체크 · 습관) 유스케이스다.
type Roadmap struct {
	repo domain.RoadmapRepository
	now  func() time.Time
}

func NewRoadmap(repo domain.RoadmapRepository) *Roadmap {
	return &Roadmap{repo: repo, now: time.Now}
}

func (uc *Roadmap) today() string { return uc.now().In(koreaTime).Format(time.DateOnly) }

// Preview 는 md 를 읽기만 한다 (저장하지 않음).
func (uc *Roadmap) Preview(md string) RoadmapDraft {
	return ParseRoadmap(md, uc.today())
}

// Import 는 md 를 읽어 저장한다. 오류가 하나라도 있으면 첫 오류를 400 으로 돌려준다.
func (uc *Roadmap) Import(ctx context.Context, userID, md string) (*domain.RoadmapDetail, error) {
	draft := uc.Preview(md)
	if len(draft.Errors) > 0 {
		e := draft.Errors[0]
		if e.Line > 0 {
			return nil, invalid("%d번째 줄: %s", e.Line, e.Text)
		}
		return nil, invalid("%s", e.Text)
	}
	d := draft.Roadmap
	if err := uc.repo.Create(ctx, userID, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (uc *Roadmap) List(ctx context.Context, userID string) ([]domain.RoadmapSummary, error) {
	return uc.repo.List(ctx, userID, uc.today()[:7])
}

func (uc *Roadmap) Get(ctx context.Context, userID, id string) (*domain.RoadmapDetail, error) {
	return uc.repo.Find(ctx, userID, id)
}

// Markdown 은 내보낼 파일 이름(제목)과 md 본문이다.
func (uc *Roadmap) Markdown(ctx context.Context, userID, id string) (string, string, error) {
	d, err := uc.repo.Find(ctx, userID, id)
	if err != nil {
		return "", "", err
	}
	return d.Title, RoadmapMarkdown(d), nil
}

// RoadmapInput 은 로드맵 정보 고치기(PUT) 본문이다.
type RoadmapInput struct {
	Title      string                `json:"title"`
	Lead       string                `json:"lead"`
	StartsOn   string                `json:"starts_on"`
	EndsOn     string                `json:"ends_on"`
	Categories []string              `json:"categories"`
	Phases     []domain.RoadmapPhase `json:"phases"`
}

func (uc *Roadmap) Update(ctx context.Context, userID, id string, in RoadmapInput) (*domain.Roadmap, error) {
	r, err := normalizeRoadmap(in)
	if err != nil {
		return nil, err
	}
	r.ID = id
	if err := uc.repo.Update(ctx, userID, r); err != nil {
		return nil, err
	}
	return r, nil
}

func normalizeRoadmap(in RoadmapInput) (*domain.Roadmap, error) {
	title, err := required("제목", oneLine(in.Title), roadmapTitleMax)
	if err != nil {
		return nil, err
	}
	lead, err := text("한 줄 설명", oneLine(in.Lead), roadmapLeadMax)
	if err != nil {
		return nil, err
	}
	starts, err := requiredDate("시작일", in.StartsOn)
	if err != nil {
		return nil, err
	}
	ends, err := requiredDate("종료일", in.EndsOn)
	if err != nil {
		return nil, err
	}
	if ends < starts {
		return nil, invalid("종료일이 시작일보다 빠릅니다")
	}

	cats := []string{}
	for _, c := range in.Categories {
		c = strings.TrimSpace(c)
		if c == "" || slices.Contains(cats, c) {
			continue
		}
		if msg := categoryName(c); msg != "" {
			return nil, invalid("%s", msg)
		}
		cats = append(cats, c)
	}
	if len(cats) > roadmapMaxCategories {
		return nil, invalid("분류는 %d개까지 넣을 수 있습니다", roadmapMaxCategories)
	}

	if len(in.Phases) > roadmapMaxPhases {
		return nil, invalid("구간은 %d개까지 넣을 수 있습니다", roadmapMaxPhases)
	}
	phases := make([]domain.RoadmapPhase, 0, len(in.Phases))
	for i, p := range in.Phases {
		field := fmt.Sprintf("%d번째 구간", i+1)
		ph := domain.RoadmapPhase{}
		if ph.Title, err = required(field+" 이름", oneLine(p.Title), roadmapPhaseTitleMax); err != nil {
			return nil, err
		}
		if ph.Goal, err = text(field+" 목표", oneLine(p.Goal), roadmapGoalMax); err != nil {
			return nil, err
		}
		if ph.From, ph.To, err = period(field, p.From, p.To); err != nil {
			return nil, err
		}
		if ph.From == "" || ph.To == "" {
			return nil, invalid("%s 의 시작 · 끝 달을 입력해 주세요", field)
		}
		phases = append(phases, ph)
	}
	if a, b, ok := overlappingPhases(phases); ok {
		return nil, invalid("구간 기간이 겹칩니다: %s, %s", phases[a].Title, phases[b].Title)
	}
	return &domain.Roadmap{Title: title, Lead: lead, StartsOn: starts, EndsOn: ends, Categories: cats, Phases: phases}, nil
}

func (uc *Roadmap) Delete(ctx context.Context, userID, id string) error {
	return uc.repo.Delete(ctx, userID, id)
}

// ItemInput 은 항목 추가(POST) · 고치기(PATCH) 본문이다. PATCH 는 보낸 칸만 바꾼다.
type ItemInput struct {
	Month    *string `json:"month"`
	Category *string `json:"category"`
	Body     *string `json:"body"`
	IsKey    *bool   `json:"is_key"`
	Done     *bool   `json:"done"`
}

func (uc *Roadmap) AddItem(ctx context.Context, userID, roadmapID string, in ItemInput) (*domain.RoadmapItem, error) {
	it := &domain.RoadmapItem{}
	if in.Month == nil || in.Body == nil {
		return nil, invalid("달과 할 일을 입력해 주세요")
	}
	if err := uc.apply(it, in); err != nil {
		return nil, err
	}
	if err := uc.repo.CreateItem(ctx, userID, roadmapID, it); err != nil {
		return nil, err
	}
	return it, nil
}

func (uc *Roadmap) PatchItem(ctx context.Context, userID, roadmapID, itemID string, in ItemInput) (*domain.RoadmapItem, error) {
	it, err := uc.repo.FindItem(ctx, userID, roadmapID, itemID)
	if err != nil {
		return nil, err
	}
	if err := uc.apply(it, in); err != nil {
		return nil, err
	}
	if err := uc.repo.UpdateItem(ctx, userID, roadmapID, it); err != nil {
		return nil, err
	}
	return it, nil
}

// apply 는 보낸 칸을 검사해 항목에 넣는다. 처음 체크하면 완료일을 한국 날짜로 붙이고, 이미 체크된 건 그대로 둔다.
func (uc *Roadmap) apply(it *domain.RoadmapItem, in ItemInput) error {
	if in.Month != nil {
		m := strings.TrimSpace(*in.Month)
		if !monthPattern.MatchString(m) {
			return invalid("달은 YYYY-MM 형식이어야 합니다")
		}
		it.Month = m
	}
	if in.Category != nil {
		c := strings.TrimSpace(*in.Category)
		if msg := categoryName(c); msg != "" {
			return invalid("%s", msg)
		}
		it.Category = c
	}
	if in.Body != nil {
		b, err := required("할 일", oneLine(*in.Body), roadmapItemMax)
		if err != nil {
			return err
		}
		it.Body = b
	}
	if in.IsKey != nil {
		it.IsKey = *in.IsKey
	}
	if in.Done != nil {
		switch {
		case !*in.Done:
			it.DoneOn = ""
		case it.DoneOn == "":
			it.DoneOn = uc.today()
		}
	}
	return nil
}

func (uc *Roadmap) DeleteItem(ctx context.Context, userID, roadmapID, itemID string) error {
	return uc.repo.DeleteItem(ctx, userID, roadmapID, itemID)
}

func (uc *Roadmap) AddHabit(ctx context.Context, userID, roadmapID, name string) (*domain.RoadmapHabit, error) {
	n, err := required("습관", oneLine(name), roadmapHabitMax)
	if err != nil {
		return nil, err
	}
	h := &domain.RoadmapHabit{Name: n, Weeks: []string{}}
	if err := uc.repo.CreateHabit(ctx, userID, roadmapID, h); err != nil {
		return nil, err
	}
	return h, nil
}

func (uc *Roadmap) DeleteHabit(ctx context.Context, userID, roadmapID, habitID string) error {
	return uc.repo.DeleteHabit(ctx, userID, roadmapID, habitID)
}

// SetHabitWeek 는 그 주(YYYY-Www)를 체크(on)하거나 지운다. 없는 주(2027-W53 등)는 막는다.
func (uc *Roadmap) SetHabitWeek(ctx context.Context, userID, roadmapID, habitID, week string, on bool) (*domain.RoadmapHabit, error) {
	if !validWeek(week) {
		return nil, invalid("주는 2026-W45 처럼 있는 주여야 합니다")
	}
	return uc.repo.SetHabitWeek(ctx, userID, roadmapID, habitID, week, on)
}

// validWeek 은 ISO 주 키가 실제로 있는 주인지 본다. 12월 28일이 든 주가 그해의 마지막 주다.
func validWeek(week string) bool {
	m := weekPattern.FindStringSubmatch(week)
	if m == nil {
		return false
	}
	year, _ := strconv.Atoi(m[1])
	w, _ := strconv.Atoi(m[2])
	_, last := time.Date(year, time.December, 28, 0, 0, 0, 0, time.UTC).ISOWeek()
	return year >= 2000 && year <= 2100 && w >= 1 && w <= last
}

// requiredDate 는 비지 않은 YYYY-MM-DD 다.
func requiredDate(field, value string) (string, error) {
	v, err := date(field, value)
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", invalid("%s 를 입력해 주세요", field)
	}
	return v, nil
}

// oneLine 은 줄바꿈을 띄어쓰기로 바꾼다 (md 한 줄에 들어가야 하는 값).
func oneLine(s string) string {
	if !strings.ContainsAny(s, "\r\n") {
		return s
	}
	return strings.Join(strings.Fields(s), " ")
}
