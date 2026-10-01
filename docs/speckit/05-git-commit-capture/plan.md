# Implementation Plan: Git Commit Capture

**Feature**: `05-git-commit-capture`  
**Status**: Implemented

## Technical Context

| Area | Rancangan |
| --- | --- |
| Command | Cobra `git`, tanpa argumen/prompt |
| Domain | `domain/activity`: commit evidence, attribution policy, typed outcome/warnings |
| Application | `application/activity`: Git capture usecase, injected GitReader/clock/persistence ports |
| Infrastructure Git | `infrastructure/gitcapture`: lokal read-only commands, immutable hash, NUL parser, deadline |
| Infrastructure SQL | Extend SQLiteActivityStore dengan transaksi capture atomik dan deduplikasi |
| Delivery | `delivery/cli/git.go`: args, output outcome, warning stderr, error mapping |
| Wiring | Lazy factory di cmd/wlog; config pattern/git options, OpenDatabase, reader, clock |
| Schema | work_activities/tickets existing; tanpa migration |
| Layout | Domain/application/infrastructure/delivery di root; tanpa internal |

## Workflow and Constitution Check

Repo tidak memiliki `.specify/`, constitution, template, extension hooks, atau setup-plan.sh. Gunakan docs/speckit yang diminta dan layout fitur sebelumnya tanpa mengganti branch atau menginisialisasi tooling baru.

- [x] Domain berisi keputusan atribusi; tidak mengimpor SQL, os/exec, atau Cobra.
- [x] Application bergantung pada port; tidak melakukan Git/SQL langsung.
- [x] Git reader dan SQLite transaksi di infrastructure; formatting di delivery.
- [x] Git subprocess selesai sebelum writer transaction dibuka.
- [x] Ticket creation dan capture atomik; dedup tidak mengubah atribusi existing.
- [x] Tidak membuat durasi, mengubah session, atau memasang hook fitur 06.
- [x] Tidak memakai folder internal atau layanan jaringan.
- [x] Review setelah desain: tetap kompatibel dengan NOTE dan nullable Git activity schema.

## Application Ports and Atomic Attribution

Reader mengembalikan snapshot commit, optional fields, status enrichment, dan warnings; root/hash wajib. Clock application menentukan captured_at dan fallback timestamp. Ticket pattern mempersiapkan kandidat message/branch; domain policy memilih attribution dengan active session yang diberikan.

Persistence port menerima evidence dan pure attribution resolver dari application. Callback resolver hanya menggunakan data yang sudah dibaca serta session aktif yang diberikan; tidak melakukan SQL, Git, clock read, atau I/O. Dengan cara ini aturan prioritas tetap di domain/application sementara session dipilih dalam transaksi SQLite.

Urutan adapter capture:

1. `BEGIN IMMEDIATE` memakai writeTransaction existing.
2. Cari activity berdasarkan canonical repository+hash. Jika ada, kembalikan AlreadyCaptured sebelum resolver atau INSERT tiket; jangan UPDATE row existing.
3. Baca active session pada koneksi transaksi, lalu panggil pure resolver. Session nil adalah keadaan normal, bukan error.
4. Bila key terdeteksi, pastikan tiket dengan INSERT ON CONFLICT DO NOTHING dan ambil ID; title tidak berubah. Tanpa key, ticket_id NULL.
5. INSERT GIT_COMMIT dengan session_id hanya bila resolver menentukan matching session. INSERT memakai parameter binding dan RETURNING ID.
6. COMMIT sebelum delivery mencetak sukses. Rollback pada failure/cancel; cleanup tidak memakai context command yang sudah dibatalkan.

Duplicate index existing menjadi pengaman akhir. Jangan memanggil ticket.Service.Upsert di luar transaksi. Dua capture bersamaan harus menghasilkan Captured + AlreadyCaptured. Stop/switch dapat berjalan sebelum atau sesudah transaksi capture; keputusan memakai session pada transaksi dan tidak bergantung pada snapshot sebelum Git subprocess.

## Git Snapshot and Parsing

Resolve non-bare worktree root, canonicalize symlink/path, lalu pin HEAD^{commit} sekali. Semua log/diff memakai hash itu. Metadata log mengambil full message, committer timestamp, dan parents; branch dari symbolic-ref, NULL pada detached HEAD. Verifikasi ulang HEAD setelah pembacaan branch: bila berubah, jangan gunakan branch untuk atribusi dan beri warning; hash evidence tetap yang dipilih awal.

Gunakan os/exec CommandContext dengan argument array, tanpa shell. Jalankan pada root repository. Nonaktifkan lazy fetch, replacement objects, external diff, dan textconv. Feature-detect kemampuan Git yang diperlukan; bila proteksi offline tidak tersedia, fail dengan pesan versi/kapabilitas sebelum capture. Untuk enrichment yang tidak tersedia, simpan identity dengan warning tanpa fallback command yang membaca working tree atau menjalankan external helper.

