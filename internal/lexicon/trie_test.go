package lexicon

import (
	"math"
	"testing"
)

func TestReplaceAndContains(t *testing.T) {
	e := NewEngine("*", false)
	tr := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	tr.Insert([]rune("坏词"))
	e.trie.Store(tr)

	if !e.Contains("这是坏词测试") {
		t.Fatalf("expected contains")
	}
	got := e.Replace("这是坏词测试")
	if got != "这是**测试" {
		t.Fatalf("unexpected replace result: %s", got)
	}
}

func TestBoundaryReduceFalsePositive(t *testing.T) {
	e := NewEngine("*", true)
	tr := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	tr.Insert([]rune("abc"))
	e.trie.Store(tr)

	if e.Contains("xabcx") {
		t.Fatalf("should not match inside word when boundary enabled")
	}
	if !e.Contains("abc,") {
		t.Fatalf("should match at boundary")
	}
}

func TestCategoryScores(t *testing.T) {
	e := NewEngine("*", false)
	all := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	all.Insert([]rune("坏词"))
	all.Insert([]rune("色词"))
	e.trie.Store(all)

	political := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	political.Insert([]rune("坏词"))
	adult := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	adult.Insert([]rune("色词"))
	categories := map[string]*Trie{
		"政治类型": political,
		"色情类型": adult,
	}
	e.category.Store(&categories)

	scores := e.CategoryScores("这是坏词和色词")
	if len(scores) != 2 {
		t.Fatalf("expected 2 category scores, got %d", len(scores))
	}
	if scores["政治类型"] <= 0 {
		t.Fatalf("expected political score > 0, got %v", scores["政治类型"])
	}
	if scores["色情类型"] <= 0 {
		t.Fatalf("expected adult score > 0, got %v", scores["色情类型"])
	}
}

func TestCategoryScoresUsesOriginalRuneCountDenominator(t *testing.T) {
	e := NewEngine("*", false)
	all := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	all.Insert([]rune("坏词"))
	e.trie.Store(all)

	category := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	category.Insert([]rune("坏词"))
	categories := map[string]*Trie{"a": category}
	e.category.Store(&categories)

	scores := e.CategoryScores("İ坏词")
	want := 2.0 / 3.0
	if diff := math.Abs(scores["a"] - want); diff > 1e-9 {
		t.Fatalf("expected score %.12f, got %.12f", want, scores["a"])
	}
}

// TestFullWidthNormalization verifies that full-width characters in input are
// matched against their half-width equivalents stored in the trie.
func TestFullWidthNormalization(t *testing.T) {
	e := NewEngine("*", false)
	// Store half-width word.
	tr := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	tr.Insert([]rune("bad"))
	e.trie.Store(tr)

	// Input uses full-width characters ｂａｄ.
	if !e.Contains("this is ｂａｄ word") {
		t.Fatalf("expected full-width input to match half-width trie entry")
	}
	got := e.Replace("ｂａｄ")
	if got != "***" {
		t.Fatalf("unexpected replace result: %q", got)
	}
}

// TestSkipNoiseCharsMatching verifies that noise characters inserted between
// sensitive-word characters are transparently skipped when skipNoiseChars is on.
func TestSkipNoiseCharsMatching(t *testing.T) {
	e := NewEngine("*", false)
	e.SetSkipNoiseChars(true)
	tr := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	tr.Insert([]rune("色情"))
	e.trie.Store(tr)

	tests := []struct {
		input string
		want  bool
	}{
		{"色情", true},
		{"色 情", true},
		{"色*情", true},
		{"色.情", true},
		{"色_情", true},
	}
	for _, tc := range tests {
		if got := e.Contains(tc.input); got != tc.want {
			t.Errorf("Contains(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestSkipNoiseCharsDisabled verifies that without skipNoiseChars, inserted
// noise characters prevent a match.
func TestSkipNoiseCharsDisabled(t *testing.T) {
	e := NewEngine("*", false)
	e.SetSkipNoiseChars(false)
	tr := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	tr.Insert([]rune("色情"))
	e.trie.Store(tr)

	if e.Contains("色 情") {
		t.Fatalf("expected no match when skipNoiseChars is off")
	}
	if !e.Contains("色情") {
		t.Fatalf("expected match for direct input")
	}
}

// TestInvisibleCharRemoval verifies that zero-width characters are always
// stripped, even without skipNoiseChars.
func TestInvisibleCharRemoval(t *testing.T) {
	e := NewEngine("*", false)
	e.SetSkipNoiseChars(false)
	tr := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	tr.Insert([]rune("坏词"))
	e.trie.Store(tr)

	// Insert zero-width space between characters.
	input := "坏\u200B词"
	if !e.Contains(input) {
		t.Fatalf("expected zero-width space to be stripped and word detected")
	}
}

// TestReplaceWithNoiseCharsSpansCoverNoise ensures that the replaced output
// covers the noise characters that were skipped during matching.
func TestReplaceWithNoiseCharsSpansCoverNoise(t *testing.T) {
	e := NewEngine("*", false)
	e.SetSkipNoiseChars(true)
	tr := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	tr.Insert([]rune("色情"))
	e.trie.Store(tr)

	got := e.Replace("色*情")
	// The match should cover all three original runes: 色, *, 情.
	if got != "***" {
		t.Fatalf("expected '***', got %q", got)
	}
}
