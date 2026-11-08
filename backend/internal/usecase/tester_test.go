package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/devsixt/careerpad/backend/internal/domain"
)

type fakeTesters struct {
	testers map[string]bool
	err     error
	resets  []string
}

func (f *fakeTesters) IsTester(_ context.Context, userID string) (bool, error) {
	return f.testers[userID], f.err
}

func (f *fakeTesters) Reset(_ context.Context, userID string) (int, error) {
	f.resets = append(f.resets, userID)
	return 7, nil
}

func TestTesterGuard(t *testing.T) {
	repo := &fakeTesters{testers: map[string]bool{"tester": true}}
	uc := NewTester(repo)

	if err := uc.Guard(context.Background(), "tester"); !errors.Is(err, domain.ErrTesterBlocked) {
		t.Errorf("tester: err = %v, want ErrTesterBlocked", err)
	}
	if err := uc.Guard(context.Background(), "member"); err != nil {
		t.Errorf("member: err = %v, want nil", err)
	}

	// 테스터 여부를 못 읽으면 막지도 통과시키지도 않고 에러를 돌려준다
	boom := errors.New("db down")
	repo.err = boom
	if err := uc.Guard(context.Background(), "member"); !errors.Is(err, boom) {
		t.Errorf("db error: err = %v", err)
	}
}

func TestTesterReset(t *testing.T) {
	repo := &fakeTesters{testers: map[string]bool{"tester": true}}
	uc := NewTester(repo)

	n, err := uc.Reset(context.Background(), "member")
	if err != nil || n != 0 || len(repo.resets) != 0 {
		t.Errorf("member: n=%d err=%v resets=%v — 테스터가 아니면 정리 함수를 부르지 않는다", n, err, repo.resets)
	}
	n, err = uc.Reset(context.Background(), "tester")
	if err != nil || n != 7 || len(repo.resets) != 1 || repo.resets[0] != "tester" {
		t.Errorf("tester: n=%d err=%v resets=%v", n, err, repo.resets)
	}
}
