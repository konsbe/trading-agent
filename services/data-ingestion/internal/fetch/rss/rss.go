// Package rss fetches macro headline feeds (Reuters, FT, etc.).
package rss

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/httpclient"
)

// Item is one RSS/Atom entry (minimal fields).
type Item struct {
	Title   string
	Link    string
	PubDate time.Time
}

type rssFeed struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Items []struct {
			Title   string `xml:"title"`
			Link    string `xml:"link"`
			PubDate string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

// FetchItems downloads feedURL and returns up to max items.
func FetchItems(ctx context.Context, feedURL string, max int) ([]Item, error) {
	if feedURL == "" {
		return nil, fmt.Errorf("rss: empty URL")
	}
	cli := httpclient.New(45 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "trading-agent-macro-intel/1.0")
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rss: HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var rf rssFeed
	if err := xml.Unmarshal(body, &rf); err != nil {
		return nil, fmt.Errorf("rss parse: %w", err)
	}
	out := make([]Item, 0, len(rf.Channel.Items))
	for _, it := range rf.Channel.Items {
		title := strings.TrimSpace(it.Title)
		if title == "" {
			continue
		}
		ts := time.Now().UTC()
		if it.PubDate != "" {
			if t, err := time.Parse(time.RFC1123Z, it.PubDate); err == nil {
				ts = t.UTC()
			} else if t, err := time.Parse(time.RFC1123, it.PubDate); err == nil {
				ts = t.UTC()
			}
		}
		out = append(out, Item{Title: title, Link: strings.TrimSpace(it.Link), PubDate: ts})
		if len(out) >= max {
			break
		}
	}
	return out, nil
}
