# Quickstart Validation: Dashboard & Today Timeline

## Status and Prerequisites

Panduan acceptance berikut dapat dipakai untuk validasi lanjutan. Hasil implementasi yang sudah dijalankan tercatat pada bagian akhir. Gunakan Go sesuai go.mod, macOS/Linux, DB sementara, fake clock/location dan writer fixture. Jangan memakai database worklog pribadi.

Unit domain/application wajib mengikuti TDD RED → GREEN → REFACTOR, termasuk perubahan kode existing. RED harus membuktikan behavior yang belum tersedia, kemudian GREEN minimal dan refactor tanpa mengubah kontrak. Pertahankan 100% statement coverage seluruh executable package kedua layer dari gate fitur 06.

```sh
rtk go test -race -coverprofile=/private/tmp/wlog-dashboard-coverage.out ./application/... ./domain/...
rtk proxy go tool cover -func=/private/tmp/wlog-dashboard-coverage.out
rtk go test -race ./...
rtk go vet ./...
rtk proxy go build -o /private/tmp/wl-dashboard-check ./cmd/wlog
```

## Isolation

Storage integration tests membuka DB pada temporary directory dengan infrastructure API existing. Application tests memakai injected clock/location/store; delivery tests memakai fake factory. Untuk executable smoke, gunakan runner test yang mengarahkan loader ke config/DB sementara atau akun OS test terisolasi. Jangan mengasumsikan ada flag `--db`, `--date` atau environment override yang belum tersedia. Git hook tidak dipasang oleh fitur ini.

## 1. Dashboard Happy Path

Fixture Asia/Jakarta, tanggal 02 Oct 2026, now 10:56:

| Session | Start | End | Expected contribution |
| --- | --- | --- | --- |
| OOT-3751, Fix tax calculation | 09:14 | active | 1h 42m |
| OOT-3751, Earlier work | 08:00 | 08:48 | 48m |
| OOT-3747 | 06:00 | 07:15 | 1h 15m |
| OOT-3668 | 07:15 | 08:00 | 45m |

Run dashboard fixture. Expected active Started 09:14/Duration 1h 42m; sorted Today OOT-3668 45m, OOT-3747 1h 15m, OOT-3751 2h 30m, Total 4h 30m. Ulangi tanpa active, seluruhnya kosong, dan activity-only ticket 0m.

## 2. Chronological Timeline

Fixture now 13:30: OOT-3751 session 09:00–11:30, NOTE 09:15, COMMIT 10:12; OOT-3668 active mulai 13:00, NOTE 13:15.

Expected output seperti [CLI contract](contracts/cli.md), enam events, Total tracked 3h. Tambahkan commit bertiket tanpa session dan commit unassigned; tiap commit muncul tepat sekali dengan key/label yang benar, total tetap 3h. Capture time hari ini dengan created_at kemarin harus excluded.

## 3. Calendar Boundaries and Duration

- Session kemarin 23:30–hari ini 00:30: 30m hari ini, STOP saja pada timeline hari ini.
- Active kemarin 23:00, now 01:00: Duration 2h, kontribusi hari ini 1h, Started menyertakan tanggal kemarin; timeline dapat kosong dengan total 1h.
- Event tepat dayStart included; nextDayStart excluded. Activity saat ini tanggal berbeda dari session menggunakan tanggal activity.
- Test zona dengan offset berubah memakai location fixture; jangan menganggap end = start + 24h. Siapkan timezone data fixture bila runtime tidak memiliki zoneinfo.
- Dua session masing-masing 40 detik: masing-masing tampil 0m, total 1m.
- Zero-length completed: START/STOP deterministik, 0m. Active start > now/completed end < start: error, data tidak berubah.
- Future completed events dalam hari tetap terlihat; kontribusi dibatasi now. Hari lain tidak ikut total.

## 4. Stable Attribution and Safe Output

Event timestamp sama harus mengikuti START/NOTE/COMMIT/STOP kemudian source ID. Dua NOTE sama tetap dua baris. Timestamp berbeda tetapi HH:mm sama tetap memakai timestamp penuh.

Gunakan commit tanpa message/hash opsional, note/judul Unicode, multiline, tab, dan ANSI/control fixture. Expected satu baris per event, fallback pesan sesuai contract, tidak ada terminal control aktif, bytes DB tetap sama. Git mismatch commit memakai ticket_id activity sendiri.

## 5. Snapshot and Failure Tests

SQLite fixture dua koneksi/barrier: tahan reader setelah pembacaan session, lakukan stop/capture writer, lalu lanjutkan pembacaan activity. Result harus seluruhnya snapshot sebelum perubahan atau sesudah perubahan, bukan campuran. Sesuaikan koordinasi dengan locking mode DB; writer dapat menunggu reader selesai.

Injection cases: store/query/scan/parse error, bad timestamp di row lama, unknown status, multiple active fixture, cancellation, writer fail, DB close fail. Expected non-zero dan tidak ada empty success; output writer failure boleh menghasilkan partial output. Snapshot gagal tidak merender partial view.

