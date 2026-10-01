# Quickstart Validation: Git Commit Capture

## Prerequisites and Isolation

Fitur 05 sudah diimplementasikan. Panduan berikut memuat acceptance scenarios yang dapat diulang; hasil verifikasi dicatat pada bagian terakhir. Gunakan Git dengan capability offline/safe enrichment sesuai [plan](plan.md), executable terbaru, repository dan DB sementara. Jangan memasang hook fitur 06 untuk pengujian manual ini.

```sh
go test -race ./...
go vet ./...
go build -o /private/tmp/wl-git-check ./cmd/wlog
```

Test application/storage/CLI memakai dependency injection untuk path database sementara. Test reader memakai repository temporary dengan identity Git lokal dan hook dimatikan; tidak melakukan fetch/push. Pengujian executable berikut dijalankan pada akun OS test dengan konfigurasi/database terpisah, atau fixture executable dengan config injection yang didokumentasikan. Jangan mengubah config/database worklog pribadi untuk acceptance failures.

## 1. Setup Repository

Pada environment test terisolasi:

```sh
CAPTURE_REPO=$(mktemp -d /private/tmp/wlog-git-XXXXXX)
git init "$CAPTURE_REPO"
cd "$CAPTURE_REPO"
git config user.name "Worklog Fixture"
git config user.email "fixture@example.invalid"
git config core.hooksPath /dev/null
git switch -c feature/OOT-3751-tax
printf 'tax calculation\n' > tax.txt
git add tax.txt
git commit -m "OOT-3751 fix tax calculation"
```

Semua perubahan hanya pada repository test. Pastikan konfigurasi wl menunjuk DB test. Bila Git capability tidak tersedia, expected error yang jelas tanpa activity baru; jangan meniadakan proteksi offline sebagai workaround test.

## 2. Matching Session and Duplicate

```sh
/private/tmp/wl-git-check s OOT-3751 "Fix tax calculation"
/private/tmp/wl-git-check git
/private/tmp/wl-git-check git
```

Expected: capture pertama tersimpan pada tiket/session aktif; capture kedua `✓ Commit already captured.` exit 0. Verifikasi satu row GIT_COMMIT dengan canonical root+full HEAD hash, full message, branch, file/stat dan metadata provenance sesuai [data model](data-model.md).

Ulangi dari subdirektori dan melalui path symlink repository. Expected identity sama, row count tetap satu. Hentikan/ganti session kemudian capture hash sama lagi: activity pertama tidak diubah atau di-attach ulang.

## 3. No Session, Mismatch, and Fallback

Hentikan session; buat commit baru dengan message OOT-3751 lalu capture. Expected ticket_id terisi, session_id NULL, output UNSESSIONED, tanpa session baru/durasi.

Mulai session OOT-3668; buat commit baru OOT-3751 dan capture. Expected warning mismatch, tiket OOT-3751 tetap dipakai, session_id NULL; session OOT-3668 tidak berubah.

Gunakan commit hash baru untuk setiap kasus agar dedup tidak menutupi atribusi:

| Fixture | Expected |
| --- | --- |
| Message tanpa key; session A aktif; branch B | Ticket A, source ACTIVE_SESSION, attach A |
| Message tanpa key; tanpa session; branch B | Ticket B, source BRANCH, session NULL |
| Message tanpa key; tanpa session; branch tanpa key | UNASSIGNED, ticket/session NULL, warning |
| Message memuat dua key, termasuk key hanya di body | First-match full message mengikuti pattern fitur 02 |
| Config pattern custom | Message/branch extraction memakai pattern custom |
| Existing master title | Title/updated_at tiket existing tetap terjaga |

## 4. Commit Metadata Fixtures

- Root commit: path/stat mencakup perubahan terhadap empty tree.
- Non-root commit: bandingkan parent pertama, tidak membaca staged/unstaged changes setelah commit.
- Merge dengan dua parent: satu perbandingan terhadap parent pertama; tidak menjumlahkan perubahan seluruh parents.
- Empty commit: collected files [] dan stats 0/0, bukan NULL.
- Binary file: nama tersimpan; binary marker; tidak menambah total baris teks.
- Rename: old/new path tercatat sebagai delete/add sesuai no-renames contract.
- Filename whitespace/newline/Unicode: JSON path utuh, parser tidak split berdasarkan newline.
- Detached HEAD: branch NULL, message/session fallback tetap bekerja tanpa warning detached.
- HEAD bergerak dalam fake reader barrier: message/stat/hash berasal dari pinned commit; branch unstable tidak dipakai sebagai tiket fallback.

Timestamp yang disimpan adalah committer time UTC. Timestamp sebelum session start tidak menghasilkan perubahan durasi; captured_at tetap waktu pencatatan terpisah.

## 5. Configuration, Partial Failure, and Privacy

Gunakan config fixture untuk mengubah capture_changed_files/capture_diff_stat secara independen. Expected field disabled NULL dengan status disabled; collected empty diff berbeda dari disabled/unavailable. capture_full_diff=true memberi warning unsupported tanpa patch.

