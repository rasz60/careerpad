package postgres

import (
	"context"
	"fmt"

	"github.com/devsixt/careerpad/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JobSearchRepository 는 career.job_searches 를 다룬다.
type JobSearchRepository struct {
	pool *pgxpool.Pool
}

func NewJobSearchRepository(pool *pgxpool.Pool) *JobSearchRepository {
	return &JobSearchRepository{pool: pool}
}

var _ domain.JobSearchRepository = (*JobSearchRepository)(nil)

const jobSearchColumns = `
	id, name, keywords, sites, location, career_years, exclude_words, sort_order, created_at, updated_at`

// List 는 idx_job_searches_user_order (user_id, sort_order, created_at) 순서 그대로 읽는다.
func (r *JobSearchRepository) List(ctx context.Context, userID string) ([]domain.JobSearch, error) {
	q := `SELECT ` + jobSearchColumns + ` FROM career.job_searches WHERE user_id = $1 ORDER BY sort_order, created_at`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, translate("job search repository: list", err)
	}
	searches, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.JobSearch, error) {
		s, err := scanJobSearch(row)
		if err != nil {
			return domain.JobSearch{}, err
		}
		return *s, nil
	})
	if err != nil {
		return nil, translate("job search repository: list scan", err)
	}
	return searches, nil
}

func (r *JobSearchRepository) Find(ctx context.Context, userID, id string) (*domain.JobSearch, error) {
	q := `SELECT ` + jobSearchColumns + ` FROM career.job_searches WHERE id = $1 AND user_id = $2`
	s, err := scanJobSearch(r.pool.QueryRow(ctx, q, id, userID))
	return s, translate("job search repository: find", err)
}

func (r *JobSearchRepository) Create(ctx context.Context, userID string, s *domain.JobSearch) error {
	q := `
		INSERT INTO career.job_searches (user_id, name, keywords, sites, location, career_years, exclude_words, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING ` + jobSearchColumns
	saved, err := scanJobSearch(r.pool.QueryRow(ctx, q,
		userID, s.Name, s.Keywords, s.Sites, s.Location, s.CareerYears, s.ExcludeWords, s.SortOrder))
	if err != nil {
		return translate("job search repository: create", err)
	}
	*s = *saved
	return nil
}

func (r *JobSearchRepository) Update(ctx context.Context, userID string, s *domain.JobSearch) error {
	q := `
		UPDATE career.job_searches
		   SET name = $3, keywords = $4, sites = $5, location = $6, career_years = $7,
		       exclude_words = $8, sort_order = $9, updated_at = now()
		 WHERE id = $1 AND user_id = $2
		RETURNING ` + jobSearchColumns
	saved, err := scanJobSearch(r.pool.QueryRow(ctx, q,
		s.ID, userID, s.Name, s.Keywords, s.Sites, s.Location, s.CareerYears, s.ExcludeWords, s.SortOrder))
	if err != nil {
		return translate("job search repository: update", err)
	}
	*s = *saved
	return nil
}

func (r *JobSearchRepository) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM career.job_searches WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return translate("job search repository: delete", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("job search repository: delete %s: %w", id, domain.ErrNotFound)
	}
	return nil
}

func scanJobSearch(row pgx.Row) (*domain.JobSearch, error) {
	var s domain.JobSearch
	err := row.Scan(&s.ID, &s.Name, &s.Keywords, &s.Sites, &s.Location, &s.CareerYears, &s.ExcludeWords,
		&s.SortOrder, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if s.ExcludeWords == nil {
		s.ExcludeWords = []string{}
	}
	return &s, nil
}
