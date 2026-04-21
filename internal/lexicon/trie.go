package lexicon

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"unicode"
)

type TrieNode struct {
	Children map[rune]*TrieNode
	End      bool
}

type Trie struct {
	Root *TrieNode
}

type Match struct {
	Start int
	End   int
}

type Engine struct {
	trie       atomic.Pointer[Trie]
	replace    atomic.Pointer[string]
	enBoundary atomic.Bool
}

func NewEngine(replace string, enableBoundary bool) *Engine {
	e := &Engine{}
	e.replace.Store(&replace)
	e.enBoundary.Store(enableBoundary)
	t := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	e.trie.Store(t)
	return e
}

func (e *Engine) SetReplaceSymbol(v string) {
	e.replace.Store(&v)
}

func (e *Engine) SetBoundary(v bool) {
	e.enBoundary.Store(v)
}

func (e *Engine) LoadDir(dir string) (int, error) {
	newTrie := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	count := 0
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(d.Name()) != ".txt" {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		n, err := loadWords(f, newTrie)
		closeErr := f.Close()
		count += n
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	e.trie.Store(newTrie)
	return count, nil
}

func loadWords(r io.Reader, trie *Trie) (int, error) {
	s := bufio.NewScanner(r)
	c := 0
	for s.Scan() {
		w := strings.TrimSpace(strings.ToLower(s.Text()))
		if w == "" || strings.HasPrefix(w, "#") {
			continue
		}
		trie.Insert([]rune(w))
		c++
	}
	return c, s.Err()
}

func (t *Trie) Insert(word []rune) {
	n := t.Root
	for _, r := range word {
		if n.Children[r] == nil {
			n.Children[r] = &TrieNode{Children: map[rune]*TrieNode{}}
		}
		n = n.Children[r]
	}
	n.End = true
}

func (e *Engine) Find(text string) []Match {
	runes := []rune(strings.ToLower(text))
	trie := e.trie.Load()
	res := make([]Match, 0, 8)
	for i := 0; i < len(runes); i++ {
		n := trie.Root
		for j := i; j < len(runes); j++ {
			next := n.Children[runes[j]]
			if next == nil {
				break
			}
			n = next
			if n.End {
				if e.enBoundary.Load() && !isBoundary(runes, i, j) {
					continue
				}
				res = append(res, Match{Start: i, End: j + 1})
			}
		}
	}
	return mergeMatches(res)
}

func mergeMatches(matches []Match) []Match {
	if len(matches) <= 1 {
		return matches
	}
	out := []Match{matches[0]}
	for _, m := range matches[1:] {
		last := &out[len(out)-1]
		if m.Start <= last.End {
			if m.End > last.End {
				last.End = m.End
			}
			continue
		}
		out = append(out, m)
	}
	return out
}

func isBoundary(runes []rune, start, end int) bool {
	left := start == 0 || !isWordRune(runes[start-1])
	right := end+1 >= len(runes) || !isWordRune(runes[end+1])
	return left && right
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (e *Engine) Contains(text string) bool {
	return len(e.Find(text)) > 0
}

func (e *Engine) Replace(text string) string {
	return e.ReplaceWithSymbol(text, "")
}

func (e *Engine) ReplaceWithSymbol(text, symbol string) string {
	runes := []rune(text)
	matches := e.Find(text)
	repVal := symbol
	if repVal == "" {
		rep := e.replace.Load()
		if rep != nil {
			repVal = *rep
		}
	}
	if repVal == "" {
		repVal = "*"
	}
	r := []rune(repVal)
	if len(r) == 0 {
		r = []rune{'*'}
	}
	for _, m := range matches {
		for i := m.Start; i < m.End && i < len(runes); i++ {
			runes[i] = r[(i-m.Start)%len(r)]
		}
	}
	return string(runes)
}
