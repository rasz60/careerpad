package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

const (
	jobCacheTTL     = 10 * time.Minute // 같은 사이트·키워드는 이 시간 동안 다시 묻지 않는다 (사이트 부담 · 응답 속도)
	jobFetchTimeout = 12 * time.Second
	maxPostingsSite = 100
)

// Job 은 job++ (채용공고 검색 조건 · 사이트별 공고 조회) 유스케이스다.
type Job struct {
	searches domain.JobSearchRepository
	sources  map[string]domain.JobSource
	order    []string // 사이트 표시 순서

	mu    sync.Mutex
	cache map[string]cachedPostings
	now   func() time.Time
}

type cachedPostings struct {
	postings []domain.JobPosting
	at       time.Time
}

func NewJob(searches domain.JobSearchRepository, sources []domain.JobSource) *Job {
	uc := &Job{
		searches: searches,
		sources:  make(map[string]domain.JobSource, len(sources)),
		cache:    map[string]cachedPostings{},
		now:      time.Now,
	}
	for _, s := range sources {
		uc.sources[s.Site().ID] = s
		uc.order = append(uc.order, s.Site().ID)
	}
	return uc
}

// Sites 는 고를 수 있는 채용 사이트 목록이다.
func (uc *Job) Sites() []domain.JobSite {
	out := make([]domain.JobSite, 0, len(uc.order))
	for _, id := range uc.order {
		out = append(out, uc.sources[id].Site())
	}
	return out
}

func (uc *Job) List(ctx context.Context, userID string) ([]domain.JobSearch, error) {
	return uc.searches.List(ctx, userID)
}

