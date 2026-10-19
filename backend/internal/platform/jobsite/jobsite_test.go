package jobsite

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
	got, err := NewWanted(srv.Client(), srv.URL).Search(context.Background(), "golang")
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
	got, err := NewJumpit(srv.Client(), srv.URL).Search(context.Background(), "java")
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
	got, err := NewSaramin(srv.Client(), srv.URL, "key").Search(context.Background(), "go")
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
	if _, err := s.Search(context.Background(), "go"); !errors.Is(err, domain.ErrNotListable) {
		t.Errorf("err = %v, want ErrNotListable", err)
	}
}

func TestHTTPErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "blocked", http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	if _, err := NewJumpit(srv.Client(), srv.URL).Search(context.Background(), "go"); err == nil {
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