Bandingkan seluruh work table rows sebelum/sesudah sukses/gagal; session tetap aktif dan tidak ada aktivitas baru. Jangan memakai file digest DB sebagai satu-satunya bukti karena SQLite/initialization dapat mengubah metadata/schema.

## 6. CLI and Bootstrap Regression

```sh
/private/tmp/wl-dashboard-check --help
/private/tmp/wl-dashboard-check today --help
/private/tmp/wl-dashboard-check help today
/private/tmp/wl-dashboard-check --version
/private/tmp/wl-dashboard-check version
```

Spy factory memastikan seluruh help/version serta `today unexpected`, unknown root args, dan unknown flags tidak mengakses config/storage. Cold fixture `wl` dan `wl today` membuat config/schema valid lalu empty output; repeated invocation tidak menduplikasi data. Regression start/stop/note/git/install-hooks fitur 01–06 harus tetap lulus.

## 7. Performance Evidence

Ukur executable hasil build pada DB terisolasi yang sudah diinisialisasi, tanpa lock contention atau build time. Fixture baseline: 10.000 session, 50.000 activity, dengan 100 session dan 500 activity hari ini; data lebih lama tetap ikut read baseline. Jalankan 30 kali per command, catat semua durasi, median dan maksimum, perangkat/platform, zona dan fixture size.

Target dashboard <200 ms, today <500 ms pada run biasa fixture ini. Catat cold initialization secara terpisah. Jika gagal, optimasi snapshot query dengan regression timestamp/calendar tests sebelum melaporkan gate lulus. Isi bukti actual TDD, coverage, race/vet/build, smoke dan benchmark setelah implementasi selesai.

## Hasil Verifikasi Implementasi — 2026-10-02

Environment: macOS arm64, Apple M1 Pro, Go 1.25.11. Linux belum dijalankan pada sesi ini.

| Gate | Hasil |
| --- | --- |
| TDD domain/application | RED compile failure pada tests baru sebelum kode production, lalu GREEN dan refactor |
| Coverage domain/application | 100% statement coverage seluruh fungsi executable termasuk fitur 01–06; 90 unit test/subtest lulus dengan race |
| Regression seluruh project | `rtk go test -race ./...` lulus, 221 test/subtest pada 22 package; tanpa race terdeteksi |
| Static checks/build | `rtk go vet ./...` dan build `/private/tmp/wl-dashboard-check` lulus |
| Production wiring pada fixture DB | Cold database, empty views, active session, note timeline, repeated reads dan config/database errors lulus |
| CLI contract | Dashboard/today, empty/overnight output, categories, attribution, minute formatting, lazy help/version/invalid args, read/write/close errors lulus |
| SQLite integration | Nullable/legacy timestamp, invalid timestamp/query/cancel errors dan consistent read transaction selama writer stop+note lulus |
| Domain rules | Midnight boundaries, 23/25-hour local day, clipping, future event, zero duration, duplicate/tied events, 40s+40s total dan invalid data lulus |
| Executable smoke | `today --help` dan `--version` hasil build lulus; command data memakai fixture integration agar worklog pribadi tidak dibuka |

Fixture tests tersedia pada `domain/dashboard/view_test.go`, `application/dashboard/service_test.go`, `infrastructure/storage/dashboard_test.go`, `delivery/cli/dashboard_test.go`, dan `cmd/wlog/dashboard_test.go`.

Benchmark reproducible:

```sh
rtk go test ./cmd/wlog -run '^$' -bench BenchmarkDailyCommands -benchtime=30x -count=1
```

| Command | Samples | Median | Maximum | Target |
| --- | --- | --- | --- | --- |
| Dashboard | 30 | 136.0 ms | 139.8 ms | <200 ms |
| Today | 30 | 136.2 ms | 147.7 ms | <500 ms |

Benchmark menggunakan binary test hasil build Go, production dashboardFactory dan root command, SQLite sementara berisi 10.000 completed session dan 50.000 NOTE; 100 session dan 500 note bertanggal hari ini. Termasuk open/schema check, snapshot full scan, domain projection, rendering dan close; output menuju io.Discard. Loader fixture menyediakan path sementara dan clock UTC tetap. Pengukuran tidak mencakup load file config pengguna, cold config initialization, startup proses baru, atau terminal lambat. Cold database initialization sudah diuji fungsional tanpa klaim timing. Karena full scan membaca history, ukuran database lebih besar dapat meningkatkan latency; target ini hanya terbukti pada fixture/platform yang dicatat.

Snapshot concurrency memakai dua koneksi SQLite dalam WAL fixture dan barrier pada query kedua: writer commit setelah reader membaca session, reader tetap mendapat activity set lama, lalu pembacaan berikutnya mendapat state baru. Tidak ada test hook pada kode production. Production tetap memakai journal mode existing; fixture menguji konsistensi snapshot saat writer dapat commit bersamaan.
