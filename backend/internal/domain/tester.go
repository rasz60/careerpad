package domain

import "context"

// TesterRepository 는 공용 테스터 계정(P-0004) 여부를 읽고, 테스터가 새로 만든 데이터를 지운다.
//
// 테스터 계정은 포털 로그인 화면의 '테스터로 입장하기'로 누구나 들어온다.
// 관리자가 잠근 기본 데이터는 남고, 그 뒤에 만든 데이터는 로그아웃 · 매일 새벽 정리 때 지워진다.
type TesterRepository interface {
	// IsTester 는 auth.users 의 app_metadata.role 이 tester 인지 본다.
	IsTester(ctx context.Context, userID string) (bool, error)
	// Reset 은 기본 데이터 기준 시각 이후에 만든 테스터 데이터를 지우고, 지운 행 수를 돌려준다.
	Reset(ctx context.Context, userID string) (int, error)
}
