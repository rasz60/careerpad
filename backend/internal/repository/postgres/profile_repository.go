package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devsixt/careerpad/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProfileRepository 는 career.profiles 를 다룬다. links · resume 은 jsonb 로 저장한다.
type ProfileRepository struct {
	pool *pgxpool.Pool
}

func NewProfileRepository(pool *pgxpool.Pool) *ProfileRepository {
	return &ProfileRepository{pool: pool}
}

var _ domain.ProfileRepository = (*ProfileRepository)(nil)

const profileColumns = `
	user_id, slug, is_public, name, headline, summary, photo_url, location,
	public_email, public_phone, links, resume, updated_at`

func (r *ProfileRepository) FindByUserID(ctx context.Context, userID string) (*domain.Profile, error) {
	q := `SELECT ` + profileColumns + ` FROM career.profiles WHERE user_id = $1`
	p, err := scanProfile(r.pool.QueryRow(ctx, q, userID))
	return p, translate("profile repository: find by user", err)
}

// FindBySlug 는 slug UNIQUE 인덱스로 찾는다.
func (r *ProfileRepository) FindBySlug(ctx context.Context, slug string) (*domain.Profile, error) {
	q := `SELECT ` + profileColumns + ` FROM career.profiles WHERE slug = $1`
	p, err := scanProfile(r.pool.QueryRow(ctx, q, slug))
	return p, translate("profile repository: find by slug", err)
}

func (r *ProfileRepository) Create(ctx context.Context, userID, slug, name string) (*domain.Profile, error) {
	q := `
		INSERT INTO career.profiles (user_id, slug, name)
		VALUES ($1, $2, $3)
		RETURNING ` + profileColumns
	p, err := scanProfile(r.pool.QueryRow(ctx, q, userID, slug, name))
	return p, translate("profile repository: create", err)
}

func (r *ProfileRepository) Update(ctx context.Context, p *domain.Profile) error {
	links, err := json.Marshal(p.Links)
	if err != nil {
		return fmt.Errorf("profile repository: encode links: %w", err)
	}
	resume, err := json.Marshal(p.Resume)
	if err != nil {
		return fmt.Errorf("profile repository: encode resume: %w", err)
	}

	const q = `
		UPDATE career.profiles
		   SET slug = $2, is_public = $3, name = $4, headline = $5, summary = $6, photo_url = $7,
		       location = $8, public_email = $9, public_phone = $10,
		       links = $11::jsonb, resume = $12::jsonb, updated_at = now()
		 WHERE user_id = $1
		RETURNING updated_at`
	err = r.pool.QueryRow(ctx, q,
		p.UserID, p.Slug, p.IsPublic, p.Name, p.Headline, p.Summary, p.PhotoURL,
		p.Location, p.PublicEmail, p.PublicPhone, string(links), string(resume),
	).Scan(&p.UpdatedAt)
	return translate("profile repository: update", err)
}

func scanProfile(row pgx.Row) (*domain.Profile, error) {
	var (
		p             domain.Profile
		links, resume []byte
	)
	err := row.Scan(&p.UserID, &p.Slug, &p.IsPublic, &p.Name, &p.Headline, &p.Summary, &p.PhotoURL, &p.Location,
		&p.PublicEmail, &p.PublicPhone, &links, &resume, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(links, &p.Links); err != nil {
		return nil, fmt.Errorf("decode links: %w", err)
	}
	if err := json.Unmarshal(resume, &p.Resume); err != nil {
		return nil, fmt.Errorf("decode resume: %w", err)
	}
	fillEmptyResume(&p)
	return &p, nil
}

// fillEmptyResume 은 빈 목록을 null 이 아니라 [] 로 내보내게 한다 (화면이 null 검사를 하지 않아도 되게).
func fillEmptyResume(p *domain.Profile) {
	if p.Links == nil {
		p.Links = []domain.Link{}
	}
	if p.Resume.Experiences == nil {
		p.Resume.Experiences = []domain.Experience{}
	}
	if p.Resume.Educations == nil {
		p.Resume.Educations = []domain.Education{}
	}
	if p.Resume.Skills == nil {
		p.Resume.Skills = []domain.SkillGroup{}
	}
	if p.Resume.Certificates == nil {
		p.Resume.Certificates = []domain.Certificate{}
	}
}
