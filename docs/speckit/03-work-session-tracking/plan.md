# Implementation Plan: Work Session Tracking

**Status**: Implemented  
**Feature**: `03-work-session-tracking`

## Technical Context

| Area | Rancangan |
|---|---|
| CLI | Cobra `start`/`s`, `stop`/`x`; parsing dan prompt di delivery |
| Domain | Session, state transition, validasi judul, perhitungan durasi |
| Application | Start, stop, snapshot session aktif, pergantian dengan ID yang disetujui |
| Storage | SQLite `work_sessions` dan `tickets` yang sudah ada |
| Time | Clock diinjeksi; simpan UTC RFC3339Nano dan tampilkan dalam timezone perangkat |
| Repository context | Adapter Git lokal, nullable jika tidak tersedia |
| Layout | `domain/session`, `application/session`, `delivery/cli`, `infrastructure/storage`; tanpa folder `internal/` |

## Architecture and Composition

Delivery memanggil application untuk memuat snapshot session aktif dan memproses start/stop. Clock dan repository context diberikan lewat constructor. Infrastructure menerapkan port penyimpanan yang dikonsumsi application. Composition root `cmd/wlog` memuat konfigurasi dan membuka database ketika command start/stop dijalankan; help/version tetap tidak membutuhkan storage.

Domain menghitung `floor((ended_at - started_at).Seconds())` dan menolak rentang negatif. SQLite hanya menangani persistence dan transaksi. Formatting durasi/timezone serta konfirmasi tetap di delivery.

## Atomic Storage Boundary

Gunakan port khusus untuk penyimpanan session, dengan operasi atomik start/replace dan complete. Jangan memanggil `ticket.Service.Upsert` lalu menulis session dalam dua transaksi terpisah: kegagalan session dapat meninggalkan record tiket baru.

Dalam transaksi start/replace, adapter memastikan tiket ada dengan konflik key yang mempertahankan title, memvalidasi ulang session aktif yang disetujui, menyelesaikan session lama jika ada, dan membuat session baru. Application menyediakan timestamp pergantian tunggal dan hasil perhitungan durasi. Infrastructure memvalidasi prasyarat snapshot secara kondisional sebelum write.

Untuk stop, gunakan pembaruan kondisional `WHERE id = ? AND status = 'ACTIVE'`. Bila tidak ada row terubah karena proses lain, kembalikan konflik dan jangan mengubah session lain. Unique index `idx_single_active_session` tetap menjadi pengaman terakhir untuk dua start bersamaan. Transaksi tidak dibiarkan terbuka selama prompt.

## Schema and Timestamp Compatibility

Tidak perlu migrasi baru: fitur 01 sudah membuat semua kolom dan partial unique index yang dibutuhkan. Adapter membaca timestamp SQLite legacy serta format UTC RFC3339Nano, dan menulis timestamp session baru dalam UTC RFC3339Nano. Parsing legacy tanpa offset memakai UTC, sesuai default `CURRENT_TIMESTAMP` SQLite.

## Constitution Check

Tidak tersedia constitution `.specify/`; gunakan skill Go Clean Architecture dan preferensi folder pengguna.

- [x] Domain tidak mengimpor Cobra, Git, atau SQLite.
- [x] Application bergantung pada port dan clock yang diinjeksi.
- [x] Prompt berada di delivery; SQL dan transaksi di infrastructure.
- [x] Pergantian session atomik dan konfirmasi tidak menahan transaksi.
- [x] Package berada di root, tanpa `internal/`.

## Phase 0 — Decisions

Lihat `research.md`: durasi aktual, waktu UTC, repository opsional, dan pengamanan pergantian terhadap perubahan session selama prompt.

## Phase 1 — Artifacts

- `data-model.md`: state dan invariant session.
- `contracts/cli.md`: command, prompt, output, dan error.
- `quickstart.md`: skenario penerimaan termasuk konkurensi dan rollback.

## Implementation Sequence

1. Tambahkan entity/state session dan perhitungan durasi dengan clock yang dapat diinjeksi.
2. Tambahkan application start/stop dan operasi penyimpanan atomik berbasis snapshot session aktif.
3. Implementasikan SQLite session store di atas koneksi fitur 01 dan identity tiket fitur 02.
4. Tambahkan adapter repository context dan penanganan konteks yang tidak tersedia.
5. Tambahkan command, alias, prompt interaktif, serta dependency wiring saat eksekusi command.
6. Verifikasi acceptance scenarios, konflik dua proses, dan rollback; perbarui README dengan command yang tersedia setelah implementasi.

## Risks

- Jam perangkat mundur: tolak durasi negatif, pertahankan session aktif, dan beri petunjuk koreksi jam.
- Session berubah saat prompt: konfirmasi terikat ID snapshot; kondisi berbeda mengembalikan konflik.
- Proses bersamaan: transaksi, update kondisional, dan unique index mencegah session aktif ganda.
- Database terkunci: timeout storage menghasilkan error; jangan membatalkan session lama tanpa transaksi berhasil.
