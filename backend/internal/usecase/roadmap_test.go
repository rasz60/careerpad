package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

// fakeRoadmaps 는 주인 한 명(owner)의 로드맵 한 장만 아는 가짜 저장소다.
type fakeRoadmaps struct {
	domain.RoadmapRepository
	owner   string
	item    domain.RoadmapItem
	created *domain.RoadmapDetail
	updated *domain.RoadmapItem
	week    string
	weekOn  bool
}

func (f *fakeRoadmaps) mine(userID string) error {
	if userID != f.owner {
		return domain.ErrNotFound
	}
	return nil
}

func (f *fakeRoadmaps) Create(_ context.Context, userID string, d *domain.RoadmapDetail) error {
	f.created = d
	return f.mine(userID)
}

func (f *fakeRoadmaps) FindItem(_ context.Context, userID, _, _ string) (*domain.RoadmapItem, error) {
	if err := f.mine(userID); err != nil {
		return nil, err
	}
	it := f.item
	return &it, nil
}

func (f *fakeRoadmaps) UpdateItem(_ context.Context, userID, _ string, it *domain.RoadmapItem) error {
	f.updated = it
	return f.mine(userID)
}

func (f *fakeRoadmaps) SetHabitWeek(_ context.Context, userID, _, _, week string, on bool) (*domain.RoadmapHabit, error) {
	if err := f.mine(userID); err != nil {
		return nil, err
	}
	f.week, f.weekOn = week, on
	return &domain.RoadmapHabit{Weeks: []string{week}}, nil
}

// 2026-11-06 23:30 UTC = 한국 2026-11-07 08:30
func newTestRoadmap(repo *fakeRoadmaps) *Roadmap {
	uc := NewRoadmap(repo)
	uc.now = func() time.Time { return time.Date(2026, 11, 6, 23, 30, 0, 0, time.UTC) }
	return uc
}

func TestImportRejectsFirstError(t *testing.T) {
	repo := &fakeRoadmaps{owner: "u1"}
	_, err := newTestRoadmap(repo).Import(context.Background(), "u1", "# x\n- [ ] 달 없음")
	if !errors.Is(err, domain.ErrInvalidInput) || err.Error() != "2번째 줄: 할 일 위에 ### 2026-11 같은 달 제목이 있어야 해요: invalid input" {
		t.Fatalf("err = %v", err)
	}
	if repo.created != nil {
		t.Error("오류가 있으면 저장하지 않는다")
	}
}

func TestImportUsesKoreanToday(t *testing.T) {
	repo := &fakeRoadmaps{owner: "u1"}
	d, err := newTestRoadmap(repo).Import(context.Background(), "u1", "# x\n### 2026-11\n- [x] 끝")
	if err != nil {
		t.Fatal(err)
	}
	if d.Items[0].DoneOn != "2026-11-07" {
		t.Errorf("done_on = %q, want 2026-11-07 (한국 날짜)", d.Items[0].DoneOn)
	}
}

func TestPatchItemDone(t *testing.T) {
	ctx := context.Background()
	yes, no := true, false
	repo := &fakeRoadmaps{owner: "u1", item: domain.RoadmapItem{ID: "i1", Month: "2026-11", Body: "할 일"}}
	uc := newTestRoadmap(repo)

	it, err := uc.PatchItem(ctx, "u1", "r1", "i1", ItemInput{Done: &yes})
	if err != nil || it.DoneOn != "2026-11-07" || it.Body != "할 일" {
		t.Fatalf("check: %+v, %v", it, err)
	}

	// 이미 체크된 항목을 다시 체크해도 완료일은 그대로
	repo.item.DoneOn = "2026-11-01"
	if it, _ = uc.PatchItem(ctx, "u1", "r1", "i1", ItemInput{Done: &yes}); it.DoneOn != "2026-11-01" {
		t.Errorf("re-check done_on = %q, want 2026-11-01", it.DoneOn)
	}
	if it, _ = uc.PatchItem(ctx, "u1", "r1", "i1", ItemInput{Done: &no}); it.DoneOn != "" {
		t.Errorf("uncheck done_on = %q, want empty", it.DoneOn)
	}
}

