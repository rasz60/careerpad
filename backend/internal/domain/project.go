package domain

import (
	"context"
	"time"
)

// ProjectKind 는 업무 / 개인 프로젝트 구분이다.
type ProjectKind string

const (
	ProjectWork     ProjectKind = "work"
	ProjectPersonal ProjectKind = "personal"
)

// Project 는 내가 한 일 한 건이다. 기간은 "YYYY-MM", EndedOn 이 비면 진행 중.
type Project struct {
	ID           string      `json:"id"`
	Kind         ProjectKind `json:"kind"`
	Title        string      `json:"title"`
	Organization string      `json:"organization"`
	Role         string      `json:"role"`
	StartedOn    string      `json:"started_on"`
	EndedOn      string      `json:"ended_on"`
	Summary      string      `json:"summary"`
	Description  string      `json:"description"`
	TechStack    []string    `json:"tech_stack"`
	Links        []Link      `json:"links"`
	IsPublic     bool        `json:"is_public"`
	SortOrder    int         `json:"sort_order"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// ProjectRepository 는 career.projects 를 다룬다. 모든 쓰기는 userID 로 범위를 묶는다.
type ProjectRepository interface {
	List(ctx context.Context, userID string, publicOnly bool) ([]Project, error)
	Create(ctx context.Context, userID string, p *Project) error
	Update(ctx context.Context, userID string, p *Project) error
	Delete(ctx context.Context, userID, id string) error
}
