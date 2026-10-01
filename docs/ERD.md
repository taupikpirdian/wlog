# ERD --- Developer Worklog CLI

## Overview

Database menggunakan **SQLite** dengan empat tabel utama:

-   `tickets` --- master tiket Jira lokal.
-   `work_sessions` --- pencatatan rentang waktu pekerjaan.
-   `work_activities` --- timeline aktivitas manual dan Git commit.
-   `ai_generations` --- histori hasil generate AI.

> Prinsip utama: `work_activities.session_id` bersifat nullable agar Git
> commit tetap dapat dicatat meskipun tidak ada work session aktif.

## Entity Relationship Diagram

``` mermaid
erDiagram
    TICKETS ||--o{ WORK_SESSIONS : has
    TICKETS ||--o{ WORK_ACTIVITIES : has
    WORK_SESSIONS ||--o{ WORK_ACTIVITIES : contains
    TICKETS ||--o{ AI_GENERATIONS : generates

    TICKETS {
        integer id PK
        text ticket_key UK
        text title
        text created_at
        text updated_at
    }

    WORK_SESSIONS {
        integer id PK
        integer ticket_id FK
        text title
        text repository
        text started_at
        text ended_at
        integer duration_seconds
        text status
        text created_at
        text updated_at
    }

    WORK_ACTIVITIES {
        integer id PK
        integer ticket_id FK
        integer session_id FK
        text type
        text description
        text repository
        text branch
        text commit_hash
        text commit_message
        text changed_files
        integer insertions
        integer deletions
        text metadata
        text created_at
    }

    AI_GENERATIONS {
        integer id PK
        integer ticket_id FK
        text type
        text content
        text generated_at
    }
```

## Relationship

``` text
TICKETS
   │
   ├────< WORK_SESSIONS
   │          │
   │          └────< WORK_ACTIVITIES
   │
   ├────< WORK_ACTIVITIES
   │
   └────< AI_GENERATIONS
```

Satu tiket dapat memiliki banyak work session dan activity. Activity
dapat terhubung ke sebuah session atau berdiri sendiri sebagai
**unsessioned activity**.

------------------------------------------------------------------------

## 1. Table `tickets`

Master tiket Jira yang pernah tercatat pada Worklog.

``` sql
CREATE TABLE tickets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    ticket_key TEXT NOT NULL UNIQUE,
    title TEXT,

    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

Contoh data:

    id ticket_key   title
  ---- ------------ -----------------------------
     1 OOT-3668     Support QA activation issue
     2 OOT-3751     Fix tax calculation

`title` dibuat nullable karena tiket dapat pertama kali terdeteksi hanya
dari commit message.

------------------------------------------------------------------------

## 2. Table `work_sessions`

Digunakan sebagai **time tracker** pekerjaan per tiket.

``` sql
CREATE TABLE work_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    ticket_id INTEGER NOT NULL,

    title TEXT NOT NULL,
    repository TEXT,

    started_at TEXT NOT NULL,
    ended_at TEXT,

    duration_seconds INTEGER,

    status TEXT NOT NULL
        CHECK(status IN ('ACTIVE', 'COMPLETED')),

    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY(ticket_id)
        REFERENCES tickets(id)
);
```

Contoh:

``` text
id               : 12
ticket_id        : 2
title            : Fix tax calculation
repository       : ms-orbiter
started_at       : 2026-10-01 09:00
ended_at         : 2026-10-01 11:30
duration_seconds : 9000
status           : COMPLETED
```

Durasi disimpan dalam detik agar mudah dihitung dan diformat oleh
aplikasi.

------------------------------------------------------------------------

## 3. Table `work_activities`

Menyimpan seluruh timeline aktivitas developer.

Untuk MVP terdapat dua activity type:

-   `NOTE`
-   `GIT_COMMIT`

``` sql
CREATE TABLE work_activities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    ticket_id INTEGER,
    session_id INTEGER,

    type TEXT NOT NULL
        CHECK(type IN (
            'NOTE',
            'GIT_COMMIT'
        )),

    description TEXT,

    repository TEXT,
    branch TEXT,

    commit_hash TEXT,
    commit_message TEXT,

    changed_files TEXT,

    insertions INTEGER DEFAULT 0,
    deletions INTEGER DEFAULT 0,

    metadata TEXT,

    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY(ticket_id)
        REFERENCES tickets(id),

    FOREIGN KEY(session_id)
        REFERENCES work_sessions(id)
);
```

### Manual Note

``` text
id          : 101
ticket_id   : 2
session_id  : 12
type        : NOTE
description : Check existing tax calculation
created_at  : 2026-10-01 09:15
```

### Git Commit

``` text
id             : 102
ticket_id      : 2
session_id     : 12
type           : GIT_COMMIT
commit_hash    : a82f91c
commit_message : OOT-3751 fix tax calculation
repository     : ms-orbiter
branch         : feature/OOT-3751-tax
insertions     : 42
deletions      : 18
created_at     : 2026-10-01 10:15
```

### Unsessioned Git Commit

Jika commit terjadi tanpa active session:

``` text
ticket_id   : 2
session_id  : NULL
type        : GIT_COMMIT
commit_hash : b71da21
```

Artinya tiket berhasil diketahui, tetapi activity tersebut tidak
mempunyai informasi durasi dari work session.

### `changed_files`

Disimpan sebagai JSON di kolom `TEXT`.

``` json
[
  "internal/service/tax.go",
  "internal/service/tax_test.go"
]
```

### `metadata`

Digunakan untuk metadata tambahan yang tidak memerlukan kolom khusus.

``` json
{
  "source": "post-commit",
  "git_root": "/Users/developer/project/ms-orbiter"
}
```

------------------------------------------------------------------------

## 4. Table `ai_generations`

Menyimpan histori output AI agar summary atau description sebelumnya
dapat dilihat kembali.

``` sql
CREATE TABLE ai_generations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    ticket_id INTEGER,

    type TEXT NOT NULL
        CHECK(type IN (
            'WORKLOG_SUMMARY',
            'TICKET_DESCRIPTION',
            'CODE_REVIEW'
        )),

    content TEXT NOT NULL,

    generated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY(ticket_id)
        REFERENCES tickets(id)
);
```

Jenis generation:

  Type                   Fungsi
  ---------------------- ----------------------------
  `WORKLOG_SUMMARY`      Hasil `wl summary`
  `TICKET_DESCRIPTION`   Hasil `wl description`
  `CODE_REVIEW`          Hasil AI pre-commit review

------------------------------------------------------------------------

## Indexes

### Work Session

``` sql
CREATE INDEX idx_work_sessions_ticket
ON work_sessions(ticket_id);

