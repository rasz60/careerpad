// Package jobsite 는 채용 사이트별 공고 검색 어댑터(domain.JobSource)다.
//
// 목록을 가져오는 사이트: 원티드 · 점핏(공개 JSON), 사람인(공식 Open API, 접근 키가 있을 때).
// 나머지(잡코리아 · 링크드인 · 키 없는 사람인)는 사이트 검색 결과로 바로 가는 링크만 준다.
// 공개 JSON 은 사이트가 예고 없이 바꿀 수 있으므로, 실패하면 해당 사이트만 오류로 표시되고 링크는 그대로 남는다.
package jobsite

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

const userAgent = "Mozilla/5.0 (compatible; careerpad/1.0; +https://ae2p.duckdns.org/career/)"

// Options 는 어댑터 공통 설정이다.
type Options struct {
	Client           *http.Client
	SaraminAccessKey string
}

// Sources 는 화면에 보일 순서대로 사이트 어댑터를 만든다.
func Sources(opts Options) []domain.JobSource {
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return []domain.JobSource{
		NewWanted(client, "https://www.wanted.co.kr"),
		NewJumpit(client, "https://jumpit-api.saramin.co.kr"),
		NewSaramin(client, "https://oapi.saramin.co.kr", opts.SaraminAccessKey),
		NewLinkOnly(domain.JobSite{ID: "jobkorea", Name: "잡코리아"}, "https://www.jobkorea.co.kr/Search/?stext="),
		NewLinkOnly(domain.JobSite{ID: "linkedin", Name: "LinkedIn"}, "https://www.linkedin.com/jobs/search/?location=South%20Korea&keywords="),
	}
}

// getJSON 은 GET 요청을 보내 JSON 을 dst 로 읽는다.
func getJSON(ctx context.Context, client *http.Client, rawURL string, header http.Header, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("jobsite: build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("jobsite: request %s: %w", req.URL.Host, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, 200))
		return fmt.Errorf("jobsite: %s: HTTP %d: %s", req.URL.Host, res.StatusCode, strings.TrimSpace(string(snippet)))
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(dst); err != nil {
		return fmt.Errorf("jobsite: decode %s: %w", req.URL.Host, err)
	}
	return nil
}

var tagPattern = regexp.MustCompile(`<[^>]*>`)

// stripTags 는 검색어 강조용 <span> 같은 태그를 지운다.
func stripTags(s string) string {
	return strings.TrimSpace(tagPattern.ReplaceAllString(s, ""))
}

// careerText 는 경력 범위를 사람이 읽는 말로 바꾼다. nil 은 제한 없음.
func careerText(min, max *int) string {
	switch {
	case min == nil && max == nil:
		return "경력 무관"
	case min != nil && *min == 0 && max != nil && *max == 0:
		return "신입"
	case min != nil && max != nil:
		if *min == 0 {
			return fmt.Sprintf("신입~%d년", *max)
		}
		return fmt.Sprintf("%d~%d년", *min, *max)
	case min != nil:
		if *min == 0 {
			return "신입 이상"
		}
		return fmt.Sprintf("%d년 이상", *min)
	default:
		return fmt.Sprintf("%d년 이하", *max)
	}
}

// linkOnly 는 목록 없이 검색 링크만 주는 사이트다.
type linkOnly struct {
	site   domain.JobSite
	prefix string
}

// NewLinkOnly 는 prefix 뒤에 검색어를 붙인 주소만 주는 어댑터를 만든다.
func NewLinkOnly(site domain.JobSite, prefix string) domain.JobSource {
	site.Listable = false
	return &linkOnly{site: site, prefix: prefix}
}

func (s *linkOnly) Site() domain.JobSite { return s.site }

func (s *linkOnly) SearchURL(keyword string) string { return s.prefix + url.QueryEscape(keyword) }

func (s *linkOnly) Search(context.Context, string) ([]domain.JobPosting, error) {
	return nil, domain.ErrNotListable
}
