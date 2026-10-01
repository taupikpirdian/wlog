# Implementation Plan: Work Activity Notes

**Feature**: `04-work-activity-notes`  
**Status**: Implemented

## Technical Context

| Area | Rancangan |
| --- | --- |
| CLI | Cobra `note` dengan alias `n`, tepat satu argumen, tanpa prompt |
| Domain | `domain/activity`: note, validasi description dan waktu terhadap start session |
| Application | `application/activity`: pemilihan session aktif, clock injeksi, penyimpanan berdasarkan snapshot |
| Infrastructure | SQLite activity store yang menerapkan port application |
| Delivery | `delivery/cli/notes.go`: parsing, output, pemetaan error |
| Composition | Wiring lazy di `cmd/wlog`, menggunakan loader/config dan `OpenDatabase` yang tersedia |
| Storage | Tabel `work_activities`, FK dan index dari migration 001; tidak perlu migrasi baru |
| Timestamp | UTC RFC3339Nano, kompatibel dengan parser timestamp fitur 02/03 |
| Context | Ticket ID, session ID, repository dari snapshot session; tidak memanggil Git |
| Verification | Fixture DB sementara, clock deterministik, dua koneksi ke DB yang sama, dan command tests |

## Project Context and Workflow

Repo tidak memiliki `.specify/`, template, constitution, extension hooks, atau script `setup-plan.sh`. Gunakan layout fitur 01–03 dalam direktori yang diminta pengguna; dokumen dibuat langsung tanpa inisialisasi tooling atau perpindahan branch. `spec.md` berisi kebutuhan produk; keputusan teknis berada dalam dokumen desain ini dan artefak pendamping.

## Architecture and Composition

Domain memiliki entity note dan aturan description nonempty serta `created_at >= started_at`. Application memilih snapshot session aktif melalui port, membaca clock yang diinjeksi, membentuk note dari session tersebut, dan memanggil operasi persistence atomik. Gunakan error session tidak aktif/konflik yang sudah tersedia bila maknanya sama; error description kosong dan clock note didefinisikan di domain.

Port yang dikonsumsi application menyediakan pembacaan session aktif serta penambahan note berdasarkan snapshot yang dipilih. Adapter SQLite dapat memakai query/scan session yang sudah tersedia, tanpa membuat application bergantung pada storage konkret. Result usecase menyertakan note tersimpan dan key tiket tujuan untuk output CLI; `ticket_key` berasal dari join/session, bukan kolom baru.

Factory note dipanggil hanya pada eksekusi command valid. Composition root memuat konfigurasi, membuka database, menginjeksi store dan `time.Now`, lalu menutup database setelah command. Bantuan note, help umum, dan version tidak memanggil factory. Pertahankan kontrak command start/stop beserta test fitur 03 saat memperluas root wiring.

## Atomic Persistence

1. Validasi deskripsi sebelum mengambil snapshot session.
2. Baca satu session aktif; bila tidak ada, kembalikan error tanpa write.
3. Application membaca waktu UTC dan membentuk note yang mewarisi ticket/session ID serta repository. Domain menolak waktu sebelum start.
4. Adapter memperoleh koneksi dan `BEGIN IMMEDIATE`, mengikuti pola session store. Di dalam transaksi, baca ulang session aktif dan cocokkan ID, ticket ID, serta waktu mulai snapshot.
5. Jika tidak cocok atau tidak lagi aktif, kembalikan konflik dan rollback; jangan mengganti tujuan note.
6. INSERT hanya kolom note yang diperlukan ke `work_activities`, menggunakan parameter binding, lalu ambil ID hasil insert.
7. COMMIT sebelum mengembalikan sukses. Kegagalan operation/commit memakai cleanup rollback yang tetap berjalan saat context command dibatalkan.

Snapshot tidak menahan transaksi terbuka. Note dan stop/switch diserialisasi melalui writer transaction. Jika note menang, aktivitas tersimpan pada session A dan proses lain boleh menyelesaikan A sesudahnya. Jika stop/switch menang setelah snapshot note dibaca, note gagal konflik. Jika perubahan sudah selesai sebelum pembacaan awal note, session aktif yang baru menjadi tujuan yang sah.

