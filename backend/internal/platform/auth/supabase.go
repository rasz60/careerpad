package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 로그인은 ae:p 포털(Supabase Auth)이 한다. 이 백엔드는 Supabase 가 발급한 access token 을 검증만 한다.
// 서명 키는 프로젝트의 공개키 목록(JWKS)에서 kid 로 고른다. 키를 바꾸면(로테이션) 모르는 kid 를 만났을 때 다시 받는다.

const (
	jwksMaxAge     = time.Hour   // 이보다 오래된 키 목록은 새로 받는다
	jwksMinRefetch = time.Minute // 모르는 kid 로 계속 두드려도 이 간격보다 자주 받지 않는다
)

// Claims 는 Supabase access token 에서 쓰는 값이다.
type Claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
}

type Verifier struct {
	jwksURL string
	issuer  string
	client  *http.Client

	mu        sync.RWMutex
	keys      map[string]any
	fetchedAt time.Time
}

// NewVerifier 는 Supabase 프로젝트 주소(https://<ref>.supabase.co)로 검증기를 만든다.
func NewVerifier(supabaseURL string) *Verifier {
	base := strings.TrimRight(supabaseURL, "/")
	return &Verifier{
		jwksURL: base + "/auth/v1/.well-known/jwks.json",
		issuer:  base + "/auth/v1",
		client:  &http.Client{Timeout: 10 * time.Second},
		keys:    map[string]any{},
	}
}

// Verify 는 서명·발급자·대상·만료를 확인하고 클레임을 돌려준다. sub 가 사용자 id(auth.users.id)다.
func (v *Verifier) Verify(ctx context.Context, token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(t *jwt.Token) (any, error) {
			kid, _ := t.Header["kid"].(string)
			return v.key(ctx, kid)
		},
		jwt.WithValidMethods([]string{"ES256", "RS256"}),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience("authenticated"),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("auth: verify token: %w", err)
	}
	if claims.Subject == "" {
		return nil, errors.New("auth: token has no subject")
	}
	return claims, nil
}

func (v *Verifier) key(ctx context.Context, kid string) (any, error) {
	v.mu.RLock()
	k, ok := v.keys[kid]
	age := time.Since(v.fetchedAt)
	fetched := !v.fetchedAt.IsZero()
	v.mu.RUnlock()

	if ok && age < jwksMaxAge {
		return k, nil
	}
	if !ok && fetched && age < jwksMinRefetch {
		return nil, fmt.Errorf("auth: unknown key id %q", kid)
	}
	if err := v.refresh(ctx); err != nil {
		if ok {
			return k, nil // 받아오기에 실패해도 이미 아는 키는 계속 쓴다
		}
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("auth: unknown key id %q", kid)
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (v *Verifier) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("auth: jwks request: %w", err)
	}
	res, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("auth: fetch jwks: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("auth: fetch jwks: HTTP %d", res.StatusCode)
	}

	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(res.Body).Decode(&set); err != nil {
		return fmt.Errorf("auth: decode jwks: %w", err)
	}

	keys := make(map[string]any, len(set.Keys))
	for _, k := range set.Keys {
		if pub, err := k.publicKey(); err == nil {
			keys[k.Kid] = pub
		}
	}
	if len(keys) == 0 {
		return errors.New("auth: jwks has no usable keys")
	}

	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = time.Now()
	v.mu.Unlock()
	return nil
}

func (k jwk) publicKey() (any, error) {
	switch k.Kty {
	case "EC":
		if k.Crv != "P-256" {
			return nil, fmt.Errorf("unsupported curve %q", k.Crv)
		}
		x, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil {
			return nil, err
		}
		y, err := base64.RawURLEncoding.DecodeString(k.Y)
		if err != nil {
			return nil, err
		}
		// 비압축 점 형식: 0x04 || X(32) || Y(32)
		point := make([]byte, 65)
		point[0] = 4
		copy(point[33-len(x):33], x)
		copy(point[65-len(y):], y)
		return ecdsa.ParseUncompressedPublicKey(elliptic.P256(), point)
	case "RSA":
		n, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			return nil, err
		}
		e, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			return nil, err
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}, nil
	default:
		return nil, fmt.Errorf("unsupported key type %q", k.Kty)
	}
}
