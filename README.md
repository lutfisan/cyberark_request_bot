# CybArBot — Telegram Bot for CyberArk PAM

CybArBot is a production-grade, Go-based Telegram chatbot that allows authorised PAM administrators and access reviewers to manage CyberArk Privileged Access Manager (PAM) incoming access requests entirely from within a Telegram DM or group chat. It is designed to reduce the Mean Time to Approve (MTTA) by eliminating the friction of VPNs and web dashboards.

## Features

- **Proactive Notifications**: A built-in watcher polls CyberArk (default every 60s) and proactively pushes alerts for new requests.
- **Inline Action Keyboards**: Confirm, reject, or view request details directly from notification messages.
- **Bulk Operations**: Multi-select bulk confirmation and rejection with 'Select All' support and shared reasoning.
- **Strict Access Control**: Hardened with a strict Telegram User and Group ID whitelist loaded at runtime.
- **Audit Traceability**: Automatically prefixes all bot-originated actions with `[CybArBot]` to keep a clean, unambiguous CyberArk audit trail when sharing a service account.
- **Resilient CyberArk Sessions**: Manages token concurrency safely via `sync.RWMutex` with proactive auto-refresh and automatic `401` re-authentication.
- **Dual Transport Modes**: Run via Telegram's Long-Polling (default) or Webhooks (for production).

## Bot Commands

| Command            | Description                                         |
|--------------------|-----------------------------------------------------|
| `/start` `/help`   | Welcome message and command list                    |
| `/requests`        | List all pending incoming requests (paginated)      |
| `/detail <id>`     | Show full confirmation details for a request        |
| `/confirm <id>`    | Confirm a single request (prompts for optional reason)|
| `/reject <id>`     | Reject a single request (prompts for mandatory reason)|
| `/confirmall`      | Multi-select bulk confirmation (Select All support) |
| `/rejectall`       | Multi-select bulk rejection (Select All support)    |
| `/status`          | Bot health, session status, active delivery mode    |
| `/notify_status`   | Notification watcher health, last poll, cache size  |
| `/cancel`          | Abort any active multi-step operation               |

## Setup & Deployment

### Prerequisites
- Go 1.23+ (or Go 1.26+)
- A Telegram Bot Token from [@BotFather](https://t.me/botfather)
- A CyberArk PAM Self-Hosted 14.6 instance

### 1. Configuration

Copy the provided template to configure the bot:
```bash
cp .env.example .env
```
Fill out the required fields in your `.env` file (see the Configuration section below).

### 2. Running Locally (Long-Polling)

The fastest way to test the bot is using the default `longpoll` mode.

```bash
# Build and run directly
make run

# OR via Docker Compose
docker-compose up -d
```

### 3. Running in Production (Webhook Mode)

For production, it is recommended to use `BOT_MODE=webhook`.
1. Update `.env`:
   - `BOT_MODE=webhook`
   - `TELEGRAM_WEBHOOK_URL=https://your.public.domain/bot`
   - `WEBHOOK_SECRET_TOKEN=a_secure_random_string_min_32_chars`
2. Ensure the bot is exposed securely over HTTPS via a reverse proxy (like Nginx or Traefik), or configure `WEBHOOK_TLS_CERT` and `WEBHOOK_TLS_KEY` to serve TLS directly.

### 4. Running Behind a Proxy or NAT (No Public IP)

If your server does not have a public IP address or sits behind a corporate proxy, direct Webhook mode will not work because Telegram cannot reach your server.

**Option A: Long-Polling (Recommended)**
Use `BOT_MODE=longpoll`. The bot will make outbound connections to fetch updates, which works perfectly through NATs and firewalls. To route traffic through an outbound proxy, pass standard Go proxy variables before running:
```bash
export HTTPS_PROXY="http://your-proxy-server:port"
export HTTP_PROXY="http://your-proxy-server:port"
./cybarbot
```

**Option B: Reverse Tunnels (If webhooks are strictly required)**
Run a tunneling daemon like **Cloudflare Tunnels (`cloudflared`)** or **ngrok** alongside the bot. Set `TELEGRAM_WEBHOOK_URL` to the public tunnel URL, and the daemon will securely route inbound webhook traffic through your proxy down to the bot.

## Configuration Reference

Key variables to configure in your `.env` file:

- `CYBERARK_BASE_URL`: The URL to your PVWA (e.g., `https://pvwa.corp.local`)
- `CYBERARK_USERNAME` / `CYBERARK_PASSWORD`: Dedicated reviewer credentials. Note that concurrent sessions are enabled by default for shared accounts.
- `TELEGRAM_BOT_TOKEN`: The bot token obtained from BotFather.
- `ALLOWED_TELEGRAM_IDS`: Comma-separated User IDs permitted to use the bot.
- `ALLOWED_GROUP_IDS`: Comma-separated Group IDs permitted to use the bot.
- `NOTIFY_ENABLED`: Toggle the notification watcher (`true` or `false`).
- `POLL_INTERVAL_SECONDS`: How often to poll CyberArk for new requests (60–180 seconds).

## Development

- **Build**: `make build`
- **Test**: `make test`
- **Lint**: `make lint` (Requires `golangci-lint`)
- **Docker Build**: `make docker-build`

*CybArBot securely manages session tokens entirely in memory and never logs sensitive information, ensuring strict compliance with enterprise security requirements.*
