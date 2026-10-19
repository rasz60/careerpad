// Package ratelimit 는 키(IP 등)별로 일정 시간 안의 요청 수를 제한하는 메모리 카운터다.
// 서버가 한 대이고 재시작하면 초기화돼도 되는 용도(연락 폼 도배 방지)에만 쓴다.
package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	max    int
	window time.Duration
	now    func() time.Time

	mu   sync.Mutex
	hits map[string][]time.Time
}

// New 는 window 동안 키 하나당 max 번까지 허용하는 제한기를 만든다.
func New(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, now: time.Now, hits: map[string][]time.Time{}}
}

// Allow 는 이번 요청을 받아도 되면 기록하고 true 를 돌려준다.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)
	recent := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= l.max {
		l.hits[key] = recent
		return false
	}
	l.hits[key] = append(recent, now)

	// 키가 쌓이지 않게 가끔 비운다
	if len(l.hits) > 10000 {
		for k, ts := range l.hits {
			if len(ts) == 0 || !ts[len(ts)-1].After(cutoff) {
				delete(l.hits, k)
			}
		}
	}
	return true
}
