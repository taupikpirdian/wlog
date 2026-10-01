# Product Requirements Document
## Developer Worklog CLI (`wl`)

**Version:** 1.0  
**Status:** Draft / MVP  
**Platform:** CLI  
**Language:** Go  
**Storage:** SQLite  
**Integration:** Git + AI  
**Primary Output:** Jira Worklog & Jira Description Draft

---

# 1. Product Overview

Developer Worklog CLI (`wl`) adalah aplikasi command-line untuk mencatat aktivitas developer selama bekerja tanpa harus mengingat kembali seluruh aktivitas pada akhir hari.

Aplikasi menggabungkan:

- Work session
- Manual note
- Git commit
- Ticket ID
- Duration tracking
- AI summarization

menjadi draft worklog yang siap dipindahkan ke Jira.

Prinsip utama:

> **Capture while working, summarize later.**

Git menjadi sumber aktivitas development, session menjadi sumber durasi, manual note menjadi sumber konteks pekerjaan non-coding, dan AI digunakan untuk merangkum data yang sudah tercatat.

---

# 2. Problem Statement

Workflow saat ini:

```text
Developer bekerja
       ↓
Coding / Support / Testing / Investigation
       ↓
Aktivitas tidak langsung dicatat
       ↓
18:00
       ↓
Mengingat pekerjaan hari tersebut
       ↓
Catat ke Notion
       ↓
Rapikan worklog
       ↓
Input Jira
```

Proses tersebut dapat membutuhkan sekitar 30 menit per hari.

Masalah utama:

1. Aktivitas mudah terlupakan.
2. Durasi pekerjaan sulit diingat.
3. Git hanya merepresentasikan pekerjaan coding.
4. Support QA dan investigasi sering tidak menghasilkan commit.
5. Pencatatan manual menjadi pekerjaan tambahan.
6. Informasi commit belum otomatis dimanfaatkan untuk worklog.
7. Description tiket tidak selalu diperbarui dengan konteks implementasi terbaru.

---

# 3. Product Goals

`wl` harus memungkinkan developer:

- Memulai pencatatan pekerjaan dalam beberapa detik.
- Mengukur durasi pekerjaan per tiket.
- Mencatat aktivitas non-coding.
- Menangkap Git commit secara otomatis.
- Mendeteksi Jira Ticket ID dari commit.
- Melihat timeline pekerjaan.
- Menghasilkan Jira Worklog menggunakan AI.
- Menghasilkan draft Description Jira.
- Menyimpan seluruh data secara lokal.

Target:

```text
Current:
±30 menit/hari

Target:
±3–5 menit/hari
```

untuk proses review dan pemindahan data ke Jira.

---

# 4. Non-Goals MVP

MVP tidak mencakup:

- Direct Jira API integration
- Automatic Jira update
- Notion API integration
- Employee monitoring
- Screenshot tracking
- Keyboard/mouse tracking
- Automatic time estimation menggunakan AI
- Cloud synchronization
- Multi-user
- Web dashboard
- Mobile application

---

# 5. Terminology

## Ticket

Jira Ticket ID.

Contoh:

```text
OOT-3668
OOT-3751
```

## Session

Rentang waktu developer mengerjakan sebuah tiket.

```text
09:00 → 11:00
OOT-3751
```

## Activity

Aktivitas yang terjadi selama pekerjaan.

Contoh:

```text
NOTE
GIT_COMMIT
TESTING
QA_SUPPORT
```

## Unsessioned Activity

Aktivitas yang memiliki Ticket ID tetapi tidak memiliki session.

Umumnya terjadi ketika commit dibuat tanpa menjalankan `wl s`.

## Unassigned Activity

Aktivitas yang tidak memiliki Ticket ID.

---

# 6. Main User Flow

## Coding Flow

```text
wl s OOT-3751 "Fix tax calculation"
             ↓
           Coding
             ↓
git commit -m "OOT-3751 fix tax calculation"
             ↓
        Git post-commit
             ↓
           wl git
             ↓
     Commit automatically
          captured
             ↓
           Coding
             ↓
           wl x
```

