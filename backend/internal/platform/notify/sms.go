package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SolapiConfig 는 솔라피(구 CoolSMS) 문자 발송 설정이다. 발신번호는 솔라피 콘솔에 등록한 번호여야 한다.
type SolapiConfig struct {
	APIKey    string
	APISecret string
	From      string // 하이픈 없는 발신번호
	BaseURL   string // 비우면 https://api.solapi.com
}

// SolapiSender 는 솔라피 REST API(v4 단건 발송)로 문자를 보낸다. 90바이트가 넘으면 솔라피가 장문(LMS)으로 바꾼다.
type SolapiSender struct {
	cfg    SolapiConfig
	client *http.Client
}

func NewSolapiSender(cfg SolapiConfig) *SolapiSender {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.solapi.com"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &SolapiSender{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}}
}

func (s *SolapiSender) Enabled() bool {
	return s.cfg.APIKey != "" && s.cfg.APISecret != "" && s.cfg.From != ""
}

func (s *SolapiSender) Send(ctx context.Context, to, text string) error {
	if !s.Enabled() {
		return fmt.Errorf("notify: solapi is not configured")
	}
	payload, err := json.Marshal(map[string]any{
		"message": map[string]string{"to": to, "from": s.cfg.From, "text": text},
	})
	if err != nil {
		return fmt.Errorf("notify: encode sms: %w", err)
	}
	auth, err := s.authorization(time.Now())
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.BaseURL+"/messages/v4/send", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("notify: build sms request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)

	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("notify: send sms: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, 300))
		return fmt.Errorf("notify: send sms: HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

// authorization 은 솔라피 HMAC-SHA256 인증 헤더다: signature = HMAC(secret, date + salt).
func (s *SolapiSender) authorization(now time.Time) (string, error) {
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", fmt.Errorf("notify: sms salt: %w", err)
	}
	salt := hex.EncodeToString(saltBytes)
	date := now.UTC().Format(time.RFC3339)
	return solapiSignature(s.cfg.APIKey, s.cfg.APISecret, date, salt), nil
}

func solapiSignature(apiKey, apiSecret, date, salt string) string {
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(date + salt))
	return fmt.Sprintf("HMAC-SHA256 apiKey=%s, date=%s, salt=%s, signature=%s",
		apiKey, date, salt, hex.EncodeToString(mac.Sum(nil)))
}
