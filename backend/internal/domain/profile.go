package domain

import (
	"context"
	"time"
)

// Link 는 화면에 그대로 거는 외부 링크다 (GitHub, 블로그, 데모 …).
type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Experience 는 이력서의 경력 한 줄이다. 기간은 "YYYY-MM", EndedOn 이 비면 재직 중.
type Experience struct {
	Company     string `json:"company"`
	Role        string `json:"role"`
	StartedOn   string `json:"started_on"`
	EndedOn     string `json:"ended_on"`
	Description string `json:"description"`
}

// Education 은 학력 한 줄이다.
type Education struct {
	School    string `json:"school"`
	Major     string `json:"major"`
	Degree    string `json:"degree"`
	StartedOn string `json:"started_on"`
	EndedOn   string `json:"ended_on"`
}

// SkillGroup 은 분류별 기술 묶음이다 (예: Backend → Go, Java).
type SkillGroup struct {
	Category string   `json:"category"`
	Items    []string `json:"items"`
}

// 자격증 상태. 예전 데이터(빈 값)는 취득으로 본다.
const (
	CertAcquired  = "acquired"
	CertPreparing = "preparing"
)

// Certificate 는 자격증·수상 한 줄이다. 준비 중인 자격 · 자격번호 · 메모는 주인에게만 보인다.
type Certificate struct {
	Name       string `json:"name"`
	Issuer     string `json:"issuer"`
	AcquiredOn string `json:"acquired_on"`
	Status     string `json:"status"`
	Grade      string `json:"grade"`      // 점수 · 등급 (TOEIC 905, 1급 …)
	CertNo     string `json:"cert_no"`    // 자격번호
	ExamOn     string `json:"exam_on"`    // 시험일 YYYY-MM-DD (준비 중)
	ExpiresOn  string `json:"expires_on"` // 유효기간 끝 YYYY-MM-DD
	Memo       string `json:"memo"`
}

// Resume 는 about me 의 이력서 본문이다. 화면 구성이 자주 바뀔 수 있어 DB 에는 jsonb 한 칸으로 둔다.
type Resume struct {
	Experiences  []Experience  `json:"experiences"`
	Educations   []Education   `json:"educations"`
	Skills       []SkillGroup  `json:"skills"`
	Certificates []Certificate `json:"certificates"`
}

// Profile 은 공개 페이지의 주인과 about me 내용이다.
type Profile struct {
	UserID      string    `json:"-"`
	Slug        string    `json:"slug"`
	IsPublic    bool      `json:"is_public"`
	Name        string    `json:"name"`
	Headline    string    `json:"headline"`
	Summary     string    `json:"summary"`
	PhotoURL    string    `json:"photo_url"`
	Location    string    `json:"location"`
	PublicEmail string    `json:"public_email"`
	PublicPhone string    `json:"public_phone"`
	Links       []Link    `json:"links"`
	Resume      Resume    `json:"resume"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProfileRepository 는 career.profiles 를 다룬다.
type ProfileRepository interface {
	FindByUserID(ctx context.Context, userID string) (*Profile, error)
	FindBySlug(ctx context.Context, slug string) (*Profile, error)
	// Create 는 빈 프로필을 만든다. slug 가 이미 있으면 ErrConflict.
	Create(ctx context.Context, userID, slug, name string) (*Profile, error)
	Update(ctx context.Context, p *Profile) error
}
