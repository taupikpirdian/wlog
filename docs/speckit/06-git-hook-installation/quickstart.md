# Quickstart Validation: Git Hook Installation

## Prerequisites and Isolation

Gunakan macOS/Linux, Git, dan executable wl terbaru. Bagian akhir mencatat hasil verifikasi implementasi; skenario acceptance berikut juga dapat dipakai untuk validasi tambahan.

```sh
go test -race -coverprofile=coverage.out ./application/... ./domain/...
go tool cover -func=coverage.out
go test -race ./...
go vet ./...
go build -o /private/tmp/wl-hooks-check ./cmd/wlog
```

Semua executable package domain/application harus memiliki unit test dengan 100% statement coverage. Package root yang hanya berisi doc.go tidak memiliki statements. Implementasi kedua layer mengikuti TDD RED → GREEN → REFACTOR; integration tests tidak menggantikan unit coverage gate.

Gunakan repository dan database sementara. Installer tidak membutuhkan config/database, tetapi hook capture akan mengakses penyimpanan fitur 05. Untuk pengujian real capture, gunakan akun OS test dengan home/config terpisah, atau executable fixture dengan dependency injection path DB yang terdokumentasi. Jangan memakai worklog pribadi atau hook repository kerja untuk failure tests.

## 1. New Install and First Commit

Siapkan repository terisolasi dengan config Git lokal:

```sh
HOOK_REPO=$(mktemp -d /private/tmp/wlog-hook-XXXXXX)
git init "$HOOK_REPO"
cd "$HOOK_REPO"
git config user.name "Hook Fixture"
git config user.email "fixture@example.invalid"
/private/tmp/wl-hooks-check install-hooks
```

Pastikan effective core.hooksPath tidak disetel pada environment test; jangan menghapus global config pengguna agar test berjalan. Fixture inspector dapat menyediakan scope config yang terisolasi untuk kasus tersebut.

Expected: instalasi pada unborn repository berhasil, target executable, tanpa original backup dan tanpa config/DB worklog dibuat. Output menyebut root dan hook actual.

Untuk real capture, letakkan executable fixture wl pada PATH Git process, lalu buat commit test:

```sh
printf 'fixture\n' > fixture.txt
git add fixture.txt
git commit -m "OOT-3751 hook capture"
```

Expected: commit berhasil, satu GIT_COMMIT tercatat pada DB test; output wl tidak muncul pada output commit. Verifikasi hash repository+commit sesuai fitur 05. Install tidak menjalankan capture; aktivitas baru muncul setelah commit.

## 2. Alias, Rerun, and Permission Repair

Jalankan alias install-hook dari subdirektori repository yang sama, lalu install-hooks lagi.

Expected: target sama, AlreadyInstalled, bytes wrapper tidak berubah, satu capture invocation saja. Tidak ada backup/blok tambahan.

Hapus owner-execute pada wrapper valid melalui fixture lalu ulangi install. Expected Repaired, perubahan hanya pada mode wrapper; bytes dan original backup tidak berubah. Invalid argumen tambahan menghasilkan error usage sebelum dependency dipanggil.

## 3. Existing Hook Preservation

Gunakan repository test baru untuk setiap kasus. Simpan digest/bytes/mode original sebelum install.

| Original fixture | Expected setelah install dan runtime |
| --- | --- |
| Executable shell menulis sentinel lalu exit 0 | Backup byte/mode-identical, sentinel dibuat, wl dipanggil sekali, status wrapper 0 |
| Executable shell menulis output lalu exit 7 | Output original utuh, wl tetap dicoba, wrapper direct invocation exit 7; commit Git tetap ada |
| Hook dengan set -e dan error | Error original tidak memutus process wrapper; capture tetap dicoba |
| Hook interpreter lain dengan shebang | Tidak ada append shell ke body; original dipanggil sebagai process |
| Nonexecutable regular hook | Backup mode sama, original tidak dijalankan, wrapper capture aktif |
| Original mengubah cwd/env | Capture masih mengikuti cwd/env wrapper, bukan mutation proses original |
| Filename-sensitive hook | Batas argv0/$0 terdokumentasi; gunakan manual integration sesuai contract |

Spy fake wl mencatat invocation, cwd, dan args tanpa membuka config/database. Direct wrapper invocation dapat menguji args/stdin forwarding; Git post-commit normal tidak memiliki positional args. Jangan source original pada test harness.

## 4. Capture Failure Does Not Change Commit

Siapkan fake wl executable pada PATH test yang mencatat invocation, mengeluarkan stdout/stderr, lalu exit non-zero. Ulangi dengan wl tidak ada pada PATH test. Environment fixture tidak mengubah HOME/config pribadi.

Expected: wl output dibuang, original output/status tetap sesuai, commit selesai dan hash baru tetap ada. Tanpa executable original, direct wrapper status 0. Dengan original exit 7, direct wrapper status 7 terlepas dari hasil wl.

Real DB-failure fixture dapat memakai injected capture store yang mengembalikan error. Expected commit tidak dibatalkan, tidak ada activity palsu, dan tidak ada prompt/background task. Retry capture manual untuk hash sama tetap memakai dedup fitur 05.

## 5. Unsupported Locations and File States

- Nonrepository/bare/Git unavailable: fail tanpa mutation.
- core.hooksPath relative/absolute/shared atau /dev/null: unsupported, path/config existing tetap utuh.
- Symlink/nonregular target/backup/lock: fail tanpa mengikuti link atau menimpa destination.
- Backup reserved preexisting: conflict; original dan backup tidak berubah.
- Wrapper marker saja, modified template, unsupported version, missing/modified original backup: conflict, tanpa auto-repair content.
- Valid managed wrapper dengan permission hilang: repair mode saja.

