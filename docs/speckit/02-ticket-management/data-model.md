# Data Model: Ticket Management

## Ticket

The `tickets` table is created by feature 01. Feature 02 uses the existing table and does not change its schema.

| Field | Type | Null | Rule |
|---|---|---:|---|
| `id` | integer | No | Primary key, auto-increment; stable local identity |
| `ticket_key` | text | No | Unique Jira key; follows configured regex, default `[A-Z][A-Z0-9]+-[0-9]+` |
| `title` | text | Yes | Optional local title; may be unknown when discovered from a commit |
| `created_at` | text | No | Defaults to current timestamp; immutable after creation |
| `updated_at` | text | No | Defaults to current timestamp; changes only when stored title changes |

## Relationships

```text
tickets
  ├── work_sessions.ticket_id      (one ticket to many sessions)
  ├── work_activities.ticket_id    (one ticket to many activities; nullable on activity)
  └── ai_generations.ticket_id     (one ticket to many generations; nullable on generation)
```

Feature 02 establishes and returns the stable ticket ID. Session, activity, and generation records are managed by their own features.

## Ticket Key Value

- Key is non-empty and the configured pattern must match the entire key in create/upsert operations.
- Default regex is case-sensitive and accepts examples including `OOT-3751`, `ABC-123`, and `PROJECT1-999`.
- Find uses exact key comparison.
- Extract scans message text and returns the first match in reading order; no match returns a not-found result without persistence side effects.
- The extracted substring is preserved as matched; it is not uppercased, trimmed, or otherwise rewritten.

## Create, Find, and Upsert Semantics

| Operation | Missing key | Existing key | Side effect |
|---|---|---|---|
| Create | Insert and return ticket | Return duplicate error | Insert one row only |
| Find | Return not-found error | Return exact row | None |
| Upsert, title absent/blank | Insert with null title | Return existing row unchanged | Update `updated_at` only on insertion |
| Upsert, title supplied | Insert with trimmed title | Update title if different | Keep ID/`created_at`; update `updated_at` only if title changes |

## Error Outcomes

- **Invalid key**: key does not match configured pattern; no storage write.
- **Ticket not found**: exact lookup has no row; no storage write.
- **Ticket already exists**: create violates uniqueness; existing row remains unchanged.
- **Storage failure**: infrastructure returns a wrapped persistence error; no duplicate row is created.
- **No extracted key**: extraction returns a non-error not-found result so Git capture can store an unassigned activity later.
