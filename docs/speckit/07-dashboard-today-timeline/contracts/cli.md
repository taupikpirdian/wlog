# CLI Contract: Dashboard & Today Timeline

## Commands

```sh
wl
wl today
```

Nol argumen posisi, tanpa prompt, date/filter/group flags atau live refresh. Output seluruh data pengguna lintas repository, memakai zona lokal perangkat. Header dan spasi alignment dapat menyesuaikan lebar key; field, urutan dan makna berikut wajib.

## Dashboard

```text
DEV WORKLOG

Active Session
OOT-3751 — Fix tax calculation

Started : 09:14
Duration: 1h 42m

Today
─────────────────────────────
OOT-3668    45m
OOT-3747    1h 15m
OOT-3751    2h 30m
Total      4h 30m
```

Contoh ringkasan dapat mencakup session sebelumnya pada tiket aktif, sehingga total tiket tidak harus sama dengan Duration aktif. Baris tiket diurutkan key ascending.

Tanpa active session: `No active session`. Tanpa tiket/session/activity hari ini: `No tracked sessions today` dan `Total 0m`. Activity-only ticket muncul 0m. Bila active start bukan hari ini, Started memuat tanggal `YYYY-MM-DD HH:mm` agar tidak ambigu; Duration tetap elapsed penuh.

Bagian tambahan hanya bila berisi event hari ini:

```text
Unsessioned
10:32  COMMIT  OOT-3747  fix CIAM response validation [abc1234]

Unassigned
12:45  COMMIT  UNASSIGNED  update documentation [def5678]
```

Unsessioned berarti punya tiket tetapi session_id null. Unassigned berarti ticket_id null, tampil sekali di kategori tersebut. Keduanya tidak menambah tracked time.

## Today Timeline

```text
02 Oct 2026
─────────────────────────────
09:00  START   OOT-3751
09:15  NOTE    OOT-3751  Check tax calculation
10:12  COMMIT  OOT-3751  fix tax calculation [abc1234]
11:30  STOP    OOT-3751
13:00  START   OOT-3668
13:15  NOTE    OOT-3668  Support QA

Total tracked: 3h
```

Contoh memakai now 13:30, session pertama 2h 30m dan session kedua 30m. START/STOP selalu menyertakan tiket walau contoh minimal goal menghilangkan key pada STOP. Tidak ada STOP untuk session aktif. Blank line antar session tidak wajib karena activity tanpa session dapat berada di antara event.

Tanggal memakai format `DD Mon YYYY` dengan nama bulan English. HH:mm dalam local timezone; sorting memakai instant timestamp penuh sebelum formatting. Pada perubahan offset, HH:mm dapat berulang/mundur tetapi instant order tetap benar. Timestamp sama memakai kind rank START, NOTE, COMMIT, STOP lalu source ID ascending.

Tidak ada event: tampilkan `No activities today` dan Total tracked. Total dapat positif bila session aktif melintasi midnight tanpa kejadian baru hari ini.

## Text and Duration Formatting

- Format durasi floor menit: 0m, 45m, 1h, 1h 42m, 25h. Jumlahkan detik sebelum format total.
- NOTE memakai description. COMMIT memakai pesan, fallback `(no commit message)`, serta maksimal tujuh karakter awal hash bila tersedia.
- Evidence/judul dirender satu baris: newline/tab menjadi spasi, ANSI escape sequence dan karakter kontrol terminal dibuang, Unicode printable dipertahankan. Tidak ada pemotongan panjang teks tambahan.
- Source timestamp dan atribusi tidak direkonstruksi dari isi note/pesan atau current cwd.

## Exit and Errors

| Kondisi | Exit | Perilaku |
| --- | --- | --- |
| Dashboard/timeline sukses, termasuk kosong | 0 | Output sesuai data snapshot |
| First run valid | 0 jika bootstrap/read sukses | Config/schema diinisialisasi seperti fitur 01, kemudian empty views |
| Argumen tambahan/unknown flag/unknown command | Non-zero | Validation sebelum factory, tanpa storage |
| Config/storage/scan/parse/snapshot error | Non-zero | Pesan error; tidak tampilkan empty success atau partial view |
| Active start setelah now/rentang invalid | Non-zero | Pesan waktu/data invalid; tidak ada durasi negatif atau koreksi data |
| Cancellation | Non-zero | Propagasi error, tanpa mutation work data |
| Output/close error | Non-zero | Data tetap utuh; output mungkin sebagian bila writer gagal |
| Help/version | 0 | Tidak membuat/membuka config/database |

Command membaca satu snapshot dan clock sekali. Perubahan concurrent setelah snapshot terlihat pada command berikutnya. Initialization dapat membuat config/schema; command tidak membuat pekerjaan atau mengakhiri session aktif.
