# Quickstart Validation: Manual Session Management

## Status and Prerequisites

Panduan acceptance berikut dapat dipakai untuk validasi lanjutan. Hasil implementasi yang sudah dijalankan tercatat pada bagian akhir. Gunakan Go sesuai go.mod, DB sementara, clock/location/repository fixture. Jangan memakai worklog pribadi untuk failure/concurrency tests.

TDD RED → GREEN → REFACTOR wajib untuk penambahan/perubahan kode domain/application. Tulis behavior tests terlebih dahulu, simpan bukti RED, implement minimal GREEN lalu refactor. Pertahankan 100% statement coverage seluruh executable package kedua layer dari gate fitur sebelumnya.

```sh
rtk go test -race -coverprofile=/private/tmp/wlog-manual-coverage.out ./application/... ./domain/...
rtk proxy go tool cover -func=/private/tmp/wlog-manual-coverage.out
rtk go test -race ./...
rtk go vet ./...
rtk proxy go build -o /private/tmp/wl-manual-check ./cmd/wlog
```

## Isolation

Storage tests memakai temporary directory/SQLite API existing. Usecase tests memakai fake store/clock/location; delivery tests fake factory. Composition fixture memakai loader yang menyediakan DB path sementara, seperti fitur 07. Jangan mengasumsikan ada --db/--date atau environment override produk. Executable smoke data memakai runner terisolasi, tanpa memodifikasi HOME atau config pribadi.

## 1. Manual Completed Happy Path

Fixture Asia/Jakarta, now 02 Oct 2026 12:00. Jalankan command goal dari [CLI contract](contracts/cli.md).

Expected: satu tiket OOT-3751, satu COMPLETED session title Fix tax calculation, start 09:00/end 11:00 lokal, duration_seconds 7200. created_at/updated_at actual 12:00, bukan 09:00. Dashboard total tiket 2h; today START 09:00/STOP 11:00. Existing ticket master title tetap sama pada pengulangan scenario dengan tiket existing dan rentang lain yang disjoint.

Tambahkan active session sejak 11:00 lalu manual 09:00–11:00 pada fixture baru. Expected manual berhasil, active ID/title/start/status/end fields tidak berubah. Di luar Git/repository reader kosong, repository null dan command tetap berhasil.

## 2. Backdated Start and Existing Commands

Fixture now 10:30, tidak ada active/range existing:

```sh
wl s OOT-3751 "Fix tax" --since 09:00
wl
wl today
```

Expected ACTIVE start 09:00, ended_at/duration_seconds null, Duration aktif 1h 30m, timeline START 09:00 tanpa STOP. Gunakan fresh fixture untuk alias start, s, --since dan -s; jangan mengulang semuanya pada DB yang sama lalu mengharapkan empat active session.

Pada clock 10:45, tambahkan note; lalu fixture commit sesuai fitur 05, dan stop pada 11:00. Expected attribution existing, completed duration 7200, note/commit tidak menambah waktu. --since tepat now diperbolehkan, initial elapsed 0m.

Dengan existing active session, --since gagal tanpa confirmation/input read; seluruh old row tidak berubah. Ordinary start tanpa --since tetap memakai interactive confirmation existing.

## 3. Validation Matrix

| Input/case | Expected |
| --- | --- |
| Missing ticket/from/to/title atau extra arg | Error sebelum factory |
| --since tanpa nilai, explicit kosong | Error, tidak fallback start-now |
| 9:00 / 24:00 / 09:60 / 09:00:30 / tanggal penuh / whitespace | Strict time error sebelum factory |
| Empty/whitespace title | Validation error, no writes |
| Key tidak sesuai configured pattern | Validation error, no writes |
| From == To / From > To | Error, no zero completed/rollover |
| To atau Since setelah now | Error, no inference kemarin |
| --from 23:00 --to 01:00 | Error, tidak cross-day |
| Zona lokal berbeda UTC | Stored UTC instant tetap setara waktu lokal input |
| Gap/fold wall time | Error tanpa normalized/selected offset |
| Unambiguous range melintasi offset change | Durasi elapsed aktual, bukan selisih angka jam |

Use location fixture America/New_York untuk gap 08 Mar 2026 02:30 dan fold 01 Nov 2026 01:30; set now setelah waktu tersebut agar future rejection tidak menyembunyikan test resolver. Sediakan zoneinfo fixture/tzdata bila runtime tidak memilikinya. Test resolver mendeteksi zero/multiple matching instants, bukan sekadar hasil default time.Date.

## 4. Overlap and Adjacency

Existing completed 09:00–10:00:

- Manual 09:30–11:00, 08:00–09:30, 08:00–11:00, serta exact duplicate 09:00–10:00: overlap rejection.
- Manual 08:00–09:00 dan 10:00–11:00: success pada fixture terpisah.
- --since 09:30: conflict; --since 10:00 dengan now 11:00/no active: success.
- Ulangi menggunakan tiket/repository berbeda: aturan global sama.
- Existing active mulai 10:00: manual yang end <=10:00 sah, manual dengan end >10:00 konflik; --since selalu active conflict.
- Existing completed nol durasi tidak menghalangi candidate positive; unknown status/end sebelum start/bad timestamp menghasilkan data error.

