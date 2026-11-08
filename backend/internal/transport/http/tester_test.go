package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devsixt/careerpad/backend/internal/domain"
	"github.com/devsixt/careerpad/backend/internal/usecase"
)

type fakeTesterRepo struct{ testers map[string]bool }

func (f fakeTesterRepo) IsTester(_ context.Context, id string) (bool, error) {
	return f.testers[id], nil
}
func (f fakeTesterRepo) Reset(context.Context, string) (int, error) { return 0, nil }

func TestBlockTester(t *testing.T) {
	tester := usecase.NewTester(fakeTesterRepo{testers: map[string]bool{"tester": true}})
	called := false
	h := blockTester(tester, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	for _, tc := range []struct {
		user       string
		wantStatus int
		wantCalled bool
	}{
		{"tester", http.StatusForbidden, false},
		{"member", http.StatusOK, true},
	} {
		called = false
		req := httptest.NewRequest(http.MethodPost, "/api/v1/me/messages", nil)
		req = req.WithContext(context.WithValue(req.Context(), ctxKeyUserID, tc.user))
		rec := httptest.NewRecorder()
		h(rec, req)

		if rec.Code != tc.wantStatus || called != tc.wantCalled {
			t.Errorf("%s: status=%d called=%v", tc.user, rec.Code, called)
		}
		if tc.wantStatus == http.StatusForbidden {
			var body errorBody
			_ = json.Unmarshal(rec.Body.Bytes(), &body)
			if body.Error != "tester_blocked" || body.Message != testerBlockedMessage {
				t.Errorf("body = %+v", body)
			}
		}
	}
}

func TestWriteErrorTesterLocked(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, wrap("roadmap repository: patch item", domain.ErrTesterLocked))
	var body errorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusForbidden || body.Error != "tester_locked" || body.Message != testerLockedMessage {
		t.Errorf("status=%d body=%+v", rec.Code, body)
	}
}

func wrap(op string, err error) error { return &opError{op: op, err: err} }

type opError struct {
	op  string
	err error
}

func (e *opError) Error() string { return e.op + ": " + e.err.Error() }
func (e *opError) Unwrap() error { return e.err }
