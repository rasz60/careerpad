package http

import (
	"net/http"

	"github.com/devsixt/careerpad/backend/internal/usecase"
)

// blockTester 는 테스터 계정(P-0004)이면 막은 기능으로 거절한다 (403 tester_blocked). requireAuth 안쪽에 둔다.
func blockTester(tester *usecase.Tester, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := tester.Guard(r.Context(), userID(r)); err != nil {
			writeError(w, err)
			return
		}
		next(w, r)
	}
}