Daftar path memakai name-only NUL output. Statistik memakai numstat NUL output, tanpa rename detection, Myers algorithm eksplisit, tanpa indent heuristic dan color. Commit non-root memakai explicit parent pertama+hash; root memakai --root. Split numstat hanya pada dua tab pertama, kemudian NUL, bukan per baris. Binary -/- dicatat sebagai binary, tidak ditambahkan ke total baris teks. Empty diff yang berhasil menghasilkan [] dan 0/0.

Daftar nama boleh memuat .env*; statistik mengecualikan path keluarga .env* pada semua direktori agar isinya tidak dibaca. Pakai pathspec exclusions pada Git command sebelum diff dihitung, bukan filter hasil setelah content dibaca. Attributes yang diperlukan untuk enrichment berasal dari commit terpilih; dukungan --attr-source diperiksa. Tidak mengumpulkan full patch, membaca file kerja/.env, atau menjalankan submodule command.

Reader memiliki budget metadata lokal 2 detik dan mengikuti cancellation parent. Timeout optional enrichment mengembalikan metadata parsial; user/context cancellation tetap fatal tanpa write. Deadline ini bukan target performa: <500 ms tetap harus diukur pada kondisi normal.

## Configuration and Metadata Availability

- capture_changed_files=true: kumpulkan JSON path; false: NULL dan status disabled.
- capture_diff_stat=true: kumpulkan jumlah baris teks non-env dan binary markers; false: NULL totals/status disabled. Kedua opsi independen.
- Optional read failure: NULL pada field terkait dan status unavailable; jangan menyamakan dengan hasil kosong/nol yang berhasil diukur.
- capture_full_diff=true: tidak mengumpulkan patch; beri warning unsupported pada outcome baru dan metadata. Default false tidak menambah warning.
- AI config tidak memicu layanan apa pun pada capture.

Commit time disimpan UTC RFC3339Nano sebagai created_at; captured_at, time_source, ticket_source, status enrichment, stats scope, binary paths, dan warning codes disimpan dalam metadata JSON versioned. Stats/commit message nullable sesuai schema existing. Source/warning bukan enum activity type baru.

## Phase 0 and Phase 1 Artifacts

- [research.md](research.md): keputusan dan dokumentasi resmi Git pendukung.
- [data-model.md](data-model.md): nullable fields, metadata, invariant/dedup.
- [contracts/cli.md](contracts/cli.md): output dan exit status.
- [quickstart.md](quickstart.md): acceptance guide, DB/repository fixture terisolasi.
- [checklists/requirements.md](checklists/requirements.md): quality review.

Tidak ada keputusan yang belum terselesaikan. Pilihan callback resolver dan metadata model perlu dipertahankan saat implementasi agar tidak memindahkan aturan atribusi ke SQL adapter.

## Implementation Sequence

1. Tambahkan evidence DTO, attribution policy, typed warnings/outcomes, dan tests prioritas tiket.
2. Tambahkan GitReader port serta adapter local metadata reader; uji immutable hash, root/merge/binary/rename/path NUL, detached HEAD, dan failure parsial.
3. Tambahkan usecase Git capture serta persistence port dengan pure resolver.
4. Implementasikan transaksi SQLite capture, ticket ensure, identity lookup, dan dedup race; regression NOTE dan session tetap lulus.
5. Tambahkan command git, lazy wiring, output/warnings, args/help tests.
6. Verifikasi rollback, concurrent captures dan stop/switch, offline/privacy, config options, dan optional metadata failure.
7. Jalankan go test -race ./..., go vet ./..., build, serta pengukuran executable fixture untuk target <500 ms. Catat batas fixture/config/lock wait.
8. Perbarui README/status dokumen setelah implementasi selesai; hook installation tetap fitur 06.

## Risks

| Risiko | Mitigasi |
| --- | --- |
| HEAD berubah | Pin hash; semua commit reads memakai hash; branch dibuang bila konteks tidak stabil |
| Prioritas tiket/active state race | Pure domain resolver diberi active session di dalam writer transaction |
| Duplicate capture meninggalkan tiket | Lookup identity dahulu dan transaksi mencakup ticket/activity INSERT |
| Filename/parser ambiguity | NUL output, tabs pertama dua, no-renames, parser fixtures |
| Metadata gagal atau disabled dianggap zero | Nullable totals/paths plus status explicit |
| Git config memicu helper/network | no-ext-diff/no-textconv/no-lazy-fetch, capability check dan process fixture |
| Env file content bocor via stats | Pathspec exclude .env* sebelum numstat; test sensitive sentinel |
| Hash-only PRD bertentangan schema | Keputusan repository+hash didokumentasikan; no migration |
| Historical manual capture terlihat seperti durasi | Commit time/captured_at provenance dan tidak menghitung durasi |
