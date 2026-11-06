package jobsite

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

// Jobalio 는 잡알리오(공공기관 채용정보) 검색이다. 공공데이터포털 '공공기관 채용정보 조회서비스'
// (data.go.kr 15125273) 목록 API 를 쓴다. 인증키(JOBALIO_SERVICE_KEY)가 없으면 검색 링크만 준다.
//
//   - 근무지는 시·도 단위 근무지역 코드(workRgnLst)로 넘긴다. 잡알리오는 시·군·구가 없어 시·도까지만 좁힌다.
//   - 공공기관 공고 제목은 '2026년 하반기 신입사원 채용'처럼 직무가 안 보일 때가 많다. 그래서 키워드가
//     NCS 대분류 이름(예: 정보통신)이면 제목 대신 NCS 분야(ncsCdLst)로 찾는다. 아니면 제목(recrutPbancTtl)에 든 공고.
//   - 경력은 채용구분(신입 · 경력 · 신입+경력)을 경력 범위로 바꿔 유스케이스가 거른다.
type Jobalio struct {
	client     *http.Client
	baseURL    string
	serviceKey string
}

func NewJobalio(client *http.Client, baseURL, serviceKey string) *Jobalio {
	// 포털의 'Encoding' 키(%2B 같은 이스케이프가 든 값)를 넣어도 되게 한 번 푼다. url.Values 가 다시 이스케이프한다.
	if strings.Contains(serviceKey, "%") {
		if k, err := url.QueryUnescape(serviceKey); err == nil {
			serviceKey = k
		}
	}
	return &Jobalio{client: client, baseURL: strings.TrimRight(baseURL, "/"), serviceKey: strings.TrimSpace(serviceKey)}
}

func (s *Jobalio) Site() domain.JobSite {
	return domain.JobSite{ID: "jobalio", Name: "잡알리오(공공기관)", Group: domain.JobGroupPublic, Listable: s.serviceKey != ""}
}

func (s *Jobalio) SearchURL(q domain.JobQuery) string {
	return "https://job.alio.go.kr/recruit.do?keyword=" + url.QueryEscape(q.Keyword)
}

// ncsCodes 는 NCS 대분류 이름 → 잡알리오 NCS 코드(job.alio.go.kr 검색 화면, 2026-11 확인)다.
var ncsCodes = map[string]string{
	"사업관리": "R600001", "경영회계사무": "R600002", "금융보험": "R600003", "교육자연사회과학": "R600004",
	"법률경찰소방교도국방": "R600005", "보건의료": "R600006", "사회복지종교": "R600007", "문화예술디자인방송": "R600008",
	"운전운송": "R600009", "영업판매": "R600010", "경비청소": "R600011", "이용숙박여행오락스포츠": "R600012",
	"음식서비스": "R600013", "건설": "R600014", "기계": "R600015", "재료": "R600016", "화학": "R600017",
	"섬유의복": "R600018", "전기전자": "R600019", "정보통신": "R600020", "식품가공": "R600021",
	"인쇄목재가구공예": "R600022", "환경에너지안전": "R600023", "농림어업": "R600024",
}

// ncsCode 는 키워드가 NCS 대분류 이름이면 그 코드를 준다 (띄어쓰기 · 가운뎃점 · 쉼표는 무시: '경영·회계·사무').
func ncsCode(keyword string) (string, bool) {
	key := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '·', '・', ',', '/', '.':
			return -1
		}
		return r
	}, keyword)
	code, ok := ncsCodes[key]
	return code, ok
}

// 채용구분 코드 (잡알리오 검색 화면): 신입 · 경력 · 신입+경력 · 외국인 전형
const (
	jobalioNewcomer   = "R2010"
	jobalioExperience = "R2020"
	jobalioBoth       = "R2030"
)

const (
	jobalioPageSize = 100
	jobalioMaxPages = 3
)

type jobalioRow struct {
	Sn         flexString `json:"recrutPblntSn"`
	InstNm     string     `json:"instNm"`
	Title      string     `json:"recrutPbancTtl"`
	WorkRegion string     `json:"workRgnNmLst"`
	RecrutSe   string     `json:"recrutSe"`
	RecrutSeNm string     `json:"recrutSeNm"`
	HireTypes  string     `json:"hireTypeNmLst"`
	NcsNames   string     `json:"ncsCdNmLst"`
	EndYmd     string     `json:"pbancEndYmd"`
	SrcURL     string     `json:"srcUrl"`
	OngoingYn  string     `json:"ongoingYn"`
	ReplmprYn  string     `json:"replmprYn"`
}

