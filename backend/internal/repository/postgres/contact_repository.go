package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/devsixt/careerpad/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ContactRepository 는 career.contact_settings · career.messages 를 다룬다.
type ContactRepository struct {
	pool *pgxpool.Pool
}

func NewContactRepository(pool *pgxpool.Pool) *ContactRepository {
	return &ContactRepository{pool: pool}
}

var _ domain.ContactRepository = (*ContactRepository)(nil)

func (r *ContactRepository) Settings(ctx context.Context, userID string) (*domain.ContactSettings, error) {
	const q = `
		SELECT notify_email, notify_phone, mail_enabled, sms_enabled
		  FROM career.contact_settings
		 WHERE user_id = $1`
	var s domain.ContactSettings
	err := r.pool.QueryRow(ctx, q, userID).Scan(&s.NotifyEmail, &s.NotifyPhone, &s.MailEnabled, &s.SMSEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return &domain.ContactSettings{MailEnabled: true}, nil
	}
	if err != nil {
		return nil, translate("contact repository: settings", err)
	}
	return &s, nil
}

func (r *ContactRepository) SaveSettings(ctx context.Context, userID string, s *domain.ContactSettings) error {
	const q = `
		INSERT INTO career.contact_settings (user_id, notify_email, notify_phone, mail_enabled, sms_enabled)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		   SET notify_email = excluded.notify_email, notify_phone = excluded.notify_phone,
		       mail_enabled = excluded.mail_enabled, sms_enabled = excluded.sms_enabled, updated_at = now()`
	_, err := r.pool.Exec(ctx, q, userID, s.NotifyEmail, s.NotifyPhone, s.MailEnabled, s.SMSEnabled)
	return translate("contact repository: save settings", err)
}

func (r *ContactRepository) CreateMessage(ctx context.Context, ownerID string, m *domain.Message, senderIP string) error {
	const q = `
		INSERT INTO career.messages (owner_id, sender_name, sender_email, sender_phone, body, delivered_by, sender_ip)
		VALUES ($1, $2, $3, $4, $5, $6, nullif($7, '')::inet)
		RETURNING id, is_read, created_at`
	err := r.pool.QueryRow(ctx, q, ownerID, m.SenderName, m.SenderEmail, m.SenderPhone, m.Body, m.DeliveredBy, senderIP).
		Scan(&m.ID, &m.IsRead, &m.CreatedAt)
	return translate("contact repository: create message", err)
}

// ListMessages 는 idx_messages_owner_created (owner_id, created_at DESC) 로 최신순 limit 건을 읽는다.
func (r *ContactRepository) ListMessages(ctx context.Context, ownerID string, limit int) ([]domain.Message, error) {
	const q = `
		SELECT id, sender_name, sender_email, sender_phone, body, delivered_by, is_read, created_at
		  FROM career.messages
		 WHERE owner_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`
	rows, err := r.pool.Query(ctx, q, ownerID, limit)
	if err != nil {
		return nil, translate("contact repository: list messages", err)
	}
	messages, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Message, error) {
		var m domain.Message
		err := row.Scan(&m.ID, &m.SenderName, &m.SenderEmail, &m.SenderPhone, &m.Body, &m.DeliveredBy, &m.IsRead, &m.CreatedAt)
		return m, err
	})
	if err != nil {
		return nil, translate("contact repository: list messages scan", err)
	}
	return messages, nil
}

func (r *ContactRepository) MarkRead(ctx context.Context, ownerID, id string, read bool) error {
	tag, err := r.pool.Exec(ctx, `UPDATE career.messages SET is_read = $3 WHERE id = $1 AND owner_id = $2`, id, ownerID, read)
	if err != nil {
		return translate("contact repository: mark read", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("contact repository: mark read %s: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r *ContactRepository) DeleteMessage(ctx context.Context, ownerID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM career.messages WHERE id = $1 AND owner_id = $2`, id, ownerID)
	if err != nil {
		return translate("contact repository: delete message", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("contact repository: delete message %s: %w", id, domain.ErrNotFound)
	}
	return nil
}

// CountDeliveredSince 는 같은 인덱스의 (owner_id, created_at) 범위만 훑는다.
func (r *ContactRepository) CountDeliveredSince(ctx context.Context, ownerID, channel string, since time.Time) (int, error) {
	const q = `
		SELECT count(*)
		  FROM career.messages
		 WHERE owner_id = $1 AND created_at >= $2 AND $3 = ANY (delivered_by)`
	var n int
	if err := r.pool.QueryRow(ctx, q, ownerID, since, channel).Scan(&n); err != nil {
		return 0, translate("contact repository: count delivered", err)
	}
	return n, nil
}
