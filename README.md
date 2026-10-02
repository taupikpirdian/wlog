# Developer Worklog CLI

`wl` adalah CLI lokal untuk mencatat sesi kerja developer berdasarkan tiket, dengan database SQLite dan konfigurasi lokal.

## Prasyarat

- Go 1.22 atau lebih baru
- macOS atau Linux
- Git dengan dukungan offline capture (`--no-lazy-fetch`) untuk `wl git`; implementasi diverifikasi pada Git 2.52.0
- Koneksi internet saat pertama kali build untuk mengunduh Go modules

## Instalasi

Clone repository lalu build executable bernama `wl`:

```sh
git clone https://github.com/taupikpirdian/wlog.git
cd wlog
mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/wl" ./cmd/wlog
```

Pastikan `~/.local/bin` ada di `PATH`. Untuk sesi terminal saat ini:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Untuk memasangnya permanen, tambahkan baris tersebut ke file konfigurasi shell, misalnya `~/.zshrc` atau `~/.bashrc`, lalu buka terminal baru.

Periksa instalasi:

```sh
wl --version
wl --help
```

Versi hasil build lokal menampilkan `dev`. Build rilis dapat menyisipkan nomor versi melalui linker flag Go.

## Menjalankan

Jalankan tanpa argumen untuk melihat dashboard session aktif dan total waktu hari ini:

```sh
wl
```

Pada pemakaian pertama, `wl` membuat:

```text
~/.worklog/
├── config.yaml
└── worklog.db
```

Startup berikutnya menggunakan konfigurasi dan database yang sama serta menerapkan migrasi yang belum tersedia. Direktori dan file baru dibuat dengan akses terbatas pada user saat ini.

Dashboard menampilkan tiket, judul, waktu mulai, dan durasi session aktif, lalu ringkasan waktu hari ini per tiket. Tanpa pekerjaan, tampilkan `No active session` dan total `0m`. Session lintas tengah malam hanya menyumbang bagian waktunya yang masuk hari ini; durasi session aktif tetap seluruh waktu sejak mulai.

Tinjau kejadian hari ini:

```sh
wl today
```

Timeline menampilkan `START`, `NOTE`, `COMMIT`, dan `STOP` dalam urutan waktu, dengan tiket pada setiap baris. Tanggal dan waktu mengikuti zona lokal perangkat, dan semua repository dalam database pengguna ikut ditampilkan. Session aktif tidak memiliki `STOP` buatan.

Note dan commit adalah evidence dan tidak menambah durasi tracked. Commit bertiket tanpa session muncul pada kategori `Unsessioned` di dashboard; commit tanpa tiket muncul pada `Unassigned`. Timeline tetap menyertakan keduanya. Durasi harian dijumlahkan dalam detik sebelum ditampilkan dalam menit, sehingga total bisa berbeda dari penjumlahan menit yang terlihat pada baris tiket.

Kedua command membaca satu snapshot yang konsisten tanpa mengubah data pekerjaan, memanggil Git, atau mengakses jaringan. Teks note/pesan commit multiline ditampilkan sebagai satu baris aman; isi tersimpan tetap utuh. Error pembacaan menghasilkan exit non-zero, bukan tampilan data kosong.

Lihat bantuan dan versi kapan saja:

```sh
wl --help
wl help
wl --version
wl version
```

Perintah help dan version tidak membuat atau membuka database.

## Mencatat sesi kerja

Mulai sesi dengan key tiket dan judul aktivitas:

```sh
wl start OOT-3751 "Fix tax calculation"
# Alias
wl s OOT-3751 "Fix tax calculation"
```

Tiket dibuat otomatis bila belum ada. Judul sesi tidak mengubah judul master tiket. Key tiket harus sesuai `ticket.pattern`; judul sesi wajib diisi. Konfigurasi dan database disiapkan otomatis bila command ini merupakan pemakaian pertama.

Hanya satu sesi dapat aktif untuk seluruh database, termasuk saat berpindah repository. Jika ada sesi aktif, `start` menampilkan detailnya dan meminta konfirmasi `[Y/n]` di terminal interaktif. Enter, `y`, atau `yes` menyelesaikan sesi lama dan memulai sesi baru secara atomik; `n` atau `no` membatalkan. Input dari pipe dan EOF tidak memberikan persetujuan.

Selesaikan sesi aktif:

```sh
wl stop
# Alias
wl x
```

Durasi dihitung dari waktu mulai hingga selesai dan disimpan dalam detik. Waktu ditampilkan sesuai timezone perangkat. Jam perangkat yang lebih awal dari waktu mulai menghasilkan error dan sesi tetap aktif. Menjalankan `stop` tanpa sesi aktif juga menghasilkan error.

Jika dijalankan dalam repository Git, sesi menyimpan path root repository. Command tetap bekerja di luar Git atau ketika Git tidak tersedia.

## Menambahkan catatan aktivitas

Saat sesi aktif, simpan konteks investigasi atau pekerjaan non-coding:

```sh
wl n "Check Splunk logs"
wl note "Found response mismatch"
wl n "Support QA retest"
```

Setiap command menyimpan satu aktivitas `NOTE` pada tiket dan sesi aktif, lalu menampilkan `✓ Note added to <ticket>`. Teks wajib diberikan sebagai satu argumen; whitespace awal/akhir dipangkas dan deskripsi kosong ditolak. Catatan dengan teks sama boleh ditambahkan beberapa kali.

