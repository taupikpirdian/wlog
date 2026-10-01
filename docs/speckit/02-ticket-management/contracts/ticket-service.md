# Ticket Service Contract

This is an application contract for worklog features. It does not add a user-facing `wl ticket` command.

## Ticket Value

```text
Ticket {
  id: integer
  key: string
  title: optional string
  created_at: timestamp
  updated_at: timestamp
}
```

## Operations

### Extract key

**Input**: Complete commit message and configured ticket pattern.  
**Output**: First matching key, or an explicit no-match result.  
**Persistence**: None.

Examples:

| Commit message | Extracted key |
|---|---|
| `fix: [OOT-3751][taupik.pirdian@salt.co.id][SP29] fix tax calculation` | `OOT-3751` |
| `fix: [OOT-3751][taupik.pirdian@salt.co.id] fix tax calculation` | `OOT-3751` |
| `fix: [taupik.pirdian@salt.co.id][OOT-3751] fix tax calculation` | `OOT-3751` |
| `fix: [taupik.pirdian@salt.co.id][ORB-3751] fix tax calculation` | `ORB-3751` |

### Create ticket

**Input**: A key fully matching the configured pattern and an optional title (blank titles are treated as absent).  
**Output**: Newly created ticket.  
**Errors**: Invalid key; key already exists; storage failure.

### Find ticket

**Input**: Exact key.  
**Output**: Matching ticket.  
**Errors**: Invalid key; ticket not found; storage failure.

### Upsert ticket

**Input**: A key fully matching the configured pattern and an optional title (blank titles are treated as absent).  
**Output**: Created or updated ticket.  
**Behavior**: Non-blank title input is trimmed. Existing title is preserved when title is absent or blank. An identical key/title input is idempotent.  
**Errors**: Invalid key; storage failure.

## Persistence Contract

- `ticket_key` is unique.
- Lookups use exact key equality.
- Create must not overwrite an existing row.
- Upsert preserves row ID and creation timestamp.
- A title change updates `updated_at`; a no-op upsert leaves timestamps unchanged.
- Repository methods use context cancellation and parameterized SQL.
