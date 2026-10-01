# Data Model: Work Activity Notes

## Note Activity

Gunakan row `work_activities` bertipe `NOTE`. Semua field sudah ada dalam migration 001.

| Field | Nilai untuk note | Aturan |
| --- | --- | --- |
| id | ID activity hasil INSERT | Identitas unik, bukan hash deskripsi |
| ticket_id | ID tiket dari session yang diverifikasi | Wajib pada note; FK tickets |
| session_id | ID session aktif yang diverifikasi | Wajib pada note; FK work_sessions |
| type | `NOTE` | Konstan; tidak diklasifikasikan menjadi QA_SUPPORT/TESTING |
| description | Teks input setelah trim tepi | Wajib, nonempty; isi internal dipertahankan |
| repository | Repository session atau NULL | Tidak mengambil cwd/Git saat note |
| branch | NULL | Tidak dikumpulkan |
| commit_hash | NULL | Note bukan commit |
| commit_message | NULL | Tidak diisi dari description |
| changed_files | NULL | Tidak mengumpulkan file/diff |
| insertions | 0 | Default schema |
| deletions | 0 | Default schema |
| metadata | NULL | Tidak ada metadata tambahan pada fitur ini |
| created_at | Waktu application UTC RFC3339Nano | Eksplisit; >= waktu mulai session |

`ticket_key` diperlukan untuk output tetapi tidak ditambahkan ke tabel activity. Nilainya berasal dari tiket/session yang diverifikasi. Timestamp legacy tetap dibaca memakai parser storage yang mendukung timestamp SQLite dan RFC3339Nano.

## Relationships

```text
Ticket #2 (OOT-3668)
  └── Session #12 (ACTIVE saat pencatatan)
        ├── Note #101 — Check Splunk logs
        ├── Note #102 — Found response mismatch
        └── Note #103 — Support QA retest
```

Satu session memiliki banyak activity. Setiap note yang dibuat fitur ini mengacu pada tepat satu session dan tiket session tersebut. Nullable `ticket_id`/`session_id` di schema tetap diperlukan oleh aktivitas lain; tidak berarti fitur note boleh membuat aktivitas tanpa session.

## Session Snapshot

Gunakan entity session fitur 03 sebagai snapshot: `ID`, `TicketID`, `TicketKey`, `StartedAt`, `Status`, dan `Repository`. Snapshot hanya valid untuk note jika status ACTIVE dan ID/ticket/start masih sama saat write. Jika session berubah, tidak ada note tersimpan; pengguna mengulangi command untuk memakai keadaan baru.

## Invariants

1. `note.ticket_id == session.ticket_id` dan `note.session_id == session.id`.
2. Session harus ACTIVE saat insert di dalam transaksi; setelah itu session boleh diselesaikan tanpa menghapus note.
3. Description setelah trim tidak kosong dan tidak dipotong atau diringkas.
4. `note.created_at >= session.started_at`; waktu sama diperbolehkan.
5. Repository note sama dengan repository snapshot, termasuk NULL.
6. Setiap invocation berhasil menambah satu row; teks/timestamp sama bukan konflik.
7. Tidak ada INSERT/UPDATE tiket atau session saat membuat note.
8. Kegagalan sebelum commit tidak menghasilkan row baru atau perubahan entity terkait.

## State and Persistence Flow

Note tidak memiliki state lifecycle terpisah: `validated → committed`, atau gagal tanpa row. Tidak ada flag ACTIVE/COMPLETED pada activity.

Penambahan note tidak mengubah lifecycle session. Session ACTIVE tetap ACTIVE; setelah stop, note tetap melekat pada session COMPLETED. Durasi dihitung oleh session, tidak oleh jumlah/waktu note.

## Schema Decision

Tidak ada schema migration atau index baru. Index ticket/session/created_at yang sudah tersedia mendukung pembacaan fitur timeline berikutnya. Unique index repository+commit_hash tidak melakukan deduplikasi NOTE karena commit_hash NULL.
