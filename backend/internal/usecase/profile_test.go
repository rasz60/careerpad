package usecase

import (
	"errors"
	"testing"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

func TestSlugFromEmail(t *testing.T) {
	cases := map[string]string{
		"dev.kim_01@example.com": "dev-kim-01",
		"Rasz60@gmail.com":       "rasz60",
		"a..b@x.com":             "a-b",
		"__x__@x.com":            "x", // 3자보다 짧으면 me-xxxxxx 로 바뀐다 (아래에서 따로 검사)
	}
	for in, want := range cases {
		got := slugFromEmail(in)
		if want == "x" {
			if !slugPattern.MatchString(got) {
				t.Errorf("slugFromEmail(%q) = %q, want a valid fallback slug", in, got)
			}
			continue
		}
		if got != want {
			t.Errorf("slugFromEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeProfileRejects(t *testing.T) {
	base := domain.Profile{Slug: "my-page"}
	cases := map[string]func(p *domain.Profile){
		"short slug":      func(p *domain.Profile) { p.Slug = "ab" },
		"upper/invalid":   func(p *domain.Profile) { p.Slug = "my page" },
		"double hyphen":   func(p *domain.Profile) { p.Slug = "my--page" },
		"reserved":        func(p *domain.Profile) { p.Slug = "api" },
		"javascript link": func(p *domain.Profile) { p.Links = []domain.Link{{Label: "x", URL: "javascript:alert(1)"}} },
		"bad email":       func(p *domain.Profile) { p.PublicEmail = "not-an-email" },
		"bad phone":       func(p *domain.Profile) { p.PublicPhone = "12345" },
		"bad month": func(p *domain.Profile) {
			p.Resume.Experiences = []domain.Experience{{Company: "A", StartedOn: "2024-13"}}
		},
		"reversed period": func(p *domain.Profile) {
			p.Resume.Experiences = []domain.Experience{{Company: "A", StartedOn: "2024-05", EndedOn: "2023-01"}}
		},
		"photo not http":   func(p *domain.Profile) { p.PhotoURL = "data:image/png;base64,xx" },
		"mailto for photo": func(p *domain.Profile) { p.PhotoURL = "mailto:a@b.c" },
	}
	for name, mutate := range cases {
		p := base
		mutate(&p)
		if _, err := normalizeProfile(p); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", name, err)
		}
	}
}

func TestNormalizeProfileCleansInput(t *testing.T) {
	got, err := normalizeProfile(domain.Profile{
		Slug:        " My-Page ",
		Name:        "  홍길동 ",
		PublicPhone: "010-1234-5678",
		Links:       []domain.Link{{Label: "", URL: "https://github.com/x"}, {Label: "빈칸", URL: ""}, {Label: "메일", URL: "mailto:a@b.co"}},
		Resume: domain.Resume{
			Experiences: []domain.Experience{{Company: "A사", StartedOn: "2020-01"}, {}},
			Skills:      []domain.SkillGroup{{Category: "BE", Items: []string{"Go", "go", " "}}, {Category: "빈", Items: nil}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Slug != "my-page" || got.Name != "홍길동" || got.PublicPhone != "01012345678" {
		t.Errorf("basic fields = %q %q %q", got.Slug, got.Name, got.PublicPhone)
	}
	if len(got.Links) != 2 || got.Links[0].Label != "https://github.com/x" {
		t.Errorf("links = %+v", got.Links)
	}
	if len(got.Resume.Experiences) != 1 {
		t.Errorf("empty experience row should be dropped: %+v", got.Resume.Experiences)
	}
	if len(got.Resume.Skills) != 1 || len(got.Resume.Skills[0].Items) != 1 {
		t.Errorf("skills = %+v", got.Resume.Skills)
	}
	if got.Resume.Educations == nil || got.Resume.Certificates == nil {
		t.Error("empty lists should be [] not nil")
	}
}

func TestCertificates(t *testing.T) {
	got, err := normalizeProfile(domain.Profile{
		Slug: "my-page",
		Resume: domain.Resume{Certificates: []domain.Certificate{
			{Name: "정보처리기사"},
			{Name: "SQLD", Status: domain.CertPreparing, ExamOn: "2026-11-15", CertNo: "x", Memo: "m"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Resume.Certificates[0].Status != domain.CertAcquired {
		t.Errorf("empty status should become acquired: %+v", got.Resume.Certificates[0])
	}
	for name, c := range map[string]domain.Certificate{
		"bad status": {Name: "A", Status: "done"},
		"bad date":   {Name: "A", ExamOn: "2026-02-30"},
	} {
		if _, err := normalizeProfile(domain.Profile{Slug: "my-page", Resume: domain.Resume{Certificates: []domain.Certificate{c}}}); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", name, err)
		}
	}

	pub := publicCertificates([]domain.Certificate{
		{Name: "A", Status: domain.CertAcquired, CertNo: "123", Memo: "m"},
		{Name: "B", Status: domain.CertPreparing},
	})
	if len(pub) != 1 || pub[0].Name != "A" || pub[0].CertNo != "" || pub[0].Memo != "" {
		t.Errorf("publicCertificates = %+v", pub)
	}
}

func TestNormalizeContact(t *testing.T) {
	if _, err := normalizeContact(ContactInput{Name: "a", Message: "hi"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("no reply channel: err = %v, want ErrInvalidInput", err)
	}
	m, err := normalizeContact(ContactInput{Name: " 김 ", Phone: "010 1234 5678", Message: " 안녕하세요 "})
	if err != nil {
		t.Fatal(err)
	}
	if m.SenderName != "김" || m.SenderPhone != "01012345678" || m.Body != "안녕하세요" {
		t.Errorf("normalizeContact = %+v", m)
	}
}
