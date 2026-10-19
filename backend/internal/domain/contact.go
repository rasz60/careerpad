package domain

import (
	"context"
	"time"
)

// 연락 알림 채널.
const (
	ChannelMail = "mail"
	ChannelSMS  = "sms"
)

// ContactSettings 는 방문자 메시지를 받을 곳이다. 공개 페이지에는 보이지 않는다.
type ContactSettings struct {
	NotifyEmail string `json:"notify_email"`
	NotifyPhone string `json:"notify_phone"`
	MailEnabled bool   `json:"mail_enabled"`
	SMSEnabled  bool   `json:"sms_enabled"`
}

// Message 는 방문자가 contact 화면에서 남긴 메시지다.
type Message struct {
	ID          string    `json:"id"`
	SenderName  string    `json:"sender_name"`
	SenderEmail string    `json:"sender_email"`
	SenderPhone string    `json:"sender_phone"`
	Body        string    `json:"body"`
	DeliveredBy []string  `json:"delivered_by"`
	IsRead      bool      `json:"is_read"`
	CreatedAt   time.Time `json:"created_at"`
}

// ContactRepository 는 career.contact_settings · career.messages 를 다룬다.
type ContactRepository interface {
	// Settings 는 저장된 설정이 없으면 기본값(메일 켬, 받는 곳 비어 있음)을 돌려준다.
	Settings(ctx context.Context, userID string) (*ContactSettings, error)
	SaveSettings(ctx context.Context, userID string, s *ContactSettings) error

	CreateMessage(ctx context.Context, ownerID string, m *Message, senderIP string) error
	ListMessages(ctx context.Context, ownerID string, limit int) ([]Message, error)
	MarkRead(ctx context.Context, ownerID, id string, read bool) error
	DeleteMessage(ctx context.Context, ownerID, id string) error
	// CountDeliveredSince 는 since 이후 해당 채널로 알림이 나간 메시지 수다 (SMS 하루 한도).
	CountDeliveredSince(ctx context.Context, ownerID, channel string, since time.Time) (int, error)
}

// Mailer 는 알림 메일을 보낸다. 설정이 없으면 Enabled 가 false.
type Mailer interface {
	Enabled() bool
	Send(ctx context.Context, to, replyTo, subject, body string) error
}

// SMSSender 는 알림 문자를 보낸다. 설정이 없으면 Enabled 가 false.
type SMSSender interface {
	Enabled() bool
	Send(ctx context.Context, to, text string) error
}
