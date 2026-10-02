# CLI Contract: Manual Session Management

## Completed Session

```sh
wl session OOT-3751 \
  --from 09:00 \
  --to 11:00 \
  --title "Fix tax calculation"
```

Tepat satu argumen tiket; --from, --to, --title wajib. Tidak ada alias session atau attach/force/date flags pada fitur ini. Waktu HH:mm pada hari lokal saat eksekusi; contoh memerlukan now >= 11:00 dan rentang tidak overlap.

```text
✓ Manual session added

OOT-3751
Fix tax calculation

09:00 → 11:00
Duration: 2h
```

Exit 0 sesudah commit dan output berhasil. Tidak ada prompt. Dashboard/today menggunakan session tersebut pada pembacaan berikutnya.

## Backdated Start

```sh
wl start OOT-3751 "Fix tax" --since 09:00
wl s OOT-3751 "Fix tax" --since 09:00
wl s OOT-3751 "Fix tax" -s 09:00
```

Ketiganya setara; `s` pertama alias command dan `-s` shorthand flag. now harus >= 09:00; tidak ada active session atau overlap. Flag --since tanpa nilai/bernilai kosong explicit tidak menjadi start-now.

```text
✓ Session started

OOT-3751
Fix tax

Started: 09:00
```

Session tetap ACTIVE sampai stop existing. Tanpa --since, start/switch behavior fitur 03 tetap berlaku. --since tidak menawarkan switch:

```text
✗ Active session exists: OOT-3668
Review it with wl before adding a backdated start.
```

Non-zero, tanpa mengubah session lama. Tidak menyarankan stop sebagai jaminan rentang --since akan tersedia; setelah stop, overlap dapat tetap menyebabkan rejection.

## Time and Overlap Rules

- HH:mm strict: 09:00 valid; 9:00, 24:00, 09:60, 09:00:30, whitespace luar, date/full timestamp tidak valid.
- Hari/zona perangkat saat command berjalan; detik input nol. From < To <= Now, Since <= Now.
- Tidak ada inference kemarin atau rollover; 23:00–01:00 invalid.
- Nonexistent/ambiguous local time ditolak, tanpa memilih offset atau melakukan normalization.
- Semua existing session lintas tiket/repository diperiksa di bawah lock. Adjacency valid; overlap/duplicate invalid. Active range memiliki end terbuka.
- Durasi adalah elapsed aktual; tampilan floor ke menit: 45m, 2h, 2h 15m. --since tepat now boleh start dengan elapsed 0m.

Contoh pesan overlap:

```text
✗ Session overlaps OOT-3668 (session #12)
Existing: 02 Oct 2026 09:30 → 02 Oct 2026 10:30
Choose a non-overlapping range.
```

Untuk existing active, end ditampilkan `active`. Konflik menyebut key/ID/rentang dari sumber tersimpan, tidak SQL atau isi aktivitas. Title/key output mengikuti safe single-line rendering fitur 07.

## Exit and Dependencies

| Kondisi | Exit | Perilaku |
| --- | --- | --- |
| Manual completed / backdated active sukses | 0 | Commit satu session dan tampilkan detail |
| Argumen/required flag/HH:mm syntax invalid | Non-zero | Usage/error sebelum factory/config/storage |
| Key configured invalid / empty title | Non-zero | Tidak menulis tiket/session |
| Future/invalid ordering/gap/fold | Non-zero | Jelaskan waktu tidak valid, tanpa inference |
| Existing active pada --since | Non-zero | No switch/no mutation |
| Overlap/duplicate | Non-zero | Tampilkan conflict, no partial write |
| Config/storage/invalid existing data/cancellation pre-commit | Non-zero | Propagasi pesan sesuai, rollback work data |
| Output/close failure sesudah commit | Non-zero | Session dapat sudah tersimpan; tidak dihapus |
| Help/version termasuk session/start help | 0 | Tanpa config/storage/repository access |
| Git unavailable / di luar repository | 0 bila operasi lain valid | Repository null, bukan blocker |

No AI/network/Git capture. Config/schema initialization mengikuti fitur 01. Clock satu kali dan location diinjeksi. Command tidak membaca stdin untuk title/waktu atau menawarkan attachment activity.

## Evidence and Retry

Commit/note existing tidak dipindahkan, walau key dan timestamp berada dalam rentang baru. session_id null tetap null. START/STOP timeline berasal dari projection entity baru.

Tidak ada idempotency token. Retry sesudah successful commit/output error akan mendapat overlap atau active conflict. Pengguna meninjau dashboard/today, lalu menentukan tindakan sendiri; fitur ini tidak mengedit session untuk recovery.