func TestPatchItemValidates(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRoadmaps{owner: "u1", item: domain.RoadmapItem{ID: "i1", Month: "2026-11", Body: "할 일"}}
	uc := newTestRoadmap(repo)
	bad, blank, comma := "2026-13", "   ", "a,b"
	for name, in := range map[string]ItemInput{"달": {Month: &bad}, "빈 내용": {Body: &blank}, "분류": {Category: &comma}} {
		if _, err := uc.PatchItem(ctx, "u1", "r1", "i1", in); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: err = %v, want invalid input", name, err)
		}
	}
	multi := "첫 줄\n둘째 줄"
	it, err := uc.PatchItem(ctx, "u1", "r1", "i1", ItemInput{Body: &multi})
	if err != nil || it.Body != "첫 줄 둘째 줄" {
		t.Errorf("줄바꿈은 띄어쓰기로: %q, %v", it.Body, err)
	}
	if _, err := uc.AddItem(ctx, "u1", "r1", ItemInput{Body: &multi}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("달 없이 추가: err = %v", err)
	}
}

func TestOtherUserGetsNotFound(t *testing.T) {
	ctx := context.Background()
	yes := true
	uc := newTestRoadmap(&fakeRoadmaps{owner: "u1"})
	if _, err := uc.PatchItem(ctx, "u2", "r1", "i1", ItemInput{Done: &yes}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("patch: err = %v", err)
	}
	if _, err := uc.SetHabitWeek(ctx, "u2", "r1", "h1", "2026-W45", true); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("week: err = %v", err)
	}
}

func TestSetHabitWeekValidates(t *testing.T) {
	repo := &fakeRoadmaps{owner: "u1"}
	uc := newTestRoadmap(repo)
	for _, w := range []string{"2027-W53", "2026-W00", "2026-45", "2026-W5"} {
		if _, err := uc.SetHabitWeek(context.Background(), "u1", "r1", "h1", w, true); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: err = %v, want invalid input", w, err)
		}
	}
	// 목요일에 시작하는 해(2026)는 53주, 금요일에 시작하는 해(2027)는 52주다
	if !validWeek("2026-W53") || validWeek("2027-W53") || !validWeek("2027-W52") {
		t.Error("ISO 주 계산이 틀렸다")
	}
	if _, err := uc.SetHabitWeek(context.Background(), "u1", "r1", "h1", "2026-W45", false); err != nil || repo.week != "2026-W45" || repo.weekOn {
		t.Errorf("uncheck: %v %q %v", err, repo.week, repo.weekOn)
	}
}

func TestUpdateRoadmapValidates(t *testing.T) {
	ok := RoadmapInput{Title: "x", StartsOn: "2026-11-01", EndsOn: "2027-01-31",
		Categories: []string{"a", " a ", ""}, Phases: []domain.RoadmapPhase{{Title: "p", From: "2026-11", To: "2026-12"}}}
	r, err := normalizeRoadmap(ok)
	if err != nil || len(r.Categories) != 1 {
		t.Fatalf("ok: %+v, %v", r, err)
	}
	cases := map[string]func(*RoadmapInput){
		"기간 거꾸로": func(in *RoadmapInput) { in.EndsOn = "2026-10-01" },
		"시작일 없음": func(in *RoadmapInput) { in.StartsOn = "" },
		"구간 겹침": func(in *RoadmapInput) {
			in.Phases = []domain.RoadmapPhase{{Title: "a", From: "2026-11", To: "2026-12"}, {Title: "b", From: "2026-12", To: "2027-01"}}
		},
		"구간 달 없음": func(in *RoadmapInput) { in.Phases = []domain.RoadmapPhase{{Title: "a", From: "2026-11"}} },
		"분류 7개":   func(in *RoadmapInput) { in.Categories = []string{"a", "b", "c", "d", "e", "f", "g"} },
	}
	for name, mutate := range cases {
		in := ok
		mutate(&in)
		if _, err := normalizeRoadmap(in); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
