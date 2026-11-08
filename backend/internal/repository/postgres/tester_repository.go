package postgres

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/devsixt/careerpad/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 테스터 트리거(migrations/0003_tester.sql)가 내는 SQLSTATE.
const (
	testerLockedCode  = "AE401"
	testerBlockedCode = "AE402"
)

// testerCacheTTL 동안은 같은 사용자의 테스터 여부를 다시 읽지 않는다 (막은 기능을 부를 때마다 읽으므로).
const testerCacheTTL = time.Minute

// TesterRepository 는 auth.users 의 app_metadata 로 테스터를 가리고, career.tester_reset 으로 정리한다.
// auth.users 는 공용 로그인(Supabase Auth)이라 읽기만 한다.
type TesterRepository struct {
	pool *pgxpool.Pool
	now  func() time.Time

	mu    sync.Mutex
	cache map[string]testerCacheEntry
}

type testerCacheEntry struct {
	isTester bool
	until    time.Time
}

func NewTesterRepository(pool *pgxpool.Pool) *TesterRepository {
	return &TesterRepository{pool: pool, now: time.Now, cache: map[string]testerCacheEntry{}}
}

var _ domain.TesterRepository = (*TesterRepository)(nil)

func (r *TesterRepository) IsTester(ctx context.Context, userID string) (bool, error) {
	now := r.now()
	r.mu.Lock()
	if e, ok := r.cache[userID]; ok && now.Before(e.until) {
		r.mu.Unlock()
		return e.isTester, nil
	}
	r.mu.Unlock()

	var isTester bool
	err := r.pool.QueryRow(ctx,
		`SELECT coalesce(raw_app_meta_data ->> 'role', '') = 'tester' FROM auth.users WHERE id = $1`, userID,
	).Scan(&isTester)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, translate("tester repository: is tester", err)
	}

	r.mu.Lock()
	// 오래된 항목을 가끔 비운다 (사용자가 적어 맵이 크게 자라지 않는다)
	if len(r.cache) > 1000 {
		r.cache = map[string]testerCacheEntry{}
	}
	r.cache[userID] = testerCacheEntry{isTester: isTester, until: now.Add(testerCacheTTL)}
	r.mu.Unlock()
	return isTester, nil
}

func (r *TesterRepository) Reset(ctx context.Context, userID string) (int, error) {
	var deleted int
	err := r.pool.QueryRow(ctx, `SELECT career.tester_reset($1)`, userID).Scan(&deleted)
	return deleted, translate("tester repository: reset", err)
}
