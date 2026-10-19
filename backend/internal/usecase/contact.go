package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

// RateLimiter 는 같은 곳에서 너무 자주 보내는 요청을 막는다 (platform/ratelimit).
type RateLimiter interface {
	Allow(key string) bool
}

// Contact 는 contact 화면(방문자 → 나) 유스케이스다.
type Contact struct {
	profiles  domain.ProfileRepository
	contacts  domain.ContactRepository
	mailer    domain.Mailer
	sms       domain.SMSSender
	limiter   RateLimiter
	smsDaily  int    // 하루에 보낼 SMS 알림 상한 (문자는 돈이 든다)
	publicURL string // 알림에 넣을 사이트 주소 (https://ae2p.duckdns.org)
	now       func() time.Time
}

// ContactOptions 는 Contact 의 설정값이다.
type ContactOptions struct {
	SMSDailyLimit int
	PublicURL     string
}

func NewContact(
	profiles domain.ProfileRepository,
	contacts domain.ContactRepository,
	mailer domain.Mailer,
	sms domain.SMSSender,
	limiter RateLimiter,
	opts ContactOptions,
) *Contact {
	return &Contact{
		profiles: profiles, contacts: contacts, mailer: mailer, sms: sms, limiter: limiter,
		smsDaily: opts.SMSDailyLimit, publicURL: strings.TrimRight(opts.PublicURL, "/"), now: time.Now,
	}
}

// ContactInput 은 방문자가 보내는 연락 폼이다. Website 는 사람 눈에 안 보이는 칸(봇 걸러내기)이다.
type ContactInput struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Message string `json:"message"`
	Website string `json:"website"`
}

// Send 는 메시지를 저장하고, 주인이 켜 둔 채널(메일 · SMS)로 알린다.
// 알림 발송이 실패해도 메시지는 받은 편지함에 남는다.
func (uc *Contact) Send(ctx context.Context, slug string, in ContactInput, senderIP string) error {
	// 봇: 저장도 알림도 하지 않고 성공처럼 보이게 끝낸다
	if strings.TrimSpace(in.Website) != "" {
		return nil
	}
	msg, err := normalizeContact(in)
	if err != nil {
		return err
	}

	owner, err := uc.profiles.FindBySlug(ctx, strings.ToLower(slug))
	if err != nil {
		return err
	}
	if !owner.IsPublic {
		return fmt.Errorf("contact: profile %q is private: %w", slug, domain.ErrNotFound)
	}
	if !uc.limiter.Allow(senderIP) {
		return fmt.Errorf("잠시 뒤에 다시 보내 주세요: %w", domain.ErrTooManyRequests)
	}

	settings, err := uc.contacts.Settings(ctx, owner.UserID)
	if err != nil {
		return err
	}

	msg.DeliveredBy = []string{}
	if settings.MailEnabled && settings.NotifyEmail != "" && uc.mailer.Enabled() {
		subject := fmt.Sprintf("[커리어++] %s 님이 연락을 남겼습니다", msg.SenderName)
		if err := uc.mailer.Send(ctx, settings.NotifyEmail, msg.SenderEmail, subject, uc.mailBody(owner, msg)); err != nil {
			slog.Error("contact: mail notify failed", "owner", owner.Slug, "error", err)
		} else {
			msg.DeliveredBy = append(msg.DeliveredBy, domain.ChannelMail)
		}
	}
	if settings.SMSEnabled && settings.NotifyPhone != "" && uc.sms.Enabled() {
		sent, err := uc.contacts.CountDeliveredSince(ctx, owner.UserID, domain.ChannelSMS, uc.now().Add(-24*time.Hour))
		switch {
		case err != nil:
			slog.Error("contact: count sms failed", "owner", owner.Slug, "error", err)
		case sent >= uc.smsDaily:
			slog.Warn("contact: sms daily limit reached", "owner", owner.Slug, "limit", uc.smsDaily)
		default:
			if err := uc.sms.Send(ctx, settings.NotifyPhone, smsBody(msg)); err != nil {
				slog.Error("contact: sms notify failed", "owner", owner.Slug, "error", err)
			} else {
				msg.DeliveredBy = append(msg.DeliveredBy, domain.ChannelSMS)
			}
		}
	}

	return uc.contacts.CreateMessage(ctx, owner.UserID, msg, senderIP)
}