CREATE INDEX idx_work_sessions_started_at
ON work_sessions(started_at);
```

### Work Activity

``` sql
CREATE INDEX idx_work_activities_ticket
ON work_activities(ticket_id);

CREATE INDEX idx_work_activities_session
ON work_activities(session_id);

CREATE INDEX idx_work_activities_created_at
ON work_activities(created_at);
```

### Unique Git Commit

Commit dibuat unik berdasarkan repository dan commit hash.

``` sql
CREATE UNIQUE INDEX idx_work_activities_repo_commit
ON work_activities(repository, commit_hash)
WHERE commit_hash IS NOT NULL;
```

### Single Active Session

MVP hanya memperbolehkan satu work session aktif.

``` sql
CREATE UNIQUE INDEX idx_single_active_session
ON work_sessions(status)
WHERE status = 'ACTIVE';
```

------------------------------------------------------------------------

## Data Flow

``` text
wl s OOT-3751 "Fix tax"
        │
        ├── UPSERT tickets
        │
        └── INSERT work_sessions
                    │
wl n "Check calculation"
        │
        └── INSERT work_activities
                    │
git commit
        │
        └── post-commit
                │
                └── wl git
                       │
                       └── INSERT work_activities
                                    │
wl x                                │
 │                                  │
 └── UPDATE work_sessions           │
                                    │
wl summary                          │
 │                                  │
 └──────────── SELECT ──────────────┘
              │
              ↓
             AI
              │
              ↓
       ai_generations
```

------------------------------------------------------------------------

## Example Ticket Timeline

``` text
OOT-3751
│
├── Session #1 — 09:00 → 11:30
│   ├── 09:15 NOTE
│   │         Check existing tax calculation
│   │
│   ├── 10:15 GIT_COMMIT
│   │         fix tax calculation
│   │
│   └── 11:00 NOTE
│             Testing calculation
│
├── Session #2 — 13:00 → 14:00
│   └── 13:30 NOTE
│             Support QA testing
│
└── 15:20 GIT_COMMIT
          fix tax rounding
          session_id = NULL
          [UNSESSIONED]
```

------------------------------------------------------------------------

## MVP Design Notes

Untuk MVP belum diperlukan tabel terpisah untuk:

-   repositories
-   branches
-   activity types
-   application settings
-   changed files

Data tersebut masih cukup disimpan langsung pada `work_activities` atau
file konfigurasi lokal.

Struktur ini menjaga database tetap sederhana sambil tetap mendukung:

-   Work session
-   Manual note
-   Automatic Git capture
-   Commit tanpa session
-   Ticket history
-   AI worklog summary
-   AI ticket description
-   AI code review
