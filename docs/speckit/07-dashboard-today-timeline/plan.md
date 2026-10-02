# Implementation Plan: Dashboard & Today Timeline

**Feature**: `07-dashboard-today-timeline`

**Status**: Implemented and verified

## Technical Context

| Area | Rancangan |
| --- | --- |
| Runtime | Go sesuai go.mod, Cobra, SQLite modernc existing; tidak ada dependency baru |
| Domain | `domain/dashboard`: day window, clipping durasi, agregasi tiket, event dan stable ordering |
| Application | `application/dashboard`: read usecases melalui snapshot port dan clock/location injeksi |
| Infrastructure | `infrastructure/storage/dashboard.go`: pembacaan transaction-consistent dari tabel existing |
| Delivery | `delivery/cli/dashboard.go`: dashboard dan command today, perubahan root command dan command tests |
| Composition | `cmd/wlog/main.go`: dashboardFactory dengan loader/clock/location injeksi; production memakai loader existing dan time.Local |
| Schema | Tidak ada migration baru; START/STOP merupakan projection work_sessions |
| Verification | Unit TDD domain/application, SQLite fixtures, CLI output/error, regression 01–06 |

## Project Context and Workflow

Repo tidak memiliki `.specify/`, active template, constitution, extension hooks, atau setup-plan.sh. Ikuti struktur dokumen fitur sebelumnya pada direktori yang diberikan pengguna, tanpa membuat tooling baru atau berpindah branch. Research memakai goal, BRD/PRD, schema, dan source lokal; tidak memerlukan pilihan library baru. Dokumen ini adalah desain, bukan laporan implementasi.

## Architecture and Composition

Domain menentukan batas hari dari tanggal kalender dalam location yang diinjeksi dan menghitung irisan session. Application membaca clock tepat sekali, memanggil snapshot store, lalu menghasilkan dashboard/timeline melalui domain. Jangan letakkan SQL, Cobra, clock global, atau filesystem di domain/application.

Port `ReadSnapshot(ctx)` mengembalikan session dan activity records dengan ticket_key dari join. Untuk baseline, baca seluruh session/activity dari satu read transaction agar validasi waktu dan mixed timestamp format tidak bergantung pada string comparison SQL. Filter/range/aggregation dilakukan setelah timestamp berhasil diparse. Optimasi range query hanya boleh dilakukan kemudian dengan regression fixture semua format timestamp existing dan bukti performa.

Usecase Dashboard dan Today masing-masing memakai satu snapshot per command. Tidak perlu mempertahankan snapshot antar dua command berbeda. Result berisi detik mentah dan typed events; delivery memformat local time, durasi, header, dan sanitasi teks. Presentasi tidak mengubah model tersimpan.

Root RunE beralih dari initializer readiness output menjadi dashboard factory lazy. Factory memuat config dan OpenDatabase sehingga cold startup tetap menyediakan bootstrap fitur 01; hindari initialize lalu membuka DB dua kali. Pertahankan initializer service untuk tests/usecases existing bila masih dibutuhkan. Ubah tests root yang memang menguji readiness agar menguji perilaku dashboard baru, sambil menjaga bootstrap service tests. Tambahkan NoArgs pada root dan today, dan pertahankan help/version serta seluruh subcommand existing. Factory hanya dipanggil setelah validasi argumen; tutup DB pada seluruh return path dan propagasikan close/output errors.

## Snapshot and Timestamp Handling

1. Baca clock satu kali dan tangkap location yang berlaku untuk command.
2. Begin read transaction; baca session + ticket key, lalu activity + nullable ticket key/session ID. LEFT JOIN tiket pada activity mempertahankan unassigned records.
3. Parse started_at, ended_at, dan created_at memakai parser SQLite existing, termasuk RFC3339Nano dan legacy CURRENT_TIMESTAMP. Scan/parse/query/cancellation error menggagalkan snapshot, bukan skip row.
4. Selesaikan pembacaan dan transaction sebelum formatting; tidak memegang database selama output terminal.
5. Validasi status/rentang session: completed wajib memiliki end >= start, active tanpa end dan start <= now; multiple active/inconsistent data gagal. Nilai duration_seconds tidak menjadi sumber clipping harian.
6. Bentuk day window kalender lokal dan event range [dayStart, nextDayStart). Hitung session interval [start, min(end atau now, now)) yang diiris dengan day window.
7. Gunakan integer detik per kontribusi seperti semantik session existing, jumlahkan sebelum format menit. Session yang seluruhnya future completed memiliki kontribusi nol; event future dalam hari tetap evidence.

