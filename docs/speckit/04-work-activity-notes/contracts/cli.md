# CLI Contract: Work Activity Notes

## Command

```sh
wl note "Check Splunk logs"
wl n "Found response mismatch"
wl n "Support QA retest"
```

Nama command `note`, alias `n`. Tepat satu argumen description diperlukan; quote teks yang memiliki spasi. Tidak ada ticket/session argument, stdin-content mode, prompt, editor, atau flag backdate pada fitur ini. Flag bantuan Cobra tetap tersedia.

Whitespace awal/akhir dipangkas, hasil kosong ditolak. Isi internal, Unicode, tanda kutip, dan newline di dalam satu argumen dipertahankan. Key tiket dalam description tidak dipakai untuk memilih tiket.

## Success

Given session `OOT-3668` aktif:

```text
✓ Note added to OOT-3668
```

Exit 0 setelah penyimpanan committed. Setiap invocation sukses menyimpan satu aktivitas NOTE. Deskripsi tidak perlu dicetak kembali pada output sukses.

## No Active Session

```text
✗ No active session.

Start one using:

wl s <ticket> "<title>"
```

Exit non-zero. Tidak membuat tiket, session, atau activity; session terakhir yang sudah selesai bukan tujuan fallback.

## Errors and Exit Status

| Kondisi | Exit | Perilaku |
| --- | --- | --- |
| Note committed | 0 | Konfirmasi key tiket tujuan |
| Tidak tepat satu argumen | Non-zero | Error argumen dan usage note, tanpa membuka storage |
| Description kosong/whitespace | Non-zero | Error description dan usage note, tanpa INSERT activity |
| Tidak ada session aktif | Non-zero | Pesan no-active dan saran start |
| Snapshot dihentikan/diganti sebelum write | Non-zero | `Active session changed; retry the command.`; tanpa note atau atribusi ulang |
| Waktu note sebelum start | Non-zero | Petunjuk koreksi jam; tanpa perubahan data |
| Read/lock/INSERT/COMMIT gagal | Non-zero | Pesan operasi gagal; jangan cetak sukses; rollback perubahan yang belum committed |
| Output gagal setelah COMMIT | Non-zero | Laporkan error output; note yang sudah committed tetap ada |
| `wl note --help`, `wl help note`, help/version umum | 0 | Bantuan/versi tanpa akses storage |

Tidak ada cancellation prompt pada note. Noninteraktif dapat digunakan jika description diberikan sebagai argumen dan session aktif.

## Attribution and Concurrency

- Tujuan berasal dari satu snapshot session aktif global per database, bukan repository command atau teks note.
- Snapshot diverifikasi di dalam transaksi sebelum insert. Perubahan sebelum pembacaan awal note boleh menentukan tujuan baru; perubahan setelah snapshot menyebabkan konflik.
- Note tersimpan lebih dahulu: stop/switch berikutnya boleh menyelesaikan session dan note tetap terkait dengannya.
- Stop/switch tersimpan lebih dahulu setelah snapshot: note gagal tanpa write.
- Dua note dengan session yang tetap aktif dapat sama-sama berhasil. Kesamaan teks atau timestamp tidak dideduplikasi.

## Compatibility

Command start/stop dan alias fitur 03 tetap tersedia. Tidak ada perubahan format output atau cara konfirmasi command tersebut. Note tidak memulai session, menghentikannya, menghitung durasi, menjalankan Git, atau memanggil AI.
