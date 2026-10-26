package jobsite

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

// Jumpit 은 점핏(개발자 채용) 공고 검색이다 (웹 화면이 쓰는 공개 JSON: /api/positions?keyword=).
// 근무지는 시·도 코드(locationTag, 사람인과 같은 체계), 경력은 career 로 넘긴다. 시·군·구는 받은 뒤 유스케이스가 거른다.
// 한 쪽이 16건이라 몇 쪽을 이어 받는다.
type Jumpit struct {
	client  *http.Client
	baseURL string
}

func NewJumpit(client *http.Client, baseURL string) *Jumpit {
	return &Jumpit{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

func (s *Jumpit) Site() domain.JobSite {
	return domain.JobSite{ID: "jumpit", Name: "점핏", Listable: true}
}

// jumpitPages 는 한 번에 이어 받는 쪽 수다 (쪽당 16건)
const jumpitPages = 3

func (s *Jumpit) SearchURL(q domain.JobQuery) string {
	return "https://jumpit.saramin.co.kr/search?keyword=" + url.QueryEscape(q.Keyword)
}

type jumpitResponse struct {
	Result struct {
		TotalCount int `json:"totalCount"`
		Positions  []struct {
			ID          int      `json:"id"`
			Title       string   `json:"title"`
			CompanyName string   `json:"companyName"`
			TechStacks  []string `json:"techStacks"`
			Newcomer    bool     `json:"newcomer"`
			MinCareer   *int     `json:"minCareer"`
			MaxCareer   *int     `json:"maxCareer"`
			Locations   []string `json:"locations"`
			AlwaysOpen  bool     `json:"alwaysOpen"`
			ClosedAt    string   `json:"closedAt"`
		} `json:"positions"`
	} `json:"result"`
}

func (s *Jumpit) Search(ctx context.Context, query domain.JobQuery) ([]domain.JobPosting, error) {
	q := url.Values{"keyword": {query.Keyword}, "sort": {"reg_dt"}}
	reg := resolveRegion(query.Location)
	q["locationTag"] = reg.saramin
	if query.CareerYears != nil {
		q.Set("career", strconv.Itoa(*query.CareerYears))
	}

	var out []domain.JobPosting
	seen := 0 // 받은 공고 수 (시·군·구로 거르기 전)
	for page := 1; page <= jumpitPages; page++ {
		q.Set("page", strconv.Itoa(page))
		var body jumpitResponse
		if err := getJSON(ctx, s.client, s.baseURL+"/api/positions?"+q.Encode(), nil, &body); err != nil {
			if page > 1 {
				break // 뒤쪽 실패는 앞쪽 결과로 충분하다
			}
			return nil, err
		}
		out = append(out, jumpitPostings(body, reg)...)
		seen += len(body.Result.Positions)
		if seen >= body.Result.TotalCount || len(body.Result.Positions) == 0 {
			break
		}
	}
	if out == nil {
		out = []domain.JobPosting{}
	}
	return out, nil
}

// jumpitPostings 는 응답을 공고로 바꾼다. 시·도는 점핏이 걸렀고, 시·군·구는 근무지 글자로 여기서 거른다.
func jumpitPostings(body jumpitResponse, reg region) []domain.JobPosting {
	out := make([]domain.JobPosting, 0, len(body.Result.Positions))
	for _, p := range body.Result.Positions {
		location := strings.Join(p.Locations, ", ")
		if reg.resolved() && location != "" && !reg.matches(location) {
			continue
		}
		min, max := p.MinCareer, p.MaxCareer
		if p.Newcomer && min == nil {
			min = new(0)
		}
		deadline := ""
		if !p.AlwaysOpen && len(p.ClosedAt) >= 10 {
			deadline = p.ClosedAt[:10]
		}
		tags := make([]string, 0, len(p.TechStacks))
		for _, t := range p.TechStacks {
			if t = stripTags(t); t != "" {
				tags = append(tags, t)
			}
		}
		id := strconv.Itoa(p.ID)
		out = append(out, domain.JobPosting{
			ID:        "jumpit:" + id,
			Title:     stripTags(p.Title),
			Company:   p.CompanyName,
			Location:  location,
			Career:    careerText(min, max),
			CareerMin: min,
			CareerMax: max,
			Deadline:  deadline,
			URL:       "https://jumpit.saramin.co.kr/position/" + id,
			Tags:      tags,

			LocationChecked: reg.resolved() && location != "",
		})
	}
	return out
}
