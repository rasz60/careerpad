package notify

import (
	"encoding/base64"
	"net/mail"
	"strings"
	"testing"
)

func TestBuildMessageEncodesAndBlocksHeaderInjection(t *testing.T) {
	from := &mail.Address{Name: "커리어++", Address: "noreply@example.com"}
	msg, err := buildMessage(from, "me@example.com", "visitor@example.com", "안녕\r\nBcc: evil@x.com", "본문입니다")
	if err != nil {
		t.Fatal(err)
	}
	head, body, ok := strings.Cut(string(msg), "\r\n\r\n")
	if !ok {
		t.Fatal("no header/body separator")
	}
	if strings.Contains(head, "Bcc:") {
		t.Errorf("subject newline must not create a new header:\n%s", head)
	}
	if !strings.Contains(head, "Reply-To: <visitor@example.com>") {
		t.Errorf("missing reply-to:\n%s", head)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(body, "\r\n", ""))
	if err != nil || string(decoded) != "본문입니다" {
		t.Errorf("body = %q (%v)", decoded, err)
	}

	if _, err := buildMessage(from, "me@example.com", "bad\r\nBcc: x@y.z", "s", "b"); err == nil {
		t.Error("reply-to with newline should be rejected")
	}
}

func TestSolapiSignature(t *testing.T) {
	// HMAC-SHA256(secret, date+salt) 의 hex — 솔라피 문서의 서명 방식
	got := solapiSignature("KEY", "SECRET", "2026-10-19T00:00:00Z", "abc")
	if !strings.HasPrefix(got, "HMAC-SHA256 apiKey=KEY, date=2026-10-19T00:00:00Z, salt=abc, signature=") {
		t.Fatalf("header = %q", got)
	}
	sig := got[strings.LastIndex(got, "=")+1:]
	if len(sig) != 64 {
		t.Errorf("signature length = %d, want 64 hex chars", len(sig))
	}
}