Pastikan pesan conflict menyebut path yang perlu ditinjau, tanpa mencetak isi hook. Installer tidak menjalankan arbitrary original hook sebagai probe.

## 6. Linked Worktrees and Path Handling

Buat commit awal di repository test lalu tambahkan linked worktree. Jalankan installer dari linked worktree; verifikasi target berada pada common default hook directory, bukan directory metadata per-worktree yang dibentuk manual.

Expected output menyebut shared-worktrees scope. Commit dari main dan linked worktree menjalankan wrapper yang sama, tetapi fake wl mencatat cwd worktree masing-masing. Real capture memakai root actual per fitur 05.

Ulangi fixture pada path yang mengandung spasi/quote; quoting wrapper harus menjaga backup lookup dan execution. Tidak ada absolute worktree path yang dibekukan di wrapper.

## 7. Concurrency, Failure, and Recovery Boundary

Gunakan barriers dan filesystem adapter failure injection:

1. Dua installer pada target sama: satu publish; lainnya busy/already installed, tanpa duplicate backup.
2. Target berubah setelah snapshot: conflict, perubahan pihak lain tidak ditimpa bila terdeteksi.
3. Write/sync/mode/backup publish/final rename gagal sebelum commit point: original target tetap byte-identical; cleanup hanya file invocation yang ownership-nya masih cocok.
4. Backup path dibuat pihak lain: no-clobber failure, file pihak lain tetap utuh.
5. Cancel sebelum publish: tidak ada publish sesudah cancellation terdeteksi.
6. Crash fixture setelah backup terbentuk sebelum wrapper publish: original masih post-commit, reserved backup/lock memicu recovery manual pada rerun.
7. Sync/cleanup/output gagal sesudah publish: integration dapat sudah installed; error menyarankan verifikasi rerun, tanpa klaim rollback target.
8. Backup/temp/lock diubah pihak lain sebelum cleanup: jangan hapus; report recovery path.

Lock hanya melindungi cooperating wl installers; tidak menjanjikan atomic compare-and-swap terhadap external editor yang mengabaikan lock. Tests memverifikasi perubahan yang dapat dideteksi, bukan asumsi universal locking.

## 8. Help and Regression Gates

```sh
/private/tmp/wl-hooks-check install-hooks --help
/private/tmp/wl-hooks-check help install-hook
/private/tmp/wl-hooks-check --help
/private/tmp/wl-hooks-check --version
```

Spy factory membuktikan help/version/invalid args tidak mengakses Git/filesystem/config/SQLite. Regression fitur 01–05 tetap lulus, termasuk 100% domain/application coverage. Tidak ada migrasi/schema atau perubahan capture attribution/duplicate behavior.

Saat implementasi selesai, catat hasil actual tests, coverage, platform, Git version, dan fixture constraints pada dokumen ini. Checklist desain tidak menjadi pengganti bukti tersebut.

## Hasil Verifikasi Implementasi — 2026-10-01

Environment: macOS arm64, Go 1.25.11, Git 2.52.0. Platform Linux belum dijalankan pada sesi ini.

| Gate | Hasil |
| --- | --- |
| Domain/application TDD | RED compile failure sebelum kode baru, kemudian GREEN dengan fake inspector/store dan pure domain tests |
| `go test -race -coverprofile=... ./application/... ./domain/...` | Lulus; 100% statement coverage setiap fungsi executable, termasuk fitur 01–05 |
| `go test -race ./...` | Lulus, tanpa race terdeteksi |
| `go vet ./...` | Lulus |
| `go build -o /private/tmp/wl-hooks-check ./cmd/wlog` | Lulus |
| Smoke executable hasil build | Alias, unborn repo, help/invalid args, rerun, permission repair, commit saat wl gagal/hilang, custom hooksPath rejection lulus |

Automated fixtures berada pada `domain/hook/install_test.go`, `application/hook/service_test.go`, `infrastructure/githook/*_test.go`, dan `delivery/cli/hooks_test.go`.

Filesystem/runtime tests memverifikasi backup byte/rwx-identical, executable/nonexecutable original, exit/output original, set -e, args/stdin, isolasi cwd/environment, path spasi/quote, symlink/nonregular rejection, backup no-clobber, busy lock, prepublication failure/cancellation, external target/backup/temp/lock mutation, ownership-aware cleanup, serta error setelah publish dan rerun. Perubahan backup yang berbagi inode dengan copy temporary mempertahankan kedua path untuk recovery.

Real Git fixtures memverifikasi default shared hook dari linked worktree dan commit dari kedua worktree dengan fake wl yang gagal. Smoke executable juga membuktikan commit tetap berhasil ketika wl tidak ada pada PATH. Fake wl merekam invocation tanpa membuka worklog pribadi; Git global/system config diisolasi lewat environment fixture tanpa mengubah HOME atau konfigurasi pengguna. Tidak ada hook yang dipasang pada repository kerja ini.

Regression fitur 05 mencakup reader Git, aturan capture, dan penyimpanan SQLite. Uji hook fitur 06 menggunakan spy wl; skenario OS-level write/sync fault dan process crash pada acceptance guide belum seluruhnya diinjeksi, sehingga tabel tersebut tidak mengklaim semua variasi fault/crash telah dieksekusi.