Helper transaksi SQLite boleh dipindahkan menjadi fungsi privat infrastructure yang dipakai session dan activity store. Pertahankan semantik cleanup, timeout, serta conditional update fitur 03; tidak perlu abstraksi transaksi pada domain/application.

## Schema Compatibility

Skema membolehkan `ticket_id`, `session_id`, dan description null karena juga melayani Git activity. Untuk note manual, application dan adapter menjamin ketiganya terisi. Jangan menambah NOT NULL global yang mengganggu unsessioned Git commit.

`created_at` note ditulis eksplisit dalam UTC RFC3339Nano dengan clock application; jangan memakai default `CURRENT_TIMESTAMP` untuk note baru. Repository mengikuti nullable snapshot. Branch/commit/diff/metadata tidak diisi. Insertions/deletions mengikuti default 0. Tidak ada write ke tabel tickets atau work_sessions.

## Constitution Check

Tidak ada constitution `.specify/`; gunakan arsitektur berlapis yang sudah tersedia dan preferensi pengguna.

- [x] Domain tidak bergantung pada Cobra, SQL, Git, atau layanan eksternal.
- [x] Application hanya bergantung pada domain, port persistence, dan clock injeksi.
- [x] SQL, lock, transaksi, dan parsing database berada di infrastructure.
- [x] Delivery menangani argumen, formatting, dan error command.
- [x] Tidak ada folder `internal/`.
- [x] Identitas tiket/session tidak ditentukan dari teks note atau current repository.
- [x] Tidak ada transaksi selama interaksi pengguna; command note tidak memakai prompt.
- [x] Review setelah desain: aturan session fitur 03 dan schema Git activity tetap kompatibel.

## Phase 0 — Decisions

Lihat [research.md](research.md) untuk keputusan atribusi, concurrency, isi note, repository, waktu, dan batas scope. Semua keputusan desain telah ditetapkan; tidak ada klarifikasi terbuka.

## Phase 1 — Artifacts

- [data-model.md](data-model.md): field note, nullability, relationships, dan invariant.
- [contracts/cli.md](contracts/cli.md): command, output, exit status, serta concurrency contract.
- [quickstart.md](quickstart.md): panduan validasi dengan penyimpanan terisolasi.
- [checklists/requirements.md](checklists/requirements.md): hasil review kualitas spesifikasi.

## Implementation Sequence

1. Tambahkan domain note, validasi whitespace, dan validasi clock terhadap start session.
2. Tambahkan application add-note dengan port snapshot dan operasi insert atomik, serta clock DI.
3. Tambahkan SQLite activity store; reuse scan session dan pola writer transaction dengan perubahan privat yang kecil bila diperlukan.
4. Tambahkan command `note`/`n`, validasi tepat satu argumen, output sesuai PRD, dan lazy dependency wiring.
5. Verifikasi input/no-active/rollback, race note-versus-stop/switch, dua note bersamaan, null fields, dan timestamp.
6. Jalankan regression fitur 01–03, `go test -race ./...`, `go vet ./...`, dan build; ukur command hasil build pada DB sementara untuk target <200 ms.
7. Perbarui README dan status dokumen saat implementasi telah selesai.

## Risks and Verification

| Risiko | Mitigasi / verifikasi |
| --- | --- |
| Session berubah setelah dibaca | Recheck snapshot dalam write transaction; test dua koneksi dengan urutan deterministik |
| Session/ticket tertulis sebagian | Insert note satu transaksi; failure trigger membuktikan rollback tanpa perubahan entity lain |
| Lock/cancel | Gunakan timeout database yang tersedia; error non-zero, rollback cleanup, dan test context sebelum commit |
| Deskripsi berubah atau terpotong | Parameter binding dan test Unicode, quote, newline serta whitespace luar |
| Jam perangkat mundur | Domain menolak waktu sebelum start; clock test tidak mengubah session |
| Retry menghasilkan duplikat | Dokumentasikan append per command dan error output setelah commit; tidak menjanjikan exactly-once lintas retry |
| Pengukuran performa bias | Ukur executable hasil build, pisahkan cold initialization dan lock wait; catat perangkat, ukuran DB, dan hasil |
