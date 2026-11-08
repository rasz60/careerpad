package postgres

import (
	"errors"
	"testing"

	"github.com/devsixt/careerpad/backend/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestTranslateTesterCodes(t *testing.T) {
	cases := map[string]error{
		testerLockedCode:  domain.ErrTesterLocked,
		testerBlockedCode: domain.ErrTesterBlocked,
	}
	for code, want := range cases {
		err := translate("session repository: update", &pgconn.PgError{Code: code, Message: "기본 데이터는 테스터가 바꿀 수 없어요"})
		if !errors.Is(err, want) {
			t.Errorf("translate(%s) = %v, want %v", code, err, want)
		}
	}
}
