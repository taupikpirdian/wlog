# Business Requirement Document
## Developer Worklog CLI

**Versi:** 1.0  
**Status:** Draft  
**Platform:** CLI  
**Backend:** Go  
**Storage:** SQLite  
**Integrasi:** Git + AI  
**Target:** Personal Developer Productivity Tool

---

# 1. Latar Belakang

Pencatatan aktivitas pekerjaan saat ini dilakukan secara manual menjelang akhir hari sebelum dimasukkan ke Jira.

Aktivitas selama bekerja dapat berupa:

- Development/coding
- Investigasi issue
- Support QA
- Testing
- Analisa log/Splunk
- Deployment
- Diskusi teknis
- Meeting
- Research

Karena pencatatan dilakukan setelah pekerjaan selesai, developer perlu mengingat kembali aktivitas yang dilakukan sepanjang hari. Proses tersebut dapat membutuhkan sekitar 30 menit dan berpotensi menyebabkan detail pekerjaan terlewat.

Git commit dapat memberikan sebagian konteks aktivitas development, tetapi tidak dapat mencatat aktivitas yang tidak menghasilkan perubahan source code.

Dibutuhkan sebuah CLI sederhana yang dapat menangkap aktivitas selama pekerjaan berlangsung dan menghasilkan rangkuman yang siap digunakan sebagai Jira Worklog dan bahan update Description tiket.

---

# 2. Tujuan

Developer Worklog CLI bertujuan untuk:

1. Mengurangi waktu pencatatan worklog harian.
2. Mengurangi kebutuhan mengingat kembali aktivitas di akhir hari.
3. Mencatat aktivitas development secara otomatis dari Git commit.
4. Mencatat aktivitas non-development melalui session.
5. Mengelompokkan aktivitas berdasarkan Jira Ticket ID.
6. Mencatat durasi pekerjaan secara akurat.
7. Menghasilkan rangkuman aktivitas menggunakan AI.
8. Menghasilkan format yang siap dipindahkan ke Jira.
9. Menyimpan data secara lokal.

Target proses akhir:

**±30 menit pencatatan manual → ±3–5 menit review dan input Jira.**

---

# 3. Scope MVP

MVP memiliki fitur utama:

- Start work session
- Stop work session
- Add activity/note
- Automatic Git commit capture
- Automatic Jira Ticket ID detection
- Daily activity timeline
- Duration tracking
- Unsessioned activity
- Manual duration correction
- AI worklog summary
- AI Jira description generator
- Local SQLite storage

Integrasi langsung dengan Jira API belum termasuk MVP karena Jira hanya dapat diakses melalui corporate access environment.

---

# 4. Konsep Utama

Sistem memiliki dua sumber aktivitas.

## 4.1 Manual Activity

Digunakan untuk pekerjaan yang tidak menghasilkan Git commit.

Contoh:

- Support QA
- Check issue
- Analisa Splunk
- Testing
- Meeting
- Deployment
- Research

Developer memulai session:

```bash
wl s OOT-3668 "Support QA check issue"
```

Menambahkan informasi jika diperlukan:

```bash
wl n "Reproduce issue di preprod"
```

```bash
wl n "Check log Splunk"
```

Kemudian mengakhiri pekerjaan:

```bash
wl x
```

---

## 4.2 Git Activity

Git commit akan otomatis dicatat melalui Git `post-commit` hook.

Contoh:

```bash
git commit -m "OOT-3751 fix tax calculation"
```

CLI mengambil:

- Commit hash
- Commit message
- Ticket ID
- Repository
- Branch
- Timestamp
- Changed files
- Statistik perubahan

Ticket ID diekstrak dari commit message.

Contoh:

```text
OOT-3751 fix tax calculation
```

Menghasilkan:

```text
Ticket: OOT-3751
Activity: fix tax calculation
Source: GIT_COMMIT
```

---

# 5. Ticket Detection

Format Jira Ticket ID:

```regex
[A-Z][A-Z0-9]+-[0-9]+
```

Contoh:

```text
OOT-3751
OOT-3668
ABC-123
PROJECT1-999
```

Prioritas deteksi ticket untuk Git activity:

1. Commit message
2. Active session
3. Branch name
4. Unassigned

Commit message menjadi sumber utama karena proses development mewajibkan ticket ID berada pada commit message.

---

# 6. Work Session

