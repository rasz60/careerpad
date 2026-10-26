package usecase

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

func posting(id, title, company, location string, min, max *int) domain.JobPosting {
	return domain.JobPosting{ID: id, Title: title, Company: company, Location: location, CareerMin: min, CareerMax: max}
}

func TestMergePostingsDedupesAcrossKeywords(t *testing.T) {
	byKeyword := map[string][]domain.JobPosting{
		"go":     {posting("a", "Go 개발자", "A사", "서울", nil, nil), posting("b", "백엔드", "B사", "서울", nil, nil)},
		"golang": {posting("b", "백엔드", "B사", "서울", nil, nil), posting("c", "플랫폼", "C사", "판교", nil, nil)},
	}

	got := mergePostings([]string{"go", "golang"}, byKeyword)

	ids := make([]string, 0, len(got))
	for _, p := range got {
		ids = append(ids, p.ID)
	}
	if !slices.Equal(ids, []string{"a", "b", "c"}) {
		t.Fatalf("ids = %v, want [a b c]", ids)
	}
	if !slices.Equal(got[1].Keywords, []string{"go", "golang"}) {
		t.Errorf("b keywords = %v, want [go golang]", got[1].Keywords)
	}
}

func TestMergePostingsEmptyIsNotNil(t *testing.T) {
	if got := mergePostings([]string{"x"}, nil); got == nil {
		t.Fatal("want empty slice, got nil (JSON 으로 null 이 나간다)")
	}
}

func TestFilterPostings(t *testing.T) {
	postings := []domain.JobPosting{
		posting("seoul-any", "Go 개발자", "A사", "서울 강남구", nil, nil),
		posting("busan", "Go 개발자", "B사", "부산 해운대구", nil, nil),
		posting("senior", "시니어 Go", "C사", "서울 중구", new(10), nil),
		posting("junior", "주니어 Go", "D사", "서울 마포구", new(0), new(2)),
		posting("sales", "Go 영업", "E사", "서울 서초구", nil, nil),
		posting("no-location", "Go 개발자", "F사", "", nil, nil),
	}
	search := &domain.JobSearch{Location: "서울", CareerYears: new(5), ExcludeWords: []string{"영업"}}

	got, unknown := filterPostings(postings, search)

	ids := make([]string, 0, len(got))
	for _, p := range got {
		ids = append(ids, p.ID)
	}
	if !slices.Equal(ids, []string{"seoul-any", "no-location"}) {
		t.Fatalf("ids = %v, want [seoul-any no-location]", ids)
	}
	if unknown != 1 {
		t.Errorf("unknown = %d, want 1 (no-location)", unknown)
	}
}

func TestFilterPostingsTrustsCheckedLocation(t *testing.T) {
	// 어댑터가 지역 필터로 이미 맞춰 본 공고는 근무지 글자가 달라도(전남광주 나주시 vs 전라남도 나주시) 남는다
	checked := posting("checked", "Go", "A사", "전남광주 나주시", nil, nil)
	checked.LocationChecked = true
	wantedStyle := posting("wanted", "Go", "B사", "", nil, nil)
	wantedStyle.LocationChecked = true
	got, unknown := filterPostings([]domain.JobPosting{checked, wantedStyle}, &domain.JobSearch{Location: "전라남도 나주시"})
	if len(got) != 2 || unknown != 0 {
		t.Fatalf("got %d postings, unknown %d; want 2, 0", len(got), unknown)
	}
}

func TestLocationMatches(t *testing.T) {
	cases := []struct {
		posting, location string
		want              bool
	}{
		{"서울 강남구", "서울", true},
		{"경기 성남시 분당구 판교역로", "성남 판교", true},
		{"경기 성남시", "성남 판교", false},
		{"경기 수원시", "서울, 경기", true},
		{"부산 해운대구", "서울, 경기", false},
	}
	for _, c := range cases {
		if got := locationMatches(c.posting, c.location); got != c.want {
			t.Errorf("locationMatches(%q, %q) = %v, want %v", c.posting, c.location, got, c.want)
		}
	}
}

