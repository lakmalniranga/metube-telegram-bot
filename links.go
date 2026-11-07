package main

import (
	"net/url"
	"regexp"
	"strings"
)

var urlRe = regexp.MustCompile(`https?://[^\s<>"]+`)

var youtubeHosts = map[string]bool{
	"youtube.com": true, "www.youtube.com": true, "m.youtube.com": true,
	"music.youtube.com": true, "youtu.be": true, "www.youtu.be": true,
}

// extractLinks returns the distinct YouTube URLs found in text, cleaned up.
func extractLinks(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, raw := range urlRe.FindAllString(text, -1) {
		raw = strings.TrimRight(raw, ".,;:!?)]}>'\"")
		if u, ok := cleanYouTube(raw); ok && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// cleanYouTube accepts YouTube video/playlist/short URLs. For a video that is
// also part of a playlist (watch?v=…&list=…) only the single video is kept, so
// a shared link never queues a whole playlist by accident.
func cleanYouTube(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || !youtubeHosts[strings.ToLower(u.Host)] {
		return "", false
	}
	q := u.Query()
	switch {
	case strings.HasSuffix(strings.ToLower(u.Host), "youtu.be"):
		id := strings.Trim(u.Path, "/")
		if id == "" {
			return "", false
		}
		return "https://www.youtube.com/watch?v=" + url.QueryEscape(id), true
	case u.Path == "/watch" && q.Get("v") != "":
		return "https://www.youtube.com/watch?v=" + url.QueryEscape(q.Get("v")), true
	case strings.HasPrefix(u.Path, "/shorts/") || strings.HasPrefix(u.Path, "/live/"):
		id := strings.Trim(strings.SplitN(strings.TrimPrefix(strings.TrimPrefix(u.Path, "/shorts/"), "/live/"), "/", 2)[0], "/")
		if id == "" {
			return "", false
		}
		return "https://www.youtube.com/watch?v=" + url.QueryEscape(id), true
	case u.Path == "/playlist" && q.Get("list") != "":
		return "https://www.youtube.com/playlist?list=" + url.QueryEscape(q.Get("list")), true
	}
	return "", false
}
