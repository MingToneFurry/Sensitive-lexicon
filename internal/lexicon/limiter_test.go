package lexicon

import (
	"testing"
	"time"
)

func TestAdaptiveLimiterAllow(t *testing.T) {
	l := NewAdaptiveLimiter(5, 1, 10, time.Second)
	defer l.Close()

	allowed := 0
	for i := 0; i < 5; i++ {
		if l.Allow() {
			allowed++
		}
	}
	if allowed == 0 {
		t.Fatal("expected some requests to be allowed")
	}
}
