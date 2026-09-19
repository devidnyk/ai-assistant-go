## referral-postbox

A Telegram bot that collects job referral requests from your network and records them in a
Google Sheet. A GitHub Actions cron job does all the work; there is no server to host.

The repository also contains an earlier personal RAG assistant (Gemini + Qdrant) under
`cmd/cli`, which is unrelated to the referral flow and needs no configuration to stay out of
the way.

### How it works

1. Someone sends your bot `/refer <candidate name>, <job id>, <resume link>`.
2. Every ~6 hours a GitHub Actions job runs `cmd/inbox`, which:
   - pulls new Telegram messages, validates them, and appends a row with `status = Pending`;
   - replies to the sender confirming what was recorded;
   - re-reads the sheet and messages anyone whose row you have flipped to `Done`.
3. You triage in the spreadsheet. Changing `status` to `Done` is the only manual step.

Because cron is stateless, both directions are made idempotent:

| Direction | Guard |
| --- | --- |
| Inbound | Telegram's `update_id` is stored on each row and re-seen updates are skipped. The offset is advanced only after rows are written. |
| Outbound | `notified_at` is stamped after a successful send. A row is only notified when `status = Done` **and** `notified_at` is empty. |

### Sheet layout

Both tabs are created automatically on first run.

`referrals`:

| Column | Written by |
| --- | --- |
| `update_id`, `received_at`, `chat_id`, `user_id`, `username`, `first_name` | bot |
| `candidate_name`, `job_id`, `resume_url` | bot (from the message) |
| `status` | bot writes `Pending`; **you** set `Done` |
| `notes` | you, freeform |
| `notified_at`, `notify_error` | bot |

`_state` holds the Telegram update offset.

Do not sort or delete rows. Notification write-back addresses rows by position, so use a
filter view instead of sorting.

### Setup

**1. Telegram bot**

Create a bot with [@BotFather](https://t.me/BotFather) and copy the token.

**2. Google Sheet + service account**

- Create a blank spreadsheet and copy its ID from the URL:
  `https://docs.google.com/spreadsheets/d/<SPREADSHEET_ID>/edit`
- In Google Cloud, enable the **Google Sheets API**, create a **service account**, and
  download a JSON key.
- Share the spreadsheet with the service account's `client_email` as an **Editor**. Skipping
  this is the most common cause of a `403` on the first run.
- Encode the key as a single line:

  ```sh
  base64 -i service-account.json | tr -d '\n'
  ```

**3. Local run**

```sh
cp .env.example .env   # then fill in the values
go run ./cmd/inbox
```

**4. Publish the bot menu**

```sh
go run ./cmd/botsetup
```

This registers the `/refer` and `/help` command menu, plus the description shown on the empty
chat screen **before** anyone sends `/start` — so people see the required format without having
to guess. It is a one-off; the settings live on Telegram's servers. Re-run it whenever the
command wording in `internal/operations/referral.go` changes.

To verify, delete your chat with the bot and reopen it: the description should appear on the
empty screen. It will not show in an existing chat.

**5. GitHub Actions**

Add these repository secrets (Settings → Secrets and variables → Actions):

- `BOT_TOKEN`
- `SPREADSHEET_ID`
- `GOOGLE_SA_JSON_B64`

The workflow lives in [.github/workflows/referral-inbox.yml](.github/workflows/referral-inbox.yml)
and can be triggered manually via **Run workflow** for testing.

### Configuration

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `BOT_TOKEN` | yes | — | Telegram bot token |
| `SPREADSHEET_ID` | yes | — | Target Google Sheet |
| `GOOGLE_SA_JSON_B64` | yes | — | Base64 service account key |
| `REFERRAL_RATE_LIMIT` | no | `1` | Max submissions per chat per window |
| `REFERRAL_RATE_LIMIT_WINDOW_DAYS` | no | `7` | Length of the rolling window, in days |
| `TELEGRAM_POLL_LIMIT` | no | `100` | Updates fetched per run (1–100) |
| `GEMINI_API_KEY`, `QDRANT_API_KEY`, `SYS_PROMPT_PATH` | no | — | Only for `cmd/cli` |

### Operational notes

- **Telegram discards undelivered updates after 24 hours.** If the workflow stays broken for
  longer than that, those messages are gone. The job exits non-zero on failure so the run goes
  red and GitHub emails you — do not ignore it.
- **Scheduled workflows are disabled after 60 days of repository inactivity.** Push something
  or re-enable it periodically.
- Notifications are at-least-once. A crash between sending and stamping can repeat a message,
  which is deliberate: a duplicate is harmless, a missed one is not.
- Each run reads the whole sheet once. That is fine for hundreds of rows; past a few thousand
  it is worth switching to a bounded range read.

### Development

```sh
go build ./...
go vet ./...
go test ./...
```
