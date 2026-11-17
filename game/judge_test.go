package game

import "testing"

var defaultOpts = JudgeOptions{Threshold: 85, StripArticles: true, StripQuestionFraming: true}

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"What is The Old Man and the Sea?", "old man and the sea"},
		{"  the   HYDROGEN!! ", "hydrogen"},
		{"Who was Eleanor Roosevelt", "eleanor roosevelt"},
		{"\"Don't Worry, Be Happy\"", "dont worry be happy"},
		{"What is Pegasus?", "pegasus"},
	}
	for _, c := range cases {
		if got := Normalize(c.in, defaultOpts); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJudge(t *testing.T) {
	cases := []struct {
		answer, correct string
		want            Verdict
	}{
		{"what is hydrogen", "hydrogen", VerdictCorrect},
		{"Hydrogen", "hydrogen", VerdictCorrect},
		{"the old man and the sea", "The Old Man and the Sea", VerdictCorrect},
		{"the old man and sea", "The Old Man and the Sea", VerdictFuzzy},
		{"polonum", "polonium", VerdictFuzzy},
		{"helium", "hydrogen", VerdictIncorrect},
		{"", "hydrogen", VerdictIncorrect},
		{"sea and the old man", "the old man and the sea", VerdictFuzzy}, // token order
		{"denvr", "Denver", VerdictFuzzy},
		{"for whom the bell tolls", "The Old Man and the Sea", VerdictIncorrect},
	}
	for _, c := range cases {
		if got := Judge(c.answer, c.correct, defaultOpts); got != c.want {
			t.Errorf("Judge(%q, %q) = %v, want %v", c.answer, c.correct, got, c.want)
		}
	}
}

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0}, {"abc", "abc", 0}, {"abc", "abd", 1},
		{"kitten", "sitting", 3}, {"héllo", "hello", 1},
	}
	for _, c := range cases {
		if got := Levenshtein(c.a, c.b); got != c.want {
			t.Errorf("Levenshtein(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestJudgeNotesFixes(t *testing.T) {
	cases := []struct {
		answer, correct string
		want            Verdict
	}{
		{"Jobs and Wozniak", "Jobs & Wozniak", VerdictCorrect},
		{"organatan", "orangutan", VerdictFuzzy},
		{"patty hurst", "Patty Hearst", VerdictFuzzy},
	}
	for _, c := range cases {
		if got := Judge(c.answer, c.correct, defaultOpts); got != c.want {
			t.Errorf("Judge(%q, %q) = %v, want %v", c.answer, c.correct, got, c.want)
		}
	}
}