## Support / Investigation Flow

```text
wl s OOT-3668 "Support QA issue CIAM"
             ↓
wl n "Reproduce issue"
             ↓
wl n "Check Splunk"
             ↓
wl n "Found FR ID null"
             ↓
wl x
```

## End-of-Day Flow

```text
wl today
    ↓
Review activity
    ↓
wl summary
    ↓
AI processing
    ↓
Jira-ready worklog
    ↓
Copy → Jira
```

---

# 7. CLI Commands

MVP menyediakan command:

```text
wl
wl start
wl note
wl stop
wl git
wl today
wl summary
wl description
wl session
wl assign
wl install-hook
```

Alias:

```text
start → s
note  → n
stop  → x
```

---

# 8. Feature — Status Dashboard

## Command

```bash
wl
```

## Requirement

Menampilkan:

- Active session
- Elapsed time
- Total tracked time hari ini
- Ticket yang dikerjakan
- Unsessioned activities
- Unassigned activities

## Example

```text
Developer Worklog
────────────────────────────

● ACTIVE

OOT-3751
Fix tax calculation

Started   13:15
Elapsed   01:24

TODAY
────────────────────────────

OOT-3668     1h 15m
OOT-3751     1h 24m

Total        2h 39m

UNSESSIONED
────────────────────────────

OOT-3747
10:32  fix CIAM response validation
```

---

# 9. Feature — Start Session

## Command

```bash
wl start OOT-3668 "Support QA issue"
```

Alias:

```bash
wl s OOT-3668 "Support QA issue"
```

## Optional

```bash
wl s OOT-3668 "Support QA issue" --since 09:00
```

Alias:

```bash
wl s OOT-3668 "Support QA issue" -s 09:00
```

## Validation

Ticket harus sesuai:

```regex
[A-Z][A-Z0-9]+-[0-9]+
```

## Stored Data

```text
ticket_key
title
started_at
status = ACTIVE
```

## Output

```text
✓ Session started

OOT-3668
Support QA issue

Started: 09:00
```

---

# 10. Active Session Constraint

MVP hanya mengizinkan satu session aktif.

Jika terdapat session aktif:

```bash
wl s OOT-3751 "Fix tax"
```

CLI menampilkan:

```text
⚠ Active session detected

OOT-3668
Support QA issue

Started : 09:00
Elapsed : 45m

Stop this session and start OOT-3751? [Y/n]
```

Jika `Y`:

```text
OOT-3668 → COMPLETED
OOT-3751 → ACTIVE
```

---

# 11. Feature — Add Note

## Command

```bash
wl note "Check Splunk response"
```

Alias:

```bash
wl n "Check Splunk response"
```

## Requirement

Note harus masuk ke active session.

## Stored Activity

```text
type        = NOTE
ticket_key  = active_session.ticket_key
session_id  = active_session.id
description = input
created_at  = now
```

## Output

```text
✓ Note added to OOT-3668
```

## No Active Session

Jika tidak ada session:

```text
✗ No active session.

Start one using:

wl s OOT-xxxx "Activity"
```

---

# 12. Feature — Stop Session

## Command

```bash
wl stop
```

Alias:

```bash
wl x
```

## Behavior

Mengambil active session kemudian mengisi:

```text
ended_at
duration_seconds
status = COMPLETED
```

## Output

```text
✓ Session completed

OOT-3668
Support QA issue

09:00 → 10:15
Duration: 1h 15m
```

---

# 13. Feature — Automatic Git Capture

Git commit harus dapat tercatat otomatis.

Developer menjalankan:

```bash
git commit -m "OOT-3751 fix tax calculation"
```

Git menjalankan:

```text
.git/hooks/post-commit
```

Hook:

```bash
#!/bin/sh

wl git >/dev/null 2>&1 || true
```

Penting:

