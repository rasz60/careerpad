package http

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/devsixt/careerpad/backend/internal/usecase"
)

// RoadmapHandler 는 개인 로드맵 API 다 (/api/v1/me/roadmaps/...). 모두 로그인한 주인만.
type RoadmapHandler struct {
	uc *usecase.Roadmap
}

func NewRoadmapHandler(uc *usecase.Roadmap) *RoadmapHandler {
	return &RoadmapHandler{uc: uc}
}

type markdownBody struct {
	Markdown string `json:"markdown"`
}

func (h *RoadmapHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.uc.List(r.Context(), userID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// Preview 는 POST /me/roadmaps/preview — md 를 읽기만 하고 결과 · 경고 · 오류를 돌려준다.
func (h *RoadmapHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var in markdownBody
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.uc.Preview(in.Markdown))
}

// Create 는 POST /me/roadmaps — md 로 로드맵을 만든다.
func (h *RoadmapHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in markdownBody
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	d, err := h.uc.Import(r.Context(), userID(r), in.Markdown)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

func (h *RoadmapHandler) Get(w http.ResponseWriter, r *http.Request) {
	d, err := h.uc.Get(r.Context(), userID(r), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *RoadmapHandler) Update(w http.ResponseWriter, r *http.Request) {
	var in usecase.RoadmapInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	rm, err := h.uc.Update(r.Context(), userID(r), r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rm)
}

func (h *RoadmapHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.uc.Delete(r.Context(), userID(r), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Markdown 은 GET /me/roadmaps/{id}/markdown — md 파일로 내려받는다 (파일 이름 = 제목).
func (h *RoadmapHandler) Markdown(w http.ResponseWriter, r *http.Request) {
	title, md, err := h.uc.Markdown(r.Context(), userID(r), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	name := strings.Map(func(c rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, c) || c < ' ' {
			return '_'
		}
		return c
	}, title) + ".md"
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="roadmap.md"; filename*=UTF-8''`+url.PathEscape(name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(md))
}

func (h *RoadmapHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
	var in usecase.ItemInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	it, err := h.uc.AddItem(r.Context(), userID(r), r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, it)
}

// PatchItem 은 PATCH /me/roadmaps/{id}/items/{itemId} — 보낸 칸만 바꾼다 (체크는 {"done": true}).
func (h *RoadmapHandler) PatchItem(w http.ResponseWriter, r *http.Request) {
	var in usecase.ItemInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	it, err := h.uc.PatchItem(r.Context(), userID(r), r.PathValue("id"), r.PathValue("itemId"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func (h *RoadmapHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	if err := h.uc.DeleteItem(r.Context(), userID(r), r.PathValue("id"), r.PathValue("itemId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RoadmapHandler) CreateHabit(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	hb, err := h.uc.AddHabit(r.Context(), userID(r), r.PathValue("id"), in.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, hb)
}

func (h *RoadmapHandler) DeleteHabit(w http.ResponseWriter, r *http.Request) {
	if err := h.uc.DeleteHabit(r.Context(), userID(r), r.PathValue("id"), r.PathValue("habitId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CheckWeek 는 PUT(체크) · DELETE(해제) /me/roadmaps/{id}/habits/{habitId}/weeks/{week}.
func (h *RoadmapHandler) CheckWeek(w http.ResponseWriter, r *http.Request) {
	hb, err := h.uc.SetHabitWeek(r.Context(), userID(r), r.PathValue("id"), r.PathValue("habitId"), r.PathValue("week"), r.Method == http.MethodPut)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, hb)
}
