# Quickstart Validation: Work Session Tracking

## Prerequisites

Build implementasi fitur 03 dan gunakan database sementara terisolasi melalui dependency konfigurasi/fixture validasi. Jangan mengubah database worklog pribadi untuk skenario kegagalan.

## Automated verification

```sh
go test -race ./...
go vet ./...
go build -o ./wl ./cmd/wlog
```

Fixture SQLite di `infrastructure/storage/sessions_test.go` memakai database sementara untuk lifecycle, validasi, custom ticket pattern, pergantian tiket sama, rollback insert, konflik snapshot, serta start/stop bersamaan lewat dua koneksi. Test domain memakai clock deterministik untuk durasi lintas tengah malam, kurang dari satu detik, dan jam mundur. Test delivery memeriksa prompt, EOF/noninteraktif, alias, argumen, serta help/version tanpa storage. Test Git memeriksa root repository, direktori di luar Git, dan Git tidak tersedia.

Target performa di bawah 200 ms tetap perlu diukur pada perangkat/database pemakai; test fungsional tidak mengukur target ini.

## Start dan stop

Jalankan `wl s OOT-3751 "Fix tax calculation"`, lanjutkan pekerjaan, lalu `wl x`.

Expected: tiket tersedia, session awal `ACTIVE` dengan akhir/durasi null, lalu `COMPLETED`. `duration_seconds` sama dengan selisih timestamp yang tersimpan dalam detik penuh.

## Alias dan validasi

Ulangi dengan `wl start`/`wl stop`. Coba key tidak valid, title whitespace-only, start dengan jumlah argumen salah, serta stop dengan argumen tambahan.

Expected: alias setara dan input tidak valid tidak menulis data.

## Pergantian session

Mulai session A, lalu start session B. Coba jawaban `n`, jawaban tidak dikenal, dan persetujuan `y`.

Expected: penolakan mempertahankan A; jawaban tidak dikenal meminta ulang; persetujuan menyelesaikan A dan membuat B dengan timestamp batas yang sama.

## Stop tanpa session

Hentikan session, lalu ulangi stop.

Expected: error tanpa mengubah record yang sudah selesai.

## Konkurensi dan snapshot

1. Jalankan dua start bersamaan pada database yang sama: maksimal satu session aktif.
2. Tahan prompt pergantian di proses A; ubah session melalui proses B; jawab prompt A: A mengembalikan konflik tanpa menutup session baru B.
3. Jalankan dua stop bersamaan: hanya satu menyelesaikan session; proses lain memperoleh hasil tidak aktif/konflik.

## Rollback

Simulasikan kegagalan insert session baru setelah konfirmasi pergantian.

Expected: session lama tetap aktif dan tidak ada tiket baru atau perubahan parsial tersisa.

## Waktu dan repository

Dengan clock yang diinjeksi, verifikasi rentang lintas tengah malam, kurang dari satu detik, dan jam mundur. Verifikasi start di repository Git dan di luar repository.

Expected: durasi lintas tanggal benar, rentang di bawah satu detik menghasilkan nol, rentang negatif ditolak, dan repository tidak tersedia tidak menghambat start.
