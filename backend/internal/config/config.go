package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Host        string
	Port        string
	DatabaseURL string
	AllowOrigin string

	// DBMaxConns 는 풀 크기, DBQueryMode 는 pgx 실행 방식이다 (운동기록 백엔드와 같은 규칙).
	DBMaxConns  int32
	DBQueryMode string

	// SupabaseURL 은 로그인 토큰 검증에 쓴다 (https://<ref>.supabase.co).
	SupabaseURL string
	// PublicURL 은 알림 메일에 넣는 사이트 주소다.
	PublicURL string

	// 사람인 Open API 접근 키. 없으면 사람인은 검색 링크만 준다.
	SaraminAccessKey string

	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	MailFrom     string

	SolapiAPIKey    string
	SolapiAPISecret string
	SMSFrom         string
	SMSDailyLimit   int

	// 방문자 IP 당 연락 폼 허용 횟수 (1시간)
	ContactPerHour int
}

// Load 는 환경변수에서 설정을 읽는다. 필수값이 없으면 에러를 반환한다.
func Load() (*Config, error) {
	cfg := &Config{
		Host:             env("HOST", "127.0.0.1"),
		Port:             env("PORT", "8092"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		AllowOrigin:      env("ALLOW_ORIGIN", "http://localhost:5173"),
		DBMaxConns:       int32(envInt("DB_MAX_CONNS", 3)),
		DBQueryMode:      os.Getenv("DB_QUERY_MODE"),
		SupabaseURL:      strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		PublicURL:        strings.TrimRight(env("PUBLIC_URL", "https://ae2p.duckdns.org"), "/"),
		SaraminAccessKey: os.Getenv("SARAMIN_ACCESS_KEY"),
		SMTPHost:         os.Getenv("SMTP_HOST"),
		SMTPPort:         env("SMTP_PORT", "587"),
		SMTPUsername:     os.Getenv("SMTP_USERNAME"),
		SMTPPassword:     os.Getenv("SMTP_PASSWORD"),
		MailFrom:         os.Getenv("MAIL_FROM"),
		SolapiAPIKey:     os.Getenv("SOLAPI_API_KEY"),
		SolapiAPISecret:  os.Getenv("SOLAPI_API_SECRET"),
		SMSFrom:          os.Getenv("SMS_FROM"),
		SMSDailyLimit:    envInt("SMS_DAILY_LIMIT", 20),
		ContactPerHour:   envInt("CONTACT_PER_HOUR", 5),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}
	if !strings.HasPrefix(cfg.SupabaseURL, "https://") {
		return nil, fmt.Errorf("config: SUPABASE_URL is required (https://<ref>.supabase.co)")
	}
	if strings.Contains(cfg.DatabaseURL, "db.") && strings.Contains(cfg.DatabaseURL, ".supabase.co") {
		slog.Warn("DATABASE_URL points at the Supabase direct host, which is IPv6-only",
			"hint", "use the Supavisor pooler host (*.pooler.supabase.com) if this network has no IPv6")
	}
	if cfg.SMTPHost == "" {
		slog.Warn("SMTP_HOST is empty: contact messages are stored but no mail notification is sent")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil && n > 0 {
		return n
	}
	return fallback
}
