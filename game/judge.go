package game

import (
	"sort"
	"strings"
	"unicode"
)

// Verdict is the outcome of judging a player answer.
type Verdict int

const (
	VerdictCorrect   Verdict = iota // exact normalized match, no prompt
	VerdictFuzzy                    // close match, confirm with the player
	VerdictIncorrect                // no match
)

// JudgeOptions mirror the gameplay config knobs.
type JudgeOptions struct {
	Threshold            int // 0-100, similarity required for a fuzzy prompt
	StripArticles        bool
	StripQuestionFraming bool
}

var questionFramings = []string{
	"what is ", "who is ", "what are ", "who are ",
	"what was ", "who was ", "where is ", "where was ",
	"whats ", "whos ", // post-punctuation-strip forms of "what's"/"who's"
}

var articles = []string{"a ", "an ", "the "}

// Normalize canonicalizes an answer string for comparison.
func Normalize(s string, opts JudgeOptions) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "&", " and ") // "Jobs & Wozniak" == "Jobs and Wozniak"
	s = strings.ReplaceAll(s, "+", " and ")
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			b.WriteRune(r)
		case unicode.IsSpace(r) || r == '-' || r == '/':
			b.WriteByte(' ')
		}
	}
	s = strings.Join(strings.Fields(b.String()), " ")

	// Framing before articles so "what is the x" reduces to "x".
	if opts.StripQuestionFraming {
		for _, p := range questionFramings {
			if strings.HasPrefix(s, p) {
				s = s[len(p):]
				break
			}
		}
	}
	if opts.StripArticles {
		for _, a := range articles {
			if strings.HasPrefix(s, a) {
				s = s[len(a):]
				break
			}
		}
	}
	return s
}

// tokenSet sorts the words of an already-normalized string, dropping
// interior articles so "old man and sea" matches "old man and the sea".
func tokenSet(s string) string {
	var toks []string
	for _, t := range strings.Fields(s) {
		if t == "a" || t == "an" || t == "the" {
			continue
		}
		toks = append(toks, t)
	}
	sort.Strings(toks)
	return strings.Join(toks, " ")
}

// Levenshtein computes edit distance over runes.
func Levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	if len(rb) == 0 {
		return len(ra)
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, min(cur[j-1]+1, prev[j-1]+cost))
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// Judge compares a player answer to the correct response.
func Judge(answer, correct string, opts JudgeOptions) Verdict {
	na := Normalize(answer, opts)
	nc := Normalize(correct, opts)
	if na == "" {
		return VerdictIncorrect
	}
	if na == nc {
		return VerdictCorrect
	}

	dist := Levenshtein(na, nc)
	// Token-set comparison catches word-order differences in multi-word answers.
	if ts := Levenshtein(tokenSet(na), tokenSet(nc)); ts < dist {
		dist = ts
	}
	if dist == 0 {
		return VerdictFuzzy // same words, different order: confirm
	}

	allowed := float64(len([]rune(nc))) * (1 - float64(opts.Threshold)/100)
	// Floor the allowance so typos and phonetic spellings still get the
	// confirm prompt ("denvr"/"Denver", "organatan"/"orangutan") even at
	// high thresholds. This only gates the y/n prompt, never auto-accepts,
	// so leniency here costs one keypress, not a wrong score.
	if n := len([]rune(nc)); opts.Threshold < 100 && n >= 4 {
		if floor := float64(n) / 3; allowed < floor {
			allowed = floor
		}
	}
	if float64(dist) <= allowed {
		return VerdictFuzzy
	}
	return VerdictIncorrect
}
