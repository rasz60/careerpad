package jobsite

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

// 실제 응답(2026-10 확인)에서 쓰는 필드만 줄여 둔 견본.
const (
	wantedFixture = `{"total_count":2,"data":[
		{"id":279560,"position":"Backend Team Lead (Go/Java)","company":{"id":1422,"name":"센트비(Sentbe)"},"annual_from":6,"annual_to":100},
		{"id":392395,"position":"블록체인 개발","company":{"name":"미투온"},"annual_from":5,"annual_to":20}
	]}`
	jumpitFixture = `{"result":{"positions":[
		{"id":55299862,"title":"정보보호 경력직","companyName":"디어유","techStacks":["AWS","<span>Java</span>Script"],
		 "newcomer":false,"minCareer":5,"maxCareer":10,"locations":["서울 강남구"],"alwaysOpen":false,"closedAt":"2026-11-16T23:59:59"}
	]}}`
	saraminFixture = `{"jobs":{"count":1,"job":[
		{"id":"48123","url":"https://www.saramin.co.kr/zf_user/jobs/relay/view?rec_idx=48123",
		 "company":{"detail":{"name":"(주)사람&amp;인"}},
		 "position":{"title":"Go 백엔드","location":{"name":"서울 &gt; 강남구"},"experience-level":{"code":2,"min":3,"max":0,"name":"경력 3년↑"}},
		 "keyword":"Go, 백엔드","expiration-timestamp":1795000000}
	]}}`
)

