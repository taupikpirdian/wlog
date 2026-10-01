# Developer Worklog CLI

`wl` adalah CLI lokal untuk menyiapkan penyimpanan worklog developer. Versi saat ini menyediakan inisialisasi database dan konfigurasi, serta perintah bantuan dan versi.

## Prasyarat

- Go 1.22 atau lebih baru
- macOS atau Linux
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

Jalankan tanpa argumen untuk menyiapkan penyimpanan lokal:

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

Lihat bantuan dan versi kapan saja:

```sh
wl --help
wl help
wl --version
wl version
```

Perintah help dan version tidak membuat atau membuka database.

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

## Status fitur

- Fitur yang tersedia saat ini adalah inisialisasi CLI dan local storage.
- Perintah session, note, Git capture, summary, dan description direncanakan untuk tahap berikutnya dan belum tersedia.

Rincian implementasi fitur awal ada di [docs/speckit/01-initiate-cli-and-local-storage](docs/speckit/01-initiate-cli-and-local-storage/).
