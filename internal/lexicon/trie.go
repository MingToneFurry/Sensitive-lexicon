package lexicon

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Match struct {
	Word  string `json:"word"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

type node struct {
	children map[rune]*node
	end      bool
}

type Trie struct {
	root *node
}

func NewTrie() *Trie {
	return &Trie{root: &node{children: make(map[rune]*node)}}
}

func (t *Trie) Add(word string) {
	w := strings.TrimSpace(word)
	if w == "" {
		return
	}
	curr := t.root
	for _, r := range []rune(w) {
		next, ok := curr.children[r]
		if !ok {
			next = &node{children: make(map[rune]*node)}
			curr.children[r] = next
		}
		curr = next
	}
	curr.end = true
}

func LoadFromDir(root string) (*Trie, int, error) {
	trie := NewTrie()
	seen := make(map[string]struct{})
	count := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".txt", ".json":
		default:
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if ext == ".json" {
			if n, err := loadJSONWords(trie, seen, f); err == nil {
				count += n
				return nil
			}
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return err
			}
		}
		n, err := loadTextWords(trie, seen, f)
		count += n
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	if count == 0 {
		return nil, 0, errors.New("no lexicon words loaded")
	}
	return trie, count, nil
}

func loadTextWords(trie *Trie, seen map[string]struct{}, r io.Reader) (int, error) {
	s := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	s.Buffer(buf, 4*1024*1024)
	count := 0
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		trie.Add(line)
		count++
	}
	return count, s.Err()
}

func loadJSONWords(trie *Trie, seen map[string]struct{}, r io.Reader) (int, error) {
	var arr []string
	if err := json.NewDecoder(r).Decode(&arr); err != nil {
		return 0, err
	}
	count := 0
	for _, w := range arr {
		line := strings.TrimSpace(w)
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		trie.Add(line)
		count++
	}
	return count, nil
}

func (t *Trie) FindMatches(text string) []Match {
	runes := []rune(text)
	matches := make([]Match, 0, 16)
	for i := 0; i < len(runes); i++ {
		t.dfs(runes, i, i, t.root, &matches)
	}
	if len(matches) == 0 {
		return nil
	}
	return normalizeMatches(matches)
}

func (t *Trie) dfs(runes []rune, start, idx int, curr *node, matches *[]Match) {
	if idx >= len(runes) {
		return
	}
	next, ok := curr.children[runes[idx]]
	if !ok {
		return
	}
	if next.end {
		*matches = append(*matches, Match{Word: string(runes[start : idx+1]), Start: start, End: idx + 1})
	}
	t.dfs(runes, start, idx+1, next, matches)
}

func normalizeMatches(matches []Match) []Match {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Start != matches[j].Start {
			return matches[i].Start < matches[j].Start
		}
		return (matches[i].End - matches[i].Start) > (matches[j].End - matches[j].Start)
	})

	out := make([]Match, 0, len(matches))
	lastEnd := -1
	for i := 0; i < len(matches); {
		best := matches[i]
		j := i + 1
		for j < len(matches) && matches[j].Start == best.Start {
			j++
		}
		if best.Start >= lastEnd {
			out = append(out, best)
			lastEnd = best.End
		}
		i = j
	}
	return out
}

func Replace(text string, matches []Match, symbol string, keepLength bool) string {
	if len(matches) == 0 {
		return text
	}
	if symbol == "" {
		symbol = "*"
	}
	runes := []rune(text)
	var b strings.Builder
	b.Grow(len(runes))
	cursor := 0
	for _, m := range matches {
		if m.Start < cursor || m.Start > len(runes) || m.End > len(runes) {
			continue
		}
		b.WriteString(string(runes[cursor:m.Start]))
		if keepLength {
			for i := 0; i < m.End-m.Start; i++ {
				b.WriteString(symbol)
			}
		} else {
			b.WriteString(symbol)
		}
		cursor = m.End
	}
	b.WriteString(string(runes[cursor:]))
	return b.String()
}
