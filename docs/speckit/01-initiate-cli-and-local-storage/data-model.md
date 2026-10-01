# Data Model: Initialize CLI and Local Storage

## Local Configuration

**Location**: `~/.worklog/config.yaml`  
**Lifecycle**: Created with defaults only when absent; never overwrite an existing config during normal startup.

| Key | Type | Default | Validation |
|---|---|---|---|
| `database.path` | string | `~/.worklog/worklog.db` | Resolve `~` against the current user's home; reject an empty path |
| `ticket.pattern` | string | `[A-Z][A-Z0-9]+-[0-9]+` | Must compile as a regular expression |
| `git.capture_changed_files` | boolean | `true` | — |
| `git.capture_diff_stat` | boolean | `true` | — |
| `git.capture_full_diff` | boolean | `false` | Must remain opt-in |
| `ai.enabled` | boolean | `true` | Does not require a provider or network for startup |
| `ai.include_diff` | boolean | `false` | Must remain opt-in; no diff leaves the machine by default |

Configuration parsing failure must identify the config path. Defaults may fill absent keys, but startup must not rewrite an existing file just to add defaults.

## Database Entities

The initial migration creates the four entities defined in `docs/ERD.md`.

### `tickets`

| Field | Type | Null | Rule |
|---|---|---:|---|
| `id` | integer | No | Primary key, auto-increment |
| `ticket_key` | text | No | Unique Jira key |
| `title` | text | Yes | May be unknown when first observed |
| `created_at` | text | No | Defaults to current timestamp |
| `updated_at` | text | No | Defaults to current timestamp |

### `work_sessions`

| Field | Type | Null | Rule |
|---|---|---:|---|
| `id` | integer | No | Primary key, auto-increment |
| `ticket_id` | integer | No | Foreign key to `tickets.id` |
| `title` | text | No | Non-empty session title at feature use time |
| `repository` | text | Yes | Repository context |
| `started_at` | text | No | Timestamp with documented local/offset representation |
| `ended_at` | text | Yes | Null while session is active |
| `duration_seconds` | integer | Yes | Calculated duration; non-negative when present |
| `status` | text | No | `ACTIVE` or `COMPLETED` |
| `created_at` | text | No | Defaults to current timestamp |
| `updated_at` | text | No | Defaults to current timestamp |

Indexes: `ticket_id`, `started_at`; partial unique index permits only one row with `status = 'ACTIVE'`.

### `work_activities`

| Field | Type | Null | Rule |
|---|---|---:|---|
| `id` | integer | No | Primary key, auto-increment |
| `ticket_id` | integer | Yes | Foreign key to `tickets.id`; null means unassigned |
| `session_id` | integer | Yes | Foreign key to `work_sessions.id`; null means unsessioned |
| `type` | text | No | `NOTE` or `GIT_COMMIT` in the current ERD |
| `description` | text | Yes | Human-readable activity context |
| `repository` | text | Yes | Repository name/path |
| `branch` | text | Yes | Branch name |
| `commit_hash` | text | Yes | Git commit hash |
| `commit_message` | text | Yes | Commit subject/message |
| `changed_files` | text | Yes | JSON array when present |
| `insertions` | integer | Yes | Diff statistic, default 0 |
| `deletions` | integer | Yes | Diff statistic, default 0 |
| `metadata` | text | Yes | Optional JSON object |
| `created_at` | text | No | Defaults to current timestamp |

Indexes: `ticket_id`, `session_id`, `created_at`; partial unique index on `(repository, commit_hash)` when `commit_hash` is not null.

### `ai_generations`

| Field | Type | Null | Rule |
|---|---|---:|---|
| `id` | integer | No | Primary key, auto-increment |
| `ticket_id` | integer | Yes | Foreign key to `tickets.id` |
| `type` | text | No | `WORKLOG_SUMMARY`, `TICKET_DESCRIPTION`, or `CODE_REVIEW` per ERD |
| `content` | text | No | Generated content |
| `generated_at` | text | No | Defaults to current timestamp |

Indexing is not specified for this table by the ERD and is not added in this feature.

## Relationships and Invariants

- A ticket can have many sessions, activities, and AI generation records.
- A session belongs to one ticket and can contain many activities.
- An activity may belong to a session; the session relationship is nullable.
- Activity may have no ticket when it is unassigned.
- At most one work session may be active at a time.
- A Git commit is unique within a repository by its commit hash.
- Foreign key constraints are enabled for every SQLite connection.
- Initial schema version is `1`. A migration failure leaves the recorded version unchanged.

## Migration Lifecycle

1. Read SQLite `user_version`.
2. If its value is greater than the highest migration embedded in this binary, return an unsupported-schema error without changing data.
3. For each pending version, begin a transaction, apply that migration's SQL, update `user_version`, and commit.
4. On any SQL or commit error, roll back and report the migration version and error.
5. Repeated startup at the latest version is a no-op.