Semua persisted rows dibaca untuk baseline; corrupt row di luar hari ini juga dapat menggagalkan pembacaan. Dokumentasikan konsekuensi ini dan ukur fixture besar sebelum menyatakan performance gate lulus. Tidak ada write transaction atau perubahan work data. OpenDatabase existing tetap dapat membuat/memigrasikan schema saat initialization.

## Constitution Check

Tidak ada constitution formal; pemeriksaan berdasarkan arsitektur repo dan goal.

- [x] Domain pure; application bergantung pada domain dan port.
- [x] SQL/parse/transaksi berada pada infrastructure; rendering pada delivery.
- [x] Root folder layer existing dipertahankan, tanpa `internal/`.
- [x] Clock/location diinjeksi; tidak ada Git/network/prompt dalam pembacaan.
- [x] TDD wajib pada kode domain/application yang diubah.
- [x] Gate existing 100% statement coverage seluruh executable package domain/application dipertahankan.
- [x] Post-design: nullable activity, timestamp legacy, bootstrap, dan lazy help/version tetap kompatibel.

## Phase 0 — Decisions

Lihat [research.md](research.md) untuk sumber lokal dan keputusan kalender, chronology, snapshot, evidence, serta format. Tidak ada keputusan desain terbuka.

## Phase 1 — Artifacts

- [data-model.md](data-model.md): read projections, invariants dan rumus.
- [contracts/cli.md](contracts/cli.md): dashboard/timeline output serta exit behavior.
- [quickstart.md](quickstart.md): acceptance guide dan gates implementasi.
- [checklists/requirements.md](checklists/requirements.md): hasil validasi dokumen.

## Implementation Sequence

1. RED domain tests untuk day window, cross-midnight/offset changes, clipping, duration validation, ticket aggregation dan event ordering; implement GREEN lalu REFACTOR.
2. RED application tests dengan fake snapshot/clock/location untuk satu now, result mapping, error/cancel; implement GREEN lalu REFACTOR.
3. Tambahkan SQLite snapshot adapter dengan integration tests untuk nullable fields, timestamp formats, no writes, parse errors, dan concurrent stop/capture menggunakan dua koneksi/barrier.
4. Tambahkan delivery rendering/dashboard/today, sanitasi kontrol dan multiline, invalid args, output/close errors, lazy help/version tests.
5. Wire factory dan update root tests/README untuk behavior baru; pertahankan semua command existing.
6. Jalankan coverage, race, vet, build, isolated smoke dan performance fixture; catat bukti implementasi sebelum mengubah status dokumen.

## Risks and Verification

| Risiko | Mitigasi / verifikasi |
| --- | --- |
| Hitung hari sebagai 24 jam | Kalender lokal hari berikutnya, fixture DST dan Asia/Jakarta |
| NOTE/COMMIT menambah waktu | Agregasi hanya session, activity-only ticket 0m |
| Snapshot tercampur saat stop | Satu read transaction; concurrent writer fixture |
| SQL lexical timestamp filter | Parse existing formats lalu filter instant di domain |
| Cold startup mengubah bootstrap | Reuse loader/OpenDatabase, cold fixture dan lazy help test |
| Multiline/control merusak terminal | Rendering satu baris, strip ANSI/control; data asli tetap utuh |
| Baseline full scan melampaui target | Ukur DB dengan 10.000 session + 50.000 activity dan catat hasil; optimasi hanya bila gagal |
| Mengklaim test sudah lulus dari checklist | Checklist dokumen terpisah dari bukti implementasi di quickstart |