> Kegagalan Worklog tidak boleh menyebabkan Git commit gagal.

---

# 14. Feature — Git Capture

## Command

```bash
wl git
```

## Data yang diambil

CLI menjalankan:

```text
git rev-parse HEAD
git log -1 --pretty=%B
git branch --show-current
git rev-parse --show-toplevel
git diff-tree
```

Data:

```text
commit_hash
commit_message
branch
repository
changed_files
insertions
deletions
timestamp
```

---

# 15. Ticket Detection

Prioritas Ticket ID:

```text
1. Commit message
2. Active session
3. Branch
4. UNASSIGNED
```

Contoh:

```text
OOT-3751 fix tax calculation
```

Detected:

```text
OOT-3751
```

Regex:

```regex
[A-Z][A-Z0-9]+-[0-9]+
```

---

# 16. Commit + Matching Session

Active session:

```text
OOT-3751
```

Commit:

```text
OOT-3751 fix tax calculation
```

Activity:

```text
session_id = current_session
ticket_key = OOT-3751
type       = GIT_COMMIT
```

Timeline:

```text
09:00 START
09:30 NOTE
10:15 COMMIT fix tax calculation
11:00 STOP
```

---

# 17. Commit Without Session

Commit:

```text
OOT-3751 fix tax calculation
```

Tidak terdapat active session.

Activity tetap disimpan:

```text
ticket_key = OOT-3751
session_id = NULL
type       = GIT_COMMIT
```

Activity memiliki status konseptual:

```text
UNSESSIONED
```

CLI tidak menentukan durasi berdasarkan timestamp commit.

---

# 18. Ticket Mismatch

Active session:

```text
OOT-3668
```

Commit:

```text
OOT-3751 fix tax calculation
```

Commit **tidak boleh** dimasukkan ke session `OOT-3668`.

Disimpan:

```text
ticket_key = OOT-3751
session_id = NULL
```

Output jika command dijalankan manual:

```text
⚠ Ticket mismatch

Active session : OOT-3668
Commit ticket  : OOT-3751

Commit saved under OOT-3751.
```

---

# 19. Duplicate Commit Protection

Git hook dapat terpanggil lebih dari satu kali.

`commit_hash` harus unique.

Database constraint:

```sql
CREATE UNIQUE INDEX idx_commit_hash
ON work_activities(commit_hash)
WHERE commit_hash IS NOT NULL;
```

Jika commit sudah tercatat:

```text
✓ Commit already captured.
```

Command tetap exit code `0`.

---

# 20. Feature — Today

## Command

```bash
wl today
```

## Output

Data dikelompokkan berdasarkan Ticket ID.

```text
01 Oct 2026
────────────────────────────

OOT-3668

09:00 START
09:20 NOTE      Reproduce issue
09:40 NOTE      Check Splunk
10:15 STOP

Tracked: 1h 15m


OOT-3751

10:30 START
10:45 COMMIT    fix tax calculation
11:20 COMMIT    add tax test
12:00 STOP

Tracked: 1h 30m

────────────────────────────

Total tracked: 2h 45m
```

---

# 21. Feature — Manual Session

Digunakan ketika developer lupa menjalankan `wl s`.

## Command

```bash
wl session OOT-3751 \
  --from 09:00 \
  --to 11:00 \
  --title "Fix tax calculation"
```

Sistem membuat completed session:

```text
09:00 → 11:00
Duration: 2h
```

Unsessioned activity `OOT-3751` yang berada pada rentang tersebut dapat ditawarkan untuk dimasukkan ke session.

```text
Found 2 activities:

09:45 COMMIT fix calculation
10:30 COMMIT add test

Attach to session? [Y/n]
```

---

# 22. Feature — AI Summary

## Command

```bash
wl summary
```

Optional:

```bash
wl summary --date 2026-10-01
```

atau:

```bash
wl summary OOT-3751
```

## Processing

