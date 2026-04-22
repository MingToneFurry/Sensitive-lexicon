package lexicon

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
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
	category   atomic.Pointer[map[string]*Trie]
	replace    atomic.Pointer[string]
	enBoundary atomic.Bool
}

func NewEngine(replace string, enableBoundary bool) *Engine {
	e := &Engine{}
	e.replace.Store(&replace)
	e.enBoundary.Store(enableBoundary)
	t := &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
	e.trie.Store(t)
	c := map[string]*Trie{}
	e.category.Store(&c)
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
	categoryTrie := map[string]*Trie{}
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
		categoryName := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		ct := categoryTrie[categoryName]
		if ct == nil {
			ct = &Trie{Root: &TrieNode{Children: map[rune]*TrieNode{}}}
			categoryTrie[categoryName] = ct
		}
		n, err := loadWords(f, newTrie, ct)
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
	e.category.Store(&categoryTrie)
	return count, nil
}

// loadWords inserts each normalized word into all provided tries.
// It is used to update the global trie and the per-category trie in one pass.
func loadWords(r io.Reader, tries ...*Trie) (int, error) {
	s := bufio.NewScanner(r)
	c := 0
	for s.Scan() {
		w := strings.TrimSpace(strings.ToLower(s.Text()))
		if w == "" || strings.HasPrefix(w, "#") {
			continue
		}
		word := []rune(w)
		for _, trie := range tries {
			trie.Insert(word)
		}
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
	return findWithTrie(runes, trie, e.enBoundary.Load())
}

func findWithTrie(runes []rune, trie *Trie, enableBoundary bool) []Match {
	if trie == nil || trie.Root == nil {
		return nil
	}
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
				if enableBoundary && !isBoundary(runes, i, j) {
					continue
				}
				res = append(res, Match{Start: i, End: j + 1})
			}
		}
	}
	return mergeMatches(res)
}

func (e *Engine) CategoryScores(text string) map[string]float64 {
	runes := []rune(strings.ToLower(text))
	totalRunes := utf8.RuneCountInString(text)
	if totalRunes == 0 {
		return map[string]float64{}
	}
	categoryTrie := e.category.Load()
	if categoryTrie == nil {
		return map[string]float64{}
	}
	scores := make(map[string]float64)
	for categoryName, trie := range *categoryTrie {
		matches := findWithTrie(runes, trie, e.enBoundary.Load())
		matchedRunes := 0
		for _, m := range matches {
			if m.End > m.Start {
				matchedRunes += (m.End - m.Start)
			}
		}
		if matchedRunes == 0 {
			continue
		}
		scores[categoryName] = float64(matchedRunes) / float64(totalRunes)
	}
	return scores
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
	return e.ReplaceWithMatches(text, symbol, e.Find(text))
}

func (e *Engine) ReplaceWithMatches(text, symbol string, matches []Match) string {
	runes := []rune(text)
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
