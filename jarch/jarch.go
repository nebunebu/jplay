// Package jarch fetches pages from J! Archive, caches them verbatim on disk,
// and parses them into JSON-friendly structs.
//
// Pipeline per page: fetch (HTTP) -> cache (.html on disk) -> parse -> output.
// Parsing always reads from the cache, so fetching and parsing are
// independently re-runnable and parsers can be developed offline against the
// fixtures in testdata/.
//
// J! Archive's disclaimer prohibits scraping. jarch is for personal,
// low-volume, incremental use only: it fetches only what it is asked for,
// cache-first, rate-limited, with a descriptive User-Agent.
package jarch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	BaseURL   = "https://j-archive.com/"
	UserAgent = "jarch/0.1 (personal, low-volume archival tool; not a crawler)"

	DefaultFetchDelay = 2 * time.Second
	DefaultIndexTTL   = 12 * time.Hour
)

// Client fetches and parses J! Archive pages through an on-disk cache.
type Client struct {
	CacheDir   string
	FetchDelay time.Duration // slept before every network fetch
	// IndexTTL is how long cached index pages (seasons list, season episode
	// lists) stay fresh; those pages gain episodes as the show airs. Game and
	// player pages are immutable and cached forever. 0 caches index pages
	// forever too.
	IndexTTL time.Duration
	Refresh  bool      // ignore the cache and re-fetch
	Offline  bool      // never touch the network; a cache miss is an error
	Warn     io.Writer // parser warnings; nil discards them
	HTTP     *http.Client
}

// New returns a Client caching in cacheDir, with JARCH_FETCH_DELAY (seconds)
// and JARCH_INDEX_TTL (seconds) honoured when set.
func New(cacheDir string) *Client {
	c := &Client{
		CacheDir:   cacheDir,
		FetchDelay: DefaultFetchDelay,
		IndexTTL:   DefaultIndexTTL,
		HTTP:       &http.Client{Timeout: 60 * time.Second},
	}
	if d, ok := envSeconds("JARCH_FETCH_DELAY"); ok {
		c.FetchDelay = d
	}
	if d, ok := envSeconds("JARCH_INDEX_TTL"); ok {
		c.IndexTTL = d
	}
	return c
}

func envSeconds(name string) (time.Duration, bool) {
	v := os.Getenv(name)
	if v == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(f * float64(time.Second)), true
}

// Page names (relative to BaseURL, and the cache file names).
func SeasonsPage() string                   { return "listseasons.php" }
func SeasonPage(id string) string           { return "showseason.php?season=" + id }
func GamePage(id int) string                { return fmt.Sprintf("showgame.php?game_id=%d", id) }
func PlayerPage(id int) string              { return fmt.Sprintf("showplayer.php?player_id=%d", id) }
func (c *Client) cachePath(p string) string { return filepath.Join(c.CacheDir, p) }

// IsCached reports whether page is in the cache.
func (c *Client) IsCached(page string) bool {
	fi, err := os.Stat(c.cachePath(page))
	return err == nil && fi.Size() > 0
}

// page returns the page's HTML, fetching it if it is missing, stale (index
// pages only) or a refresh was requested.
func (c *Client) page(page string, index bool) (string, error) {
	path := c.cachePath(page)
	fi, err := os.Stat(path)
	cached := err == nil && fi.Size() > 0
	stale := cached && index && c.IndexTTL > 0 && time.Since(fi.ModTime()) >= c.IndexTTL

	if !cached || stale || c.Refresh {
		if c.Offline {
			if !cached {
				return "", fmt.Errorf("%s: not cached (offline)", page)
			}
		} else if err := c.fetch(page, path); err != nil {
			return "", err
		}
	}
	b, err := os.ReadFile(path)
	return string(b), err
}

func (c *Client) fetch(page, path string) error {
	if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
		return err
	}
	time.Sleep(c.FetchDelay)

	url := BaseURL + page
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("fetch failed: %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch failed: %s: %s", url, resp.Status)
	}

	tmp, err := os.CreateTemp(c.CacheDir, filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	_, err = io.Copy(tmp, resp.Body)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("fetch failed: %s: %w", url, err)
	}
	return nil
}

// Seasons parses listseasons.php.
func (c *Client) Seasons() ([]Season, error) {
	html, err := c.page(SeasonsPage(), true)
	if err != nil {
		return nil, err
	}
	return parseSeasons(html), nil
}

// Season parses showseason.php?season=<id> (id is opaque: "42", "jm", "pcj").
func (c *Client) Season(id string) ([]GameRow, error) {
	if id == "" {
		return nil, errors.New("empty season id")
	}
	html, err := c.page(SeasonPage(id), true)
	if err != nil {
		return nil, err
	}
	return parseSeason(html, c.Warn), nil
}

// Game parses showgame.php?game_id=<id>.
func (c *Client) Game(id int) (*Game, error) {
	html, err := c.page(GamePage(id), false)
	if err != nil {
		return nil, err
	}
	return parseGame(id, html, c.Warn)
}

// Player parses showplayer.php?player_id=<id>.
func (c *Client) Player(id int) (*Player, error) {
	html, err := c.page(PlayerPage(id), false)
	if err != nil {
		return nil, err
	}
	return parsePlayer(id, html, c.Warn), nil
}

// Marshal renders v as jarch's canonical JSON: two-space indent, HTML
// characters unescaped, trailing newline (matching the original jq output).
func Marshal(v any) ([]byte, error) { return marshal(v, "  ") }

func marshal(v any, indent string) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