```text
SQLite
  ↓
Group Ticket
  ↓
Sessions
  +
Notes
  +
Commits
  +
Git metadata
  ↓
AI
  ↓
Jira Worklog
```

---

# 23. AI Context

Default context yang boleh dikirim:

```text
Ticket ID
Session title
Duration
Manual notes
Commit messages
Changed filenames
Insertions/deletions
```

Tidak dikirim secara default:

```text
Full source code
Full Git diff
Secrets
Environment variables
Credentials
```

---

# 24. AI Worklog Output

AI menghasilkan format:

```text
OOT-3751
Time Spent: 2h 30m

Activity:
Melakukan analisa dan perbaikan perhitungan pajak.

Investigation:
- Melakukan pengecekan existing calculation
- Mengidentifikasi perhitungan inclusive yang masih digunakan

Implementation:
- Menyesuaikan perhitungan pajak
- Menambahkan validasi terkait calculation

Testing:
- Melakukan pengujian perhitungan
- Memvalidasi hasil pada beberapa skenario transaksi

Result:
Perhitungan berhasil disesuaikan dan hasil pengujian sesuai expected.
```

Section yang tidak memiliki evidence boleh dihilangkan.

AI tidak boleh membuat aktivitas yang tidak terdapat pada data input.

---

# 25. Feature — Description Generator

## Command

```bash
wl description OOT-3751
```

AI membaca seluruh history lokal tiket.

Output:

```text
Background

Terdapat perbedaan hasil perhitungan pajak pada transaksi tertentu.

Investigation

Dilakukan pengecekan terhadap existing calculation dan ditemukan
flow yang masih menggunakan perhitungan inclusive.

Changes

- Menyesuaikan tax calculation
- Menambahkan validation
- Menambahkan test

Testing

Pengujian dilakukan pada beberapa skenario transaksi.

Result

Hasil perhitungan sudah sesuai dengan expected calculation.
```

Description bersifat **draft**, bukan otomatis dikirim ke Jira.

---

# 26. AI Hallucination Prevention

AI harus mengikuti rule:

```text
DO NOT:
- invent activity
- invent duration
- invent root cause
- invent testing result
- invent deployment status
```

Jika data tidak cukup:

```text
Result:
Belum terdapat informasi hasil akhir pada worklog.
```

Lebih baik menghasilkan informasi tidak lengkap daripada membuat fakta baru.

---

# 27. Database Schema

## work_sessions

```sql
CREATE TABLE work_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    ticket_key TEXT NOT NULL,
    title TEXT NOT NULL,

    started_at DATETIME NOT NULL,
    ended_at DATETIME,

    duration_seconds INTEGER,

    status TEXT NOT NULL,

    repository TEXT,

    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
```

Status:

```text
ACTIVE
COMPLETED
```

## work_activities

```sql
CREATE TABLE work_activities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    session_id INTEGER,
    ticket_key TEXT,

    type TEXT NOT NULL,
    description TEXT,

    repository TEXT,
    branch TEXT,

    commit_hash TEXT,
    commit_message TEXT,

    changed_files TEXT,

    insertions INTEGER,
    deletions INTEGER,

    metadata TEXT,

    created_at DATETIME NOT NULL,

    FOREIGN KEY(session_id)
        REFERENCES work_sessions(id)
);
```

---

# 28. Activity Types

MVP:

```text
NOTE
GIT_COMMIT
```

Future:

```text
QA_SUPPORT
INVESTIGATION
TESTING
MEETING
DEPLOYMENT
RESEARCH
```

Pada MVP, konteks detail cukup disimpan sebagai `NOTE`.

AI dapat mengklasifikasikan aktivitas saat proses summary tanpa mengubah raw data.

---

# 29. Configuration

Location:

```text
~/.worklog/config.yaml
```

Example:

```yaml
database:
  path: ~/.worklog/worklog.db

ticket:
  pattern: "[A-Z][A-Z0-9]+-[0-9]+"

git:
  capture_changed_files: true
  capture_diff_stat: true
  capture_full_diff: false

ai:
  enabled: true
  include_diff: false
```

