// Package notify 는 연락 알림 발송(메일 · 문자)이다.
package notify

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// SMTPConfig 는 메일 서버 설정이다 (Gmail 앱 비밀번호 · Resend 등, Supabase 초대 메일과 같은 것을 써도 된다).
type SMTPConfig struct {
	Host     string
	Port     string // 587(STARTTLS) 또는 465(TLS)
	Username string
	Password string
	From     string // 보내는 사람 (예: "커리어++ <noreply@example.com>")
}

// SMTPMailer 는 표준 라이브러리 net/smtp 로 메일을 보낸다.
type SMTPMailer struct {
	cfg SMTPConfig
}

func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	if cfg.Port == "" {
		cfg.Port = "587"
	}
	return &SMTPMailer{cfg: cfg}
}

func (m *SMTPMailer) Enabled() bool {
	return m.cfg.Host != "" && m.cfg.From != ""
}

// Send 는 text/plain UTF-8 메일 한 통을 보낸다. replyTo 가 있으면 답장이 보낸 방문자에게 간다.
func (m *SMTPMailer) Send(ctx context.Context, to, replyTo, subject, body string) error {
	if !m.Enabled() {
		return fmt.Errorf("notify: smtp is not configured")
	}
	from, err := mail.ParseAddress(m.cfg.From)
	if err != nil {
		return fmt.Errorf("notify: parse MAIL_FROM: %w", err)
	}
	msg, err := buildMessage(from, to, replyTo, subject, body)
	if err != nil {
		return err
	}

	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}

	// net/smtp 는 컨텍스트를 받지 않으므로 별도 고루틴에서 보내고 취소·시간 초과를 기다린다.
	done := make(chan error, 1)
	go func() { done <- m.send(auth, from.Address, to, msg) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("notify: send mail: %w", ctx.Err())
	case <-time.After(20 * time.Second):
		return fmt.Errorf("notify: send mail: timed out")
	}
}

func (m *SMTPMailer) send(auth smtp.Auth, from, to string, msg []byte) error {
	addr := net.JoinHostPort(m.cfg.Host, m.cfg.Port)
	if m.cfg.Port != "465" {
		// SendMail 은 서버가 STARTTLS 를 지원하면 알아서 암호화한다.
		if err := smtp.SendMail(addr, auth, from, []string{to}, msg); err != nil {
			return fmt.Errorf("notify: smtp send: %w", err)
		}
		return nil
	}

	// 465 는 처음부터 TLS 로 연결한다.
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, &tls.Config{ServerName: m.cfg.Host})
	if err != nil {
		return fmt.Errorf("notify: smtp tls dial: %w", err)
	}
	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("notify: smtp client: %w", err)
	}
	defer client.Close()
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("notify: smtp auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("notify: smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("notify: smtp rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("notify: smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("notify: smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("notify: smtp close data: %w", err)
	}
	return client.Quit()
}

// buildMessage 는 헤더 + base64 본문을 만든다. 주소는 다시 파싱해 헤더 주입(줄바꿈)을 막는다.
func buildMessage(from *mail.Address, to, replyTo, subject, body string) ([]byte, error) {
	toAddr, err := mail.ParseAddress(to)
	if err != nil {
		return nil, fmt.Errorf("notify: parse recipient: %w", err)
	}

	var b strings.Builder
	header := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	header("From", from.String())
	header("To", toAddr.String())
	if replyTo != "" {
		r, err := mail.ParseAddress(replyTo)
		if err != nil {
			return nil, fmt.Errorf("notify: parse reply-to: %w", err)
		}
		header("Reply-To", r.String())
	}
	header("Subject", mime.BEncoding.Encode("UTF-8", strings.NewReplacer("\r", " ", "\n", " ").Replace(subject)))
	header("Date", time.Now().Format(time.RFC1123Z))
	header("MIME-Version", "1.0")
	header("Content-Type", `text/plain; charset="UTF-8"`)
	header("Content-Transfer-Encoding", "base64")
	b.WriteString("\r\n")

	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	for len(encoded) > 76 {
		b.WriteString(encoded[:76])
		b.WriteString("\r\n")
		encoded = encoded[76:]
	}
	b.WriteString(encoded)
	b.WriteString("\r\n")
	return []byte(b.String()), nil
}