func (uc *Contact) Settings(ctx context.Context, userID string) (*domain.ContactSettings, error) {
	return uc.contacts.Settings(ctx, userID)
}

func (uc *Contact) SaveSettings(ctx context.Context, userID string, in domain.ContactSettings) (*domain.ContactSettings, error) {
	var (
		out = domain.ContactSettings{MailEnabled: in.MailEnabled, SMSEnabled: in.SMSEnabled}
		err error
	)
	if out.NotifyEmail, err = email("알림 받을 이메일", in.NotifyEmail); err != nil {
		return nil, err
	}
	if out.NotifyPhone, err = phone("알림 받을 전화번호", in.NotifyPhone); err != nil {
		return nil, err
	}
	if err := uc.contacts.SaveSettings(ctx, userID, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Channels 는 서버에 실제로 설정된 발송 수단이다 (설정 화면에서 꺼진 채널을 안내하려고).
func (uc *Contact) Channels() map[string]bool {
	return map[string]bool{domain.ChannelMail: uc.mailer.Enabled(), domain.ChannelSMS: uc.sms.Enabled()}
}

func (uc *Contact) Messages(ctx context.Context, userID string) ([]domain.Message, error) {
	return uc.contacts.ListMessages(ctx, userID, 200)
}

func (uc *Contact) MarkRead(ctx context.Context, userID, id string, read bool) error {
	return uc.contacts.MarkRead(ctx, userID, id, read)
}

func (uc *Contact) DeleteMessage(ctx context.Context, userID, id string) error {
	return uc.contacts.DeleteMessage(ctx, userID, id)
}

func normalizeContact(in ContactInput) (*domain.Message, error) {
	var (
		m   domain.Message
		err error
	)
	if m.SenderName, err = required("이름", in.Name, 60); err != nil {
		return nil, err
	}
	if m.SenderEmail, err = email("이메일", in.Email); err != nil {
		return nil, err
	}
	if m.SenderPhone, err = phone("전화번호", in.Phone); err != nil {
		return nil, err
	}
	if m.SenderEmail == "" && m.SenderPhone == "" {
		return nil, invalid("답장 받을 이메일이나 전화번호 중 하나는 적어 주세요")
	}
	if m.Body, err = required("내용", in.Message, 2000); err != nil {
		return nil, err
	}
	return &m, nil
}

func (uc *Contact) mailBody(owner *domain.Profile, m *domain.Message) string {
	var b strings.Builder
	fmt.Fprintf(&b, "커리어++ 공개 페이지(%s/career/%s)에서 새 연락이 왔습니다.\n\n", uc.publicURL, owner.Slug)
	fmt.Fprintf(&b, "보낸 사람: %s\n", m.SenderName)
	if m.SenderEmail != "" {
		fmt.Fprintf(&b, "이메일: %s (이 메일에 답장하면 보낸 사람에게 갑니다)\n", m.SenderEmail)
	}
	if m.SenderPhone != "" {
		fmt.Fprintf(&b, "전화번호: %s\n", m.SenderPhone)
	}
	fmt.Fprintf(&b, "\n%s\n\n— 받은 메시지 전체: %s/career/%s/contact\n", m.Body, uc.publicURL, owner.Slug)
	return b.String()
}

// smsBody 는 문자 알림 본문이다. 너무 길면 장문(LMS) 요금이 커지므로 앞부분만 보낸다.
func smsBody(m *domain.Message) string {
	reply := m.SenderPhone
	if reply == "" {
		reply = m.SenderEmail
	}
	body := m.Body
	if utf8.RuneCountInString(body) > 300 {
		body = string([]rune(body)[:300]) + "…"
	}
	return fmt.Sprintf("[커리어++] %s (%s)\n%s", m.SenderName, reply, body)
}
