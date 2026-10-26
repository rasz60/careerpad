package domain

import (
	"context"
	"errors"
	"time"
)

// ErrNotListable 는 공고 목록을 가져올 수 없는(검색 링크만 주는) 사이트에서 Search 를 부를 때 돌려준다.
var ErrNotListable = errors.New("job site does not provide listings")

// JobSearch 는 저장해 둔 채용공고 검색 조건이다.
type JobSearch struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Keywords     []string  `json:"keywords"`
	Sites        []string  `json:"sites"`
	Location     string    `json:"location"`
	CareerYears  *int      `json:"career_years"`
	ExcludeWords []string  `json:"exclude_words"`
	SortOrder    int       `json:"sort_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// JobSearchRepository 는 career.job_searches 를 다룬다.
type JobSearchRepository interface {
	List(ctx context.Context, userID string) ([]JobSearch, error)
	Find(ctx context.Context, userID, id string) (*JobSearch, error)
	Create(ctx context.Context, userID string, s *JobSearch) error
	Update(ctx context.Context, userID string, s *JobSearch) error
	Delete(ctx context.Context, userID, id string) error
}

// JobSite 는 채용 사이트 정보다. Listable 이 false 면 목록 대신 사이트 검색 링크만 준다.
type JobSite struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Listable bool   `json:"listable"`
}

// JobPosting 은 사이트에서 가져온 공고 한 건이다. 경력 범위가 없으면(무관·모름) nil.
type JobPosting struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Company   string   `json:"company"`
	Location  string   `json:"location"`
	Career    string   `json:"career"`
	CareerMin *int     `json:"career_min"`
	CareerMax *int     `json:"career_max"`
	Deadline  string   `json:"deadline"` // YYYY-MM-DD, 비면 상시
	URL       string   `json:"url"`
	Tags      []string `json:"tags"`
	// Keywords 는 이 공고가 걸린 검색어들이다 (키워드마다 따로 검색해서 합친다).
	Keywords []string `json:"keywords"`
	// LocationChecked 면 어댑터가 근무지 조건을 이미 맞춰 봤다 (사이트의 지역 필터 · 지역 이름표).
	// 아니면 유스케이스가 근무지 글자로 거른다.
	LocationChecked bool `json:"-"`
}

// JobQuery 는 사이트에 묻는 검색 한 번이다. 근무지 · 경력은 사이트가 받는 만큼 검색 조건으로 넘기고,
// 받은 결과는 유스케이스가 한 번 더 거른다 (사이트가 조건을 못 받았거나 넓게 받았을 때를 위해).
type JobQuery struct {
	Keyword     string
	Location    string
	CareerYears *int
}

// JobSource 는 채용 사이트 하나의 검색 어댑터다.
type JobSource interface {
	Site() JobSite
	// SearchURL 은 사이트에서 직접 검색 결과를 여는 주소다.
	SearchURL(q JobQuery) string
	// Search 는 조건 하나로 최신 공고를 가져온다. 목록을 못 주는 사이트는 ErrNotListable.
	// 사이트가 근무지 조건을 받아 서버에서 걸렀다면 공고의 Location 이 비어 있어도 된다.
	Search(ctx context.Context, q JobQuery) ([]JobPosting, error)
}

// KeywordLink 는 키워드별 사이트 검색 링크다.
type KeywordLink struct {
	Keyword string `json:"keyword"`
	URL     string `json:"url"`
}

// SiteResult 는 검색 조건 하나를 사이트 하나에 돌린 결과다.
type SiteResult struct {
	Site     JobSite       `json:"site"`
	Links    []KeywordLink `json:"links"`
	Postings []JobPosting  `json:"postings"`
	// Error 는 사이트 조회가 실패했을 때의 이유다 (다른 사이트 결과는 그대로 보여 준다).
	Error string `json:"error,omitempty"`
	// Note 는 조건이 다 걸리지 않았을 때의 안내다 (근무지를 모르는 공고를 남겼다 · 링크에는 키워드만 들어간다 등).
	Note string `json:"note,omitempty"`
}
