package lexicon

import (
	"runtime"
	"sync"
	"time"
)

type AdaptiveLimiter struct {
	mu       sync.Mutex
	baseRPS  int64
	minRPS   int64
	maxRPS   int64
	rps      int64
	tokens   float64
	updated  time.Time
	interval time.Duration
	stop     chan struct{}
}

func NewAdaptiveLimiter(baseRPS, minRPS, maxRPS int64, interval time.Duration) *AdaptiveLimiter {
	if baseRPS <= 0 {
		baseRPS = 100
	}
	if minRPS <= 0 {
		minRPS = 10
	}
	if maxRPS < baseRPS {
		maxRPS = baseRPS
	}
	l := &AdaptiveLimiter{baseRPS: baseRPS, minRPS: minRPS, maxRPS: maxRPS, rps: baseRPS, tokens: float64(baseRPS), updated: time.Now(), interval: interval, stop: make(chan struct{})}
	if l.interval <= 0 {
		l.interval = 2 * time.Second
	}
	go l.loop()
	return l
}

func (l *AdaptiveLimiter) loop() {
	tk := time.NewTicker(l.interval)
	defer tk.Stop()
	for {
		select {
		case <-tk.C:
			l.recalculateRPS()
		case <-l.stop:
			return
		}
	}
}

func (l *AdaptiveLimiter) recalculateRPS() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	goroutines := runtime.NumGoroutine()
	newRPS := l.baseRPS

	if goroutines > 500 {
		newRPS /= 4
	} else if goroutines > 200 {
		newRPS /= 2
	}
	if m.Sys > 0 && m.Alloc > (m.Sys*80/100) {
		newRPS /= 2
	}
	if newRPS < l.minRPS {
		newRPS = l.minRPS
	}
	if newRPS > l.maxRPS {
		newRPS = l.maxRPS
	}

	l.mu.Lock()
	l.rps = newRPS
	if l.tokens > float64(newRPS) {
		l.tokens = float64(newRPS)
	}
	l.mu.Unlock()
}

func (l *AdaptiveLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(l.updated).Seconds()
	l.updated = now
	l.tokens += elapsed * float64(l.rps)
	if l.tokens > float64(l.rps) {
		l.tokens = float64(l.rps)
	}
	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}

func (l *AdaptiveLimiter) CurrentRPS() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rps
}

func (l *AdaptiveLimiter) Close() {
	close(l.stop)
}