Work session digunakan untuk mencatat durasi pekerjaan.

Contoh:

```bash
wl s OOT-3668 "Support QA issue activation"
```

Sistem menyimpan:

```text
Ticket       : OOT-3668
Title        : Support QA issue activation
Started At   : 14:05
Status       : ACTIVE
```

Ketika pekerjaan selesai:

```bash
wl x
```

Sistem menyimpan:

```text
Started : 14:05
Ended   : 15:20
Duration: 1h 15m
```

---

# 7. Active Session

Dalam MVP hanya diperbolehkan terdapat **satu active session**.

Jika:

```bash
wl s OOT-3751 "Check tax calculation"
```

dijalankan ketika `OOT-3668` masih aktif, CLI harus memberikan pilihan:

```text
OOT-3668 is currently active.

Started : 14:05
Elapsed : 45m

Stop OOT-3668 and start OOT-3751? [Y/n]
```

Jika dikonfirmasi:

```text
OOT-3668 → STOP
OOT-3751 → START
```

---

# 8. Manual Note

Developer dapat menambahkan note pada session aktif:

```bash
wl n "Check response di Splunk"
```

Contoh timeline:

```text
OOT-3668

14:05 START
14:20 NOTE    Reproduce issue
14:35 NOTE    Check Splunk
14:50 NOTE    Response API ditemukan null
15:20 STOP
```

Note tidak wajib digunakan.

Tujuannya hanya menyimpan informasi penting yang akan membantu AI memahami pekerjaan.

---

# 9. Automatic Git Capture

CLI menyediakan:

```bash
wl git
```

Command tersebut digunakan oleh Git `post-commit` hook:

```bash
#!/bin/sh

wl git
```

Setelah:

```bash
git commit -m "OOT-3751 fix tax calculation"
```

secara otomatis:

```text
✓ Commit captured

Ticket : OOT-3751
Commit : a82f91c
Message: fix tax calculation
```

Tidak diperlukan input tambahan dari developer.

---

# 10. Git Commit dengan Active Session

Jika active session:

```text
OOT-3751
```

dan commit:

```text
OOT-3751 fix tax calculation
```

commit dimasukkan sebagai activity pada session.

Contoh:

```text
OOT-3751
09:00 START
09:30 NOTE
10:15 COMMIT fix tax calculation
10:45 COMMIT add tax calculation test
11:00 STOP
```

---

# 11. Git Commit Tanpa Active Session

Jika tidak terdapat session aktif, commit tetap disimpan berdasarkan Ticket ID pada commit message.

Contoh:

```text
10:15 COMMIT
OOT-3751 fix tax calculation
```

Disimpan sebagai:

```text
Ticket : OOT-3751
Source : GIT_COMMIT
Session: NULL
```

Status:

```text
UNSESSIONED
```

Commit tetap dapat digunakan sebagai konteks AI, tetapi **tidak digunakan untuk menentukan durasi pekerjaan secara otomatis**.

---

# 12. Ticket Mismatch

Jika active session:

```text
OOT-3668
```

tetapi commit:

```text
OOT-3751 fix tax calculation
```

maka Ticket ID dari commit harus tetap dianggap sebagai ticket commit.

CLI memberikan warning:

```text
⚠ Ticket mismatch

Active Session : OOT-3668
Commit Ticket   : OOT-3751

Commit saved to OOT-3751 as unsessioned activity.
```

Commit tidak boleh secara otomatis dimasukkan ke `OOT-3668`.

---

# 13. Manual Time Adjustment

Developer dapat memperbaiki waktu session.

Contoh:

```bash
wl session OOT-3751 --from 09:00 --to 11:00
```

Atau ketika lupa memulai session:

```bash
wl s OOT-3751 "Fix tax calculation" --since 09:00
```

Shortcut:

```bash
wl s OOT-3751 "Fix tax calculation" -s 09:00
```

Sistem tidak boleh menggunakan AI untuk mengarang waktu mulai atau selesai.

---

# 14. Status Dashboard

Command:

```bash
wl
```

menampilkan status saat ini.

Contoh:

```text
Developer Worklog

● ACTIVE

OOT-3751
Fix tax calculation

Started : 13:15
Elapsed : 1h 24m

TODAY

OOT-3668     1h 15m
OOT-3751     1h 24m

Tracked      2h 39m

UNSESSIONED ACTIVITIES

OOT-3747
10:32  fix CIAM response validation

OOT-3751
12:45  add tax calculation test
```

