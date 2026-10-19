package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devsixt/careerpad/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProjectRepository 는 career.projects 를 다룬다. 기간은 DB 에서 date(그달 1일), 밖에서는 "YYYY-MM".
type ProjectRepository struct {
	pool *pgxpool.Pool
}

func NewProjectRepository(pool *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{pool: pool}
}

var _ domain.ProjectRepository = (*ProjectRepository)(nil)

const projectColumns = `
	id, kind, title, organization, role,
	coalesce(to_char(started_on, 'YYYY-MM'), ''), coalesce(to_char(ended_on, 'YYYY-MM'), ''),
	summary, description, tech_stack, links, is_public, sort_order, created_at, updated_at`

// List 는 idx_projects_user_order (user_id, sort_order, started_on DESC) 순서 그대로 읽는다.
func (r *ProjectRepository) List(ctx context.Context, userID string, publicOnly bool) ([]domain.Project, error) {
	q := `
		SELECT ` + projectColumns + `
		  FROM career.projects
		 WHERE user_id = $1 AND (NOT $2 OR is_public)
		 ORDER BY sort_order, started_on DESC, created_at DESC`
	rows, err := r.pool.Query(ctx, q, userID, publicOnly)
	if err != nil {
		return nil, translate("project repository: list", err)
	}
	projects, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Project, error) {
		p, err := scanProject(row)
		if err != nil {
			return domain.Project{}, err
		}
		return *p, nil
	})
	if err != nil {
		return nil, translate("project repository: list scan", err)
	}
	return projects, nil
}

func (r *ProjectRepository) Create(ctx context.Context, userID string, p *domain.Project) error {
	links, err := json.Marshal(p.Links)
	if err != nil {
		return fmt.Errorf("project repository: encode links: %w", err)
	}
	q := `
		INSERT INTO career.projects
		       (user_id, kind, title, organization, role, started_on, ended_on,
		        summary, description, tech_stack, links, is_public, sort_order)
		VALUES ($1, $2, $3, $4, $5, to_date(nullif($6, ''), 'YYYY-MM'), to_date(nullif($7, ''), 'YYYY-MM'),
		        $8, $9, $10, $11::jsonb, $12, $13)
		RETURNING ` + projectColumns
	saved, err := scanProject(r.pool.QueryRow(ctx, q,
		userID, p.Kind, p.Title, p.Organization, p.Role, p.StartedOn, p.EndedOn,
		p.Summary, p.Description, p.TechStack, string(links), p.IsPublic, p.SortOrder))
	if err != nil {
		return translate("project repository: create", err)
	}
	*p = *saved
	return nil
}

// Update 는 내 프로젝트만 바꾼다 (user_id 조건). 없거나 남의 것이면 ErrNotFound.
func (r *ProjectRepository) Update(ctx context.Context, userID string, p *domain.Project) error {
	links, err := json.Marshal(p.Links)
	if err != nil {
		return fmt.Errorf("project repository: encode links: %w", err)
	}
	q := `
		UPDATE career.projects
		   SET kind = $3, title = $4, organization = $5, role = $6,
		       started_on = to_date(nullif($7, ''), 'YYYY-MM'), ended_on = to_date(nullif($8, ''), 'YYYY-MM'),
		       summary = $9, description = $10, tech_stack = $11, links = $12::jsonb,
		       is_public = $13, sort_order = $14, updated_at = now()
		 WHERE id = $1 AND user_id = $2
		RETURNING ` + projectColumns
	saved, err := scanProject(r.pool.QueryRow(ctx, q,
		p.ID, userID, p.Kind, p.Title, p.Organization, p.Role, p.StartedOn, p.EndedOn,
		p.Summary, p.Description, p.TechStack, string(links), p.IsPublic, p.SortOrder))
	if err != nil {
		return translate("project repository: update", err)
	}
	*p = *saved
	return nil
}

func (r *ProjectRepository) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM career.projects WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return translate("project repository: delete", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("project repository: delete %s: %w", id, domain.ErrNotFound)
	}
	return nil
}

func scanProject(row pgx.Row) (*domain.Project, error) {
	var (
		p     domain.Project
		links []byte
	)
	err := row.Scan(&p.ID, &p.Kind, &p.Title, &p.Organization, &p.Role, &p.StartedOn, &p.EndedOn,
		&p.Summary, &p.Description, &p.TechStack, &links, &p.IsPublic, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(links, &p.Links); err != nil {
		return nil, fmt.Errorf("decode project links: %w", err)
	}
	if p.Links == nil {
		p.Links = []domain.Link{}
	}
	if p.TechStack == nil {
		p.TechStack = []string{}
	}
	return &p, nil
}
