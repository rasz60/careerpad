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
}

// NewRouter 는 Go 1.22 ServeMux 패턴 라우팅으로 전체 API 를 구성한다.
// Caddy 가 /api/career/* 의 접두어를 떼고 넘기므로 여기서는 /api/v1/... 로 받는다.
func NewRouter(d Deps) http.Handler {
	public := NewPublicHandler(d.Profile, d.Contact)
	me := NewMeHandler(d.Profile, d.Project, d.Job, d.Contact)

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
	protected("GET /api/v1/me/profile", me.Profile)
	protected("PUT /api/v1/me/profile", me.UpdateProfile)

	protected("GET /api/v1/me/projects", me.Projects)
	protected("POST /api/v1/me/projects", me.CreateProject)
	protected("PUT /api/v1/me/projects/{id}", me.UpdateProject)
	protected("DELETE /api/v1/me/projects/{id}", me.DeleteProject)

	protected("GET /api/v1/me/job-sites", me.JobSites)
	protected("GET /api/v1/me/job-searches", me.JobSearches)
	protected("POST /api/v1/me/job-searches", me.CreateJobSearch)
	protected("PUT /api/v1/me/job-searches/{id}", me.UpdateJobSearch)
	protected("DELETE /api/v1/me/job-searches/{id}", me.DeleteJobSearch)
	protected("GET /api/v1/me/job-searches/{id}/results", me.JobResults)

	protected("GET /api/v1/me/contact-settings", me.ContactSettings)
	protected("PUT /api/v1/me/contact-settings", me.SaveContactSettings)
	protected("GET /api/v1/me/messages", me.Messages)
	protected("PATCH /api/v1/me/messages/{id}", me.MarkMessage)
	protected("DELETE /api/v1/me/messages/{id}", me.DeleteMessage)

	return withLogging(withCORS(d.Config.AllowOrigin, mux))
}