type jobalioResponse struct {
	ResultCode flexString   `json:"resultCode"`
	ResultMsg  string       `json:"resultMsg"`
	TotalCount int          `json:"totalCount"`
	Result     []jobalioRow `json:"result"`
	// 공공데이터포털 게이트웨이 오류 (인증키 없음 · 틀림 · 한도 초과)
	Gateway *struct {
		Header struct {
			ErrMsg     string `json:"errMsg"`
			AuthMsg    string `json:"returnAuthMsg"`
			ReasonCode string `json:"returnReasonCode"`
		} `json:"cmmMsgHeader"`
	} `json:"OpenAPI_ServiceResponse"`
}

func (s *Jobalio) Search(ctx context.Context, query domain.JobQuery) ([]domain.JobPosting, error) {
	if s.serviceKey == "" {
		return nil, domain.ErrNotListable
	}
	q := url.Values{
		"serviceKey": {s.serviceKey},
		"resultType": {"json"},
		"pageNo":     {"1"},
		"numOfRows":  {fmt.Sprint(jobalioPageSize)},
		"ongoingYn":  {"Y"},
	}
	if code, ok := ncsCode(query.Keyword); ok {
		q.Set("ncsCdLst", code)
	} else {
		q.Set("recrutPbancTtl", query.Keyword)
	}
	reg := resolveRegion(query.Location)
	if len(reg.jobalio) > 0 {
		q.Set("workRgnLst", strings.Join(reg.jobalio, ","))
	}

	// 채용구분(신입 · 경력)은 목록 API 가 한 값만 받아 '신입 + 신입+경력' 같은 조건을 못 건다.
	// 그래서 경력은 받은 뒤 거르고, 대신 쪽을 넘겨 최대 jobalioMaxPages 쪽까지 받는다.
	var rows []jobalioRow
	for page := 1; page <= jobalioMaxPages; page++ {
		q.Set("pageNo", fmt.Sprint(page))
		var body jobalioResponse
		if err := getJSON(ctx, s.client, s.baseURL+"/list?"+q.Encode(), nil, &body); err != nil {
			if page > 1 && len(rows) > 0 {
				break // 앞 쪽은 받았으니 그만큼이라도 보여 준다
			}
			return nil, err
		}
		if g := body.Gateway; g != nil {
			return nil, fmt.Errorf("jobsite: 잡알리오 인증 오류 %s: %s", g.Header.ReasonCode, strings.TrimSpace(g.Header.AuthMsg+" "+g.Header.ErrMsg))
		}
		if code := string(body.ResultCode); code != "" && code != "200" && code != "00" {
			return nil, fmt.Errorf("jobsite: 잡알리오 %s: %s", code, body.ResultMsg)
		}
		rows = append(rows, body.Result...)
		if len(body.Result) < jobalioPageSize || len(rows) >= body.TotalCount {
			break
		}
	}

	out := make([]domain.JobPosting, 0, len(rows))
	for _, r := range rows {
		if r.OngoingYn == "N" {
			continue
		}
		var min, max *int
		switch r.RecrutSe {
		case jobalioNewcomer:
			min, max = new(0), new(0)
		case jobalioExperience:
			min = new(1)
		case jobalioBoth:
			min = new(0)
		}
		career := r.RecrutSeNm
		if career == "" {
			career = careerText(min, max)
		}
		tags := splitKeywords(r.HireTypes)
		for _, n := range splitKeywords(r.NcsNames) {
			if len(tags) >= 6 {
				break
			}
			tags = append(tags, n)
		}
		if r.ReplmprYn == "Y" {
			tags = append(tags, "대체인력")
		}
		link := r.SrcURL
		if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
			link = "https://job.alio.go.kr/recruitview.do?idx=" + url.QueryEscape(string(r.Sn))
		}
		out = append(out, domain.JobPosting{
			ID:        "jobalio:" + string(r.Sn),
			Title:     strings.TrimSpace(r.Title),
			Company:   strings.TrimSpace(r.InstNm),
			Location:  strings.ReplaceAll(r.WorkRegion, ",", ", "),
			Career:    career,
			CareerMin: min,
			CareerMax: max,
			Deadline:  ymd(r.EndYmd),
			URL:       link,
			Tags:      tags,

			// 근무지역 코드로 사이트가 이미 걸렀다 (시·도 단위)
			LocationChecked: len(reg.jobalio) > 0,
		})
	}
	return out, nil
}

// ymd 는 "20261120" · "2026-11-20" · "2026.11.20" 을 YYYY-MM-DD 로 바꾼다. 모르면 빈 값(상시).
func ymd(s string) string {
	d := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	if len(d) < 8 {
		return ""
	}
	return d[:4] + "-" + d[4:6] + "-" + d[6:8]
}
