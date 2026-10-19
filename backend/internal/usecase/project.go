package usecase

import (
	"context"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

// Project 는 업무 / 개인 프로젝트 이력 유스케이스다.
type Project struct {
	projects domain.ProjectRepository
}

func NewProject(projects domain.ProjectRepository) *Project {
	return &Project{projects: projects}
}

// List 는 내 프로젝트 전부(비공개 포함)다.
func (uc *Project) List(ctx context.Context, userID string) ([]domain.Project, error) {
	return uc.projects.List(ctx, userID, false)
}

func (uc *Project) Create(ctx context.Context, userID string, in domain.Project) (*domain.Project, error) {
	p, err := normalizeProject(in)
	if err != nil {
		return nil, err
	}
	if err := uc.projects.Create(ctx, userID, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (uc *Project) Update(ctx context.Context, userID, id string, in domain.Project) (*domain.Project, error) {
	p, err := normalizeProject(in)
	if err != nil {
		return nil, err
	}
	p.ID = id
	if err := uc.projects.Update(ctx, userID, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (uc *Project) Delete(ctx context.Context, userID, id string) error {
	return uc.projects.Delete(ctx, userID, id)
}

func normalizeProject(in domain.Project) (*domain.Project, error) {
	var (
		out = domain.Project{IsPublic: in.IsPublic, SortOrder: in.SortOrder}
		err error
	)
	switch in.Kind {
	case domain.ProjectWork, domain.ProjectPersonal:
		out.Kind = in.Kind
	default:
		return nil, invalid("프로젝트 구분은 work 또는 personal 입니다")
	}
	if out.Title, err = required("프로젝트 이름", in.Title, 200); err != nil {
		return nil, err
	}
	if out.Organization, err = text("소속", in.Organization, 60); err != nil {
		return nil, err
	}
	if out.Role, err = text("역할", in.Role, 60); err != nil {
		return nil, err
	}
	if out.StartedOn, out.EndedOn, err = period("프로젝트 기간", in.StartedOn, in.EndedOn); err != nil {
		return nil, err
	}
	if out.Summary, err = text("요약", in.Summary, 300); err != nil {
		return nil, err
	}
	if out.Description, err = text("상세 설명", in.Description, 10000); err != nil {
		return nil, err
	}
	if out.TechStack, err = words("기술 스택", in.TechStack, 30, 30); err != nil {
		return nil, err
	}
	if out.Links, err = links("링크", in.Links, 10); err != nil {
		return nil, err
	}
	return &out, nil
}
