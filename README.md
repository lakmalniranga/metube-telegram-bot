# metube-telegram-bot

> **Notice:** this project was written by AI (Claude, by Anthropic), directed by a human.
> It has unit tests for the link parsing only. Review the code before running it, and
> use it at your own risk.

A tiny Telegram bot that watches a group chat and sends every YouTube link posted there to
[MeTube](https://github.com/alexta69/metube) for download. Share a link with the family group and
the audio ends up in your music library, ready for Navidrome, Jellyfin or similar.

* Single static Go binary, standard library only, no dependencies.
* Long polling, so no public URL or webhook is needed.
* Reacts 👍 to a message once its links are queued, 👎 if MeTube refused one.
* Recognises `youtube.com`, `m.youtube.com`, `music.youtube.com`, `youtu.be`, Shorts and live links.
* A `watch?v=…&list=…` link downloads only that video, not the whole playlist.
  A pure playlist link (`/playlist?list=…`) is queued, limited to 100 items.
* The same link posted again within an hour is skipped.
* Only chats listed in `TELEGRAM_ALLOWED_CHATS` are served.

Downloads are requested as audio, mp3, best quality. Change the request in `addToMeTube` in
`main.go` if you want something else.

## Setup

1. Talk to [@BotFather](https://t.me/BotFather): `/newbot`, and copy the token.
2. Send `/setprivacy` to BotFather, pick your bot and choose **Disable**. Otherwise the bot
   cannot see ordinary messages in groups.
3. Add the bot to your group (remove and re-add it if you changed privacy afterwards).
4. Run it (see below) with the token and no allowed chats, then post a YouTube link in the group.
   The log shows `ignoring link from chat that is not allowed chat_id=-100…`.
5. Put that id into `TELEGRAM_ALLOWED_CHATS` and restart.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `TELEGRAM_BOT_TOKEN` | – | bot token from BotFather (required; the bot idles without it) |
| `TELEGRAM_ALLOWED_CHATS` | – | comma-separated chat ids to serve (group ids are negative). Empty = serve nothing |
| `METUBE_URL` | `http://metube:8081` | base URL of your MeTube instance |

Keep the token secret and out of version control. Anyone who has it controls the bot.

## Run with Docker Compose

```yaml
services:
  tube-bot:
    build: ./metube-telegram-bot
    environment:
      TELEGRAM_BOT_TOKEN: "${TELEGRAM_BOT_TOKEN}"
      TELEGRAM_ALLOWED_CHATS: "${TELEGRAM_ALLOWED_CHATS}"
      METUBE_URL: "http://metube:8081"
    depends_on: [metube]
    restart: unless-stopped
```

with the values in a `.env` file next to the compose file. The bot and MeTube must share a
Docker network (same compose project works).

## Develop

Go is not required locally:

```sh
docker run --rm -v "$PWD":/src -w /src golang:1 go test ./...
```

## Security notes

* The bot has no web interface and listens on no port; it only makes outgoing requests to
  `api.telegram.org` and to MeTube.
* Everyone in an allowed chat can queue downloads. Use a chat whose members you trust.
* Downloading content may be restricted by YouTube's terms of service or local law.
  You are responsible for how you use this.

## License

[MIT](LICENSE).
