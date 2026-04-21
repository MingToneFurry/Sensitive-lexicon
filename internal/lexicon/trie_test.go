package lexicon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindMatchesPreferLongestNonOverlap(t *testing.T) {
	trie := NewTrie()
	trie.Add("敏感")
	trie.Add("敏感词")
	trie.Add("词库")

	matches := trie.FindMatches("这是敏感词库")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Word != "敏感词" {
		t.Fatalf("expected longest word, got %s", matches[0].Word)
	}
}

func TestReplace(t *testing.T) {
	trie := NewTrie()
	trie.Add("测试")
	m := trie.FindMatches("这是测试文本")
	got := Replace("这是测试文本", m, "#", true)
	if got != "这是##文本" {
		t.Fatalf("unexpected replace result: %s", got)
	}
}

func TestLoadFromDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("词条A\n词条B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.json"), []byte("[\"词条C\",\"词条D\"]"), 0o644); err != nil {
		t.Fatal(err)
	}
	trie, count, err := LoadFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("expected 4 words, got %d", count)
	}
	if len(trie.FindMatches("这里有词条D")) != 1 {
		t.Fatal("expected match from loaded words")
	}
}