---

# 30. Local Data Directory

```text
~/.worklog/
├── config.yaml
├── worklog.db
└── logs/
```

Tidak ada data project yang ditulis ke repository kecuali Git hook.

---

# 31. Git Hook Installation

## Command

```bash
wl install-hook
```

Requirement:

1. Validasi current directory adalah Git repository.
2. Cari `.git/hooks`.
3. Jangan menghapus hook existing.
4. Jika `post-commit` sudah ada, append integration secara aman.
5. Pastikan executable.

Output:

```text
✓ Git integration installed

Repository:
~/projects/ms-orbiter

Commits will automatically be captured.
```

---

# 32. Failure Handling

Worklog tidak boleh mengganggu pekerjaan utama.

Prinsip:

```text
Git > Worklog
```

Jika database gagal:

```text
⚠ Worklog capture failed.
Git commit was not affected.
```

Jika AI gagal:

```text
✗ Unable to generate summary.

Your local worklog data is safe.
```

Jika Git metadata gagal dibaca, simpan data yang tersedia.

---

# 33. Privacy & Security

Requirement:

- SQLite hanya lokal.
- Full Git diff disabled by default.
- `.env` tidak pernah dibaca.
- AI API key tidak disimpan dalam SQLite.
- API key diambil dari environment variable.

Contoh:

```bash
export WORKLOG_AI_API_KEY="..."
```

CLI tidak boleh menampilkan API key pada log.

---

# 34. Performance Requirements

Target operasi lokal:

```text
wl                < 200ms
wl n              < 200ms
wl s              < 200ms
wl x              < 200ms
wl git            < 500ms
wl today          < 500ms
```

AI operation tidak termasuk target tersebut.

Git post-commit capture harus terasa tidak mengganggu proses development.

---

# 35. Recommended Go Project Structure

```text
worklog/
├── cmd/
│   ├── root.go
│   ├── start.go
│   ├── stop.go
│   ├── note.go
│   ├── git.go
│   ├── today.go
│   ├── summary.go
│   ├── description.go
│   ├── session.go
│   └── install_hook.go
│
├── internal/
│   ├── activity/
│   ├── session/
│   ├── git/
│   ├── ticket/
│   ├── summary/
│   ├── ai/
│   ├── config/
│   └── storage/
│
├── migrations/
│   └── 001_initial.sql
│
├── main.go
├── go.mod
└── README.md
```

---

# 36. Recommended Dependencies

CLI:

```text
github.com/spf13/cobra
```

SQLite:

```text
modernc.org/sqlite
```

Configuration:

```text
github.com/spf13/viper
```

Architecture tetap dibuat sederhana. Repository/service abstraction hanya ditambahkan jika memberikan manfaat nyata.

---

# 37. Acceptance Criteria — Session

### AC-01

Given tidak terdapat active session  
When:

```bash
wl s OOT-3668 "Support QA"
```

Then session `OOT-3668` berstatus `ACTIVE`.

### AC-02

Given terdapat active session  
When user menjalankan session baru  
Then CLI meminta konfirmasi sebelum menutup session lama.

### AC-03

When:

```bash
wl x
```

Then:

```text
ended_at != NULL
duration_seconds > 0
status = COMPLETED
```

---

# 38. Acceptance Criteria — Note

### AC-04

Given session `OOT-3668` aktif  
When:

```bash
wl n "Check Splunk"
```

Then note terhubung dengan session tersebut.

### AC-05

Given tidak terdapat active session  
When menjalankan `wl n`  
Then CLI tidak membuat activity dan memberikan informasi bahwa session belum aktif.

---

# 39. Acceptance Criteria — Git

### AC-06

Given commit:

```text
OOT-3751 fix tax calculation
```

When Git hook dijalankan  
Then `OOT-3751` berhasil dideteksi.

### AC-07

