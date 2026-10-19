package http

import (
	"net/http"

	"github.com/devsixt/careerpad/backend/internal/usecase"
)

// PublicHandler 는 로그인 없이 여는 공개 페이지 API 다.
type PublicHandler struct {
	profile *usecase.Profile
	contact *usecase.Contact
}

func NewPublicHandler(profile *usecase.Profile, contact *usecase.Contact) *PublicHandler {
	return &PublicHandler{profile: profile, contact: contact}
}

// Page 는 GET /api/v1/public/{slug} — 프로필 + 프로젝트. 주인이 보면 비공개 항목까지.
func (h *PublicHandler) Page(w http.ResponseWriter, r *http.Request) {
	page, err := h.profile.Public(r.Context(), r.PathValue("slug"), userID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	// 손님에게는 잠깐 캐시해도 되지만, 주인은 고친 내용을 바로 봐야 한다
	if page.IsOwner {
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=60")
	}
	writeJSON(w, http.StatusOK, page)
}

// Contact 는 POST /api/v1/public/{slug}/contact — 방문자가 연락을 남긴다.
func (h *PublicHandler) Contact(w http.ResponseWriter, r *http.Request) {
	var in usecase.ContactInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	if err := h.contact.Send(r.Context(), r.PathValue("slug"), in, clientIP(r)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}
