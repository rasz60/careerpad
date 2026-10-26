package usecase

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

// 입력 검증 도우미. 실패는 모두 domain.ErrInvalidInput 으로 감싸 400 으로 나간다.

var (
	monthPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)
	phonePattern = regexp.MustCompile(`^0\d{8,10}$`) // 하이픈 뺀 국내 번호
)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), domain.ErrInvalidInput)
}

// text 는 앞뒤 공백을 지우고 글자 수(룬) 한도를 검사한다.
func text(field, value string, maxLen int) (string, error) {
	v := strings.TrimSpace(value)
	if utf8.RuneCountInString(v) > maxLen {
		return "", invalid("%s 는 %d자까지 쓸 수 있습니다", field, maxLen)
	}
	return v, nil
}

// required 는 text 와 같고 빈 값을 막는다.
func required(field, value string, maxLen int) (string, error) {
	v, err := text(field, value, maxLen)
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", invalid("%s 를 입력해 주세요", field)
	}
	return v, nil
}

// month 는 "YYYY-MM" 이나 빈 값만 받는다.
func month(field, value string) (string, error) {
	v := strings.TrimSpace(value)
	if v != "" && !monthPattern.MatchString(v) {
		return "", invalid("%s 는 YYYY-MM 형식이어야 합니다", field)
	}
	return v, nil
}

// date 는 "YYYY-MM-DD" 이나 빈 값만 받는다.
func date(field, value string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", nil
	}
	if _, err := time.Parse(time.DateOnly, v); err != nil {
		return "", invalid("%s 는 YYYY-MM-DD 형식이어야 합니다", field)
	}
	return v, nil
}

// period 는 시작·끝 월을 검사한다. 끝이 시작보다 앞서면 안 된다 ("YYYY-MM" 은 문자열 비교가 곧 날짜 비교).
func period(field, started, ended string) (string, string, error) {
	s, err := month(field+" 시작", started)
	if err != nil {
		return "", "", err
	}
	e, err := month(field+" 종료", ended)
	if err != nil {
		return "", "", err
	}
	if s != "" && e != "" && e < s {
		return "", "", invalid("%s 종료가 시작보다 빠릅니다", field)
	}
	return s, e, nil
}

// webURL 은 http(s) 주소만 받는다. 공개 페이지에 그대로 거는 값이라 javascript: 같은 스킴을 막는다.
func webURL(field, value string, allowMailto bool) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", nil
	}
	u, err := url.Parse(v)
	if err != nil {
		return "", invalid("%s 주소가 올바르지 않습니다", field)
	}
	switch {
	case (u.Scheme == "http" || u.Scheme == "https") && u.Host != "":
	case allowMailto && u.Scheme == "mailto" && u.Opaque != "":
	default:
		return "", invalid("%s 는 http(s) 주소여야 합니다", field)
	}
	return v, nil
}

func email(field, value string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", nil
	}
	addr, err := mail.ParseAddress(v)
	if err != nil || addr.Address != v || len(v) > 254 {
		return "", invalid("%s 형식을 확인해 주세요", field)
	}
	return v, nil
}

// phone 은 숫자만 남겨(010-1234-5678 → 01012345678) 국내 번호인지 본다.
func phone(field, value string) (string, error) {
	v := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		if r == '-' || r == ' ' || r == '.' {
			return -1
		}
		return 'x' // 숫자·구분자 말고 다른 글자가 있으면 형식 오류로
	}, strings.TrimSpace(value))
	if v == "" {
		return "", nil
	}
	if !phonePattern.MatchString(v) {
		return "", invalid("%s 형식을 확인해 주세요 (예: 010-1234-5678)", field)
	}
	return v, nil
}

func links(field string, in []domain.Link, maxCount int) ([]domain.Link, error) {
	if len(in) > maxCount {
		return nil, invalid("%s 는 %d개까지 넣을 수 있습니다", field, maxCount)
	}
	out := make([]domain.Link, 0, len(in))
	for _, l := range in {
		u, err := webURL(field, l.URL, true)
		if err != nil {
			return nil, err
		}
		label, err := text(field+" 이름", l.Label, 40)
		if err != nil {
			return nil, err
		}
		if u == "" {
			continue
		}
		if label == "" {
			label = u
		}
		out = append(out, domain.Link{Label: label, URL: u})
	}
	return out, nil
}

// words 는 공백 정리 · 빈 값 제거 · 중복 제거(대소문자 무시)를 한다.
func words(field string, in []string, maxCount, maxLen int) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, w := range in {
		v, err := text(field, w, maxLen)
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(v)
		if v == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v)
	}
	if len(out) > maxCount {
		return nil, invalid("%s 는 %d개까지 넣을 수 있습니다", field, maxCount)
	}
	return out, nil
}
