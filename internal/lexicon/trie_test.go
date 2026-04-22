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
