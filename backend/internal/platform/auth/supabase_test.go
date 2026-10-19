package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 가짜 Supabase: JWKS 를 내려주고, 요청 횟수를 센다
type fakeSupabase struct {
	srv   *httptest.Server
	key   *ecdsa.PrivateKey
	kid   string
	calls atomic.Int32
}

func newFakeSupabase(t *testing.T) *fakeSupabase {
	t.Helper()
	f := &fakeSupabase{kid: "key-1"}
	f.key = mustKey(t)
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		pub := f.key.PublicKey
		b, _ := pub.Bytes() // 0x04 || X || Y
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": f.kid, "kty": "EC", "crv": "P-256", "alg": "ES256",
			"x": base64.RawURLEncoding.EncodeToString(b[1:33]),
			"y": base64.RawURLEncoding.EncodeToString(b[33:]),
		}}})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func mustKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func (f *fakeSupabase) token(t *testing.T, key *ecdsa.PrivateKey, kid string, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    f.srv.URL + "/auth/v1",
			Audience:  jwt.ClaimStrings{"authenticated"},
			ExpiresAt: jwt.NewNumericDate(exp),
		},
		Email: "me@example.com",
	})
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVerifier(t *testing.T) {
	f := newFakeSupabase(t)
	v := NewVerifier(f.srv.URL)
	ctx := context.Background()
	soon := time.Now().Add(time.Hour)

	claims, err := v.Verify(ctx, f.token(t, f.key, f.kid, soon))
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if claims.Subject != "user-1" || claims.Email != "me@example.com" {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	// 키 목록은 캐시해서 두 번째 검증에는 JWKS 를 다시 받지 않는다
	if _, err := v.Verify(ctx, f.token(t, f.key, f.kid, soon)); err != nil || f.calls.Load() != 1 {
		t.Fatalf("expected cached jwks (calls=%d, err=%v)", f.calls.Load(), err)
	}

	if _, err := v.Verify(ctx, f.token(t, f.key, f.kid, time.Now().Add(-time.Minute))); err == nil {
		t.Fatal("expired token accepted")
	}
	if _, err := v.Verify(ctx, f.token(t, mustKey(t), f.kid, soon)); err == nil {
		t.Fatal("token signed by another key accepted")
	}
}

func TestVerifierKeyRotation(t *testing.T) {
	f := newFakeSupabase(t)
	v := NewVerifier(f.srv.URL)
	ctx := context.Background()
	soon := time.Now().Add(time.Hour)

	if _, err := v.Verify(ctx, f.token(t, f.key, f.kid, soon)); err != nil {
		t.Fatal(err)
	}

	// Supabase 에서 키를 바꿨다: 새 kid 를 처음 보면 JWKS 를 다시 받아야 한다
	f.key, f.kid = mustKey(t), "key-2"
	v.fetchedAt = time.Now().Add(-2 * jwksMinRefetch) // 최소 간격은 지났다고 친다
	if _, err := v.Verify(ctx, f.token(t, f.key, f.kid, soon)); err != nil {
		t.Fatalf("rotated key rejected: %v", err)
	}
}
