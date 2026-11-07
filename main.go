// Command tube-bot watches a Telegram group and sends every YouTube link
// posted there to a MeTube instance for download.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type bot struct {
	token   string
	metube  string
	allowed map[int64]bool
	http    *http.Client
	log     *slog.Logger

	mu   sync.Mutex
	seen map[string]time.Time // url -> when queued (dedupe window)
}

type update struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		MessageID int    `json:"message_id"`
		Text      string `json:"text"`
		Caption   string `json:"caption"`
		Chat      struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"chat"`
		From struct {
			FirstName string `json:"first_name"`
		} `json:"from"`
	} `json:"message"`
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	b := &bot{
		token:   os.Getenv("TELEGRAM_BOT_TOKEN"),
		metube:  strings.TrimRight(envOr("METUBE_URL", "http://metube:8081"), "/"),
		allowed: map[int64]bool{},
		http:    &http.Client{Timeout: 70 * time.Second},
		log:     log,
		seen:    map[string]time.Time{},
	}
	for _, s := range strings.Split(os.Getenv("TELEGRAM_ALLOWED_CHATS"), ",") {
		if s = strings.TrimSpace(s); s != "" {
			id, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				log.Error("bad TELEGRAM_ALLOWED_CHATS entry", "value", s)
				os.Exit(1)
			}
			b.allowed[id] = true
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if b.token == "" {
		log.Warn("TELEGRAM_BOT_TOKEN is not set; idling")
		<-ctx.Done()
		return
	}
	if len(b.allowed) == 0 {
		log.Warn("TELEGRAM_ALLOWED_CHATS is empty: nothing will be downloaded. Post in your group and copy the chat id from the log lines below")
	}
	b.run(ctx)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func (b *bot) run(ctx context.Context) {
	offset := 0
	backoff := time.Second
	for ctx.Err() == nil {
		ups, err := b.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			b.log.Error("getUpdates failed", "err", err, "retry_in", backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < time.Minute {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		for _, u := range ups {
			offset = u.UpdateID + 1
			b.handle(ctx, u)
		}
	}
}

func (b *bot) api(ctx context.Context, method string, params url.Values, out any) error {
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.telegram.org/bot"+b.token+"/"+method, strings.NewReader(params.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := b.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %s", method, strings.ReplaceAll(err.Error(), b.token, "***"))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var env struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &env); err != nil || !env.OK {
		return fmt.Errorf("%s: %d %s", method, resp.StatusCode, env.Description)
	}
	if out != nil {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}

func (b *bot) getUpdates(ctx context.Context, offset int) ([]update, error) {
	var ups []update
	err := b.api(ctx, "getUpdates", url.Values{
		"offset": {strconv.Itoa(offset)}, "timeout": {"50"}, "allowed_updates": {`["message"]`},
	}, &ups)
	return ups, err
}

func (b *bot) handle(ctx context.Context, u update) {
	m := u.Message
	if m == nil {
		return
	}
	text := m.Text + " " + m.Caption
	links := extractLinks(text)
	if !b.allowed[m.Chat.ID] {
		if len(links) > 0 {
			b.log.Warn("ignoring link from chat that is not allowed", "chat_id", m.Chat.ID, "title", m.Chat.Title)
		}
		return
	}
	ok, fail := 0, 0
	for _, l := range links {
		if b.recentlySeen(l) {
			b.log.Info("duplicate, skipped", "url", l)
			ok++
			continue
		}
		if err := b.addToMeTube(ctx, l); err != nil {
			b.log.Error("metube add failed", "url", l, "err", err)
			fail++
			continue
		}
		b.log.Info("queued", "url", l, "by", m.From.FirstName)
		ok++
	}
	switch {
	case fail > 0:
		b.react(ctx, m.Chat.ID, m.MessageID, "👎")
	case ok > 0:
		b.react(ctx, m.Chat.ID, m.MessageID, "👍")
	}
}

func (b *bot) recentlySeen(l string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	for k, t := range b.seen {
		if now.Sub(t) > time.Hour {
			delete(b.seen, k)
		}
	}
	if _, dup := b.seen[l]; dup {
		return true
	}
	b.seen[l] = now
	return false
}

func (b *bot) forget(l string) {
	b.mu.Lock()
	delete(b.seen, l)
	b.mu.Unlock()
}

func (b *bot) addToMeTube(ctx context.Context, link string) error {
	body, _ := json.Marshal(map[string]any{
		"url": link, "download_type": "audio", "format": "mp3", "quality": "best",
		"playlist_item_limit": 100,
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", b.metube+"/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.http.Do(req)
	if err != nil {
		b.forget(link)
		return err
	}
	defer resp.Body.Close()
	var st struct {
		Status string `json:"status"`
		Msg    string `json:"msg"`
	}
	data, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(data, &st); err != nil || resp.StatusCode != 200 || st.Status != "ok" {
		b.forget(link)
		return fmt.Errorf("metube replied %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}

func (b *bot) react(ctx context.Context, chat int64, msg int, emoji string) {
	err := b.api(ctx, "setMessageReaction", url.Values{
		"chat_id": {strconv.FormatInt(chat, 10)}, "message_id": {strconv.Itoa(msg)},
		"reaction": {fmt.Sprintf(`[{"type":"emoji","emoji":"%s"}]`, emoji)},
	}, nil)
	if err != nil {
		b.log.Warn("reaction failed", "err", err)
	}
}
