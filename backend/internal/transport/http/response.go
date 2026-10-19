package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("write json response", "error", err)
	}
}

// writeError 는 도메인 센티널 에러를 HTTP 상태 코드로 매핑한다.
// 검증·충돌 에러는 유스케이스가 만든 한국어 문장을 그대로 보여 주고, 나머지는 정해진 문장을 쓴다.
func writeError(w http.ResponseWriter, err error) {
	var (
		status   = http.StatusInternalServerError
		code     = "internal_error"
		message  = "요청을 처리할 수 없습니다."
		sentinel error
	)
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		status, code, sentinel = http.StatusBadRequest, "invalid_input", domain.ErrInvalidInput
		message = "입력값을 확인해 주세요."
	case errors.Is(err, domain.ErrUnauthorized):
		status, code, message = http.StatusUnauthorized, "unauthorized", "로그인이 필요합니다."
	case errors.Is(err, domain.ErrForbidden):
		status, code, message = http.StatusForbidden, "forbidden", "권한이 없습니다."
	case errors.Is(err, domain.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", "찾을 수 없습니다."
	case errors.Is(err, domain.ErrConflict):
		status, code, sentinel = http.StatusConflict, "conflict", domain.ErrConflict
		message = "이미 있는 값입니다."
	case errors.Is(err, domain.ErrTooManyRequests):
		status, code, sentinel = http.StatusTooManyRequests, "too_many_requests", domain.ErrTooManyRequests
		message = "잠시 뒤에 다시 시도해 주세요."
	}

	switch {
	case status >= http.StatusInternalServerError:
		slog.Error("request failed", "error", err)
	case sentinel != nil:
		if m := userMessage(err, sentinel); m != "" {
			message = m
		}
	default:
		slog.Info("request rejected", "status", status, "error", err)
	}
	writeJSON(w, status, errorBody{Error: code, Message: message})
}

// userMessage 는 "이름 을 입력해 주세요: invalid input" 에서 센티널 꼬리를 떼어 사용자 문장만 남긴다.
// 저장소 계층이 붙인 영문 문맥("project repository: ...")은 사용자 문장이 아니므로 버린다.
func userMessage(err error, sentinel error) string {
	m := strings.TrimSuffix(err.Error(), ": "+sentinel.Error())
	if m == err.Error() || strings.Contains(m, "repository:") || strings.Contains(m, "decode") {
		return ""
	}
	return m
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return errors.Join(domain.ErrInvalidInput, err)
	}
	return nil
}
