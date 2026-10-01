# CLI Contract: Work Session Tracking

## Start

```sh
wl start OOT-3751 "Fix tax calculation"
wl s OOT-3751 "Fix tax calculation"
```

Tepat dua argumen diperlukan. Alias `s` mempunyai perilaku yang sama dengan `start`.

```text
✓ Session started

OOT-3751
Fix tax calculation

Started: 09:00
```

## Konfirmasi pergantian

```text
Active session detected

OOT-3668
Support QA issue
Started: 09:00
Elapsed: 45m

Stop this session and start OOT-3751? [Y/n]
```

`y`, `yes`, atau Enter pada terminal interaktif menyetujui; `n`/`no` membatalkan. Jawaban tidak dikenal diminta ulang. EOF atau input noninteraktif tidak memberikan persetujuan otomatis. Jika session berubah saat prompt, command gagal dan meminta pengguna mengulang.

## Stop

```sh
wl stop
wl x
```

Tidak menerima argumen.

```text
✓ Session completed

OOT-3751
Fix tax calculation

09:00 → 10:15
Duration: 1h 15m
```

Rentang yang melewati tanggal menampilkan tanggal juga. Waktu ditampilkan dalam timezone lokal. Durasi tampilan tidak mengubah detik yang disimpan.

## Errors and Exit Status

| Kondisi | Exit | Perilaku |
|---|---:|---|
| Start/stop berhasil | 0 | Tampilkan session dan waktunya |
| Pengguna membatalkan | 0 | Tampilkan pembatalan, tanpa write |
| Argumen/key/judul salah | Non-zero | Pesan validasi dan usage command |
| Stop tanpa session aktif | Non-zero | Jelaskan bahwa session belum aktif; sarankan `wl s <ticket> "<title>"` |
| Konflik session saat write | Non-zero | Minta pengguna mengulang untuk melihat keadaan terbaru |
| Jam perangkat lebih awal dari start | Non-zero | Jelaskan koreksi jam; session tetap aktif |
| Storage gagal | Non-zero | Pesan operasi gagal; transaksi tidak diterapkan sebagian |

Help/version tetap tersedia tanpa membuka storage. `--since` dan `-s` tidak termasuk command contract fitur ini.
