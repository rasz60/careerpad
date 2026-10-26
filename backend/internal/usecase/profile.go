package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$`)

// reservedSlugs 는 공개 주소(/career/<slug>)로 쓰면 화면 경로와 헷갈리는 이름이다.
var reservedSlugs = map[string]bool{
	"api": true, "me": true, "admin": true, "login": true, "logout": true, "new": true,
	"edit": true, "settings": true, "assets": true, "static": true, "career": true,
}

// Profile 은 about me(프로필 · 이력서) 유스케이스다.
type Profile struct {
	profiles domain.ProfileRepository
	projects domain.ProjectRepository
}

func NewProfile(profiles domain.ProfileRepository, projects domain.ProjectRepository) *Profile {
	return &Profile{profiles: profiles, projects: projects}
}

// PublicPage 는 공개 페이지 한 장에 필요한 값이다.
type PublicPage struct {
	Profile  *domain.Profile  `json:"profile"`
	Projects []domain.Project `json:"projects"`
	// IsOwner 면 비공개 프로젝트까지 보이고 편집·job++ 메뉴가 열린다.
	IsOwner bool `json:"is_owner"`
}

// Public 은 slug 의 공개 페이지를 돌려준다. 비공개 프로필은 주인에게만 보이고 다른 사람에게는 없는 페이지다.
func (uc *Profile) Public(ctx context.Context, slug, viewerID string) (*PublicPage, error) {
	p, err := uc.profiles.FindBySlug(ctx, strings.ToLower(slug))
	if err != nil {
		return nil, err
	}
	owner := viewerID != "" && viewerID == p.UserID
	if !p.IsPublic && !owner {
		return nil, fmt.Errorf("profile %q is private: %w", slug, domain.ErrNotFound)
	}
	projects, err := uc.projects.List(ctx, p.UserID, !owner)
	if err != nil {
		return nil, err
	}
	if !owner {
		p.Resume.Certificates = publicCertificates(p.Resume.Certificates)
	}
	return &PublicPage{Profile: p, Projects: projects, IsOwner: owner}, nil
}

// publicCertificates 는 방문자에게 보일 자격만 남긴다: 취득한 것만, 자격번호 · 메모 · 시험일은 빼고.
func publicCertificates(in []domain.Certificate) []domain.Certificate {
	out := make([]domain.Certificate, 0, len(in))
	for _, c := range in {
		if c.Status == domain.CertPreparing {
			continue
		}
		c.CertNo, c.Memo, c.ExamOn = "", "", ""
		out = append(out, c)
	}
	return out
}

// Mine 은 로그인한 사용자의 프로필이다. 처음이면 이메일 아이디로 주소를 정해 빈 프로필을 만든다.
func (uc *Profile) Mine(ctx context.Context, userID, userEmail string) (*domain.Profile, error) {
	p, err := uc.profiles.FindByUserID(ctx, userID)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	base := slugFromEmail(userEmail)
	name, _, _ := strings.Cut(userEmail, "@")
	// 같은 주소가 이미 있으면 뒤에 숫자를 붙여 몇 번 더 시도한다.
	for attempt := range 5 {
		slug := base
		if attempt > 0 {
			slug = fmt.Sprintf("%s-%d", trimSlug(base, 27), randomInt(9000)+1000)
		}
		p, err = uc.profiles.Create(ctx, userID, slug, name)
		if err == nil {
			return p, nil
		}
		if !errors.Is(err, domain.ErrConflict) {
			return nil, err
		}
		// 동시에 두 요청이 들어와 내 프로필이 먼저 생긴 경우
		if mine, findErr := uc.profiles.FindByUserID(ctx, userID); findErr == nil {
			return mine, nil
		}
	}
	return nil, fmt.Errorf("profile: could not allocate slug for %s: %w", userID, domain.ErrConflict)
}

// Update 는 내 프로필을 통째로 바꾼다 (편집 화면이 전체를 보낸다).
func (uc *Profile) Update(ctx context.Context, userID string, in domain.Profile) (*domain.Profile, error) {
	current, err := uc.profiles.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	next, err := normalizeProfile(in)
	if err != nil {
		return nil, err
	}
	next.UserID = current.UserID
	if err := uc.profiles.Update(ctx, next); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return nil, fmt.Errorf("이미 다른 사람이 쓰는 주소입니다: %w", domain.ErrConflict)
		}
		return nil, err
	}
	return next, nil
}

// normalizeProfile 은 입력을 다듬고 검사한다 (DB 를 거치지 않는 순수 함수라 단위 테스트 대상).
func normalizeProfile(in domain.Profile) (*domain.Profile, error) {
	var (
		out = domain.Profile{IsPublic: in.IsPublic}
		err error
	)
	out.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
	if !slugPattern.MatchString(out.Slug) || strings.Contains(out.Slug, "--") {
		return nil, invalid("주소는 영문 소문자·숫자·하이픈 3~32자로 정해 주세요")
	}
	if reservedSlugs[out.Slug] {
		return nil, invalid("%q 는 주소로 쓸 수 없습니다", out.Slug)
	}
	if out.Name, err = text("이름", in.Name, 40); err != nil {
		return nil, err
	}
	if out.Headline, err = text("한 줄 소개", in.Headline, 80); err != nil {
		return nil, err
	}
	if out.Summary, err = text("소개", in.Summary, 3000); err != nil {
		return nil, err
	}
	if out.PhotoURL, err = webURL("사진", in.PhotoURL, false); err != nil {
		return nil, err
	}
	if out.Location, err = text("지역", in.Location, 40); err != nil {
		return nil, err
	}
	if out.PublicEmail, err = email("공개 이메일", in.PublicEmail); err != nil {
		return nil, err
	}
	if out.PublicPhone, err = phone("공개 전화번호", in.PublicPhone); err != nil {
		return nil, err
	}
	if out.Links, err = links("링크", in.Links, 10); err != nil {
		return nil, err
	}
	if out.Resume, err = normalizeResume(in.Resume); err != nil {
		return nil, err
	}
	return &out, nil
}

func normalizeResume(in domain.Resume) (domain.Resume, error) {
	const maxRows = 50
	out := domain.Resume{
		Experiences:  []domain.Experience{},
		Educations:   []domain.Education{},
		Skills:       []domain.SkillGroup{},
		Certificates: []domain.Certificate{},
	}
	if len(in.Experiences) > maxRows || len(in.Educations) > maxRows || len(in.Skills) > maxRows || len(in.Certificates) > maxRows {
		return out, invalid("이력서 항목은 종류별로 %d개까지 넣을 수 있습니다", maxRows)
	}

	for _, e := range in.Experiences {
		var (
			row domain.Experience
			err error
		)
		if row.Company, err = text("회사", e.Company, 60); err != nil {
			return out, err
		}
		if row.Role, err = text("직무", e.Role, 60); err != nil {
			return out, err
		}
		if row.StartedOn, row.EndedOn, err = period("경력 기간", e.StartedOn, e.EndedOn); err != nil {
			return out, err
		}
		if row.Description, err = text("경력 설명", e.Description, 2000); err != nil {
			return out, err
		}
		if row.Company != "" || row.Role != "" {
			out.Experiences = append(out.Experiences, row)
		}
	}
	for _, e := range in.Educations {
		var (
			row domain.Education
			err error
		)
		if row.School, err = text("학교", e.School, 60); err != nil {
			return out, err
		}
		if row.Major, err = text("전공", e.Major, 60); err != nil {
			return out, err
		}
		if row.Degree, err = text("학위", e.Degree, 30); err != nil {
			return out, err
		}
		if row.StartedOn, row.EndedOn, err = period("학력 기간", e.StartedOn, e.EndedOn); err != nil {
			return out, err
		}
		if row.School != "" {
			out.Educations = append(out.Educations, row)
		}
	}
	for _, s := range in.Skills {
		category, err := text("기술 분류", s.Category, 30)
		if err != nil {
			return out, err
		}
		items, err := words("기술", s.Items, 40, 30)
		if err != nil {
			return out, err
		}
		if len(items) > 0 {
			out.Skills = append(out.Skills, domain.SkillGroup{Category: category, Items: items})
		}
	}
	for _, c := range in.Certificates {
		var (
			row domain.Certificate
			err error
		)
		if row.Name, err = text("자격증", c.Name, 60); err != nil {
			return out, err
		}
		if row.Issuer, err = text("발급 기관", c.Issuer, 60); err != nil {
			return out, err
		}
		if row.AcquiredOn, err = month("취득일", c.AcquiredOn); err != nil {
			return out, err
		}
		switch row.Status = strings.TrimSpace(c.Status); row.Status {
		case "":
			row.Status = domain.CertAcquired
		case domain.CertAcquired, domain.CertPreparing:
		default:
			return out, invalid("자격증 상태가 올바르지 않습니다")
		}
		if row.Grade, err = text("점수 · 등급", c.Grade, 30); err != nil {
			return out, err
		}
		if row.CertNo, err = text("자격번호", c.CertNo, 40); err != nil {
			return out, err
		}
		if row.ExamOn, err = date("시험일", c.ExamOn); err != nil {
			return out, err
		}
		if row.ExpiresOn, err = date("유효기간", c.ExpiresOn); err != nil {
			return out, err
		}
		if row.Memo, err = text("자격증 메모", c.Memo, 500); err != nil {
			return out, err
		}
		if row.Name != "" {
			out.Certificates = append(out.Certificates, row)
		}
	}
	return out, nil
}

// slugFromEmail 은 이메일 아이디로 기본 공개 주소를 만든다 (dev.kim_01@x.com → dev-kim-01).
func slugFromEmail(addr string) string {
	local, _, _ := strings.Cut(strings.ToLower(addr), "@")
	var b strings.Builder
	for _, r := range local {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
	}
	slug := trimSlug(b.String(), 32)
	if len(slug) < 3 {
		slug = fmt.Sprintf("me-%d", randomInt(900000)+100000)
	}
	return slug
}

func trimSlug(s string, maxLen int) string {
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return strings.Trim(s, "-")
}

func randomInt(n int64) int64 {
	v, err := rand.Int(rand.Reader, big.NewInt(n))
	if err != nil {
		return 0
	}
	return v.Int64()
}
