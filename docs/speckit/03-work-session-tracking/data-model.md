# Data Model: Work Session Tracking

## Work Session

Gunakan tabel `work_sessions` dari ERD dan migrasi fitur 01.

| Field | Aturan |
|---|---|
| `id` | ID lokal stabil |
| `ticket_id` | Wajib, FK ke tiket fitur 02 |
| `title` | Wajib, dipangkas whitespace, tidak kosong |
| `repository` | Nullable; path root Git ketika start |
| `started_at` | Timestamp aktual UTC, wajib |
| `ended_at` | Null ketika aktif; timestamp aktual ketika selesai |
| `duration_seconds` | Null ketika aktif; detik penuh non-negatif ketika selesai |
| `status` | `ACTIVE` atau `COMPLETED` |
| `created_at` | Waktu pembuatan, tidak berubah |
| `updated_at` | Diperbarui pada penyelesaian |

## State Transitions

```text
Start → ACTIVE → Stop → COMPLETED
              → Confirm replace → COMPLETED + new ACTIVE
```

Session selesai tidak dibuka kembali oleh fitur ini. Pembatalan prompt tidak menghasilkan transisi.

## Invariants

- Maksimal satu row `ACTIVE` per database, dijaga `idx_single_active_session`.
- Session aktif mempunyai `ended_at = NULL` dan `duration_seconds = NULL`.
- Session selesai mempunyai waktu selesai tidak lebih awal dari waktu mulai dan durasi sesuai selisih timestamp.
- Tiket baru dan session baru dibuat dalam transaksi yang sama; title master tiket lama dipertahankan.
- Pergantian session menggunakan satu timestamp untuk akhir session lama dan awal session baru.
- Konflik atau kegagalan transaksi tidak mengubah session maupun tiket.

## Domain/Application Outcomes

`InvalidTicketKey`, `EmptyTitle`, `NoActiveSession`, `ActiveSessionConflict`, `InvalidTimeRange`, dan error storage merupakan hasil terpisah yang dapat dipetakan delivery ke pesan CLI. Snapshot konfirmasi berisi ID, tiket, judul, dan waktu mulai session aktif; ID tersebut diperiksa lagi ketika write dilakukan.