---

# 15. Daily Timeline

Command:

```bash
wl today
```

menampilkan seluruh aktivitas hari tersebut.

Contoh:

```text
01 Oct 2026

OOT-3668

09:00 START
09:15 NOTE     Reproduce issue
09:35 NOTE     Check Splunk
10:10 STOP

Duration: 1h 10m


OOT-3751

10:30 START
10:45 COMMIT   fix tax calculation
11:20 COMMIT   add tax test
12:00 STOP

Duration: 1h 30m
```

---

# 16. AI Worklog Summary

Command:

```bash
wl summary
```

AI membaca:

- Session
- Duration
- Notes
- Git commits
- Commit messages
- Repository
- Branch
- Changed files
- Git diff/stat jika diizinkan

Data dikelompokkan berdasarkan Ticket ID.

Contoh input:

```text
OOT-3668
Duration: 1h 15m

- Support QA
- Reproduce issue
- Check Splunk
- Response CIAM FR ID null
```

Output:

```text
OOT-3668
Time Spent: 1h 15m

Activity:
Melakukan support QA untuk investigasi issue pada proses CIAM.

Investigation:
- Melakukan reproduce issue pada environment preprod
- Melakukan pengecekan request dan response melalui Splunk
- Mengidentifikasi kondisi FR ID null

Result:
Penyebab issue berhasil diidentifikasi dan hasil investigasi
disampaikan kepada QA.
```

---

# 17. Jira Description Generator

Command:

```bash
wl description OOT-3668
```

AI membaca seluruh aktivitas yang tersedia untuk ticket tersebut.

Output:

```text
Background
Terdapat issue pada proses validasi CIAM.

Investigation
- Reproduce issue pada environment preprod
- Analisa request dan response melalui Splunk

Root Cause
Response CIAM mengembalikan FR ID null pada kondisi tertentu.

Changes
Melakukan penyesuaian handling terhadap response CIAM.

Testing
Melakukan pengujian kembali pada environment preprod.

Result
Flow berhasil menangani kondisi tersebut sesuai expected behavior.
```

Output hanya berupa draft dan harus direview developer sebelum digunakan pada Jira.

---

# 18. Data Model

## work_sessions

```text
id
ticket_key
title
started_at
ended_at
duration_seconds
status
repository
created_at
updated_at
```

Status:

```text
ACTIVE
COMPLETED
```

## work_activities

```text
id
session_id
ticket_key
type
description
repository
branch
commit_hash
commit_message
metadata
created_at
```

Activity type:

```text
NOTE
GIT_COMMIT
INVESTIGATION
TESTING
QA_SUPPORT
MEETING
DEPLOYMENT
OTHER
```

`session_id` nullable agar Git commit tanpa active session tetap dapat disimpan.

---

# 19. Command Specification

| Command | Fungsi |
|---|---|
| `wl` | Melihat status |
| `wl s <ticket> "<title>"` | Start session |
| `wl n "<note>"` | Tambah note |
| `wl x` | Stop session |
| `wl git` | Capture Git commit |
| `wl today` | Aktivitas hari ini |
| `wl summary` | Generate Jira worklog |
| `wl description <ticket>` | Generate Jira description |
| `wl session` | Manual session/time |
| `wl assign` | Assign activity jika diperlukan |

Shortcut dibuat agar pencatatan aktivitas tidak mengganggu pekerjaan developer.

---

# 20. Git Hook Installation

CLI menyediakan command:

```bash
wl install-hook
```

Command memasang `post-commit` hook pada repository aktif.

Hook:

```bash
#!/bin/sh
wl git
```

Output:

```text
✓ Worklog Git hook installed
```

Dengan demikian developer tidak perlu menjalankan `wl git` secara manual.

---

# 21. AI Security

Source code perusahaan dianggap sebagai data sensitif.

Default AI context hanya:

```text
Ticket ID
Session title
Manual notes
Commit message
Changed file names
Git diff statistics
```

Contoh:

```text
3 files changed
42 insertions
18 deletions
```

Full Git diff **tidak dikirim ke AI secara default**.

Jika nantinya AI yang digunakan telah disetujui untuk source code perusahaan, tersedia konfigurasi:

```text
ai.include_diff = true
```

Default:

```text
ai.include_diff = false
```

---

