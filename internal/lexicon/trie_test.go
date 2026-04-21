package lexicon

import "testing"

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
