# Product Requirements Document
## CybArBot — Telegram Bot for CyberArk PAM Request Management

| Field        | Value                                |
|--------------|--------------------------------------|
| **Version**  | 1.2.0                                |
| **Status**   | Draft                                |
| **Language** | Go 1.26+ (latest stable)             |
| **Target**   | CyberArk PAM Self-Hosted 14.6        |
| **Date**     | 2026-06-01                           |

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Background & Motivation](#2-background--motivation)
3. [Goals & Non-Goals](#3-goals--non-goals)
4. [Personas & Stakeholders](#4-personas--stakeholders)
5. [Functional Requirements](#5-functional-requirements)
6. [Bot Commands & Interaction Design](#6-bot-commands--interaction-design)
7. [Security & Access Control](#7-security--access-control)
8. [Technical Architecture](#8-technical-architecture)
9. [CyberArk API Integration](#9-cyberark-api-integration)
10. [Configuration & Environment](#10-configuration--environment)
11. [Non-Functional Requirements](#11-non-functional-requirements)
12. [Error Handling & Resilience](#12-error-handling--resilience)
13. [Logging & Observability](#13-logging--observability)
14. [Project Structure](#14-project-structure)
15. [Milestones & Delivery](#15-milestones--delivery)
16. [Open Questions](#16-open-questions)
17. [Appendix](#17-appendix)

---

## 1. Executive Summary

**CybArBot** is a production-grade, Go-based Telegram chatbot that allows authorised PAM administrators and access reviewers to manage CyberArk Privileged Access Manager (PAM) incoming access requests — list, inspect, confirm, bulk-confirm, reject, and bulk-reject — entirely from within a Telegram DM or group chat, without opening a browser or requiring a VPN session to PVWA.

A built-in **Notification Watcher** goroutine polls CyberArk for new incoming requests on a configurable interval (60–180 seconds) and **proactively pushes alert messages** to designated notification targets the moment a new request arrives. Each alert carries full request context, an inline quick-action keyboard (Confirm / Reject / View Details), and persists a record of its sent Telegram Message ID so that when a request is subsequently actioned — whether by the bot itself or externally via PVWA — the bot **automatically edits the original notification message** to reflect the new status and removes the action buttons.

Bulk operations (confirm-all, reject-all) support **Select All / Deselect All** toggling and a **single shared reason** that is applied uniformly to all selected requests. The bot supports both **long-polling** (default) and **Telegram Webhook** mode, selectable by a single env var.

Session token concurrency is handled via a `sync.RWMutex` — the recommended approach for this read-heavy, write-rare pattern. The service account is a dedicated bot-only account that may be co-owned with another machine-to-machine system; all bot-originated confirmation and rejection reasons are automatically prefixed with `[CybArBot]` to unambiguously distinguish them in the PVWA audit trail.

Access is strictly gate-kept by a static whitelist of permitted **Telegram User IDs** and **Telegram Group IDs** loaded from the runtime environment, following the same pattern used in Hermes Agent and OpenClaw. All credentials and tokens reside exclusively in a `.env` file; zero secrets are ever hardcoded.

---

## 2. Background & Motivation

CyberArk PAM raises access-confirmation requests that appointed reviewers must action within a tight SLA window. The incumbent workflow requires reviewers to:

1. Establish a VPN tunnel to the internal network
2. Navigate to the PVWA web console
3. Authenticate with a privileged account
4. Locate, inspect, and action each pending request

This introduces significant friction and extends mean-time-to-approve (MTTA), particularly outside business hours or when reviewers are mobile. A purpose-built Telegram bot eliminates every step except the approval decision itself, reducing MTTA from minutes to seconds.

---

## 3. Goals & Non-Goals

### 3.1 Goals

| ID  | Goal |
|-----|------|
| G1  | Allow whitelisted Telegram users and groups to view all incoming CyberArk access requests |
| G2  | Allow inspection of full confirmation details for individual requests |
| G3  | Allow single confirmation of a request with an optional reason |
| G4  | Allow bulk confirmation of multiple requests in one interaction |
| G5  | Allow single rejection of a request with a mandatory reason |
| G6  | Allow bulk rejection of multiple requests with a shared mandatory reason |
| G7  | Enforce strict whitelist-based access control on every incoming Telegram update |
| G8  | Store all secrets in `.env`; zero hardcoded credentials anywhere in the codebase |
| G9  | Build on the latest stable Go release with idiomatic, production-grade patterns |
| G10 | Maintain a persistent CyberArk session with proactive automatic token renewal; session token guarded by `sync.RWMutex` for safe concurrent access |
| G11 | Emit structured, auditable logs for every confirm/reject action |
| G12 | Proactively notify designated Telegram targets when a new incoming request is detected; automatically edit notification messages when the request status changes |
| G13 | Support Telegram Webhook mode as an alternative update-delivery mechanism to long-polling |
| G14 | Prefix all bot-originated confirm/reject reasons with `[CybArBot]` for unambiguous audit trail attribution when the service account is shared with another M2M system |

### 3.2 Non-Goals

| ID   | Non-Goal |
|------|----------|
| NG1  | Creating, modifying, or deleting CyberArk Safes, accounts, or policies |
| NG2  | LDAP / MFA-based Telegram user authentication |
| NG3  | A web dashboard or REST API frontend |
| NG4  | Support for CyberArk Privilege Cloud — PAM Self-Hosted 14.6 only |
| NG5  | Dynamic whitelist management through bot commands (requires env reload) |
| NG6  | Persistent seen-request storage across process restarts (in-memory cache only; see FR-93) |

---

## 4. Personas & Stakeholders

| Persona            | Role                                                              | Primary Interaction                      |
|--------------------|-------------------------------------------------------------------|------------------------------------------|
| **PAM Reviewer**   | Primary user; reviews and actions access requests                 | Sends bot commands via DM or group chat  |
| **PAM Admin**      | Configures the bot; manages `.env` and whitelist                  | Deploys and maintains the binary/container |
| **Security Team**  | Audits all confirm/reject decisions                               | Consumes structured audit log output     |
| **Access Requester** | End user who initiated the CyberArk access request              | Indirect; receives approved/rejected status from CyberArk |

---

## 5. Functional Requirements

### 5.1 Authentication & Session Management

| ID    | Requirement |
|-------|-------------|
| FR-01 | The bot **MUST** authenticate to CyberArk PAM using the CyberArk native authentication method (`POST /PasswordVault/API/auth/CyberArk/Logon`) on startup |
| FR-02 | The returned session token **MUST** be stored in memory only — never persisted to disk, env file, or any external store |
| FR-03 | A background goroutine **MUST** proactively refresh the session token at an interval of `SESSION_TTL_MINUTES - 2` minutes before expiry (default effective interval: 18 min for a 20 min TTL) |
| FR-04 | The bot **MUST** call `POST /PasswordVault/API/auth/Logoff` on graceful shutdown (SIGTERM / SIGINT) |
| FR-05 | On receiving an HTTP `401` from any CyberArk API call, the bot **MUST** attempt a single re-authentication before surfacing an error to the user |
| FR-06 | If re-authentication fails after retry, the bot **MUST** send a critical alert to `ADMIN_TELEGRAM_ID` and disable all CyberArk-bound commands until the process is restarted with valid credentials |

### 5.2 Whitelist Access Control

| ID    | Requirement |
|-------|-------------|
| FR-10 | **Every** incoming Telegram update **MUST** pass a whitelist check before any application logic executes |
| FR-11 | The whitelist **MUST** support both individual `TELEGRAM_USER_ID` values and `TELEGRAM_GROUP_ID` values (including supergroups, which have negative IDs) |
| FR-12 | Messages from non-whitelisted senders **MUST** be silently dropped when `WHITELIST_SILENT=true` (default), or replied with the value of `WHITELIST_REJECT_MSG` when `WHITELIST_SILENT=false` |
| FR-13 | The whitelist **MUST** be loaded from the `.env` file at startup; the process **SHOULD** re-read whitelist values on `SIGHUP` without a full restart |

### 5.3 Request Listing

| ID    | Requirement |
|-------|-------------|
| FR-20 | `/requests` **MUST** fetch all pending incoming requests via `GET /PasswordVault/API/IncomingRequests?onlywaiting=true` and display them |
| FR-21 | Each list entry **MUST** display: Request ID, Requester name, Safe name, Account name, Status, and Creation timestamp |
| FR-22 | Results **MUST** be paginated at `REQUESTS_PAGE_SIZE` items (default: 10) using Telegram `InlineKeyboardMarkup` **Prev / Next** navigation buttons |
| FR-23 | An empty result set **MUST** produce a user-friendly "✅ No pending requests" message |

### 5.4 Request Detail Inspection

| ID    | Requirement |
|-------|-------------|
| FR-30 | `/detail <RequestID>` **MUST** fetch confirmation details via `GET /PasswordVault/API/IncomingRequests/<RequestID>` |
| FR-31 | The detail view **MUST** include: Request ID, Requester, Account, Safe, Access Type, Expiration Time, Requester Reason, Current Status, and all workflow confirmation steps with their respective states |

### 5.5 Single Request Confirmation

| ID    | Requirement |
|-------|-------------|
| FR-40 | `/confirm <RequestID>` **MUST** initiate a confirmation flow for the specified request |
| FR-41 | The bot **MUST** prompt the reviewer with an inline keyboard offering `Skip (no reason)` or `Enter Reason` before calling the CyberArk API |
| FR-42 | On `Enter Reason`, the bot transitions to `WAITING_CONFIRM_REASON` FSM state and collects the next text message from that chat as the reason |
| FR-43 | The bot **MUST** call `POST /PasswordVault/API/IncomingRequests/<RequestID>/Confirm` with the reason payload |
| FR-44 | Before calling the API, the bot **MUST** automatically prepend `[CybArBot] ` to any reviewer-supplied reason, or use `[CybArBot] Approved via CybArBot` when no reason is given — ensuring bot-originated actions are unambiguously identifiable in the PVWA audit trail when the service account is shared with another M2M system |
| FR-45 | Success **MUST** reply with a receipt: Request ID, reviewer Telegram username, final reason string (including prefix), and UTC timestamp |

### 5.6 Bulk Confirmation

| ID    | Requirement |
|-------|-------------|
| FR-50 | `/confirmall` **MUST** display all pending requests as a multi-select `InlineKeyboardMarkup` with individual toggle buttons, a **`☑ Select All` / `☐ Deselect All`** toggle button, and a `✅ Confirm Selected` action button |
| FR-51 | The `☑ Select All` button **MUST** toggle all displayed requests to selected state; pressing it again (now labelled `☐ Deselect All`) clears all selections — this is a single in-place keyboard edit, not a new message |
| FR-52 | On pressing `✅ Confirm Selected`, the bot **MUST** prompt for an **optional** shared confirmation reason (mirroring the single-confirm optional reason flow) before calling the API |
| FR-53 | After the optional reason step, the bot **MUST** call `POST /PasswordVault/API/IncomingRequests/Confirm` with the array of selected Request IDs and the `[CybArBot]`-prefixed shared reason |
| FR-54 | The bot **MUST** report per-request success or failure based on the bulk API response |
| FR-55 | If no requests are selected when `✅ Confirm Selected` is pressed, the bot **MUST** reply with a validation warning and leave the keyboard intact |

### 5.7 Single Request Rejection

| ID    | Requirement |
|-------|-------------|
| FR-60 | `/reject <RequestID>` **MUST** prompt the reviewer to provide a mandatory rejection reason (rejection without reason is disallowed by the CyberArk API) |
| FR-61 | The bot transitions to `WAITING_REJECT_REASON` FSM state and collects the next text message as the reason |
| FR-62 | Before calling the API, the bot **MUST** automatically prepend `[CybArBot] ` to the reviewer-supplied reason |
| FR-63 | The bot **MUST** call `POST /PasswordVault/API/IncomingRequests/<RequestID>/Reject` with the prefixed mandatory reason payload |
| FR-64 | Success **MUST** reply with a rejection receipt including Request ID, final reason string (including prefix), reviewer username, and UTC timestamp |

### 5.8 Bulk Rejection

| ID    | Requirement |
|-------|-------------|
| FR-70 | `/rejectall` **MUST** display all pending requests as a multi-select `InlineKeyboardMarkup` with individual toggle buttons, a **`☑ Select All` / `☐ Deselect All`** toggle button, and a `❌ Reject Selected` action button |
| FR-71 | The `☑ Select All` / `☐ Deselect All` toggle **MUST** behave identically to FR-51 |
| FR-72 | On pressing `❌ Reject Selected`, the bot **MUST** transition to `WAITING_BULK_REJECT_REASON` FSM state and prompt for a **mandatory** shared rejection reason (applied uniformly to all selected requests) |
| FR-73 | The bot **MUST** call `POST /PasswordVault/API/IncomingRequests/Reject` with the selected Request IDs and the `[CybArBot]`-prefixed shared mandatory reason |
| FR-74 | Per-request outcome **MUST** be reported from the bulk API response |
| FR-75 | If no requests are selected when `❌ Reject Selected` is pressed, the bot **MUST** reply with a validation warning and leave the keyboard intact |

### 5.9 Utility Commands

| ID    | Requirement |
|-------|-------------|
| FR-80 | `/start` and `/help` **MUST** display the full command reference with brief descriptions |
| FR-81 | `/status` **MUST** report: bot uptime, bot version, CyberArk session health (active/inactive), last token refresh timestamp, notification watcher status, and last poll timestamp |
| FR-82 | `/cancel` **MUST** abort any in-progress multi-step FSM interaction and return the chat to `IDLE` state, with a confirmation message |

### 5.10 Proactive Request Notifications

| ID    | Requirement |
|-------|-------------|
| FR-90 | A **Notification Watcher** background goroutine **MUST** poll `GET /PasswordVault/API/IncomingRequests?onlywaiting=true` at a configurable interval (`POLL_INTERVAL_SECONDS`; default: `60`; valid range: `60–180`) |
| FR-91 | The watcher **MUST** maintain an in-memory **Seen-Request Cache** keyed by Request ID, where each entry stores: `SeenAt time.Time`, `LastStatus string`, and `Dispatches []SentMessage` — where `SentMessage = {ChatID int64, MessageID int}` — recording every Telegram message dispatched for that request |
| FR-92 | On each poll cycle, the watcher **MUST** run two passes: **(Pass 1 — New)** diff API results against cache; for IDs not in cache dispatch notifications and add to cache. **(Pass 2 — Stale)** diff cache against API results; for IDs in cache that are absent from the latest `onlywaiting=true` results, the request is no longer pending — trigger a status-update edit (FR-105) and evict from cache |
| FR-93 | On bot startup, if `NOTIFY_ON_RESTART=false` (default), the watcher **MUST** pre-populate the Seen-Request Cache with all currently pending requests **without** sending notifications — ensuring reviewers are only alerted for requests that arrive after the bot starts. If `NOTIFY_ON_RESTART=true`, all pending requests at startup trigger notifications |
| FR-94 | When a request is confirmed or rejected via the bot (single or bulk), the bot **MUST** immediately edit all notification messages stored in that request's `Dispatches` list to show the confirmed/rejected status and actor before evicting the entry from the cache |
| FR-95 | Notifications **MUST** be dispatched to all chat IDs listed in `NOTIFY_TELEGRAM_IDS` and `NOTIFY_GROUP_IDS`. If `NOTIFY_TELEGRAM_IDS` is not explicitly set, it defaults to the value of `ALLOWED_TELEGRAM_IDS`; if `NOTIFY_GROUP_IDS` is not set, it defaults to `ALLOWED_GROUP_IDS` |
| FR-96 | Each notification message **MUST** include: 🔔 header, Request ID, Requester, Safe, Account, Access Type, Expiration Time, Requester Reason, and Creation timestamp |
| FR-97 | Each notification **MUST** include an inline quick-action keyboard with three buttons: `✅ Confirm`, `❌ Reject`, and `🔍 View Details` — all pre-bound to the Request ID |
| FR-98 | Pressing `✅ Confirm` from a notification **MUST** enter the same FSM flow as `/confirm <id>` (optional reason prompt) — the chat ID and Request ID are already known from the callback |
| FR-99 | Pressing `❌ Reject` from a notification **MUST** enter the same FSM flow as `/reject <id>` (mandatory reason prompt) |
| FR-100 | Pressing `🔍 View Details` from a notification **MUST** immediately send the full detail view as a follow-up message (same output as `/detail <id>`) |
| FR-101 | If a notification dispatch fails for a target chat (e.g., bot blocked by user), the watcher **MUST** log the failure at `WARN` level and continue dispatching to remaining targets — one failed target **MUST NOT** block others |
| FR-102 | The Notification Watcher **MUST** be independently togglable via `NOTIFY_ENABLED=true/false` without redeploying |
| FR-103 | `/notify_status` **MUST** report: watcher enabled/disabled, poll interval, last poll time, last poll result (n seen, m new, k stale-edited), and current Seen-Request Cache size |
| FR-104 | When a request is actioned externally (detected via Pass 2 — Stale), the bot **MUST** edit each message in `Dispatches` to replace the body with a status banner: `⚠️ This request is no longer pending. It was actioned externally or has expired.` and **MUST** remove the inline keyboard |
| FR-105 | When the bot itself confirms or rejects a request, the edited notification message **MUST** display the final state prominently: `✅ CONFIRMED by @<actor> at <UTC>` or `❌ REJECTED by @<actor> at <UTC> — Reason: <reason>`, and **MUST** remove the inline keyboard |
| FR-106 | If editing a sent notification message fails (e.g., message was deleted by user, or is older than Telegram's 48-hour edit window), the bot **MUST** log the failure at `WARN` level and continue — edit failures are non-fatal |
| FR-107 | The `SentMessage` `MessageID` values stored in `Dispatches` **MUST** be populated from the Telegram `Message.MessageID` returned by the send operation; if the send itself fails, no entry is added to `Dispatches` for that target |

### 5.11 Webhook Mode

| ID    | Requirement |
|-------|-------------|
| FR-110 | The bot **MUST** support two mutually exclusive update-delivery modes, selected via `BOT_MODE` env var: `longpoll` (default) and `webhook` |
| FR-111 | In `webhook` mode, the bot **MUST** register the configured `TELEGRAM_WEBHOOK_URL` with the Telegram Bot API on startup and start an HTTPS listener on `WEBHOOK_LISTEN_ADDR` (default: `:8443`) |
| FR-112 | The webhook HTTPS listener **MUST** verify the `X-Telegram-Bot-Api-Secret-Token` header on every incoming request against `WEBHOOK_SECRET_TOKEN`; requests that fail verification **MUST** return `HTTP 401` and be discarded without processing |
| FR-113 | The bot **MUST** support TLS termination either by providing `WEBHOOK_TLS_CERT` + `WEBHOOK_TLS_KEY` paths directly, or by running behind a reverse proxy (when both vars are unset, the listener uses plain HTTP and assumes a TLS-terminating proxy in front) |
| FR-114 | In `webhook` mode, the bot **MUST** call `deleteWebhook` on graceful shutdown (SIGTERM / SIGINT) to prevent Telegram from continuing to push updates to a dead endpoint |
| FR-115 | All command handling, whitelist gate, FSM, and notification logic **MUST** behave identically regardless of which update-delivery mode is active; `BOT_MODE` is purely a transport concern |
| FR-116 | `/status` **MUST** include the active delivery mode (`Long-Poll` or `Webhook`) and, in webhook mode, the registered webhook URL |

---

## 6. Bot Commands & Interaction Design

### 6.1 Command Reference Table

| Command            | Description                                         |
|--------------------|-----------------------------------------------------|
| `/start`           | Welcome message and command list                    |
| `/help`            | Full command reference                              |
| `/status`          | Bot health, session status, active delivery mode    |
| `/notify_status`   | Notification watcher health, last poll, cache size  |
| `/requests`        | List all pending incoming requests (paginated)      |
| `/detail <id>`     | Show full confirmation details for a request        |
| `/confirm <id>`    | Confirm a single request (optional reason)          |
| `/confirmall`      | Multi-select bulk confirmation (Select All support) |
| `/reject <id>`     | Reject a single request (mandatory reason)          |
| `/rejectall`       | Multi-select bulk rejection (Select All support)    |
| `/cancel`          | Abort any active multi-step operation               |

### 6.2 Sample Interaction Flows

#### 6.2.1 `/requests` — Paginated List
```
User: /requests

Bot:  📋 Pending Requests (Page 1 / 3)
      ─────────────────────────────────────────────
      [REQ-001] john.doe → IT-Admin-Safe   | 2026-05-31 22:10 UTC
      [REQ-002] jane.doe → HR-Database     | 2026-06-01 00:05 UTC
      [REQ-003] bob.smith → Dev-Safe       | 2026-06-01 01:30 UTC
      ...
      ─────────────────────────────────────────────
      [◀ Prev]  [Page 1/3]  [Next ▶]
```

#### 6.2.2 `/confirm <id>` — Single Confirm with Optional Reason
```
User: /confirm REQ-001

Bot:  ⚠️ Confirm Request REQ-001?
      Requester : john.doe
      Safe      : IT-Admin-Safe
      Account   : svc_deploy
      ─────────────────────────
      Add a reason?
      [Skip — No Reason]  [✏️ Enter Reason]

User: [✏️ Enter Reason]

Bot:  Please type your reason:

User: Approved for scheduled maintenance window.

Bot:  ✅ Request REQ-001 Confirmed
      Reason : Approved for scheduled maintenance window.
      By     : @pam_reviewer
      At     : 2026-06-01 09:14:32 UTC
```

#### 6.2.3 `/reject <id>` — Single Reject with Mandatory Reason
```
User: /reject REQ-002

Bot:  ✏️ Please provide a rejection reason for REQ-002
      (This field is mandatory):

User: Not within the approved change window.

Bot:  ❌ Request REQ-002 Rejected
      Reason : Not within the approved change window.
      By     : @pam_reviewer
      At     : 2026-06-01 09:15:00 UTC
```

#### 6.2.4 `/confirmall` — Bulk Confirm with Select All and Shared Reason
```
User: /confirmall

Bot:  Select requests to confirm:
      ──────────────────────────────────────────
      ☐  REQ-001 — john.doe / IT-Admin-Safe
      ☐  REQ-002 — jane.doe / HR-Database
      ☐  REQ-003 — bob.smith / Dev-Safe
      ──────────────────────────────────────────
      [☑ Select All]  [✅ Confirm Selected]  [🚫 Cancel]

User: [☑ Select All]

Bot:  (keyboard updates in-place — all rows now checked)
      ☑  REQ-001 — john.doe / IT-Admin-Safe
      ☑  REQ-002 — jane.doe / HR-Database
      ☑  REQ-003 — bob.smith / Dev-Safe
      ──────────────────────────────────────────
      [☐ Deselect All]  [✅ Confirm Selected]  [🚫 Cancel]

User: Deselects REQ-002 → [✅ Confirm Selected]

Bot:  Add a shared reason for REQ-001, REQ-003?
      [Skip — No Reason]  [✏️ Enter Reason]

User: [✏️ Enter Reason]

Bot:  Please type a shared reason for all 2 selected requests:

User: Approved for emergency deployment window.

Bot:  ✅ REQ-001 — Confirmed
      ✅ REQ-003 — Confirmed
      ─────────────────────────────────────────────────────
      Reason : [CybArBot] Approved for emergency deployment window.
      By     : @pam_reviewer  |  At: 2026-06-01 09:18:44 UTC
```

#### 6.2.5 `/rejectall` — Bulk Reject with Select All and Shared Mandatory Reason
```
User: /rejectall

Bot:  Select requests to reject:
      ──────────────────────────────────────────
      ☐  REQ-002 — jane.doe / HR-Database
      ☐  REQ-004 — tom.hanks / Finance-Safe
      ──────────────────────────────────────────
      [☑ Select All]  [❌ Reject Selected]  [🚫 Cancel]

User: [☑ Select All] → [❌ Reject Selected]

Bot:  ✏️ Please type a shared rejection reason for all 2 selected requests:

User: Access not authorised under current change freeze.

Bot:  ❌ REQ-002 — Rejected
      ❌ REQ-004 — Rejected
      ─────────────────────────────────────────────────────
      Reason : [CybArBot] Access not authorised under current change freeze.
      By     : @pam_reviewer  |  At: 2026-06-01 09:20:10 UTC
```

#### 6.2.6 Proactive Notification — New Incoming Request
```
[Notification Watcher detects REQ-005 — not in Seen-Request Cache]

Bot → Group Chat:
      ──────────────────────────────────────────────────
      🔔 New Access Request

      Request ID   : REQ-005
      Requester    : alice.wonder
      Safe         : Prod-DB-Safe
      Account      : db_admin_prod
      Access Type  : SingleUse
      Expires At   : 2026-06-01 11:00 UTC
      Reason       : Need to run emergency schema migration
      Received At  : 2026-06-01 09:45:02 UTC
      ──────────────────────────────────────────────────
      [✅ Confirm]  [❌ Reject]  [🔍 View Details]

User: [✅ Confirm] → [Skip — No Reason]

Bot:  ✅ Request REQ-005 Confirmed
      Reason : [CybArBot] Approved via CybArBot
      By     : @pam_reviewer
      At     : 2026-06-01 09:45:18 UTC

[Bot simultaneously edits the original notification message in Group Chat:]

      ──────────────────────────────────────────────────
      🔔 New Access Request

      Request ID   : REQ-005
      Requester    : alice.wonder
      ...
      ──────────────────────────────────────────────────
      ✅ CONFIRMED by @pam_reviewer at 2026-06-01 09:45:18 UTC
      [keyboard removed]
```

#### 6.2.7 Notification Edited — External Action
```
[REQ-006 was actioned directly in PVWA — it disappears from onlywaiting=true poll results]

[Bot detects absence in Pass 2 — Stale and edits the original notification:]

      ──────────────────────────────────────────────────
      🔔 New Access Request

      Request ID   : REQ-006
      Requester    : charlie.brown
      ...
      ──────────────────────────────────────────────────
      ⚠️ This request is no longer pending.
      It was actioned externally or has expired.
      [keyboard removed]
```

#### 6.2.8 `/notify_status` — Watcher Health
```
User: /notify_status

Bot:  🔔 Notification Watcher Status
      ────────────────────────────────────
      Enabled        : ✅ Yes
      Poll Interval  : 60 seconds
      Last Poll      : 2026-06-01 09:45:02 UTC
      Last Result    : 5 seen, 1 new, 1 stale-edited
      Cache Size     : 4 requests
      Notify Targets : 1 user, 1 group
```

#### 6.2.9 `/status` — Bot & Session Health
```
User: /status

Bot:  🤖 CybArBot v1.2.0
      ──────────────────────────────────────
      Uptime           : 4h 32m 18s
      CyberArk Session : ✅ Active
      Last Refresh     : 2026-06-01 09:02:01 UTC
      Next Refresh     : 2026-06-01 09:20:01 UTC
      PVWA Host        : pvwa.example.com
      Delivery Mode    : Long-Poll
      Notif. Watcher   : ✅ Running (every 60s)
      Last Poll        : 2026-06-01 09:45:02 UTC
```

---

## 7. Security & Access Control

### 7.1 Whitelist Enforcement Architecture

The whitelist mechanism mirrors the approach used in Hermes Agent and OpenClaw. Every incoming Telegram `Update` is subject to an early-exit gate before any handler executes:

```go
// internal/whitelist/whitelist.go

type Whitelist struct {
    allowedUsers  map[int64]struct{}
    allowedGroups map[int64]struct{}
    mu            sync.RWMutex
}

func (w *Whitelist) IsAllowed(update tgbotapi.Update) bool {
    w.mu.RLock()
    defer w.mu.RUnlock()

    senderID := extractSenderID(update)   // chat.ID for groups, from.ID for DMs
    _, okUser  := w.allowedUsers[senderID]
    _, okGroup := w.allowedGroups[senderID]
    return okUser || okGroup
}
```

The gate is applied in the update dispatcher before routing to any command handler. Non-whitelisted updates are either:
- **Silently dropped** (`WHITELIST_SILENT=true` — default, recommended for production), or
- **Replied with a rejection message** (`WHITELIST_SILENT=false`, using `WHITELIST_REJECT_MSG`)

### 7.2 Environment-Based Secret Management

| Variable                | Description                                              | Example                        |
|-------------------------|----------------------------------------------------------|--------------------------------|
| `CYBERARK_BASE_URL`     | Full base URL of the PVWA instance                       | `https://pvwa.corp.local`      |
| `CYBERARK_USERNAME`     | CyberArk service account username                        | `svc_bot_reviewer`             |
| `CYBERARK_PASSWORD`     | CyberArk service account password                        | *(strong random secret)*       |
| `TELEGRAM_BOT_TOKEN`    | Token from @BotFather                                    | `123456:ABC-DEF…`              |
| `ADMIN_TELEGRAM_ID`     | Telegram User ID to receive critical error alerts        | `123456789`                    |
| `ALLOWED_TELEGRAM_IDS`  | Comma-separated permitted Telegram User IDs              | `123456789,987654321`          |
| `ALLOWED_GROUP_IDS`     | Comma-separated permitted Group/Supergroup IDs           | `-1001234567890,-1009876543210`|

All values are loaded via `github.com/joho/godotenv` at startup. The `.env` file is listed in `.gitignore` and **never committed** to version control.

> **M2M Account Sharing Note:** The CyberArk service account (`CYBERARK_USERNAME`) is a dedicated bot account that is also shared with one other machine-to-machine system. Because all API calls from both systems appear under the same PVWA account, the bot automatically prepends `[CybArBot]` to every reason string (see FR-44, FR-62, FR-53, FR-73) so that PVWA audit logs unambiguously distinguish bot-originated actions from M2M-originated actions. The bot's structured audit log also records the Telegram actor's user ID and username as independent evidence of who triggered each action.

### 7.3 Transport Security

- All CyberArk API calls **MUST** use HTTPS with valid TLS certificate verification enabled by default
- `CYBERARK_SKIP_TLS_VERIFY=false` is the only safe production value; setting it to `true` is restricted to lab environments only and **MUST** be documented as a security risk in the README
- The session token value **MUST NOT** appear in any log output at any log level, including `DEBUG`
- In **long-poll** mode, Telegram communication uses the official Telegram Bot API endpoint over HTTPS
- In **webhook** mode, the incoming listener either serves HTTPS directly (when `WEBHOOK_TLS_CERT` + `WEBHOOK_TLS_KEY` are provided) or accepts plain HTTP on the assumption that TLS is terminated by a reverse proxy; direct plain-HTTP exposure to the internet without a TLS proxy is **FORBIDDEN** and **MUST** be documented as a security misconfiguration in the README
- The `WEBHOOK_SECRET_TOKEN` **MUST** be treated as a credential: never logged, never committed, minimum 32 random characters

---

## 8. Technical Architecture

### 8.1 Technology Stack

| Layer                | Technology                                      | Rationale                                                    |
|----------------------|-------------------------------------------------|--------------------------------------------------------------|
| Language             | **Go 1.26+** (latest stable)                    | Static binary, native concurrency, minimal runtime footprint |
| Telegram SDK         | `github.com/go-telegram/bot` | Supports both long-poll and webhook modes; context-based |
| HTTP Client          | `net/http` (stdlib) + `github.com/hashicorp/go-retryablehttp` | Configurable retry/back-off for PVWA calls    |
| Webhook Server       | `net/http` (stdlib)                             | Webhook mode listener; no additional dependency required     |
| Env Loading          | `github.com/joho/godotenv`                      | Twelve-Factor App config pattern                             |
| Logging              | `log/slog` (stdlib, structured JSON)            | Zero external dependency; native structured logging in Go 1.21+ |
| Session Token Guard  | `sync.RWMutex` within `AuthManager` struct      | Read-heavy, write-rare pattern: many concurrent API goroutines read the token; only the refresh goroutine and re-auth path write it. `RWMutex` allows N concurrent readers with zero contention; a write lock is held only for the microsecond duration of the token swap. The serialised-goroutine alternative would add a channel round-trip to every API call — unnecessary latency for zero safety gain |
| Build Tooling        | `Makefile` + `goreleaser`                       | Single-command build, cross-platform static binary releases  |
| Container            | Multi-stage Docker (`golang:1.26-alpine` → `scratch`) | Minimal final image; < 20 MB target                 |

### 8.2 Component Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                            CybArBot Process                              │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │               Update Ingestion (BOT_MODE selects one)            │   │
│  │                                                                  │   │
│  │  [longpoll]  Telegram Long Poller (goroutine)                    │   │
│  │              OR                                                  │   │
│  │  [webhook]   net/http Webhook Listener (:8443)                   │   │
│  │              + X-Telegram-Bot-Api-Secret-Token verification      │   │
│  └──────────────────────────┬───────────────────────────────────────┘   │
│                             │ tgbotapi.Update                           │
│                             ▼                                           │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │   Update Dispatcher  ──▶  Whitelist Gate (ALLOWED IDs)            │  │
│  └──────────────────────────────────┬─────────────────────────────── ┘  │
│                                     │  ✅ Allowed                       │
│                                     ▼                                   │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │                    Command Router + FSM                            │  │
│  │  /requests /detail /confirm /confirmall /reject /rejectall        │  │
│  │  /status   /notify_status  /help  /cancel                         │  │
│  │  Per-chat FSM state: sync.Map[chatID → State]                     │  │
│  └──────────────────────────────┬─────────────────────────────────── ┘  │
│                                 │                                        │
│  ┌──────────────────────────────▼─────────────────────────────────────┐  │
│  │                      CyberArk API Client                           │  │
│  │  ┌─────────────────────────────────┐  ┌──────────────────────┐    │  │
│  │  │  AuthManager                    │  │  HTTP Pool           │    │  │
│  │  │  token string                   │  │  (retryable,         │    │  │
│  │  │  mu sync.RWMutex  ← OQ-1 choice │  │   timeout,           │    │  │
│  │  │  refresh goroutine (TTL-2 min)  │  │   TLS config)        │    │  │
│  │  └─────────────────────────────────┘  └──────────────────────┘    │  │
│  │  ┌──────────────────────────────────────────────────────────────┐  │  │
│  │  │  RequestService: List, Detail, Confirm, BulkConfirm,         │  │  │
│  │  │                  Reject, BulkReject                          │  │  │
│  │  └──────────────────────────┬───────────────────────────────────┘  │  │
│  └─────────────────────────────│──────────────────────────────────────┘  │
│                                │ poll results                            │
│  ┌─────────────────────────────▼──────────────────────────────────────┐  │
│  │                    Notification Watcher                            │  │
│  │                                                                    │  │
│  │  Ticker (POLL_INTERVAL_SECONDS ± 10% jitter)                      │  │
│  │       │                                                            │  │
│  │       ▼ each tick: GET /IncomingRequests?onlywaiting=true          │  │
│  │  ┌────────────────────────────────────────────────────────────┐   │  │
│  │  │  Seen-Request Cache                                        │   │  │
│  │  │  sync.Map[requestID → CacheEntry{                          │   │  │
│  │  │     SeenAt     time.Time                                   │   │  │
│  │  │     LastStatus string                                      │   │  │
│  │  │     Dispatches []SentMessage{ChatID, MessageID}            │   │  │
│  │  │  }]                                                        │   │  │
│  │  └──────────┬──────────────────────────┬─────────────────────┘   │  │
│  │             │ Pass 1 — new IDs          │ Pass 2 — stale IDs      │  │
│  │             ▼                           ▼                          │  │
│  │  Send notifications              Edit existing messages            │  │
│  │  (fan-out to NOTIFY_TARGETS)     (remove keyboard, show status)    │  │
│  └────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────┘
     │ HTTPS (long-poll OR webhook push)        │ HTTPS (REST API)
     ▼                                          ▼
Telegram Bot API                   CyberArk PVWA 14.6
(api.telegram.org)                 (pvwa.corp.local)
```

### 8.3 Session Lifecycle State Machine

```
                        ┌─────────────────────┐
               Start ──▶│      Unauthenticated │
                        └──────────┬──────────┘
                                   │ POST /auth/CyberArk/Logon
                                   ▼
                        ┌─────────────────────┐
                        │    Authenticated     │◀──────────────────────────┐
                        │  (token in memory)   │                           │
                        └──────────┬──────────┘                           │
                                   │                                       │
                     ┌─────────────┴────────────┐                         │
                     │                          │                         │
              Handle Commands            Refresh Goroutine         Re-Auth on 401
              (normal operation)       (every TTL-2 minutes)     (single retry)
                     │                          │                         │
                     └─────────────┬────────────┘                         │
                                   │                                       │
                              401 Received ──────────────────────────────▶│
                                   │                                       │
                              SIGTERM / SIGINT                             │
                                   │                                       │
                                   ▼                                       │
                        ┌─────────────────────┐                           │
                        │      Logoff          │                           │
                        │ POST /auth/Logoff    │                           │
                        └─────────────────────┘                           │
                                   │                                       │
                                  Exit                            Alert Admin if
                                                                 re-auth fails

Notification Watcher Startup Sequence (runs in parallel after Logon):

  Authenticated
       │
       ▼
  [NOTIFY_ON_RESTART=false?]
       │ Yes                           │ No
       ▼                               ▼
  Pre-populate Seen Cache         Skip pre-population
  (no notifications sent)         (all current requests
       │                           will trigger alerts)
       └──────────────┬─────────────────┘
                      ▼
             Start polling ticker
             (every POLL_INTERVAL_SECONDS ± jitter)
                      │
               ┌──────┴──────────────────┐
               │  Poll cycle              │
               │  GET /IncomingRequests   │
               │  ?onlywaiting=true       │
               │                          │
               │  Pass 1 — New IDs:       │
               │  responseIDs ∖ cacheIDs  │
               │  → dispatch notification │
               │  → add to cache          │
               │                          │
               │  Pass 2 — Stale IDs:     │
               │  cacheIDs ∖ responseIDs  │
               │  → edit existing msgs    │
               │  → evict from cache      │
               └──────────────────────────┘

Auth Manager — Resolved: sync.RWMutex (OQ-1):

  ┌─────────────────────────────────────────────────────┐
  │  type AuthManager struct {                          │
  │      token  string                                  │
  │      mu     sync.RWMutex                            │
  │  }                                                  │
  │                                                     │
  │  func (a *AuthManager) Token() string {             │
  │      a.mu.RLock()                                   │
  │      defer a.mu.RUnlock()                           │
  │      return a.token   // N concurrent readers, OK   │
  │  }                                                  │
  │                                                     │
  │  func (a *AuthManager) setToken(t string) {         │
  │      a.mu.Lock()                                    │
  │      defer a.mu.Unlock()                            │
  │      a.token = t     // held for microseconds only  │
  │  }                                                  │
  └─────────────────────────────────────────────────────┘
```

### 8.4 Conversation FSM States (Per Chat)

```
IDLE
 │
 ├─ /confirm <id> ──────────▶ WAITING_CONFIRM_REASON ──▶ (API call) ──▶ IDLE
 │                              or Skip → IDLE directly
 │
 ├─ /reject <id>  ──────────▶ WAITING_REJECT_REASON  ──▶ (API call) ──▶ IDLE
 │
 ├─ /confirmall   ──────────▶ BULK_CONFIRM_SELECT ──▶ (API call) ──▶ IDLE
 │
 └─ /rejectall    ──────────▶ BULK_REJECT_SELECT  ──▶ WAITING_BULK_REJECT_REASON ──▶ (API call) ──▶ IDLE

/cancel resets any state → IDLE at any point
```

State is stored in a thread-safe `sync.Map` keyed by `chatID int64`.

---

## 9. CyberArk API Integration

### 9.1 API Endpoint Map

| Operation            | HTTP Method | Endpoint                                                          |
|----------------------|-------------|-------------------------------------------------------------------|
| Logon                | `POST`      | `/PasswordVault/API/auth/CyberArk/Logon`                         |
| Logoff               | `POST`      | `/PasswordVault/API/auth/Logoff`                                  |
| List Requests        | `GET`       | `/PasswordVault/API/IncomingRequests?onlywaiting=true`            |
| Get Request Detail   | `GET`       | `/PasswordVault/API/IncomingRequests/{requestId}`                 |
| Confirm Single       | `POST`      | `/PasswordVault/API/IncomingRequests/{requestId}/Confirm`         |
| Bulk Confirm         | `POST`      | `/PasswordVault/API/IncomingRequests/Confirm`                     |
| Reject Single        | `POST`      | `/PasswordVault/API/IncomingRequests/{requestId}/Reject`          |
| Bulk Reject          | `POST`      | `/PasswordVault/API/IncomingRequests/Reject`                      |

### 9.2 Authentication Headers

Every API request after logon **MUST** include:

```
Authorization: <session_token>
Content-Type:  application/json
```

> ⚠️ Note: CyberArk uses the raw token string in the `Authorization` header — **not** `Bearer <token>`. This is a common integration mistake.

### 9.3 Request / Response Contracts

#### Logon
```json
// Request Body
{
  "username": "svc_bot_reviewer",
  "password": "S3cr3tP@ssw0rd!",
  "concurrentSession": true
}

// Response Body (raw string — strip surrounding quotes)
"<opaque_session_token>"
```

#### Confirm Single
```json
// Request Body
{
  "Reason": "Approved by @pam_reviewer via CybArBot — scheduled maintenance"
}
```

#### Bulk Confirm
```json
// Request Body
{
  "RequestIDs": ["REQ-001", "REQ-003"],
  "Reason": "Bulk approved during change window"
}
```

#### Reject Single
```json
// Request Body
{
  "Reason": "Not within approved change window"
}
```

#### Bulk Reject
```json
// Request Body
{
  "RequestIDs": ["REQ-002", "REQ-004"],
  "Reason": "Access not authorised under current change freeze"
}
```

### 9.4 Notable CyberArk API Behaviours

| Behaviour | Handling Strategy |
|-----------|-------------------|
| Logon returns a raw quoted JSON string, not an object | Strip surrounding `"` quotes before storing token |
| `401 Unauthorized` can be triggered by token expiry, not just wrong credentials | Attempt re-authentication before surfacing error to user |
| Rejection `Reason` is mandatory at the API level | Bot enforces this at the interaction layer — no API call is made without a reason |
| Bulk API returns per-request success/failure in the response body | Parse and display individual outcomes to the reviewer |
| `409 Conflict` on double-action | Inform user the request was already actioned |

---

## 10. Configuration & Environment

### 10.1 `.env` File — Full Reference

```dotenv
# ── CyberArk PVWA ─────────────────────────────────────────────────────────
CYBERARK_BASE_URL=https://pvwa.corp.local
CYBERARK_USERNAME=svc_bot_reviewer
CYBERARK_PASSWORD=YourStrongPassword!
CYBERARK_SKIP_TLS_VERIFY=false          # Set true ONLY in lab environments
SESSION_TTL_MINUTES=20                  # Match PVWA session timeout setting

# ── Telegram ──────────────────────────────────────────────────────────────
TELEGRAM_BOT_TOKEN=123456:ABC-DEFxxxxxxxxxxxxxxxxxxxxxxx
ADMIN_TELEGRAM_ID=123456789             # Receives critical error alerts

# ── Update Delivery Mode ──────────────────────────────────────────────────
BOT_MODE=longpoll                       # longpoll | webhook

# Webhook mode only (ignored when BOT_MODE=longpoll):
TELEGRAM_WEBHOOK_URL=https://bot.corp.local/telegram/webhook
WEBHOOK_LISTEN_ADDR=:8443              # Address:port for the webhook listener
WEBHOOK_SECRET_TOKEN=<min-32-random-chars>  # Telegram webhook secret header value
WEBHOOK_TLS_CERT=                       # Path to TLS cert PEM (leave empty if behind reverse proxy)
WEBHOOK_TLS_KEY=                        # Path to TLS key PEM (leave empty if behind reverse proxy)

# ── Whitelist ─────────────────────────────────────────────────────────────
ALLOWED_TELEGRAM_IDS=123456789,987654321
ALLOWED_GROUP_IDS=-1001234567890,-1009876543210
WHITELIST_SILENT=true                   # true = drop silently; false = reply reject msg
WHITELIST_REJECT_MSG=⛔ You are not authorised to use this bot.

# ── Bot Behaviour ─────────────────────────────────────────────────────────
REQUESTS_PAGE_SIZE=10                   # Items per /requests page (max 10 recommended)
HTTP_TIMEOUT_SECONDS=30                 # Timeout per CyberArk API call
HTTP_MAX_RETRIES=3                      # Retries per failed API call

# ── Notification Watcher ──────────────────────────────────────────────────
NOTIFY_ENABLED=true                     # Master toggle for the Notification Watcher
POLL_INTERVAL_SECONDS=60                # Valid range: 60–180. How often to poll CyberArk
NOTIFY_ON_RESTART=false                 # false = suppress alerts for requests already pending at startup
NOTIFY_TELEGRAM_IDS=                    # Leave empty to default to ALLOWED_TELEGRAM_IDS
NOTIFY_GROUP_IDS=                       # Leave empty to default to ALLOWED_GROUP_IDS

# ── Observability ─────────────────────────────────────────────────────────
LOG_LEVEL=info                          # debug | info | warn | error
LOG_AUDIT_FILE=/var/log/cybarbot/audit.jsonl  # Leave empty to log to stdout only
```

### 10.2 `.env.example` (Committed to Repository)

An `.env.example` file containing all the above keys with `<PLACEHOLDER>` values **MUST** be committed to the repository. The actual `.env` file **MUST** be in `.gitignore`.

### 10.3 Configuration Validation at Startup

The `config` package **MUST** validate all required fields at startup and fail fast with a descriptive error if any mandatory variable is missing or malformed:

```
FATAL: missing required env variable TELEGRAM_BOT_TOKEN — set it in .env
FATAL: CYBERARK_BASE_URL must be a valid HTTPS URL
FATAL: ALLOWED_TELEGRAM_IDS and ALLOWED_GROUP_IDS cannot both be empty
FATAL: BOT_MODE must be one of: longpoll, webhook
FATAL: TELEGRAM_WEBHOOK_URL is required when BOT_MODE=webhook
FATAL: WEBHOOK_SECRET_TOKEN must be at least 32 characters when BOT_MODE=webhook
FATAL: POLL_INTERVAL_SECONDS must be between 60 and 180 (got: 30)
```

When `NOTIFY_TELEGRAM_IDS` is empty, the config package **MUST** silently populate it from `ALLOWED_TELEGRAM_IDS` at load time, and similarly for `NOTIFY_GROUP_IDS` from `ALLOWED_GROUP_IDS`. This defaulting is logged at `INFO` level on startup so operators can see the effective configuration.

---

## 11. Non-Functional Requirements

| ID     | Category        | Requirement                                                                                        |
|--------|-----------------|----------------------------------------------------------------------------------------------------|
| NFR-01 | Performance     | Bot response latency < 2 s (P95) for list/detail operations; < 3 s for confirm/reject (CyberArk API latency is dominant) |
| NFR-02 | Availability    | Bot process auto-restarts on crash via Docker `restart: always` or systemd `Restart=on-failure`    |
| NFR-03 | Binary Size     | Final Docker image < 20 MB (using `scratch` base + statically compiled binary)                     |
| NFR-04 | Go Version      | Minimum `go 1.26`; the `go.mod` `toolchain` directive must pin the exact Go version used at release |
| NFR-05 | Test Coverage   | Unit test coverage ≥ 70% for the `internal/cyberark/` and `internal/bot/` packages                 |
| NFR-06 | Linting         | `golangci-lint run` must pass with `errcheck`, `gosec`, `exhaustive`, and `staticcheck` enabled     |
| NFR-07 | Secret Safety   | `gosec` must report zero hardcoded credential findings (G101, G401, G501 rules)                     |
| NFR-08 | Concurrency     | Session token **MUST** be guarded by `sync.RWMutex` within `AuthManager` (OQ-1 resolved). All other shared state (FSM map, whitelist, seen-request cache) must also use appropriate Go sync primitives (`sync.Map` or `sync.RWMutex` as applicable) |
| NFR-09 | Graceful Shutdown | SIGTERM handling must complete in-flight requests, stop the Notification Watcher cleanly, call `deleteWebhook` if in webhook mode, and call CyberArk Logoff — all within 10 seconds |
| NFR-10 | Notification Delivery | A failure to dispatch or edit a notification for one target chat **MUST NOT** affect delivery to other targets; all fan-out and edit operations **MUST** be independent per target |
| NFR-11 | Poll Jitter     | The Notification Watcher **MUST** apply ±10% random jitter to `POLL_INTERVAL_SECONDS` to avoid thundering-herd behaviour in multi-instance deployments |
| NFR-12 | Webhook Security | When `BOT_MODE=webhook`, every incoming request **MUST** be validated against `WEBHOOK_SECRET_TOKEN` before processing; unvalidated requests return `HTTP 401` within 5ms |
| NFR-13 | Message Editing | Editing a stale notification message **MUST** be non-blocking and non-fatal; edit failures are logged at `WARN` and never surface to the user or block the next poll cycle |

---

## 12. Error Handling & Resilience

### 12.1 CyberArk API Error Response Matrix

| HTTP Status | Meaning                              | Bot User Message                                                    |
|-------------|--------------------------------------|---------------------------------------------------------------------|
| `400`       | Bad request / invalid payload        | `⚠️ Bad request: <CyberArk error message>`                          |
| `401`       | Session expired / invalid token      | Triggers re-auth silently; error shown only if re-auth fails        |
| `403`       | Insufficient permissions             | `🚫 Permission denied. Verify your CyberArk reviewer role.`         |
| `404`       | Request not found                    | `❓ Request ID not found or already actioned.`                       |
| `409`       | Conflict (already actioned)          | `⚠️ This request has already been confirmed or rejected.`            |
| `429`       | Rate limited                         | Retry with exponential back-off; then `⏳ Rate limit hit. Try again.`|
| `5xx`       | PVWA server error                    | Retry 3×; then `🔴 CyberArk API unavailable. Please try again later.`|
| Timeout     | HTTP timeout exceeded                | `⏱️ Request timed out after {n}s. CyberArk may be slow.`             |
| Poll failure | Notification Watcher poll error     | No message to user; logged at `WARN`; watcher continues on next tick |

### 12.2 Retry Policy

```
Attempt 1: Immediate
Attempt 2: +1 second delay
Attempt 3: +3 second delay
─────────────────────────
After 3 failures: Surface error to user + emit ERROR log
```

Implemented via `go-retryablehttp` with a custom `CheckRetry` function that only retries on `5xx` and network errors (not `4xx`).

### 12.3 Panic Recovery

Each update handler goroutine is wrapped in a `recover()` block. A panicking handler logs the stack trace at `ERROR` level, notifies the user with a generic error message, and allows the bot to continue serving other requests.

```go
defer func() {
    if r := recover(); r != nil {
        slog.Error("handler panic", "recover", r, "stack", debug.Stack())
        bot.Send(tgbotapi.NewMessage(chatID, "🔴 An internal error occurred. Please try again."))
    }
}()
```

---

## 13. Logging & Observability

### 13.1 Structured Log Format

All logs are emitted via `log/slog` in JSON format:

```json
{
  "time":             "2026-06-01T09:14:32.001Z",
  "level":            "INFO",
  "msg":              "request confirmed",
  "component":        "bot.commands",
  "telegram_user_id": 123456789,
  "telegram_username": "pam_reviewer",
  "request_id":       "REQ-001",
  "cyberark_status":  200,
  "duration_ms":      447
}
```

### 13.2 Sensitive Data — What Is and Is Not Logged

| Field                  | Logged?    | Notes                                         |
|------------------------|------------|-----------------------------------------------|
| CyberArk password      | ❌ Never   | Only in memory during config load             |
| Session token          | ❌ Never   | Even partial/truncated forms are forbidden    |
| Telegram bot token     | ❌ Never   |                                               |
| Request ID             | ✅ Yes     |                                               |
| Telegram User ID       | ✅ Yes     | Numeric ID, not phone number                  |
| Telegram Username      | ✅ Yes     |                                               |
| Confirm/Reject Reason  | ✅ Yes     | Required for audit trail                      |
| PVWA hostname          | ✅ Yes     | For correlation with PVWA logs                |

### 13.3 Audit Log

Every confirm and reject action is additionally written to a dedicated audit log stream (stdout or `LOG_AUDIT_FILE` if set):

```json
{
  "time":              "2026-06-01T09:14:32Z",
  "level":             "AUDIT",
  "action":            "CONFIRM",
  "request_id":        "REQ-001",
  "bulk":              false,
  "actor_telegram_id": 123456789,
  "actor_username":    "pam_reviewer",
  "reason":            "Approved for scheduled maintenance window."
}
```

---

## 14. Project Structure

```
cybarbot/
├── cmd/
│   └── cybarbot/
│       └── main.go                  # Entry point: config load, component wiring, signal handling
│
├── internal/
│   ├── bot/
│   │   ├── bot.go                   # Bot init; starts long-poll goroutine OR webhook server based on BOT_MODE
│   │   ├── webhook.go               # net/http webhook listener, secret-token validation, update parsing
│   │   ├── dispatcher.go            # Whitelist gate + update routing
│   │   ├── commands.go              # Handler functions for each /command
│   │   ├── fsm.go                   # Per-chat FSM (sync.Map); states: IDLE, WAITING_CONFIRM_REASON,
│   │   │                            #   WAITING_REJECT_REASON, BULK_CONFIRM_SELECT,
│   │   │                            #   WAITING_BULK_CONFIRM_REASON, BULK_REJECT_SELECT,
│   │   │                            #   WAITING_BULK_REJECT_REASON
│   │   ├── keyboard.go              # InlineKeyboard builders: pagination, multi-select+SelectAll, quick-action
│   │   ├── formatter.go             # Request list/detail/notification/edit → Telegram message formatters
│   │   ├── notifier.go              # Notification Watcher: ticker+jitter, dual-pass diff, fan-out, message edits
│   │   └── middleware.go            # Logging middleware wrapper for all handlers
│   │
│   ├── cyberark/
│   │   ├── client.go                # Base HTTP client with retry, timeout, TLS config
│   │   ├── auth.go                  # Logon, Logoff; AuthManager{token, mu sync.RWMutex}; refresh goroutine
│   │   ├── requests.go              # List, Detail, Confirm, BulkConfirm, Reject, BulkReject
│   │   └── models.go                # All CyberArk API request/response Go structs
│   │
│   ├── config/
│   │   └── config.go                # .env loading, struct parsing, startup validation, notify-target defaulting
│   │
│   └── whitelist/
│       └── whitelist.go             # Whitelist struct, Contains(), LoadFromConfig(), SIGHUP reload
│
├── .env.example                     # Template with <PLACEHOLDER> values — committed to repo
├── .gitignore                       # Must include: .env, cybarbot (binary), *.log
├── Dockerfile                       # Multi-stage: golang:1.26-alpine → scratch
├── docker-compose.yml               # Convenience: volume-mounts .env, restart: always
├── Makefile                         # Targets: build, run, test, lint, docker-build, release
├── go.mod
├── go.sum
└── README.md                        # Setup guide, .env reference, long-poll vs webhook deployment guide
```

---

## 15. Milestones & Delivery

| Phase | Deliverable | Acceptance Criteria |
|-------|-------------|---------------------|
| **P0 — Foundation** | Repo scaffold, `.env` loading and validation, whitelist gate, CyberArk logon/logoff, `sync.RWMutex`-based `AuthManager`, session auto-refresh | Bot starts, authenticates to PVWA, rejects non-whitelisted messages, gracefully shuts down |
| **P1 — Core Read** | `/requests` paginated list, `/detail <id>` | Data correctly fetched and formatted; pagination functional |
| **P2 — Core Write** | `/confirm <id>` with optional reason + `[CybArBot]` prefix, `/reject <id>` with mandatory reason + prefix | Single confirm and reject working end-to-end; reason prefixing verified in PVWA audit trail |
| **P3 — Bulk Ops** | `/confirmall` and `/rejectall` with multi-select + **Select All / Deselect All** toggle + shared optional/mandatory reason prompts | Bulk API calls succeed; Select All works; shared reason with `[CybArBot]` prefix applied; per-request results displayed |
| **P4 — Notifications** | Notification Watcher goroutine, dual-pass diff (new + stale), extended `CacheEntry` with `Dispatches`, fan-out dispatcher, quick-action keyboard, `NOTIFY_ON_RESTART` logic, `NOTIFY_*` defaulting to `ALLOWED_*`, `/notify_status` | Bot sends push alerts for new requests; edits stale messages when requests are actioned externally; `/notify_status` reports stale-edited count |
| **P5 — Message Editing** | Bot-triggered message edits on confirm/reject (FR-105), edit-failure resilience (FR-106), `MessageID` capture on send (FR-107) | Notification message is edited immediately on bot action; keyboard is removed; edit failures logged but non-fatal |
| **P6 — Webhook Mode** | `BOT_MODE=webhook`, `net/http` listener, `WEBHOOK_SECRET_TOKEN` validation, `deleteWebhook` on shutdown, `/status` delivery-mode field | Bot operates identically in webhook mode; unauthenticated requests return 401; TLS direct and reverse-proxy modes both work |
| **P7 — Hardening & Release** | Retry logic, back-off, session re-auth on 401, full error matrix, audit log, `/status` with watcher+mode info, `/cancel`, Dockerfile, `docker-compose.yml`, `Makefile`, `README.md`, `.env.example`, goreleaser config | All NFRs met; `gosec` clean; `golangci-lint` clean; unit test coverage ≥ 70%; `make docker-build && docker-compose up` produces a fully functional bot |

---

## 16. Open Questions

| #   | Question / Resolution                                                                                                                      | Owner        | Status |
|-----|--------------------------------------------------------------------------------------------------------------------------------------------|--------------|--------|
| ~~OQ-1~~ | ~~sync.RWMutex vs serialised goroutine for session token?~~ **→ `sync.RWMutex` within `AuthManager`.** Rationale: token reads dominate (every API call); writes occur only every ~18 min (refresh) or on re-auth. `RWMutex` allows N concurrent readers with no contention; write lock is held for microseconds only. Serialised-goroutine alternative adds a channel round-trip to every API call — unnecessary latency. See §8.1 and §8.3. | Dev Lead | ✅ Resolved |
| ~~OQ-2~~ | ~~Per-request or shared reason for bulk operations?~~ **→ Single shared reason applied uniformly to all selected requests** for both bulk confirm and bulk reject. Both `/confirmall` and `/rejectall` include a `☑ Select All / ☐ Deselect All` toggle. See FR-50–FR-55 and FR-70–FR-75. | PAM Admin | ✅ Resolved |
| ~~OQ-3~~ | ~~Should `/requests` optionally include already-actioned requests?~~ **→ No.** `/requests` shows only `onlywaiting=true` results. | Stakeholder | ✅ Resolved — closed |
| ~~OQ-4~~ | ~~Dedicated bot account or shared with human reviewer?~~ **→ Dedicated bot-only account, co-owned with one other M2M system.** All bot-originated reasons are prefixed `[CybArBot]` for audit trail disambiguation. See §7.2, FR-44, FR-62, FR-53, FR-73. | Security Team | ✅ Resolved |
| ~~OQ-5~~ | ~~Webhook mode support?~~ **→ Yes, implemented.** `BOT_MODE=webhook` activates an `net/http` listener with `WEBHOOK_SECRET_TOKEN` validation. See FR-110–FR-116, §8.1, §8.2, §14, P6. | Ops | ✅ Resolved |
| ~~OQ-6~~ | ~~Should the bot notify reviewers when new requests arrive?~~ | Stakeholder | ✅ Resolved — implemented FR-90–FR-107 (P4, P5) |
| ~~OQ-7~~ | ~~Should NOTIFY_* default to ALLOWED_*?~~ **→ Yes.** When `NOTIFY_TELEGRAM_IDS` / `NOTIFY_GROUP_IDS` are empty, the config package silently defaults them to `ALLOWED_TELEGRAM_IDS` / `ALLOWED_GROUP_IDS` at load time, logged at `INFO`. See FR-95, §10.1, §10.3. | PAM Admin | ✅ Resolved |
| ~~OQ-8~~ | ~~Should the bot edit/delete notification messages when a request is actioned externally?~~ **→ Yes, edit** (not delete). The `CacheEntry` stores `Dispatches []SentMessage` per notification. Pass 2 of the poll cycle detects stale IDs and edits each stored message. The bot also edits immediately on its own confirm/reject actions. Edit failures are non-fatal (`WARN` log). See FR-92, FR-94, FR-104–FR-107, §8.2, P5. | Dev Lead | ✅ Resolved |
| ~~OQ-9~~ | ~~What is the acceptable MTTA SLA / poll interval?~~ **→ Default `POLL_INTERVAL_SECONDS=60`; valid range 60–180 s.** Config validation rejects values outside this range. See FR-90, §10.1, §10.3. | Stakeholder | ✅ Resolved |

---

## 17. Appendix

### A. CyberArk API Reference Links (PAM Self-Hosted 14.6)

| API                    | Official Documentation URL |
|------------------------|---------------------------|
| Logon                  | https://docs.cyberark.com/pam-self-hosted/14.6/en/content/sdk/cyberark%20authentication%20-%20logon_v10.htm |
| Logoff                 | https://docs.cyberark.com/pam-self-hosted/14.6/en/content/sdk/cyberark%20authentication%20-%20logoff_v10.htm |
| List Incoming Requests | https://docs.cyberark.com/pam-self-hosted/14.6/en/content/webservices/getincomingrequestlist.htm |
| Get Request Detail     | https://docs.cyberark.com/pam-self-hosted/14.6/en/content/webservices/getdetailsrequestconfirmation.htm |
| Confirm Single         | https://docs.cyberark.com/pam-self-hosted/14.6/en/content/webservices/confirmrequest.htm |
| Bulk Confirm           | https://docs.cyberark.com/pam-self-hosted/14.6/en/content/webservices/bulkconfirmrequest.htm |
| Reject Single          | https://docs.cyberark.com/pam-self-hosted/14.6/en/content/webservices/rejectrequest.htm |
| Bulk Reject            | https://docs.cyberark.com/pam-self-hosted/14.6/en/content/webservices/bulkrejectrequest.htm |

### B. Key Go Dependencies

| Package                                              | Version     | Purpose                          |
|------------------------------------------------------|-------------|----------------------------------|
| `github.com/go-telegram/bot` | `v1.21.0+`   | Telegram Bot API client          |
| `github.com/joho/godotenv`                           | `v1.5.1+`   | `.env` file loading              |
| `github.com/hashicorp/go-retryablehttp`              | `v0.7.7+`   | Retryable HTTP client            |

All other dependencies use the Go standard library (`net/http`, `log/slog`, `sync`, `os/signal`, `context`).

### C. Glossary

| Term           | Definition                                                                                          |
|----------------|-----------------------------------------------------------------------------------------------------|
| **PVWA**       | Password Vault Web Access — the CyberArk web interface and REST API gateway                         |
| **PAM**        | Privileged Access Management                                                                        |
| **FSM**        | Finite State Machine — manages multi-step conversation context per Telegram chat                    |
| **MTTA**       | Mean Time to Approve — the primary operational metric this bot is designed to reduce                 |
| **Session Token** | Opaque string returned by the PVWA Logon API; passed as the `Authorization` header value in all subsequent requests; guarded by `sync.RWMutex` within `AuthManager` |
| **Whitelist**  | The explicit list of permitted Telegram User IDs and Group IDs; all others are denied              |
| **Long-Poll**  | Telegram bot update delivery method: the bot continuously polls the Telegram API for new updates |
| **Webhook Mode** | Telegram update delivery method: the bot registers an HTTPS endpoint and Telegram pushes updates to it; enabled by `BOT_MODE=webhook` |
| **Notification Watcher** | Background goroutine that periodically polls CyberArk, runs a dual-pass diff against the Seen-Request Cache, dispatches new-request alerts, and edits stale notification messages |
| **Seen-Request Cache** | In-memory `sync.Map` keyed by Request ID; each value is a `CacheEntry{SeenAt, LastStatus, Dispatches}`; reset on process restart |
| **CacheEntry** | Per-request struct stored in the Seen-Request Cache: `SeenAt time.Time`, `LastStatus string`, `Dispatches []SentMessage{ChatID int64, MessageID int}` |
| **Notify Targets** | The set of Telegram User IDs and Group IDs that receive proactive push notifications; defaults to the whitelist if not explicitly configured |
| **Quick-Action Keyboard** | Inline keyboard attached to a notification message with Confirm / Reject / View Details buttons pre-bound to a Request ID; removed when the message is edited with a final status |
| **[CybArBot] Prefix** | String prepended to all bot-originated confirm/reject reason fields in the CyberArk API; disambiguates bot actions from M2M actions when both share the same service account |
| **M2M**        | Machine-to-Machine — a non-human automated system that may share the same CyberArk service account as the bot |
| **Pass 1 — New** | First half of each Notification Watcher poll cycle: detects Request IDs not yet in the Seen-Request Cache and sends notifications |
| **Pass 2 — Stale** | Second half of each poll cycle: detects Request IDs in cache that have disappeared from `onlywaiting=true` results and edits their notification messages |

---

*End of Document — CybArBot PRD v1.2.0*
