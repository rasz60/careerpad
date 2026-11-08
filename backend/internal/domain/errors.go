package domain

import "errors"

// 유스케이스 계층이 반환하는 센티널 에러. transport 계층에서 HTTP 상태로 매핑한다.
var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource already exists")
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

// 공용 테스터 계정(P-0004) 에러. 둘 다 403 이고, 화면에는 정해진 문장을 보여 준다.
var (
	// ErrTesterLocked 는 테스터가 기본 데이터(관리자가 잠근 데이터)를 바꾸려 한 것이다. DB 트리거 오류 AE401.
	ErrTesterLocked = errors.New("tester baseline data is locked")
	// ErrTesterBlocked 는 테스터 계정에 막은 기능이다 (채용공고 · 메시지 · 알림 설정). DB 트리거 오류 AE402.
	ErrTesterBlocked = errors.New("feature blocked for tester")
)

// ErrTooManyRequests 는 같은 곳에서 너무 자주 보낸 요청이다 (연락 폼 도배 방지).
var ErrTooManyRequests = errors.New("too many requests")
