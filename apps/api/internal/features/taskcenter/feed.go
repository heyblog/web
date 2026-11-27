package taskcenter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	edgeclient "heyblog.local/edgerequest"

	"github.com/jackc/pgx/v5"
)

type feedItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Published string `json:"date_published"`
}
type xmlFeed struct {
	XMLName xml.Name
	Channel struct {
		Items []struct {
			ID        string `xml:"guid"`
			Title     string `xml:"title"`
			URL       string `xml:"link"`
			Published string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
	Entries []struct {
		ID    string `xml:"id"`
		Title string `xml:"title"`
		Links []struct {
			URL string `xml:"href,attr"`
			Rel string `xml:"rel,attr"`
		} `xml:"link"`
		Published string `xml:"published"`
	} `xml:"entry"`
}

func parseFeed(body []byte) ([]feedItem, error) {
	if !utf8.Valid(body) || strings.IndexByte(string(body), 0) >= 0 {
		return nil, errors.New("invalid feed text encoding")
	}
	var entries []feedItem
	if strings.HasPrefix(strings.TrimSpace(string(body)), "{") {
		var feed struct {
			Version string     `json:"version"`
			Items   []feedItem `json:"items"`
		}
		if err := json.Unmarshal(body, &feed); err != nil {
			return nil, err
		}
		if feed.Version != "https://jsonfeed.org/version/1" && feed.Version != "https://jsonfeed.org/version/1.1" {
			return nil, errors.New("invalid JSON feed version")
		}
		entries = feed.Items
	} else {
		var feed xmlFeed
		if err := xml.Unmarshal(body, &feed); err != nil {
			return nil, err
		}
		switch feed.XMLName.Local {
		case "rss":
			for _, item := range feed.Channel.Items {
				entries = append(entries, feedItem{ID: item.ID, Title: item.Title, URL: item.URL, Published: item.Published})
			}
		case "feed":
			for _, entry := range feed.Entries {
				item := feedItem{ID: entry.ID, Title: entry.Title, Published: entry.Published}
				for _, link := range entry.Links {
					if link.Rel == "" || link.Rel == "alternate" {
						item.URL = link.URL
						break
					}
				}
				entries = append(entries, item)
			}
		default:
			return nil, errors.New("unsupported feed format")
		}
	}
	if len(entries) > 1000 {
		return nil, errors.New("too many feed items")
	}
	result := []feedItem{}
	seen := map[string]bool{}
	for _, item := range entries {
		if strings.IndexByte(item.ID, 0) >= 0 || strings.IndexByte(item.Title, 0) >= 0 || strings.IndexByte(item.URL, 0) >= 0 || strings.IndexByte(item.Published, 0) >= 0 {
			return nil, errors.New("invalid feed item text")
		}
		address, err := url.Parse(item.URL)
		if err != nil || address.Host == "" || address.User != nil || (address.Scheme != "https" && address.Scheme != "http") || len(item.URL) > 4096 || len(item.Title) > 4096 {
			return nil, errors.New("invalid feed item")
		}
		if item.ID == "" {
			item.ID = item.URL
		}
		digest := sha256.Sum256([]byte(item.ID))
		item.ID = hex.EncodeToString(digest[:])
		if !seen[item.ID] {
			seen[item.ID] = true
			result = append(result, item)
		}
	}
	return result, nil
}

func storedFeedHeader(name, value string) string {
	if value == "" || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	if name == "Last-Modified" {
		if _, err := http.ParseTime(value); err != nil {
			return ""
		}
		return value
	}
	if name != "ETag" {
		return ""
	}
	etag := value
	etag = strings.TrimPrefix(etag, "W/")
	if len(etag) < 2 || etag[0] != '"' || etag[len(etag)-1] != '"' {
		return ""
	}
	escaped := false
	for _, r := range etag[1 : len(etag)-1] {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			return ""
		}
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	if escaped {
		return ""
	}
	return value
}
func (e *Execution) feedHeaders(ctx context.Context, id string, h http.Header) error {
	var etag, modified string
	err := e.repository.db.QueryRow(ctx, `SELECT etag,last_modified FROM operations.feed_documents WHERE feed_id=$1`, id).Scan(&etag, &modified)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if etag != "" {
		if etag = storedFeedHeader("ETag", etag); etag != "" {
			h.Set("If-None-Match", etag)
		}
	}
	if modified != "" {
		if modified = storedFeedHeader("Last-Modified", modified); modified != "" {
			h.Set("If-Modified-Since", modified)
		}
	}
	return nil
}
func (e *Execution) storeFeed(ctx context.Context, l Lease, response edgeclient.Result) (bool, error) {
	if l.Target.FeedID == nil {
		return false, nil
	}
	if response.Status == http.StatusNotModified {
		var exists bool
		err := e.repository.transaction(ctx, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(73921408)`); err != nil {
				return err
			}
			var siteID string
			err := tx.QueryRow(ctx, `SELECT s.id::text FROM operations.targets t JOIN directory.sites s ON s.id=t.site_id WHERE t.id=$1 AND t.lease_token=$2 AND t.state='running' AND t.lease_until>$3 AND s.monitoring_generation=t.generation FOR UPDATE OF t,s`, l.Target.ID, l.Target.LeaseToken, e.repository.now()).Scan(&siteID)
			if errors.Is(err, pgx.ErrNoRows) {
				return conflict()
			}
			if err != nil {
				return err
			}
			etag := storedFeedHeader("ETag", response.Headers.Get("ETag"))
			modified := storedFeedHeader("Last-Modified", response.Headers.Get("Last-Modified"))
			_, err = tx.Exec(ctx, `UPDATE operations.feed_documents SET etag=COALESCE(NULLIF($2,''),etag),last_modified=COALESCE(NULLIF($3,''),last_modified),checked_at=now() WHERE feed_id=$1`, *l.Target.FeedID, etag, modified)
			if err != nil {
				return err
			}
			return tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM operations.feed_documents WHERE feed_id=$1)`, *l.Target.FeedID).Scan(&exists)
		})
		return exists, err
	}
	if response.Status != http.StatusOK {
		return false, nil
	}
	items, err := parseFeed(response.Body)
	if err != nil {
		return false, nil
	}
	err = e.repository.transaction(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(73921408)`); err != nil {
			return err
		}
		var siteID string
		err := tx.QueryRow(ctx, `SELECT s.id::text FROM operations.targets t JOIN directory.sites s ON s.id=t.site_id WHERE t.id=$1 AND t.lease_token=$2 AND t.state='running' AND t.lease_until>$3 AND s.monitoring_generation=t.generation FOR UPDATE OF t,s`, l.Target.ID, l.Target.LeaseToken, e.repository.now()).Scan(&siteID)
		if errors.Is(err, pgx.ErrNoRows) {
			return conflict()
		}
		if err != nil {
			return err
		}
		for _, item := range items {
			if _, err := tx.Exec(ctx, `INSERT INTO operations.feed_items(feed_id,item_key,title,url,published) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, *l.Target.FeedID, item.ID, item.Title, item.URL, item.Published); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO operations.feed_documents(feed_id,etag,last_modified) VALUES($1,$2,$3) ON CONFLICT(feed_id) DO UPDATE SET etag=EXCLUDED.etag,last_modified=EXCLUDED.last_modified,checked_at=now()`, *l.Target.FeedID, storedFeedHeader("ETag", response.Headers.Get("ETag")), storedFeedHeader("Last-Modified", response.Headers.Get("Last-Modified")))
		return err
	})
	return err == nil, err
}