func serve(t *testing.T, path, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestWantedSearch(t *testing.T) {
	srv := serve(t, "/api/chaos/search/v1/position", wantedFixture)
	got, err := NewWanted(srv.Client(), srv.URL).Search(context.Background(), domain.JobQuery{Keyword: "golang"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	p := got[0]
	if p.ID != "wanted:279560" || p.Company != "센트비(Sentbe)" || p.URL != "https://www.wanted.co.kr/wd/279560" {
		t.Errorf("posting = %+v", p)
	}
	if p.CareerMin == nil || *p.CareerMin != 6 || p.CareerMax != nil || p.Career != "6년 이상" {
		t.Errorf("career = %v %v %q, want 6 / nil (100 = 상한 없음) / 6년 이상", p.CareerMin, p.CareerMax, p.Career)
	}
	if got[1].Career != "5~20년" {
		t.Errorf("second career = %q, want 5~20년", got[1].Career)
	}
}

func TestJumpitSearch(t *testing.T) {
	srv := serve(t, "/api/positions", jumpitFixture)
	got, err := NewJumpit(srv.Client(), srv.URL).Search(context.Background(), domain.JobQuery{Keyword: "java"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	p := got[0]
	if p.Deadline != "2026-11-16" || p.Career != "5~10년" || p.Tags[1] != "JavaScript" {
		t.Errorf("posting = %+v", p)
	}
}

func TestSaraminSearch(t *testing.T) {
	srv := serve(t, "/job-search", saraminFixture)
	got, err := NewSaramin(srv.Client(), srv.URL, "key").Search(context.Background(), domain.JobQuery{Keyword: "go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	p := got[0]
	if p.Company != "(주)사람&인" || p.Location != "서울 강남구" || p.CareerMin == nil || *p.CareerMin != 3 || p.CareerMax != nil {
		t.Errorf("posting = %+v", p)
	}
	if p.Deadline == "" || len(p.Tags) != 2 {
		t.Errorf("deadline/tags = %q %v", p.Deadline, p.Tags)
	}
}

func TestSaraminWithoutKeyIsLinkOnly(t *testing.T) {
	s := NewSaramin(http.DefaultClient, "http://unused", "")
	if s.Site().Listable {
		t.Error("without access key saramin should not be listable")
	}
	if _, err := s.Search(context.Background(), domain.JobQuery{Keyword: "go"}); !errors.Is(err, domain.ErrNotListable) {
		t.Errorf("err = %v, want ErrNotListable", err)
	}
}

func TestHTTPErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "blocked", http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	if _, err := NewJumpit(srv.Client(), srv.URL).Search(context.Background(), domain.JobQuery{Keyword: "go"}); err == nil {
		t.Fatal("want error on HTTP 403")
	}
}

func TestCareerText(t *testing.T) {
	cases := []struct {
		min, max *int
		want     string
	}{
		{nil, nil, "경력 무관"},
		{new(0), new(0), "신입"},
		{new(0), new(3), "신입~3년"},
		{new(2), new(5), "2~5년"},
		{new(0), nil, "신입 이상"},
		{new(7), nil, "7년 이상"},
		{nil, new(4), "4년 이하"},
	}
	for _, c := range cases {
		if got := careerText(c.min, c.max); got != c.want {
			t.Errorf("careerText(%v, %v) = %q, want %q", c.min, c.max, got, c.want)
		}
	}
}

func TestResolveRegion(t *testing.T) {
	cases := []struct {
		in      string
		wanted  []string
		saramin []string
	}{
		{"", nil, nil},
		{"나주", []string{"s-jeolla.naju-si"}, []string{"112000"}},
		{"전라남도 나주시", []string{"s-jeolla.naju-si"}, []string{"112000"}},
		{"서울", []string{"seoul.all"}, []string{"101000"}},
		{"서울특별시", []string{"seoul.all"}, []string{"101000"}},
		{"서울, 경기", []string{"seoul.all", "gyeonggi.all"}, []string{"101000", "102000"}},
		{"서울 강서구", []string{"seoul.gangseo-gu"}, []string{"101000"}},
		{"강서구", []string{"seoul.gangseo-gu", "busan.gangseo-gu"}, []string{"101000", "106000"}},
		// 시·도 이름이 먼저: 광주는 광주광역시, 경기 광주시는 '경기 광주'
		{"광주", []string{"gwangju.all"}, []string{"103000"}},
		{"경기 광주", []string{"gyeonggi.gwangju-si"}, []string{"102000"}},
		// 점핏의 통합 표기
		{"전남광주", []string{"gwangju.all", "s-jeolla.all"}, []string{"103000", "112000"}},
		{"판교", nil, nil},
	}
	for _, c := range cases {
		got := resolveRegion(c.in)
		if !slices.Equal(got.wanted, c.wanted) || !slices.Equal(got.saramin, c.saramin) {
			t.Errorf("resolveRegion(%q) = %v %v, want %v %v", c.in, got.wanted, got.saramin, c.wanted, c.saramin)
		}
	}
	if r := resolveRegion("나주"); !r.matches("전남광주 나주시") || r.matches("전남광주 목포시") {
		t.Error("나주 should match '전남광주 나주시' only")
	}
	if r := resolveRegion("전남"); !r.matches("전남광주 목포시") || r.matches("서울 강남구") {
		t.Error("전남 should match 점핏 '전남광주' 표기")
	}
}

// captureQuery 는 요청 주소의 쿼리를 받아 두고 body 를 돌려주는 서버다.
func captureQuery(t *testing.T, body string, got *url.Values) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestWantedSendsLocationAndYears(t *testing.T) {
	var q url.Values
	srv := captureQuery(t, wantedFixture, &q)
	got, err := NewWanted(srv.Client(), srv.URL).Search(context.Background(), domain.JobQuery{Keyword: "개발", Location: "나주", CareerYears: new(4)})
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("locations") != "s-jeolla.naju-si" || q.Get("years") != "4" {
		t.Errorf("query = %v", q)
	}
	if !got[0].LocationChecked {
		t.Error("wanted filtered by region: postings should be LocationChecked")
	}

	// 못 알아본 근무지는 걸지 않고, 유스케이스가 거르도록 남긴다
	got, _ = NewWanted(srv.Client(), srv.URL).Search(context.Background(), domain.JobQuery{Keyword: "개발", Location: "판교"})
	if q.Has("locations") || got[0].LocationChecked {
		t.Errorf("unknown location: query = %v, checked = %v", q, got[0].LocationChecked)
	}
}

func TestJumpitFiltersDistrict(t *testing.T) {
	const body = `{"result":{"totalCount":2,"positions":[
		{"id":1,"title":"A","companyName":"가","locations":["전남광주 나주시"],"alwaysOpen":true},
		{"id":2,"title":"B","companyName":"나","locations":["전남광주 광산구"],"alwaysOpen":true}
	]}}`
	var q url.Values
	srv := captureQuery(t, body, &q)
	got, err := NewJumpit(srv.Client(), srv.URL).Search(context.Background(), domain.JobQuery{Keyword: "개발", Location: "나주", CareerYears: new(4)})
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("locationTag") != "112000" || q.Get("career") != "4" {
		t.Errorf("query = %v", q)
	}
	if len(got) != 1 || got[0].Location != "전남광주 나주시" || !got[0].LocationChecked {
		t.Errorf("postings = %+v", got)
	}
}
