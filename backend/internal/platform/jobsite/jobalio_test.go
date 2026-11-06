package jobsite

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

// 공공데이터포털 '공공기관 채용정보 조회서비스' 목록 응답 견본 (쓰는 필드만).
const jobalioFixture = `{"resultCode":200,"resultMsg":"성공했습니다.","totalCount":3,"result":[
	{"recrutPblntSn":306793,"instNm":"한전KDN","recrutPbancTtl":"2026년 하반기 신입사원 채용","workRgnNmLst":"전남광주통합특별시",
	 "recrutSe":"R2010","recrutSeNm":"신입","hireTypeNmLst":"정규직","ncsCdNmLst":"정보통신,전기·전자","pbancEndYmd":"20261120",
	 "srcUrl":"https://www.kdn.com/recruit","ongoingYn":"Y","replmprYn":"N"},
	{"recrutPblntSn":"306800","instNm":"한국전력공사","recrutPbancTtl":"ICT 경력직 채용","workRgnNmLst":"전남광주통합특별시,서울특별시",
	 "recrutSe":"R2020","recrutSeNm":"경력","hireTypeNmLst":"정규직","ncsCdNmLst":"정보통신","pbancEndYmd":"2026-11-30",
	 "srcUrl":"","ongoingYn":"Y","replmprYn":"Y"},
	{"recrutPblntSn":306801,"instNm":"마감된 기관","recrutPbancTtl":"지난 공고","workRgnNmLst":"서울특별시",
	 "recrutSe":"R2030","pbancEndYmd":"20261001","ongoingYn":"N"}
]}`

func TestJobalioSearch(t *testing.T) {
	var q url.Values
	srv := captureQuery(t, jobalioFixture, &q)
	got, err := NewJobalio(srv.Client(), srv.URL, "my-key").Search(context.Background(),
		domain.JobQuery{Keyword: "정보통신", Location: "광주, 전남"})
	if err != nil {
		t.Fatal(err)
	}

	// 키워드가 NCS 대분류 이름이면 제목이 아니라 NCS 분야로 찾는다. 광주 · 전남은 잡알리오에서 한 코드다
	if q.Get("ncsCdLst") != "R600020" || q.Has("recrutPbancTtl") {
		t.Errorf("ncs query = %v", q)
	}
	if q.Get("workRgnLst") != "R3027" || q.Get("ongoingYn") != "Y" || q.Get("serviceKey") != "my-key" || q.Get("resultType") != "json" {
		t.Errorf("query = %v", q)
	}

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (진행 중이 아닌 공고는 뺀다)", len(got))
	}
	p := got[0]
	if p.ID != "jobalio:306793" || p.Company != "한전KDN" || p.Deadline != "2026-11-20" || p.URL != "https://www.kdn.com/recruit" {
		t.Errorf("posting = %+v", p)
	}
	if p.Career != "신입" || p.CareerMin == nil || *p.CareerMin != 0 || p.CareerMax == nil || *p.CareerMax != 0 || !p.LocationChecked {
		t.Errorf("career / location = %q %v %v %v", p.Career, p.CareerMin, p.CareerMax, p.LocationChecked)
	}
	if !slices.Equal(p.Tags, []string{"정규직", "정보통신", "전기·전자"}) {
		t.Errorf("tags = %v", p.Tags)
	}
	second := got[1]
	if second.URL != "https://job.alio.go.kr/recruitview.do?idx=306800" || second.Deadline != "2026-11-30" {
		t.Errorf("원문 주소가 없으면 잡알리오 상세로: %+v", second)
	}
	if second.CareerMin == nil || *second.CareerMin != 1 || second.CareerMax != nil || !slices.Contains(second.Tags, "대체인력") {
		t.Errorf("second = %+v", second)
	}
	if second.Location != "전남광주통합특별시, 서울특별시" {
		t.Errorf("location = %q", second.Location)
	}
}

func TestJobalioTitleKeyword(t *testing.T) {
	var q url.Values
	srv := captureQuery(t, `{"resultCode":200,"result":[]}`, &q)
	got, err := NewJobalio(srv.Client(), srv.URL, "k").Search(context.Background(), domain.JobQuery{Keyword: "전산"})
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("recrutPbancTtl") != "전산" || q.Has("ncsCdLst") || q.Has("workRgnLst") {
		t.Errorf("query = %v", q)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("got = %v, want empty (not nil)", got)
	}
}