Fake runner mensimulasikan missing message/timestamp/branch/stat, timeout optional enrichment, dan missing required root/hash. Expected optional failure menyimpan data tersedia dengan warning/fallback; required identity failure tidak menulis. Detached branch bukan unavailable warning.

Tambahkan tracked .env dan nested .env.local dengan sentinel rahasia pada repository test. Expected nama dapat tersimpan tetapi numstat mengecualikan keluarga .env* sebelum pembacaan content; output/metadata tidak memuat sentinel. Process spy memverifikasi tidak ada patch/textconv/external diff/lazy fetch atau pembacaan working .env. Hasil statistik menandai non_env_text_files.

Fixture attributes/external helper harus membuktikan helper tidak dijalankan. Jalankan tanpa network access; partial-clone missing object tidak memicu fetch. Versi/capability Git unsupported memberi error atau unavailable enrichment sesuai kontrak, bukan fallback yang kehilangan proteksi.

## 6. Concurrency and Rollback

Gunakan dua koneksi ke DB fixture yang sama dan barrier deterministic:

1. Dua capture identity sama → satu Captured, satu AlreadyCaptured, satu row activity dan satu master tiket.
2. Dua root repository berbeda dengan hash sama → masing-masing boleh satu row capture.
3. Tahan metadata capture sebelum transaksi; stop/switch session di koneksi lain → resolver memakai active session pada write, bukan snapshot stale.
4. Capture menang writer lock dahulu → attribution sesuai session saat write, kemudian stop dapat menyelesaikan session tersebut.
5. Trigger INSERT activity gagal setelah ticket ensure → rollback termasuk tiket baru; existing session/ticket tidak berubah.
6. Context dibatalkan sebelum commit → tidak ada write parsial; operasi berikutnya tetap dapat memakai DB.
7. Output gagal setelah commit → row tetap ada; retry menghasilkan AlreadyCaptured.

Expected tidak ada perhitungan durasi commit, mutation session, atau duplicate attribution update.

## 7. Errors, Help, and Regression

```sh
/private/tmp/wl-git-check git --help
/private/tmp/wl-git-check help git
/private/tmp/wl-git-check git unexpected
```

Spy factory membuktikan help/invalid args tidak mengakses Git/config/storage. Uji Git unavailable, nonrepository, bare repository, unborn HEAD, config invalid, identity failure, dan DB lock/read/write failure. Semua fatal error non-zero tanpa pesan capture berhasil.

Regression session/NOTE tetap lulus. Command git tidak memasang hook atau mengubah ref/index/working files. Post-commit error swallowing baru divalidasi pada fitur 06.

## 8. Performance

Ukur executable hasil build, bukan go run, pada repository/database terinisialisasi dan tanpa lock contention. Buat minimal 20 commit kecil dengan identity berbeda; ukur tiap new capture serta repeated capture secara terpisah. Catat platform, versi/capability Git, ukuran repository/database, konfigurasi capture, median/p95/max, dan apakah config loader production atau fixture dipakai.

Target new capture normal <500 ms. Pisahkan cold initialization, binary/big commits, metadata timeout, dan writer lock wait. Dokumen desain tidak menyatakan target ini sudah tercapai.

## Implementation Verification — 2026-10-01

- `go test -race ./...`: lulus, termasuk regression session/NOTE, attribution priority, custom pattern, config options, identity failures, serta CLI warning/exit/help behavior.
- `go vet ./...`: bersih. Executable production dan fixture database terisolasi berhasil dibuild.
- Real Git reader tests meliputi root/merge/rename/empty/binary, Unicode/tab/newline path, full message body, detached HEAD, uncommitted changes, sensitive env exclusions, optional read failure, immutable hash saat HEAD berubah, unborn/bare/nonrepository, dan cancelled read.
- SQLite tests meliputi matching/mismatch/fallback/unassigned, metadata/nullability, master title preservation, dedup tanpa tiket baru, identity antar-checkout, rollback setelah ticket ensure, dua capture bersamaan, dan pemilihan session pada write setelah state berubah saat reader berjalan.
- Smoke fixture dengan real Git+CLI+SQLite membuktikan 20 new captures dan retry tanpa duplikat, statistik tanpa env content, matching session, dan mismatch warning.
- Pengukuran pada macOS 26.5.2 arm64 / Git 2.52.0 memakai 20 commit kecil, flags file/stat aktif, satu activity awal, dan DB sudah terinisialisasi: new capture median **76,44 ms**, p95 **106,28 ms**, maksimum **118,54 ms**; duplicate median **75,30 ms**, p95 **78,30 ms**, maksimum **78,92 ms**.
- Fixture memakai production command/application/reader/storage dengan path DB diinjeksi; pembacaan config pribadi tidak diukur. Putaran awal yang mencakup inisialisasi memiliki satu maksimum **720,74 ms**; hasil tersebut tidak dipakai sebagai klaim target kondisi terinisialisasi. Target end-to-end pada database/perangkat pengguna tetap memerlukan pengukuran tersendiri; tidak menjanjikan target saat timeout/lock contention.
