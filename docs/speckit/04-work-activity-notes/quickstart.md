# Quickstart Validation: Work Activity Notes

## Prerequisites and Isolation

Panduan ini dijalankan setelah fitur 04 diimplementasikan; saat ini merupakan acceptance guide, bukan hasil test. Build executable terbaru, bukan binary fitur 03.

```sh
go test -race ./...
go vet ./...
go build -o /private/tmp/wl-notes-check ./cmd/wlog
```

Gunakan database sementara untuk test application/storage melalui `OpenDatabase` dan factory command yang diinjeksi; fixture ini tidak membuka config/worklog pribadi. Untuk pengujian executable di bawah, gunakan akun OS pengujian dengan home terpisah, konfirmasi `database.path` menunjuk database test, dan pastikan tidak ada session aktif dari skenario sebelumnya. Jangan menjalankan skenario kegagalan/concurrency pada worklog pribadi.

Test implementasi harus menambahkan kasus note; kelulusan test fitur 01–03 saja belum membuktikan fitur ini. Lihat [CLI contract](contracts/cli.md) dan [data model](data-model.md) untuk hasil yang harus diperiksa.

## 1. Goal Flow and Alias

```sh
/private/tmp/wl-notes-check s OOT-3668 "Support QA issue"
/private/tmp/wl-notes-check n "Check Splunk logs"
/private/tmp/wl-notes-check n "Found response mismatch"
/private/tmp/wl-notes-check note "Support QA retest"
```

Expected: tiga konfirmasi `✓ Note added to OOT-3668`. Periksa tiga row NOTE di database test melalui fixture SQL/read helper, semuanya memiliki ticket_id/session_id yang sama dengan session aktif. Description cocok dengan input; created_at eksplisit UTC; completion fields session tetap NULL dan status ACTIVE.

## 2. Invalid Input and Content Preservation

Pada session aktif, jalankan:

```sh
/private/tmp/wl-notes-check n
/private/tmp/wl-notes-check n ""
/private/tmp/wl-notes-check n "   "
/private/tmp/wl-notes-check n "one" "two"
/private/tmp/wl-notes-check n "  Investigasi respons QA — OOT-9999  "
```

Expected: empat command pertama gagal non-zero tanpa row baru; command terakhir menyimpan `Investigasi respons QA — OOT-9999` pada session OOT-3668. Key di dalam teks tidak mengganti tiket tujuan.

Tambahkan fixture satu argumen dengan newline dan tanda kutip/teks SQL-like. Expected: isi internal utuh, diperlakukan sebagai data. Jalankan description yang sama dua kali; expected dua row berbeda.

## 3. No Active Session

```sh
/private/tmp/wl-notes-check x
/private/tmp/wl-notes-check n "Support QA retest"
```

Expected: stop berhasil; note gagal dengan informasi no-active dan saran start. Tidak membuat activity/session/tiket baru atau memodifikasi session selesai. Ulangi pada DB test baru tanpa history untuk hasil sama.

## 4. Repository Independence

Mulai session dalam repository test, lalu jalankan note dari direktori di luar Git. Ulangi dengan session tanpa repository dan fixture Git executable tidak tersedia.

Expected: note mengikuti session aktif global, repository sama dengan session atau NULL; tidak ada lookup Git atau kegagalan karena cwd. Tidak ada branch/commit/diff/metadata tambahan.

## 5. Concurrent Note versus Stop/Switch

Gunakan dua koneksi OpenDatabase ke file fixture yang sama. Gunakan barrier di test untuk menahan note setelah snapshot session A dibaca, tanpa transaksi terbuka.

1. Proses B stop A, lalu lanjutkan note A → konflik, tidak ada row note baru.
2. Proses B switch A ke B, lalu lanjutkan note A → konflik, tidak ada note di A maupun B.
3. Note A commit dahulu, lalu stop/switch → note tetap terkait A; session dapat selesai.
4. Jalankan dua note bersamaan ketika session tetap aktif → dua row NOTE dengan relasi benar.
5. Switch selesai sebelum pembacaan awal note → note memilih session baru secara normal.

Expected: tidak ada note yang dipindahkan otomatis karena snapshot stale, tidak ada perubahan durasi akibat note, dan tidak ada activity parsial. Jalankan test dengan race detector.

## 6. Failure and Clock Fixtures

- Gunakan trigger fixture yang menolak INSERT NOTE: command gagal tanpa konfirmasi sukses; row count, tiket, dan session tidak berubah.
- Batalkan context sebelum write/commit, kemudian jalankan operasi lain: tidak ada row parsial dan database tetap dapat digunakan.
- Injeksi clock sebelum start session: error koreksi jam tanpa write.
- Injeksi clock sama dengan start: note valid.
- Injeksi waktu sesudah tengah malam pada session hari sebelumnya: note valid dengan tanggal aktual, tanpa session baru.
- Injeksi timestamp sama untuk dua note: dua row berbeda; tanpa deduplikasi.

## 7. Help and Regression

```sh
/private/tmp/wl-notes-check note --help
/private/tmp/wl-notes-check help note
/private/tmp/wl-notes-check --help
/private/tmp/wl-notes-check --version
```

Expected: bantuan mencantumkan alias dan satu argumen. Spy factory command membuktikan tidak ada akses config/storage. Regression start/stop, konfirmasi switch, ticket title preservation, serta migration initialization fitur 01–03 tetap lulus.

## 8. Performance Measurement

Pada database test yang sudah diinisialisasi dengan satu session aktif, ukur executable hasil build untuk penambahan note. Jangan memakai `go run` sebagai pengukuran karena mencakup proses build. Ulangi minimal 20 kali; catat perangkat, jumlah row sebelum test, waktu tiap operasi, dan median/p95.

Expected: command note pada penggunaan lokal biasa selesai <200 ms sesuai PRD. Pisahkan hasil cold initialization dan lock wait; jangan menyatakan target telah terpenuhi sebelum pengukuran dilakukan.
