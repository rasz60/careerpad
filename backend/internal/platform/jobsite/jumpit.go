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

func (s *Jumpit) SearchURL(keyword string) string {
	return "https://jumpit.saramin.co.kr/search?keyword=" + url.QueryEscape(keyword)
}

type jumpitResponse struct {
	Result struct {
		Positions []struct {
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

func (s *Jumpit) Search(ctx context.Context, keyword string) ([]domain.JobPosting, error) {
	q := url.Values{"keyword": {keyword}, "sort": {"reg_dt"}, "page": {"1"}}
	var body jumpitResponse
	if err := getJSON(ctx, s.client, s.baseURL+"/api/positions?"+q.Encode(), nil, &body); err != nil {
		return nil, err
	}

	out := make([]domain.JobPosting, 0, len(body.Result.Positions))
	for _, p := range body.Result.Positions {
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
			Location:  strings.Join(p.Locations, ", "),
			Career:    careerText(min, max),
			CareerMin: min,
			CareerMax: max,
			Deadline:  deadline,
			URL:       "https://jumpit.saramin.co.kr/position/" + id,
			Tags:      tags,
		})
	}
	return out, nil
}
