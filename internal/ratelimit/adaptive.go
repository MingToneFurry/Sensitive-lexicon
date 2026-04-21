package ratelimit

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	memoryThrottleThresholdBytes = 512 << 20
	minRPS                       = 20
)

type AdaptiveLimiter struct {
	mu       sync.Mutex
	baseRPS  int
	tokens   float64
	last     time.Time
	memTS    time.Time
	memAlloc uint64
}

func New(baseRPS int) *AdaptiveLimiter {
	if baseRPS <= 0 {
		baseRPS = 100
	}
	return &AdaptiveLimiter{baseRPS: baseRPS, tokens: float64(baseRPS), last: time.Now()}
}

func (l *AdaptiveLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	rate := float64(l.currentRPS())
	elapsed := now.Sub(l.last).Seconds()
	l.tokens += elapsed * rate
	if l.tokens > rate {
		l.tokens = rate
	}
	l.last = now
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

func (l *AdaptiveLimiter) currentRPS() int {
	target := l.baseRPS
	load := linuxLoadAverage()
	if load > 0 {
		cpus := float64(runtime.NumCPU())
		ratio := load / cpus
		switch {
		case ratio > 1.5:
			target = int(float64(target) * 0.4)
		case ratio > 1.0:
			target = int(float64(target) * 0.6)
		case ratio > 0.8:
			target = int(float64(target) * 0.8)
		}
	}
	now := time.Now()
	if now.Sub(l.memTS) > time.Second {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		l.memAlloc = m.Alloc
		l.memTS = now
	}
	if l.memAlloc > memoryThrottleThresholdBytes {
		target = int(float64(target) * 0.5)
	}
	if target < minRPS {
		target = minRPS
	}
	return target
}

func linuxLoadAverage() float64 {
	f, err := os.Open("/proc/loadavg")
	if err != nil {
		return 0
	}
	defer f.Close()
	r := bufio.NewReader(f)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return 0
	}
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	return v
}
