package http

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/devsixt/careerpad/backend/internal/domain"
	"github.com/devsixt/careerpad/backend/internal/platform/auth"
)

type ctxKey string

const (
	ctxKeyUserID ctxKey = "user_id"
	ctxKeyEmail  ctxKey = "email"
)

// requireAuth 는 Supabase access token(Bearer)을 검증하고 사용자 id · 이메일을 컨텍스트에 심는다.
func requireAuth(verifier *auth.Verifier, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeError(w, fmt.Errorf("auth middleware: missing bearer token: %w", domain.ErrUnauthorized))
			return
		}
		claims, err := verifier.Verify(r.Context(), token)
		if err != nil {
			writeError(w, fmt.Errorf("auth middleware: %v: %w", err, domain.ErrUnauthorized))
			return
		}
		next(w, r.WithContext(withUser(r.Context(), claims)))
	}
}

// optionalAuth 는 공개 화면용이다. 토큰이 있고 맞으면 사용자를 심고, 없거나 틀려도 손님으로 통과시킨다.
func optionalAuth(verifier *auth.Verifier, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token, ok := bearerToken(r.Header.Get("Authorization")); ok {
			if claims, err := verifier.Verify(r.Context(), token); err == nil {
				r = r.WithContext(withUser(r.Context(), claims))
			}
		}
		next(w, r)
	}
}

func withUser(ctx context.Context, claims *auth.Claims) context.Context {
	ctx = context.WithValue(ctx, ctxKeyUserID, claims.Subject)
	return context.WithValue(ctx, ctxKeyEmail, claims.Email)
}

func userID(r *http.Request) string {
	id, _ := r.Context().Value(ctxKeyUserID).(string)
	return id
}

func userEmail(r *http.Request) string {
	email, _ := r.Context().Value(ctxKeyEmail).(string)
	return email
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return strings.TrimSpace(header[len(prefix):]), true
	}
	return "", false
}

// clientIP 는 방문자 IP 다. Caddy 가 같은 서버(루프백)에서 넘겨줄 때만 X-Forwarded-For 를 믿는다
// (Caddy 는 바깥에서 온 X-Forwarded-For 를 버리고 실제 접속 IP 를 넣는다).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	remote := net.ParseIP(host)
	if remote != nil && remote.IsLoopback() {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			first, _, _ := strings.Cut(fwd, ",")
			if ip := net.ParseIP(strings.TrimSpace(first)); ip != nil {
				return ip.String()
			}
		}
	}
	if remote == nil {
		return ""
	}
	return remote.String()
}

// withCORS 는 로컬 개발(Vite 개발 서버)용이다. 배포는 같은 도메인이라 필요 없다.
func withCORS(allowOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Vary", "Origin")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withLogging 은 요청 1건당 1줄을 남긴다.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(recorder, r)

		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
