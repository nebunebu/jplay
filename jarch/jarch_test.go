package jarch

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The *.golden.json files were produced by the original bash/pup/jq jarch
// against the cached pages in testdata/; the Go port must reproduce them
// byte-for-byte.

func offline(t *testing.T) *Client {
	t.Helper()
	c := New("testdata")
	c.Offline = true
	c.IndexTTL = 0
	return c
}

func checkGolden(t *testing.T, golden string, v any, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", golden))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		out := filepath.Join(t.TempDir(), golden)
		os.WriteFile(out, got, 0o644)
		t.Errorf("%s mismatch; got written to %s", golden, out)
	}
}

func TestSeasons(t *testing.T) {
	v, err := offline(t).Seasons()
	checkGolden(t, "listseasons.golden.json", v, err)
}

func TestSeason(t *testing.T) {
	for _, id := range []string{"42", "pcj"} {
		v, err := offline(t).Season(id)
		checkGolden(t, "season-"+id+".golden.json", v, err)
	}
}

func TestGame(t *testing.T) {
	for _, id := range []int{6789, 6379} {
		v, err := offline(t).Game(id)
		checkGolden(t, map[int]string{6789: "6789.golden.json", 6379: "6379.golden.json"}[id], v, err)
	}
}

func TestPlayer(t *testing.T) {
	v, err := offline(t).Player(5162)
	checkGolden(t, "player-5162.golden.json", v, err)
}

func TestOfflineMiss(t *testing.T) {
	if _, err := offline(t).Game(1); err == nil {
		t.Fatal("expected error for uncached page in offline mode")
	}
}

func TestCLI(t *testing.T) {
	t.Setenv("JARCH_DIR", "testdata")
	var out, errb bytes.Buffer
	if code := RunCLI([]string{"game", "6789", "--stdout"}, "unused", &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	want, _ := os.ReadFile("testdata/6789.golden.json")
	if !bytes.Equal(out.Bytes(), want) {
		t.Error("CLI output differs from golden")
	}
	if code := RunCLI([]string{"game", "abc"}, "unused", &out, &errb); code == 0 {
		t.Error("expected failure for non-integer game id")
	}
}

func TestAssignNicknames(t *testing.T) {
	names := func(ns ...string) []Contestant {
		var cs []Contestant
		for _, n := range ns {
			cs = append(cs, Contestant{Name: n})
		}
		return cs
	}
	cases := []struct {
		name  string
		cs    []Contestant
		nicks []string // podium order (reverse of contestant order)
		want  []string
	}{
		{"first names", names("Carolyn Meek", "Leonard Schmidt", "Hansford Epes"),
			[]string{"Hansford", "Leonard", "Carolyn"}, []string{"Carolyn", "Leonard", "Hansford"}},
		{"shortened", names("Matthew Amodio", "Daniel Pecoraro", "Adriana Granados"),
			[]string{"Adriana", "Daniel", "Matt"}, []string{"Matt", "Daniel", "Adriana"}},
		{"shared first name", names("Mike Smith", "Ann Lee", "Mike Jones"),
			[]string{"Mike J.", "Ann", "Mike S."}, []string{"Mike S.", "Ann", "Mike J."}},
		{"unrelated nickname", names("Robert Ford", "Ann Lee", "Cat Wu"),
			[]string{"Cat", "Ann", "Bob"}, []string{"Bob", "Ann", "Cat"}},
	}
	for _, c := range cases {
		assignNicknames(c.cs, c.nicks)
		for i, want := range c.want {
			if got := c.cs[i].Nickname; got == nil || *got != want {
				t.Errorf("%s: contestant %d nickname = %v, want %q", c.name, i, got, want)
			}
		}
	}
}

func TestGameIDForEpisode(t *testing.T) {
	c := offline(t)
	if id, err := c.GameIDForEpisode("pcj", 49); err != nil || id != 9454 {
		t.Errorf("pcj/49 = %d, %v; want 9454", id, err)
	}
	if _, err := c.GameIDForEpisode("pcj", 999999); err == nil {
		t.Error("expected error for unknown episode")
	}
	// CLI form: resolves via the season page, then needs the game page,
	// which is not cached -> offline-style failure is fine; check the
	// resolution error path instead.
	t.Setenv("JARCH_DIR", "testdata")
	var out, errb bytes.Buffer
	if code := RunCLI([]string{"game", "pcj/999999", "--stdout"}, "", &out, &errb); code == 0 ||
		!bytes.Contains(errb.Bytes(), []byte("no episode #999999")) {
		t.Errorf("code %d, stderr %q", code, errb.String())
	}
}
