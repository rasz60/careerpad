// Package jobsite 는 채용 사이트별 공고 검색 어댑터(domain.JobSource)다.
//
// 목록을 가져오는 사이트: 원티드 · 점핏(공개 JSON), 사람인(공식 Open API, 접근 키가 있을 때),
// 잡알리오(공공데이터포털 공식 API, 인증키가 있을 때).
// 나머지(잡코리아 · 리멤버 · 링크드인 · SK · 삼성 · LG, 키 없는 사람인 · 잡알리오)는 사이트로 가는 링크만 준다.
// 기업 채용 사이트는 공개 API 가 없고 SK 는 robots.txt 가 검색 주소 수집을 막으므로 서버가 긁지 않는다.
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
	// JobalioServiceKey 는 공공데이터포털 '공공기관 채용정보 조회서비스' 인증키다 (일반 인증키).
	JobalioServiceKey string
}

// Sources 는 화면에 보일 순서대로 사이트 어댑터를 만든다.
func Sources(opts Options) []domain.JobSource {
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	const corpNote = "이 사이트는 검색어를 주소로 받지 않아요. 채용 화면에서 키워드를 입력해 주세요."
	return []domain.JobSource{
		NewWanted(client, "https://www.wanted.co.kr"),
		NewJumpit(client, "https://jumpit-api.saramin.co.kr"),
		NewSaramin(client, "https://oapi.saramin.co.kr", opts.SaraminAccessKey),
		NewLinkOnly(domain.JobSite{ID: "jobkorea", Name: "잡코리아", Group: domain.JobGroupBoard}, "https://www.jobkorea.co.kr/Search/?stext="),
		NewFixedLink(domain.JobSite{ID: "remember", Name: "리멤버", Group: domain.JobGroupBoard}, "https://career.rememberapp.co.kr/job/postings", corpNote),
		NewLinkOnly(domain.JobSite{ID: "linkedin", Name: "LinkedIn", Group: domain.JobGroupBoard}, "https://www.linkedin.com/jobs/search/?location=South%20Korea&keywords="),
		NewJobalio(client, "https://apis.data.go.kr/1051000/recruitment", opts.JobalioServiceKey),
		NewLinkOnly(domain.JobSite{ID: "sk", Name: "SK Careers", Group: domain.JobGroupCorp}, "https://www.skcareers.com/Recruit?searchText="),
		NewFixedLink(domain.JobSite{ID: "samsung", Name: "삼성 채용", Group: domain.JobGroupCorp}, "https://www.samsungcareers.com/hr/", corpNote),
		NewFixedLink(domain.JobSite{ID: "lg", Name: "LG 채용", Group: domain.JobGroupCorp}, "https://careers.lg.com/apply", corpNote),
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

func (s *linkOnly) SearchURL(q domain.JobQuery) string { return s.prefix + url.QueryEscape(q.Keyword) }

func (s *linkOnly) Search(context.Context, domain.JobQuery) ([]domain.JobPosting, error) {
	return nil, domain.ErrNotListable
}

// fixedLink 는 검색어를 주소로 받지 않는 사이트다. 채용 첫 화면 하나와 안내만 준다 (domain.FixedLinker).
type fixedLink struct {
	site domain.JobSite
	url  string
	note string
}

// NewFixedLink 는 키워드와 상관없이 같은 주소로 가는 어댑터를 만든다.
func NewFixedLink(site domain.JobSite, url, note string) domain.JobSource {
	site.Listable = false
	return &fixedLink{site: site, url: url, note: note}
}

func (s *fixedLink) Site() domain.JobSite { return s.site }

func (s *fixedLink) SearchURL(domain.JobQuery) string { return s.url }

func (s *fixedLink) FixedLink() (string, string) { return s.url, s.note }

func (s *fixedLink) Search(context.Context, domain.JobQuery) ([]domain.JobPosting, error) {
	return nil, domain.ErrNotListable
}