func (uc *Job) Create(ctx context.Context, userID string, in domain.JobSearch) (*domain.JobSearch, error) {
	s, err := uc.normalize(in)
	if err != nil {
		return nil, err
	}
	if err := uc.searches.Create(ctx, userID, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (uc *Job) Update(ctx context.Context, userID, id string, in domain.JobSearch) (*domain.JobSearch, error) {
	s, err := uc.normalize(in)
	if err != nil {
		return nil, err
	}
	s.ID = id
	if err := uc.searches.Update(ctx, userID, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (uc *Job) Delete(ctx context.Context, userID, id string) error {
	return uc.searches.Delete(ctx, userID, id)
}

// Results 는 저장한 조건으로 사이트마다 공고를 모은다. 사이트끼리는 동시에 묻고,
// 한 사이트가 실패해도 나머지 결과는 그대로 돌려준다.
func (uc *Job) Results(ctx context.Context, userID, searchID string) ([]domain.SiteResult, error) {
	search, err := uc.searches.Find(ctx, userID, searchID)
	if err != nil {
		return nil, err
	}

	results := make([]domain.SiteResult, len(search.Sites))
	var wg sync.WaitGroup
	for i, siteID := range search.Sites {
		source, ok := uc.sources[siteID]
		if !ok {
			results[i] = domain.SiteResult{
				Site:  domain.JobSite{ID: siteID, Name: siteID},
				Links: []domain.KeywordLink{}, Postings: []domain.JobPosting{},
				Error: "더 이상 지원하지 않는 사이트입니다",
			}
			continue
		}
		wg.Go(func() { results[i] = uc.siteResult(ctx, source, search) })
	}
	wg.Wait()
	return results, nil
}

func (uc *Job) siteResult(ctx context.Context, source domain.JobSource, search *domain.JobSearch) domain.SiteResult {
	res := domain.SiteResult{Site: source.Site(), Links: []domain.KeywordLink{}, Postings: []domain.JobPosting{}}
	query := func(kw string) domain.JobQuery {
		return domain.JobQuery{Keyword: kw, Location: search.Location, CareerYears: search.CareerYears}
	}
	for _, kw := range search.Keywords {
		res.Links = append(res.Links, domain.KeywordLink{Keyword: kw, URL: source.SearchURL(query(kw))})
	}
	if !res.Site.Listable {
		if search.Location != "" || search.CareerYears != nil {
			res.Note = "검색 링크에는 키워드만 들어가요. 근무지 · 경력은 사이트에서 골라 주세요."
		}
		return res
	}

	byKeyword := make(map[string][]domain.JobPosting, len(search.Keywords))
	var failures []string
	for _, kw := range search.Keywords {
		postings, err := uc.fetch(ctx, source, query(kw))
		if err != nil {
			failures = append(failures, kw)
			continue
		}
		byKeyword[kw] = postings
	}
	if len(failures) > 0 {
		res.Error = fmt.Sprintf("일부 키워드를 가져오지 못했습니다: %s", strings.Join(failures, ", "))
	}
	var unknown int
	res.Postings, unknown = filterPostings(mergePostings(search.Keywords, byKeyword), search)
	if unknown > 0 {
		res.Note = fmt.Sprintf("근무지를 알 수 없는 공고 %d건은 빼지 않고 남겼어요.", unknown)
	}
	if len(res.Postings) > maxPostingsSite {
		res.Postings = res.Postings[:maxPostingsSite]
	}
	return res
}

// fetch 는 사이트·키워드 하나를 캐시를 거쳐 가져온다.
func (uc *Job) fetch(ctx context.Context, source domain.JobSource, q domain.JobQuery) ([]domain.JobPosting, error) {
	years := ""
	if q.CareerYears != nil {
		years = fmt.Sprint(*q.CareerYears)
	}
	key := strings.Join([]string{source.Site().ID, strings.ToLower(q.Keyword), strings.TrimSpace(q.Location), years}, "\x00")

	uc.mu.Lock()
	hit, ok := uc.cache[key]
	uc.mu.Unlock()
	if ok && uc.now().Sub(hit.at) < jobCacheTTL {
		return hit.postings, nil
	}

	fetchCtx, cancel := context.WithTimeout(ctx, jobFetchTimeout)
	defer cancel()
	postings, err := source.Search(fetchCtx, q)
	if err != nil {
		if errors.Is(err, domain.ErrNotListable) {
			return nil, err
		}
		return nil, fmt.Errorf("job: %s %q: %w", source.Site().ID, q.Keyword, err)
	}

	uc.mu.Lock()
	uc.cache[key] = cachedPostings{postings: postings, at: uc.now()}
	// 오래된 항목 정리 (검색 조건이 많지 않아 전체를 훑어도 싸다)
	for k, v := range uc.cache {
		if uc.now().Sub(v.at) >= jobCacheTTL {
			delete(uc.cache, k)
		}
	}
	uc.mu.Unlock()
	return postings, nil
}

// mergePostings 는 키워드별 결과를 공고 ID 로 합치고, 각 공고에 걸린 키워드를 모은다.
// 순서는 키워드 순서 → 사이트가 준 순서(최신순)를 따른다.
func mergePostings(keywords []string, byKeyword map[string][]domain.JobPosting) []domain.JobPosting {
	var (
		out   []domain.JobPosting
		index = map[string]int{}
	)
	for _, kw := range keywords {
		for _, p := range byKeyword[kw] {
			if i, ok := index[p.ID]; ok {
				if !slices.Contains(out[i].Keywords, kw) {
					out[i].Keywords = append(out[i].Keywords, kw)
				}
				continue
			}
			p.Keywords = []string{kw}
			index[p.ID] = len(out)
			out = append(out, p)
		}
	}
	if out == nil {
		out = []domain.JobPosting{}
	}
	return out
}

// filterPostings 는 근무지 · 경력 · 제외어 조건을 적용한다. 사이트가 이미 조건을 걸었어도 한 번 더 본다.
// 근무지는 어댑터가 맞춰 본 공고(LocationChecked)는 그대로 두고, 나머지는 근무지 글자로 본다.
// 사이트가 근무지 · 경력을 주지 않은 공고는 모르는 것이므로 빼지 않는다 (unknown: 근무지를 모르는 채 남긴 수).
func filterPostings(postings []domain.JobPosting, search *domain.JobSearch) (out []domain.JobPosting, unknown int) {
	location := strings.TrimSpace(search.Location)
	excludes := make([]string, 0, len(search.ExcludeWords))
	for _, w := range search.ExcludeWords {
		excludes = append(excludes, strings.ToLower(w))
	}

	out = make([]domain.JobPosting, 0, len(postings))
	for _, p := range postings {
		if location != "" && !p.LocationChecked && p.Location != "" && !locationMatches(p.Location, location) {
			continue
		}
		if y := search.CareerYears; y != nil {
			if p.CareerMin != nil && *y < *p.CareerMin {
				continue
			}
			if p.CareerMax != nil && *y > *p.CareerMax {
				continue
			}
		}
		haystack := strings.ToLower(p.Title + " " + p.Company)
		if slices.ContainsFunc(excludes, func(w string) bool { return strings.Contains(haystack, w) }) {
			continue
		}
		if location != "" && !p.LocationChecked && p.Location == "" {
			unknown++
		}
		out = append(out, p)
	}
	return out, unknown
}

// locationMatches 는 근무지 글자 비교다. 쉼표로 나눈 후보 중 하나라도, 그 안의 낱말이 모두 들어 있으면 맞다
// ("서울, 경기" · "성남 판교"). 지역 이름표가 없는 경우를 위한 것이라 단순 비교만 한다.
func locationMatches(postingLocation, location string) bool {
	have := strings.ToLower(postingLocation)
	for alt := range strings.SplitSeq(location, ",") {
		words := strings.Fields(strings.ToLower(alt))
		if len(words) > 0 && !slices.ContainsFunc(words, func(w string) bool { return !strings.Contains(have, w) }) {
			return true
		}
	}
	return false
}

func (uc *Job) normalize(in domain.JobSearch) (*domain.JobSearch, error) {
	var (
		out = domain.JobSearch{SortOrder: in.SortOrder}
		err error
	)
	if out.Name, err = required("조건 이름", in.Name, 60); err != nil {
		return nil, err
	}
	if out.Keywords, err = words("키워드", in.Keywords, 10, 40); err != nil {
		return nil, err
	}
	if len(out.Keywords) == 0 {
		return nil, invalid("키워드를 하나 이상 넣어 주세요")
	}
	for _, id := range in.Sites {
		if _, ok := uc.sources[id]; !ok {
			return nil, invalid("모르는 사이트입니다: %s", id)
		}
		if !slices.Contains(out.Sites, id) {
			out.Sites = append(out.Sites, id)
		}
	}
	if len(out.Sites) == 0 {
		return nil, invalid("사이트를 하나 이상 골라 주세요")
	}
	if out.Location, err = text("근무지", in.Location, 30); err != nil {
		return nil, err
	}
	if y := in.CareerYears; y != nil {
		if *y < 0 || *y > 40 {
			return nil, invalid("경력은 0~40년으로 넣어 주세요")
		}
		v := *y
		out.CareerYears = &v
	}
	if out.ExcludeWords, err = words("제외어", in.ExcludeWords, 20, 30); err != nil {
		return nil, err
	}
	return &out, nil
}
