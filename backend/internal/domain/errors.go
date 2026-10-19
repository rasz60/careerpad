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

// ErrTooManyRequests 는 같은 곳에서 너무 자주 보낸 요청이다 (연락 폼 도배 방지).
var ErrTooManyRequests = errors.New("too many requests")
