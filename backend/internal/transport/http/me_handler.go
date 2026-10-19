package http

import (
	"net/http"

	"github.com/devsixt/careerpad/backend/internal/domain"
	"github.com/devsixt/careerpad/backend/internal/usecase"
)

// MeHandler 는 로그인한 주인이 자기 페이지를 고치는 API 다 (/api/v1/me/...).
type MeHandler struct {
	profile *usecase.Profile
	project *usecase.Project
	job     *usecase.Job
	contact *usecase.Contact
}

func NewMeHandler(profile *usecase.Profile, project *usecase.Project, job *usecase.Job, contact *usecase.Contact) *MeHandler {
	return &MeHandler{profile: profile, project: project, job: job, contact: contact}
}

// ── 프로필 ───────────────────────────────

// Profile 은 GET /me/profile — 처음이면 프로필을 만든다 (공개 주소는 이메일 아이디).
func (h *MeHandler) Profile(w http.ResponseWriter, r *http.Request) {
	p, err := h.profile.Mine(r.Context(), userID(r), userEmail(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *MeHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var in domain.Profile
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	p, err := h.profile.Update(r.Context(), userID(r), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// ── 프로젝트 ─────────────────────────────

func (h *MeHandler) Projects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.project.List(r.Context(), userID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (h *MeHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var in domain.Project
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	p, err := h.project.Create(r.Context(), userID(r), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *MeHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	var in domain.Project
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	p, err := h.project.Update(r.Context(), userID(r), r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *MeHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	if err := h.project.Delete(r.Context(), userID(r), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── job++ ────────────────────────────────

func (h *MeHandler) JobSites(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.job.Sites())
}

func (h *MeHandler) JobSearches(w http.ResponseWriter, r *http.Request) {
	searches, err := h.job.List(r.Context(), userID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, searches)
}

func (h *MeHandler) CreateJobSearch(w http.ResponseWriter, r *http.Request) {
	var in domain.JobSearch
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	s, err := h.job.Create(r.Context(), userID(r), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (h *MeHandler) UpdateJobSearch(w http.ResponseWriter, r *http.Request) {
	var in domain.JobSearch
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	s, err := h.job.Update(r.Context(), userID(r), r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *MeHandler) DeleteJobSearch(w http.ResponseWriter, r *http.Request) {
	if err := h.job.Delete(r.Context(), userID(r), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// JobResults 는 GET /me/job-searches/{id}/results — 저장한 조건으로 사이트별 공고를 모은다.
func (h *MeHandler) JobResults(w http.ResponseWriter, r *http.Request) {
	results, err := h.job.Results(r.Context(), userID(r), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// ── 연락 ─────────────────────────────────

type contactSettingsResponse struct {
	*domain.ContactSettings
	// Available 은 서버에 실제로 설정된 발송 수단이다 (mail, sms).
	Available map[string]bool `json:"available"`
}

func (h *MeHandler) ContactSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.contact.Settings(r.Context(), userID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, contactSettingsResponse{ContactSettings: s, Available: h.contact.Channels()})
}

func (h *MeHandler) SaveContactSettings(w http.ResponseWriter, r *http.Request) {
	var in domain.ContactSettings
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	s, err := h.contact.SaveSettings(r.Context(), userID(r), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, contactSettingsResponse{ContactSettings: s, Available: h.contact.Channels()})
}

func (h *MeHandler) Messages(w http.ResponseWriter, r *http.Request) {
	messages, err := h.contact.Messages(r.Context(), userID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

type markReadRequest struct {
	IsRead bool `json:"is_read"`
}

func (h *MeHandler) MarkMessage(w http.ResponseWriter, r *http.Request) {
	var in markReadRequest
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	if err := h.contact.MarkRead(r.Context(), userID(r), r.PathValue("id"), in.IsRead); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *MeHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	if err := h.contact.DeleteMessage(r.Context(), userID(r), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