# 22. Configuration

File konfigurasi:

```text
~/.worklog/config.yaml
```

Contoh:

```yaml
database:
  path: ~/.worklog/worklog.db

ticket:
  pattern: "[A-Z][A-Z0-9]+-[0-9]+"

git:
  auto_capture: true
  capture_changed_files: true
  capture_diff_stat: true
  capture_full_diff: false

ai:
  enabled: true
  include_diff: false
```

---

# 23. Error Handling

CLI harus menangani kondisi berikut:

**Tidak ada session ketika note ditambahkan**

```text
✗ No active work session.

Start one with:
wl s OOT-xxxx "Activity"
```

**Commit tidak memiliki Ticket ID**

```text
⚠ Ticket ID not found.

Commit saved as UNASSIGNED.
```

**Ticket commit berbeda dengan active session**

```text
⚠ Ticket mismatch.

Active : OOT-3668
Commit : OOT-3751

Commit saved to OOT-3751.
```

**Session sudah aktif**

Developer diminta mengakhiri session sebelumnya sebelum session baru dimulai.

---

# 24. Non-Functional Requirements

CLI harus:

- Berjalan cepat (<1 detik untuk operasi lokal).
- Tidak membutuhkan koneksi internet kecuali fitur AI.
- Menyimpan data secara lokal.
- Tidak mengganggu proses `git commit` jika worklog gagal.
- Tidak menyebabkan Git commit gagal jika SQLite/worklog mengalami error.
- Mendukung Linux dan macOS pada tahap awal.
- Memiliki backup/export data sederhana.
- Tidak menyimpan credential AI dalam database plaintext.

---

# 25. Future Development

Setelah MVP stabil, fitur dapat dikembangkan menjadi:

```text
MVP
 │
 ├── Git auto capture
 ├── Work session
 ├── Manual notes
 ├── Daily timeline
 └── AI summary
       │
       ▼
Phase 2
 ├── Automatic Jira integration
 ├── Notion integration
 ├── Weekly summary
 ├── Performance report
 ├── Multi-device synchronization
 ├── IDE integration
 ├── Automatic activity classification
 └── Background desktop agent
```

Notion juga nantinya dapat dihilangkan dari workflow jika `wl today` dan `wl summary` sudah menjadi sumber pencatatan utama.

---

# 26. Target User Flow

Flow ideal sehari-hari:

```text
Mulai Ticket
     │
     ▼
wl s OOT-3668 "Investigasi issue"
     │
     ├──── Support/Investigation
     │          │
     │          └── wl n "..."
     │
     ├──── Coding
     │          │
     │          └── git commit
     │                    │
     │                    └── Auto Capture
     │
     ▼
wl x
     │
     ▼
Ticket berikutnya
     │
     ▼
...
     │
     ▼
Akhir Hari
     │
     ▼
wl today
     │
     ▼
wl summary
     │
     ▼
AI Generate
     │
     ├── Jira Worklog
     └── Jira Description
     │
     ▼
Developer Review
     │
     ▼
Copy ke Jira
```

# 27. Success Criteria

MVP dianggap berhasil apabila:

1. Git commit dapat tercatat otomatis.
2. Ticket ID berhasil dideteksi dari commit message.
3. Aktivitas non-coding dapat dicatat dengan maksimal 1 command.
4. Durasi pekerjaan dapat dihitung berdasarkan session.
5. Lupa menjalankan session tidak menyebabkan Git activity hilang.
6. Aktivitas dapat dilihat berdasarkan tanggal dan ticket.
7. AI dapat menghasilkan Jira Worklog berdasarkan aktivitas yang tercatat.
8. AI dapat menghasilkan draft Description tiket.
9. Developer hanya membutuhkan beberapa menit di akhir hari untuk review dan input Jira.
10. Source code tidak dikirim ke layanan AI secara default.

---

# 28. Prinsip Utama

Developer Worklog CLI harus mengikuti prinsip:

**Capture while working, summarize later.**

Developer tidak perlu mengingat kembali seluruh pekerjaan pada akhir hari karena sistem telah menangkap konteks aktivitas selama pekerjaan berlangsung.

Git digunakan sebagai **automatic evidence**, session digunakan sebagai **time tracker**, manual note digunakan sebagai **context capture**, dan AI digunakan sebagai **summarizer**, bukan sebagai sumber kebenaran durasi pekerjaan.