func TestFilterPostingsWithoutConditionsKeepsAll(t *testing.T) {
	postings := []domain.JobPosting{posting("a", "x", "y", "z", nil, nil), posting("b", "x", "y", "z", new(3), new(5))}
	if got, _ := filterPostings(postings, &domain.JobSearch{}); len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

// ── Results: 사이트별 동시 조회 · 실패 격리 · 캐시 ────────────────

type fakeSource struct {
	site  domain.JobSite
	calls atomic.Int32
	last  domain.JobQuery
	err   error
	items []domain.JobPosting
}

func (f *fakeSource) Site() domain.JobSite { return f.site }
func (f *fakeSource) SearchURL(q domain.JobQuery) string {
	return "https://example.com/?q=" + q.Keyword
}
func (f *fakeSource) Search(_ context.Context, q domain.JobQuery) ([]domain.JobPosting, error) {
	f.calls.Add(1)
	f.last = q
	if !f.site.Listable {
		return nil, domain.ErrNotListable
	}
	return f.items, f.err
}

type fakeSearches struct {
	domain.JobSearchRepository
	search domain.JobSearch
}

func (f *fakeSearches) Find(context.Context, string, string) (*domain.JobSearch, error) {
	s := f.search
	return &s, nil
}

func TestResultsIsolatesFailuresAndCaches(t *testing.T) {
	ok := &fakeSource{site: domain.JobSite{ID: "ok", Listable: true}, items: []domain.JobPosting{posting("1", "Go", "A", "서울", nil, nil)}}
	broken := &fakeSource{site: domain.JobSite{ID: "broken", Listable: true}, err: errors.New("boom")}
	link := &fakeSource{site: domain.JobSite{ID: "link", Listable: false}}
	repo := &fakeSearches{search: domain.JobSearch{Keywords: []string{"go"}, Sites: []string{"ok", "broken", "link", "gone"}}}

	uc := NewJob(repo, []domain.JobSource{ok, broken, link})
	now := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	uc.now = func() time.Time { return now }

	results, err := uc.Results(context.Background(), "u", "s")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("len = %d, want 4", len(results))
	}
	if len(results[0].Postings) != 1 || results[0].Error != "" {
		t.Errorf("ok site = %+v", results[0])
	}
	if results[1].Error == "" || len(results[1].Postings) != 0 {
		t.Errorf("broken site should report error with no postings: %+v", results[1])
	}
	if len(results[2].Links) != 1 || link.calls.Load() != 0 {
		t.Errorf("link-only site should give links without calling Search: %+v (calls %d)", results[2], link.calls.Load())
	}
	if results[3].Error == "" {
		t.Errorf("unknown site should report error: %+v", results[3])
	}

	// 캐시 시간 안에 다시 부르면 사이트를 다시 묻지 않는다
	if _, err := uc.Results(context.Background(), "u", "s"); err != nil {
		t.Fatal(err)
	}
	if ok.calls.Load() != 1 {
		t.Errorf("ok calls = %d, want 1 (cached)", ok.calls.Load())
	}
	// 캐시가 지나면 다시 묻는다
	now = now.Add(jobCacheTTL + time.Second)
	if _, err := uc.Results(context.Background(), "u", "s"); err != nil {
		t.Fatal(err)
	}
	if ok.calls.Load() != 2 {
		t.Errorf("ok calls = %d, want 2 (cache expired)", ok.calls.Load())
	}
}

func TestResultsPassesConditionsToSites(t *testing.T) {
	src := &fakeSource{site: domain.JobSite{ID: "ok", Listable: true}, items: []domain.JobPosting{posting("1", "Go", "A", "", nil, nil)}}
	link := &fakeSource{site: domain.JobSite{ID: "link", Listable: false}}
	repo := &fakeSearches{search: domain.JobSearch{Keywords: []string{"개발"}, Sites: []string{"ok", "link"}, Location: "나주", CareerYears: new(4)}}
	uc := NewJob(repo, []domain.JobSource{src, link})

	results, err := uc.Results(context.Background(), "u", "s")
	if err != nil {
		t.Fatal(err)
	}
	if src.last.Keyword != "개발" || src.last.Location != "나주" || src.last.CareerYears == nil || *src.last.CareerYears != 4 {
		t.Errorf("query = %+v, want keyword · location · years passed to the site", src.last)
	}
	if results[0].Note == "" {
		t.Error("posting without location should be reported in Note")
	}
	if results[1].Note == "" {
		t.Error("link-only site should say links carry only the keyword")
	}

	// 근무지를 바꾸면 캐시를 쓰지 않고 다시 묻는다
	repo.search.Location = "서울"
	if _, err := uc.Results(context.Background(), "u", "s"); err != nil {
		t.Fatal(err)
	}
	if src.calls.Load() != 2 {
		t.Errorf("calls = %d, want 2 (cache key includes location)", src.calls.Load())
	}
}

func TestNormalizeJobSearch(t *testing.T) {
	uc := NewJob(nil, []domain.JobSource{&fakeSource{site: domain.JobSite{ID: "wanted", Listable: true}}})

	got, err := uc.normalize(domain.JobSearch{
		Name: " 백엔드 ", Keywords: []string{"Go", " go ", "", "Kotlin"}, Sites: []string{"wanted", "wanted"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "백엔드" || !slices.Equal(got.Keywords, []string{"Go", "Kotlin"}) || !slices.Equal(got.Sites, []string{"wanted"}) {
		t.Errorf("normalize = %+v", got)
	}

	cases := map[string]domain.JobSearch{
		"no keywords":  {Name: "x", Sites: []string{"wanted"}},
		"unknown site": {Name: "x", Keywords: []string{"go"}, Sites: []string{"nope"}},
		"no site":      {Name: "x", Keywords: []string{"go"}},
		"bad career":   {Name: "x", Keywords: []string{"go"}, Sites: []string{"wanted"}, CareerYears: new(-1)},
	}
	for name, in := range cases {
		if _, err := uc.normalize(in); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", name, err)
		}
	}
}
