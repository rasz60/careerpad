package jobsite

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

// Saramin 은 사람인 공식 Open API(https://oapi.saramin.co.kr/job-search) 검색이다.
// 접근 키(SARAMIN_ACCESS_KEY)가 없으면 목록 없이 검색 링크만 준다.
// 근무지는 1차 지역 코드(loc_mcd)로 넘기고, 경력은 응답의 경력 범위로 유스케이스가 거른다.
type Saramin struct {
	client    *http.Client
	baseURL   string
	accessKey string
}

func NewSaramin(client *http.Client, baseURL, accessKey string) *Saramin {
	return &Saramin{client: client, baseURL: strings.TrimRight(baseURL, "/"), accessKey: accessKey}
}

func (s *Saramin) Site() domain.JobSite {
	return domain.JobSite{ID: "saramin", Name: "사람인", Listable: s.accessKey != ""}
}

func (s *Saramin) SearchURL(q domain.JobQuery) string {
	return "https://www.saramin.co.kr/zf_user/search/recruit?searchword=" + url.QueryEscape(q.Keyword)
}

type saraminResponse struct {
	Jobs struct {
		Job []struct {
			ID      flexString `json:"id"`
			URL     string     `json:"url"`
			Company struct {
				Detail struct {
					Name string `json:"name"`
				} `json:"detail"`
			} `json:"company"`
			Position struct {
				Title    string `json:"title"`
				Location struct {
					Name string `json:"name"`
				} `json:"location"`
				ExperienceLevel struct {
					Code int    `json:"code"` // 1 신입, 2 경력, 3 신입/경력, 4 경력무관
					Min  int    `json:"min"`
					Max  int    `json:"max"`
					Name string `json:"name"`
				} `json:"experience-level"`
			} `json:"position"`
			Keyword             string     `json:"keyword"`
			ExpirationTimestamp flexString `json:"expiration-timestamp"`
		} `json:"job"`
	} `json:"jobs"`
}

var kst = time.FixedZone("KST", 9*60*60)

func (s *Saramin) Search(ctx context.Context, query domain.JobQuery) ([]domain.JobPosting, error) {
	if s.accessKey == "" {
		return nil, domain.ErrNotListable
	}
	q := url.Values{
		"access-key": {s.accessKey},
		"keywords":   {query.Keyword},
		"count":      {"50"},
		"sort":       {"pd"}, // 게시일 최신순
	}
	reg := resolveRegion(query.Location)
	if reg.resolved() {
		q.Set("loc_mcd", strings.Join(reg.saramin, ","))
	}
	var body saraminResponse
	if err := getJSON(ctx, s.client, s.baseURL+"/job-search?"+q.Encode(), nil, &body); err != nil {
		return nil, err
	}

	out := make([]domain.JobPosting, 0, len(body.Jobs.Job))
	for _, j := range body.Jobs.Job {
		location := strings.ReplaceAll(html.UnescapeString(j.Position.Location.Name), " > ", " ")
		if reg.resolved() && location != "" && !reg.matches(location) {
			continue
		}
		lv := j.Position.ExperienceLevel
		var min, max *int
		switch lv.Code {
		case 1:
			min, max = new(0), new(0)
		case 2:
			min = new(lv.Min)
			if lv.Max > 0 {
				max = new(lv.Max)
			}
		case 3:
			min = new(0)
			if lv.Max > 0 {
				max = new(lv.Max)
			}
		}
		career := lv.Name
		if career == "" {
			career = careerText(min, max)
		}
		out = append(out, domain.JobPosting{
			ID:        "saramin:" + string(j.ID),
			Title:     html.UnescapeString(j.Position.Title),
			Company:   html.UnescapeString(j.Company.Detail.Name),
			Location:  location,
			Career:    career,
			CareerMin: min,
			CareerMax: max,
			Deadline:  unixDate(string(j.ExpirationTimestamp)),
			URL:       j.URL,
			Tags:      splitKeywords(html.UnescapeString(j.Keyword)),

			LocationChecked: reg.resolved() && location != "",
		})
	}
	return out, nil
}

// unixDate 는 유닉스 초 문자열을 KST 날짜로 바꾼다. 상시 채용(아주 먼 날짜)은 빈 값.
func unixDate(sec string) string {
	n, err := strconv.ParseInt(sec, 10, 64)
	if err != nil || n <= 0 {
		return ""
	}
	t := time.Unix(n, 0).In(kst)
	if t.Year() >= 2090 {
		return ""
	}
	return t.Format(time.DateOnly)
}

// flexString 은 문자열("123")과 숫자(123) 어느 쪽으로 와도 문자열로 읽는다.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*f = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = flexString(n.String())
	return nil
}

func splitKeywords(s string) []string {
	out := []string{}
	for k := range strings.SplitSeq(s, ",") {
		if k = strings.TrimSpace(k); k != "" {
			out = append(out, k)
		}
	}
	return out
}