Note tetap bekerja di luar Git dan mengikuti konteks repository sesi aktif. Menambahkan note tidak mengubah durasi sesi. Tanpa sesi aktif, command gagal dan menyarankan `wl s <ticket> "<title>"`. Jika sesi berubah saat penyimpanan, command gagal; ulangi untuk memakai sesi terbaru.

## Mencatat Git commit

Di dalam repository yang sudah memiliki commit, jalankan:

```sh
wl git
```

Command mencatat HEAD terbaru beserta message, branch bila tersedia, file yang berubah, statistik baris teks, dan waktu commit. Tiket dipilih berdasarkan urutan: commit message, sesi aktif, branch, lalu `UNASSIGNED`. Jika tiket sama dengan sesi aktif, commit dikaitkan ke sesi tersebut. Jika berbeda, commit disimpan pada tiketnya tanpa sesi dan menampilkan warning.

Commit tetap tercatat tanpa sesi aktif dan tidak menambah durasi pekerjaan. Capture ulang repository+hash yang sama menampilkan `✓ Commit already captured.` tanpa menggandakan atau memindahkan catatan lama.

`capture_changed_files` dan `capture_diff_stat` dapat dinonaktifkan secara terpisah. Statistik mengecualikan isi `.env*`; binary tidak menambah jumlah baris teks. Full diff belum dikumpulkan, termasuk bila `capture_full_diff: true` diatur—command memberi warning dan tetap menyimpan metadata. Metadata tambahan yang tidak tersedia menghasilkan warning; kegagalan identitas Git atau penyimpanan menghasilkan exit non-zero.

## Memasang Git hook otomatis

Jalankan dari root atau subdirektori repository Git:

```sh
wl install-hooks
# Alias
wl install-hook
```

Installer memasang `post-commit` pada directory hook default Git, termasuk repository yang belum memiliki commit. Output menampilkan repository dan path hook. Install tidak membuat konfigurasi/database atau menjalankan capture. Pada commit berikutnya, wrapper menjalankan:

```sh
wl git >/dev/null 2>&1 || true
```

Pastikan `wl` ada pada `PATH` proses Git, termasuk bila commit dilakukan dari editor/GUI. Output capture dibuang; kegagalan capture atau executable yang tidak tersedia tidak membatalkan commit. Capture manual dengan `wl git` tetap tersedia.

Hook existing berupa regular file disalin utuh ke sibling `post-commit.wlog-original`, dengan permission rwx yang sama. Hook original yang executable dijalankan sebelum capture; output dan exit statusnya dipertahankan. Original nonexecutable disimpan tanpa dijalankan. Karena original dijalankan dari nama backup, hook yang bergantung pada `$0`/basename perlu integrasi manual.

Install ulang wrapper valid menampilkan `already installed` tanpa menggandakan capture. Permission executable yang hilang diperbaiki tanpa mengubah isi wrapper. Default hook linked worktree digunakan bersama seluruh worktree repository; scope ini ditampilkan pada output.

`core.hooksPath` yang disetel, symlink/nonregular hook, backup reserved yang berbenturan dengan instalasi baru, dan wrapper/backup managed yang dimodifikasi ditolak dengan error. Untuk custom hook manager/path, tambahkan pemanggilan capture di atas secara manual pada hook yang dikelola. Installer tidak mengubah konfigurasi Git.

Instalasi memakai lock dan publish atomik. Lock stale atau file recovery yang tersisa setelah crash perlu diperiksa manual melalui path pada error. Lock mengoordinasikan installer `wl`; perubahan oleh editor lain yang terdeteksi membatalkan instalasi, tetapi editor tersebut tetap dapat berlomba setelah pemeriksaan terakhir.

## Menjalankan dari source

Untuk menjalankan langsung tanpa memasang executable:

```sh
go run ./cmd/wlog
```

Atau build di direktori repository:

```sh
go build -o ./wl ./cmd/wlog
./wl
```

## Konfigurasi

File konfigurasi berada di `~/.worklog/config.yaml` dan dibuat otomatis:

```yaml
database:
  path: ~/.worklog/worklog.db

ticket:
  pattern: "[A-Z][A-Z0-9]+-[0-9]+"

git:
  capture_changed_files: true
  capture_diff_stat: true
  capture_full_diff: false

ai:
  enabled: true
  include_diff: false
```

Nilai yang sudah ada dipertahankan saat `wl` dijalankan kembali. `capture_full_diff` dan `include_diff` nonaktif secara default.

## Unit test dan coverage

Seluruh package dengan kode executable di `application` dan `domain` memiliki unit test dengan **100% statement coverage**. Package root yang hanya berisi `doc.go` tidak memiliki statement yang dapat diukur.

Jalankan unit test kedua layer dan buat laporan coverage:

```sh
go test -race -coverprofile=coverage.out ./application/... ./domain/...
go tool cover -func=coverage.out
go tool cover -html=coverage.out
```

Untuk regression test seluruh project:

```sh
go test -race ./...
go vet ./...
```

## Status fitur

- Tersedia: inisialisasi CLI/local storage, pengelolaan tiket di layer application/storage, start/stop session, note aktivitas, Git commit capture, pemasangan Git hook, dashboard, dan timeline hari ini.
- Summary, description, dan koreksi waktu sesi belum tersedia.