Given active session `OOT-3751`  
When commit `OOT-3751` terjadi  
Then commit terhubung ke session.

### AC-08

Given tidak ada active session  
When commit `OOT-3751` terjadi  
Then activity tetap tersimpan dengan `session_id = NULL`.

### AC-09

Given active session `OOT-3668`  
When commit `OOT-3751` terjadi  
Then commit tidak boleh terhubung ke session `OOT-3668`.

### AC-10

Given commit hash sudah tersimpan  
When hook dijalankan kembali  
Then duplicate activity tidak dibuat.

---

# 40. Acceptance Criteria — AI

### AC-11

Given terdapat session dan activity  
When:

```bash
wl summary
```

Then worklog dikelompokkan berdasarkan Ticket ID.

### AC-12

AI tidak boleh mengubah calculated duration.

### AC-13

AI tidak boleh membuat activity yang tidak terdapat pada input.

### AC-14

Unsessioned Git commit boleh digunakan sebagai konteks tetapi tidak menambah tracked duration.

---

# 41. MVP Development Priority

## P0 — Core

```text
[ ] SQLite initialization
[ ] Config initialization
[ ] wl
[ ] wl start
[ ] wl note
[ ] wl stop
[ ] wl today
```

## P1 — Git

```text
[ ] Ticket extraction
[ ] wl git
[ ] Commit metadata
[ ] Duplicate protection
[ ] Ticket mismatch
[ ] wl install-hook
```

## P2 — Recovery

```text
[ ] Unsessioned activity
[ ] Manual session
[ ] Manual time adjustment
```

## P3 — AI

```text
[ ] AI provider
[ ] wl summary
[ ] Jira worklog prompt
[ ] wl description
[ ] Hallucination guard
```

---

# 42. MVP Definition of Done

MVP dianggap selesai apabila skenario berikut dapat dilakukan:

```text
08:30

$ wl s OOT-3668 "Support QA activation issue"

09:00

$ wl n "Reproduce issue di preprod"

09:30

$ wl n "Check Splunk"

10:00

$ wl x


10:15

$ wl s OOT-3751 "Fix tax calculation"

10:45

$ git commit -m "OOT-3751 fix tax calculation"

11:30

$ git commit -m "OOT-3751 add tax calculation test"

12:00

$ wl x


18:00

$ wl today

$ wl summary
```

Dan menghasilkan:

```text
OOT-3668
Time Spent: 1h 30m

Activity:
Support QA dan melakukan investigasi activation issue.

Investigation:
- Reproduce issue pada environment preprod
- Melakukan pengecekan melalui Splunk


OOT-3751
Time Spent: 1h 45m

Activity:
Melakukan perbaikan tax calculation.

Implementation:
- Memperbaiki tax calculation
- Menambahkan test terkait perhitungan pajak
```

Developer hanya perlu melakukan review sebelum memindahkan hasil tersebut ke Jira.

---

# 43. Future Roadmap

### Phase 2 — Productivity

- Weekly summary
- Search ticket history
- Export Markdown
- Export JSON
- Worklog templates
- Interactive TUI
- Shell autocomplete

### Phase 3 — Intelligence

- AI commit analysis
- Optional Git diff analysis
- Automatic activity classification
- Generate root cause draft
- Generate testing summary
- Detect missing context before summary

### Phase 4 — Integration

Jika akses memungkinkan:

```text
wl sync jira
```

dapat:

- Fetch ticket
- Update description
- Submit worklog
- Add comment

Tetap membutuhkan user confirmation sebelum melakukan perubahan Jira.

---

# 44. Product Principle

`wl` tidak ditujukan untuk memonitor developer.

`wl` adalah **personal work memory assistant**.

Sistem bertugas menangkap fakta:

```text
When?
What ticket?
What activity?
What commit?
How long?
```

AI kemudian membantu menjawab:

```text
Apa yang sebenarnya dikerjakan hari ini?
```

tanpa developer harus mengingat ulang seluruh aktivitas pada akhir hari.