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
// 이 응답에는 근무지·마감일이 없어 비워 둔다 (근무지 조건은 이런 공고를 빼지 않는다).
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

func (s *Wanted) SearchURL(keyword string) string {
	return "https://www.wanted.co.kr/search?tab=position&query=" + url.QueryEscape(keyword)
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

func (s *Wanted) Search(ctx context.Context, keyword string) ([]domain.JobPosting, error) {
	q := url.Values{"query": {keyword}, "limit": {"50"}, "offset": {"0"}}
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
		})
	}
	return out, nil
}