func TestJobalioNCSNameIgnoresDots(t *testing.T) {
	if code, ok := ncsCode("경영·회계·사무"); !ok || code != "R600002" {
		t.Errorf("경영·회계·사무 = %q %v", code, ok)
	}
	if _, ok := ncsCode("데이터센터"); ok {
		t.Error("NCS 이름이 아닌 키워드는 제목 검색")
	}
}

func TestJobalioGatewayError(t *testing.T) {
	var q url.Values
	body := `{"OpenAPI_ServiceResponse":{"cmmMsgHeader":{"errMsg":"SERVICE ERROR","returnAuthMsg":"SERVICE_KEY_IS_NOT_REGISTERED_ERROR","returnReasonCode":"30"}}}`
	srv := captureQuery(t, body, &q)
	_, err := NewJobalio(srv.Client(), srv.URL, "wrong").Search(context.Background(), domain.JobQuery{Keyword: "정보통신"})
	if err == nil || !strings.Contains(err.Error(), "SERVICE_KEY_IS_NOT_REGISTERED_ERROR") {
		t.Fatalf("err = %v", err)
	}
}

func TestJobalioEncodedKeyIsDecoded(t *testing.T) {
	var q url.Values
	srv := captureQuery(t, `{"resultCode":200,"result":[]}`, &q)
	if _, err := NewJobalio(srv.Client(), srv.URL, "ab%2Bcd%3D%3D").Search(context.Background(), domain.JobQuery{Keyword: "x"}); err != nil {
		t.Fatal(err)
	}
	if q.Get("serviceKey") != "ab+cd==" {
		t.Errorf("serviceKey = %q, want ab+cd== (Encoding 키를 넣어도 한 번만 이스케이프)", q.Get("serviceKey"))
	}
}

func TestJobalioWithoutKeyIsLinkOnly(t *testing.T) {
	s := NewJobalio(http.DefaultClient, "http://unused", "")
	if s.Site().Listable || s.Site().Group != domain.JobGroupPublic {
		t.Errorf("site = %+v", s.Site())
	}
	if _, err := s.Search(context.Background(), domain.JobQuery{Keyword: "x"}); !errors.Is(err, domain.ErrNotListable) {
		t.Errorf("err = %v", err)
	}
	if u := s.SearchURL(domain.JobQuery{Keyword: "정보통신"}); u != "https://job.alio.go.kr/recruit.do?keyword=%EC%A0%95%EB%B3%B4%ED%86%B5%EC%8B%A0" {
		t.Errorf("link = %s", u)
	}
}

func TestResolveRegionJobalio(t *testing.T) {
	cases := map[string][]string{
		"나주":     {"R3027"},
		"광주, 전남": {"R3027"},
		"서울, 경기": {"R3010", "R3017"},
		"판교":     nil,
	}
	for in, want := range cases {
		if got := resolveRegion(in).jobalio; !slices.Equal(got, want) {
			t.Errorf("resolveRegion(%q).jobalio = %v, want %v", in, got, want)
		}
	}
}

// 한 쪽(100건)이 꽉 차면 다음 쪽을 받는다 (최대 3쪽). 경력은 받은 뒤 거르므로 앞 쪽만으로는 빠지는 공고가 생긴다
func TestJobalioPages(t *testing.T) {
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("pageNo")
		pages = append(pages, page)
		n := jobalioPageSize
		if page == "2" {
			n = 3
		}
		rows := make([]string, n)
		for i := range rows {
			rows[i] = fmt.Sprintf(`{"recrutPblntSn":"%s-%d","instNm":"기관","recrutPbancTtl":"공고","ongoingYn":"Y"}`, page, i)
		}
		fmt.Fprintf(w, `{"resultCode":200,"totalCount":%d,"result":[%s]}`, jobalioPageSize+3, strings.Join(rows, ","))
	}))
	t.Cleanup(srv.Close)
	got, err := NewJobalio(srv.Client(), srv.URL, "k").Search(context.Background(), domain.JobQuery{Keyword: "정보통신"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != jobalioPageSize+3 || !slices.Equal(pages, []string{"1", "2"}) {
		t.Errorf("got %d postings from pages %v, want %d from [1 2]", len(got), pages, jobalioPageSize+3)
	}
}
