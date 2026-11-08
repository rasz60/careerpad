package http

import (
	"net/http"

	"github.com/devsixt/careerpad/backend/internal/config"
	"github.com/devsixt/careerpad/backend/internal/platform/auth"
	"github.com/devsixt/careerpad/backend/internal/usecase"
)

// Deps 는 라우터가 필요로 하는 유스케이스 묶음이다.
type Deps struct {
	Config   *config.Config
	Verifier *auth.Verifier
	Profile  *usecase.Profile
	Project  *usecase.Project
	Job      *usecase.Job
	Contact  *usecase.Contact
	Roadmap  *usecase.Roadmap
	Tester   *usecase.Tester
}

// NewRouter 는 Go 1.22 ServeMux 패턴 라우팅으로 전체 API 를 구성한다.
// Caddy 가 /api/career/* 의 접두어를 떼고 넘기므로 여기서는 /api/v1/... 로 받는다.
func NewRouter(d Deps) http.Handler {
	public := NewPublicHandler(d.Profile, d.Contact)
	me := NewMeHandler(d.Profile, d.Project, d.Job, d.Contact, d.Tester)
	roadmap := NewRoadmapHandler(d.Roadmap)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// 로그인 없이: 공개 페이지 · 연락 남기기 (주인이 보면 비공개 항목까지)
	mux.HandleFunc("GET /api/v1/public/{slug}", optionalAuth(d.Verifier, public.Page))
	mux.HandleFunc("POST /api/v1/public/{slug}/contact", public.Contact)

	// 주인만
	protected := func(pattern string, handler http.HandlerFunc) {
		mux.HandleFunc(pattern, requireAuth(d.Verifier, handler))
	}
	// 테스터 계정(P-0004)에게 막은 기능: 채용공고 · 메시지 · 메시지 알림 설정
	blocked := func(pattern string, handler http.HandlerFunc) {
		protected(pattern, blockTester(d.Tester, handler))
	}
	protected("GET /api/v1/me/profile", me.Profile)
	protected("PUT /api/v1/me/profile", me.UpdateProfile)

	protected("GET /api/v1/me/projects", me.Projects)
	protected("POST /api/v1/me/projects", me.CreateProject)
	protected("PUT /api/v1/me/projects/{id}", me.UpdateProject)
	protected("DELETE /api/v1/me/projects/{id}", me.DeleteProject)

	blocked("GET /api/v1/me/job-sites", me.JobSites)
	blocked("GET /api/v1/me/job-searches", me.JobSearches)
	blocked("POST /api/v1/me/job-searches", me.CreateJobSearch)
	blocked("PUT /api/v1/me/job-searches/{id}", me.UpdateJobSearch)
	blocked("DELETE /api/v1/me/job-searches/{id}", me.DeleteJobSearch)
	blocked("GET /api/v1/me/job-searches/{id}/results", me.JobResults)

	blocked("GET /api/v1/me/contact-settings", me.ContactSettings)
	blocked("PUT /api/v1/me/contact-settings", me.SaveContactSettings)
	blocked("GET /api/v1/me/messages", me.Messages)
	blocked("PATCH /api/v1/me/messages/{id}", me.MarkMessage)
	blocked("DELETE /api/v1/me/messages/{id}", me.DeleteMessage)

	// 포털이 테스터 로그아웃 직전에 부른다. 테스터가 아니면 아무것도 지우지 않는다
	protected("POST /api/v1/me/tester/reset", me.ResetTester)

	// 개인 로드맵 (md 가져오기 · 체크 · 매주 습관)
	protected("GET /api/v1/me/roadmaps", roadmap.List)
	protected("POST /api/v1/me/roadmaps/preview", roadmap.Preview)
	protected("POST /api/v1/me/roadmaps", roadmap.Create)
	protected("GET /api/v1/me/roadmaps/{id}", roadmap.Get)
	protected("PUT /api/v1/me/roadmaps/{id}", roadmap.Update)
	protected("DELETE /api/v1/me/roadmaps/{id}", roadmap.Delete)
	protected("GET /api/v1/me/roadmaps/{id}/markdown", roadmap.Markdown)
	protected("POST /api/v1/me/roadmaps/{id}/items", roadmap.CreateItem)
	protected("PATCH /api/v1/me/roadmaps/{id}/items/{itemId}", roadmap.PatchItem)
	protected("DELETE /api/v1/me/roadmaps/{id}/items/{itemId}", roadmap.DeleteItem)
	protected("POST /api/v1/me/roadmaps/{id}/habits", roadmap.CreateHabit)
	protected("DELETE /api/v1/me/roadmaps/{id}/habits/{habitId}", roadmap.DeleteHabit)
	protected("PUT /api/v1/me/roadmaps/{id}/habits/{habitId}/weeks/{week}", roadmap.CheckWeek)
	protected("DELETE /api/v1/me/roadmaps/{id}/habits/{habitId}/weeks/{week}", roadmap.CheckWeek)

	return withLogging(withCORS(d.Config.AllowOrigin, mux))
}