Snapshot seluruh tabel sebelum/sesudah failure; pastikan tidak ada tiket baru, session berubah, atau activity relink. Data activity fixture dengan session_id null dan timestamp berada dalam range baru harus tetap null.

## 5. Atomicity and Concurrency

Gunakan dua koneksi SQLite/clock deterministik serta barriers:

1. Dua manual completed overlap: tepat satu sukses, satu conflict; maksimal satu session/tiket baru.
2. Dua --since saat awal tanpa active: tepat satu active; lainnya conflict, tanpa duplicate tiket.
3. Manual completed versus --since dengan candidate overlap: hanya satu operasi berhasil, tidak ada double tracked time.
4. Ordinary start lebih dahulu commit: --since melihat active conflict; manual rentang sebelum active start dapat sukses.
5. Note/capture concurrent: session/attribution evidence tetap aturan existing; manual tidak melakukan activity updates.
6. Inject INSERT session failure sesudah ticket insert: rollback tiket/session, old rows tetap utuh.
7. Lock timeout/read/parse/commit/cancel errors: non-zero dan no partial work data. Cleanup rollback tetap berjalan saat context cancellation.
8. Writer/output/close failure sesudah commit: session tetap tersimpan; retry ditolak sebagai overlap/active, tidak menambah row kedua.

Legacy CURRENT_TIMESTAMP dan RFC3339Nano dicampur pada existing rows untuk membuktikan overlap memakai parsed instants, bukan lexical compare SQL.

## 6. Help, Regression and Evidence

```sh
/private/tmp/wl-manual-check session --help
/private/tmp/wl-manual-check start --help
/private/tmp/wl-manual-check help session
/private/tmp/wl-manual-check --version
```

Spy factory membuktikan help/version/invalid args/invalid time syntax tidak mengakses config/database/repository. Test --since Changed dan shorthand -s agar ordinary start tidak berubah. Output title Unicode/multiline/control aman, bytes title DB tidak berubah.

Regression fitur 01–07 termasuk bootstrap, ordinary confirmation, stop, note, commit capture/dedup, install-hooks, dashboard/today dan 100% coverage gate harus lulus. Catat hasil actual RED/GREEN, coverage, race/vet/build, platform, fixture constraints dan smoke pada bagian verifikasi baru setelah implementasi selesai; checklist desain bukan bukti test.

## Hasil Verifikasi Implementasi — 2026-10-02

Environment: macOS arm64, Go 1.25.11. Platform Linux belum dijalankan pada sesi ini.

| Gate | Hasil |
| --- | --- |
| TDD domain/application | RED compile failure dari tests baru sebelum production code, kemudian GREEN; tests melalui public interfaces |
| Coverage domain/application | 100% statement coverage seluruh fungsi executable fitur 01–08; 117 test/subtest lulus dengan race |
| Regression seluruh project | `rtk go test -race ./...` lulus; 271 test/subtest pada 22 package; tanpa race terdeteksi |
| Static checks/build | `rtk go vet ./...` dan build `/private/tmp/wl-manual-check` lulus |
| Domain time/range | Strict HH:mm, local date/UTC mapping, future/positive range, gap/fold, actual offset-change elapsed, overlap/adjacency/active dan malformed ranges lulus |
| Application | One clock, key/title errors, repository optional, cancellation sebelum/sesudah repository lookup, storage error dan completed/active mapping lulus |
| SQLite integration | Ticket title preservation, audit time, legacy timestamps, overlap/active conflict, trigger rollback, corrupt timestamp/cancellation, evidence attribution preservation lulus |
| Concurrency | Completed/completed, active/active, completed/active dan backdated/ordinary-start races hanya menghasilkan satu winner untuk candidate yang konflik |
| Composition lifecycle | Production factories pada DB sementara: completed manual → backdated start → note → dashboard/today → stop; total/session linkage sesuai dan data utuh |
| CLI contract | session/from/to/title, --since/-s/alias s, lazy help/invalid input, active/overlap messages, safe text, open/service/output/close errors lulus |
| Ordinary start compatibility | Tanpa --since tetap memakai factory start existing; noninteractive switch tetap ditolak tanpa write |
| Executable smoke | session/start --help menampilkan flags baru; invalid --since 24:00 exit non-zero sebelum config/storage |

Tests tersedia pada `domain/session/manual_test.go`, `application/session/manual_test.go`, `infrastructure/storage/manual_sessions_test.go`, `delivery/cli/manual_sessions_test.go`, dan `cmd/wlog/manual_sessions_test.go`.

Time resolver mengevaluasi offset pada zone intervals di sekitar tanggal input, lalu mencocokkan kembali wall time; gap memiliki nol matching instant dan fold lebih dari satu. Fixture America/New_York membuktikan kedua rejection serta elapsed 2h/4h untuk rentang unambiguous yang melintasi offset change.

Semua data smoke/lifecycle memakai loader/factory fixture dan SQLite temporary directory tanpa membuka worklog pribadi. Executable production dijalankan untuk help/invalid syntax saja. Tidak ada migration baru, reattachment activity, atau AI call. Bukti ini tidak mengklaim seluruh variasi OS-level disk/commit fault, lock timeout, concurrent Git capture, atau terminal interaktif real telah dieksekusi; skenario acceptance tersebut tetap tersedia untuk validasi tambahan.
