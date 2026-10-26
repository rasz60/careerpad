package jobsite

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

// Wanted 는 원티드 공고 검색이다. 사이트 검색창과 같은 공개 JSON(/api/chaos/search/v1/position, 관련도순)을 쓴다.
// 제목만 보는 /api/v4/jobs?query= 는 "golang" 같은 기술 키워드를 놓쳐서 쓰지 않는다.
// 이 응답에는 근무지·마감일이 없다. 그래서 근무지 · 경력은 검색 조건(locations · years)으로 넘겨 원티드가 거르게 한다.
type Wanted struct {
	client  *http.Client
	baseURL string
}

func NewWanted(client *http.Client, baseURL string) *Wanted {
	return &Wanted{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

func (s *Wanted) Site() domain.JobSite {
	return domain.JobSite{ID: "wanted", Name: "원티드", Listable: true}
}

func (s *Wanted) SearchURL(q domain.JobQuery) string {
	return "https://www.wanted.co.kr/search?tab=position&query=" + url.QueryEscape(q.Keyword)
}

type wantedResponse struct {
	Data []struct {
		ID       int    `json:"id"`
		Position string `json:"position"`
		Company  struct {
			Name string `json:"name"`
		} `json:"company"`
		AnnualFrom *int `json:"annual_from"`
		AnnualTo   *int `json:"annual_to"`
	} `json:"data"`
}

// wantedNoMax 는 원티드가 "경력 상한 없음"에 쓰는 값이다.
const wantedNoMax = 100

func (s *Wanted) Search(ctx context.Context, query domain.JobQuery) ([]domain.JobPosting, error) {
	q := url.Values{"query": {query.Keyword}, "limit": {"50"}, "offset": {"0"}}
	// 근무지를 못 알아보면 근무지 없이 묻는다. 그 공고들은 근무지를 모르는 채로 남는다 (유스케이스가 안내)
	reg := resolveRegion(query.Location)
	q["locations"] = reg.wanted
	if query.CareerYears != nil {
		q.Set("years", strconv.Itoa(*query.CareerYears))
	}
	var body wantedResponse
	if err := getJSON(ctx, s.client, s.baseURL+"/api/chaos/search/v1/position?"+q.Encode(), nil, &body); err != nil {
		return nil, err
	}

	out := make([]domain.JobPosting, 0, len(body.Data))
	for _, j := range body.Data {
		min, max := j.AnnualFrom, j.AnnualTo
		if max != nil && *max >= wantedNoMax {
			max = nil
		}
		id := strconv.Itoa(j.ID)
		out = append(out, domain.JobPosting{
			ID:        "wanted:" + id,
			Title:     j.Position,
			Company:   j.Company.Name,
			Career:    careerText(min, max),
			CareerMin: min,
			CareerMax: max,
			URL:       "https://www.wanted.co.kr/wd/" + id,
			Tags:      []string{},
			// 원티드가 지역 필터로 이미 걸렀다
			LocationChecked: reg.resolved(),
		})
	}
	return out, nil
}
