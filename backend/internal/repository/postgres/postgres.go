package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/devsixt/careerpad/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Options 는 커넥션 풀 동작을 조정한다.
type Options struct {
	// MaxConns 는 풀 크기다. 0 이면 10. Supabase 무료 플랜처럼
	// 동시 연결 한도가 작은 환경에서는 낮춰야 한다.
	MaxConns int32

	// QueryMode 는 pgx 의 쿼리 실행 방식이다.
	//   ""               포트로 자동 판단 (6543 → exec, 그 외 → cache_statement)
	//   cache_statement  named prepared statement 캐시. 직접 연결 / session 풀러용
	//   exec             unnamed prepared statement. transaction 풀러(6543)용
	//   simple           simple protocol. 위 둘이 모두 거부될 때의 최후 수단
	QueryMode string
}

// supavisorTransactionPort 는 Supabase Supavisor 의 transaction 모드 포트다.
// 이 모드는 named prepared statement 를 유지할 수 없어 pgx 기본 캐시가 깨진다.
const supavisorTransactionPort = 6543

// Connect 는 커넥션 풀을 열고 ping 으로 확인한다.
func Connect(ctx context.Context, databaseURL string, opts Options) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse config: %w", err)
	}

	cfg.MaxConns = 10
	if opts.MaxConns > 0 {
		cfg.MaxConns = opts.MaxConns
	}
	cfg.MaxConnLifetime = time.Hour

	mode := opts.QueryMode
	if mode == "" {
		mode = "cache_statement"
		if cfg.ConnConfig.Port == supavisorTransactionPort {
			mode = "exec"
		}
	}
	if err := applyQueryMode(cfg.ConnConfig, mode); err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping %s:%d: %w", cfg.ConnConfig.Host, cfg.ConnConfig.Port, err)
	}
	return pool, nil
}

func applyQueryMode(cfg *pgx.ConnConfig, mode string) error {
	switch mode {
	case "cache_statement":
		cfg.DefaultQueryExecMode = pgx.QueryExecModeCacheStatement
	case "exec":
		// 커넥션이 매 트랜잭션마다 다른 백엔드로 갈 수 있으므로 캐시를 모두 끈다.
		cfg.DefaultQueryExecMode = pgx.QueryExecModeExec
		cfg.StatementCacheCapacity = 0
		cfg.DescriptionCacheCapacity = 0
	case "simple":
		cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
		cfg.StatementCacheCapacity = 0
		cfg.DescriptionCacheCapacity = 0
	default:
		return fmt.Errorf("postgres: unknown DB_QUERY_MODE %q (cache_statement | exec | simple): %w", mode, domain.ErrInvalidInput)
	}
	return nil
}

// translate 는 pgx 에러를 도메인 센티널 에러로 바꾼다.
func translate(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, domain.ErrNotFound)
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%s: %s: %w", op, pgErr.ConstraintName, domain.ErrConflict)
		case "23503": // foreign_key_violation
			return fmt.Errorf("%s: %s: %w", op, pgErr.ConstraintName, domain.ErrInvalidInput)
		case "23514": // check_violation
			return fmt.Errorf("%s: %s: %w", op, pgErr.ConstraintName, domain.ErrInvalidInput)
		case "22P02": // invalid_text_representation — 주소의 id 가 uuid 모양이 아니면 그런 항목은 없다
			return fmt.Errorf("%s: %w", op, domain.ErrNotFound)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}
