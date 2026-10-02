# Feature Specification: Manual Session Management

**Feature**: `08-manual-session-management`

**Created**: 2026-10-02

**Status**: Implemented and verified

**Sumber**: [goal.md](goal.md), BRD bagian 13, PRD bagian 9–10 dan 21, serta fitur 03 dan 07.

## Ringkasan

Developer yang lupa menyalakan tracker dapat mencatat session selesai melalui `wl session <ticket> --from HH:mm --to HH:mm --title "<title>"`, atau memulai session aktif dari waktu sebelumnya melalui `wl s <ticket> "<title>" --since HH:mm`. Waktu berasal dari input pengguna dan jam lokal perangkat; sistem tidak memakai AI atau evidence commit untuk menaksir durasi. Session baru langsung tersedia pada dashboard dan timeline existing.

## User Scenarios and Testing

### P1 — Mencatat pekerjaan yang sudah selesai

Sebagai developer, saya ingin memasukkan rentang pekerjaan yang terlewat agar total tracked tetap sesuai waktu yang saya tentukan.

**Independent test**: Dengan waktu sekarang 12:00 lokal dan tidak ada session pada 09:00–11:00, jalankan contoh goal; pastikan satu completed session OOT-3751, judul Fix tax calculation, dan durasi 2h.

1. Given tiket belum ada, when `wl session OOT-3751 --from 09:00 --to 11:00 --title "Fix tax calculation"`, then buat tiket dan satu completed session secara utuh; tampilkan tiket, judul, rentang 09:00 → 11:00 dan Duration 2h.
2. Given tiket sudah ada dengan judul master, then gunakan tiket tersebut tanpa mengubah judul master; judul input menjadi judul session baru.
3. Given ada session aktif sejak 11:00, when mencatat rentang 09:00–11:00, then session selesai baru berhasil dan session aktif tetap utuh.
4. Given completed session baru berhasil, then dashboard menambahkan 2h pada tiket tersebut dan `wl today` menampilkan START 09:00 serta STOP 11:00 berdasarkan waktu input.
5. Given penyimpanan gagal sebelum commit, then tidak ada tiket/session parsial atau perubahan session existing.

### P1 — Memulai tracker dari waktu sebelumnya

Sebagai developer, saya ingin melanjutkan pekerjaan yang masih berlangsung dari waktu mulai yang saya ingat.

**Independent test**: Dengan waktu sekarang 10:30 lokal dan tidak ada session aktif/overlap, jalankan `wl s OOT-3751 "Fix tax" --since 09:00`; pastikan satu active session sejak 09:00 dan dashboard Duration 1h 30m.

1. Given tidak ada session aktif dan rentang tersedia, when `wl start OOT-3751 "Fix tax" --since 09:00`, then buat session aktif dengan waktu mulai 09:00 hari lokal ini; waktu selesai dan durasi final belum diisi.
2. Given memakai alias `wl s` atau shorthand flag `-s 09:00`, then hasil sama dengan nama lengkap/flag panjang.
3. Given ada session aktif, then `--since` gagal tanpa menyelesaikan, mengganti, atau memundurkan session tersebut; tampilkan petunjuk meninjau session aktif.
4. Given session --since berhasil, then note dan commit berikutnya mengikuti aturan atribusi existing; `wl stop` menyelesaikannya menggunakan waktu aktual.
5. Given `wl start` tidak memakai --since, then perilaku start/switch confirmation existing tetap berlaku.

### P1 — Menolak waktu dan rentang yang tidak sah

Sebagai developer, saya ingin input salah ditolak agar sistem tidak menciptakan waktu kerja yang tidak saya maksudkan.

1. Given waktu bukan format tepat HH:mm, termasuk 9:00, 24:00, 09:60, teks kosong atau tanggal penuh, then gagal sebelum membuka penyimpanan.
2. Given from >= to, then session manual gagal tanpa dianggap melintasi tengah malam.
3. Given to atau since lebih akhir dari waktu sekarang, then gagal tanpa menebak tanggal kemarin atau waktu di masa depan.
4. Given waktu lokal tidak ada atau memiliki dua kemungkinan instant karena perubahan offset zona, then gagal dengan pesan waktu lokal tidak dapat ditentukan secara unik; tidak memilih offset otomatis.
5. Given argumen/flag wajib tidak lengkap, key tidak sesuai konfigurasi, atau judul kosong setelah pemangkasan whitespace, then gagal tanpa menulis session/tiket.

### P1 — Mencegah waktu yang dihitung dua kali

Sebagai developer, saya ingin rentang manual tidak bertabrakan dengan session yang sudah tercatat.

**Independent test**: Siapkan completed session 09:00–10:00; coba manual 09:30–11:00 dan --since 09:30. Keduanya gagal. Manual 10:00–11:00 berhasil.

1. Given overlap dengan session existing pada tiket sama atau berbeda, termasuk repository berbeda, then kedua flow manual menolak penambahan dan menampilkan session konflik beserta rentangnya.
2. Given rentang berakhir tepat saat session lain mulai atau mulai tepat saat session selesai, then tidak dianggap overlap.
3. Given command yang sama diulang setelah sukses, then rentang duplikat ditolak sebagai overlap; tidak membuat session kedua.
4. Given dua penambahan manual bersamaan dengan rentang overlap, then maksimal satu berhasil; proses lain melihat konflik tanpa tiket/session parsial.
5. Given normal start/stop berjalan bersamaan, then pemeriksaan existing state dan insert manual memakai satu perubahan atomik; --since tidak dapat menghasilkan dua session aktif.

## Edge Cases

- HH:mm selalu tanggal lokal saat command dijalankan; tidak mendukung kemarin, tanggal historis, atau rollover otomatis. `--from 23:00 --to 01:00` ditolak.
- Waktu detik input adalah nol; waktu acuan aktual mempertahankan detik. --since tepat pada now diperbolehkan, sementara completed session wajib memiliki durasi positif.
- Hari dengan perubahan offset dapat memiliki elapsed berbeda dari selisih angka jam; durasi memakai elapsed aktual antar instant yang unik.
- Existing completed session dengan durasi nol tidak menghalangi rentang baru. Active session tetap menghalangi --since walau elapsed masih nol.
- Rentang completed baru yang seluruhnya sebelum session aktif boleh ditambahkan. Rentang yang menyentuh bagian setelah active start ditolak.
- Commit/note existing, termasuk yang berada dalam rentang manual dan memiliki tiket sama, tetap memiliki keterkaitan session dan waktu semula. Tidak ada attach otomatis atau prompt pada fitur ini.
- Repository opsional berasal dari direktori command dengan aturan fitur 03. Command tetap dapat digunakan di luar Git atau saat Git tidak tersedia.
- Output gagal setelah commit tidak membatalkan session. Retry rentang yang sama akan menghasilkan overlap/active conflict; tidak menjanjikan respons sukses yang sama pada retry.
- Data existing dengan timestamp/rentang rusak menghasilkan error, tanpa memperbaiki atau mengabaikan data secara diam-diam.

## Functional Requirements

- **FR-001**: Sediakan `wl session <ticket>` dengan tepat satu argumen tiket dan tiga flag wajib --from, --to, --title; buat satu completed session setelah seluruh validasi dan persistence berhasil.
- **FR-002**: Tambahkan --since dengan shorthand -s pada `wl start`/alias `s`; tanpa flag tersebut pertahankan start/switch behavior existing.
- **FR-003**: Terima HH:mm tepat dua digit jam dan menit, dalam rentang 00:00–23:59. Input waktu dan kelengkapan argumen divalidasi sebelum factory/storage dipanggil.
- **FR-004**: Resolve waktu pada tanggal/zona lokal perangkat memakai satu now per eksekusi. Tolak waktu future, waktu lokal tidak ada/ambigu, serta manual from >= to. Jangan melakukan rollover, estimasi, atau koreksi otomatis.
- **FR-005**: Validasi ticket key memakai aturan konfigurasi existing; pangkas whitespace luar title, tolak kosong, dan pertahankan isi lainnya tanpa mengubah judul master tiket.
- **FR-006**: Completed session baru memiliki start/end dari input, status selesai, dan durasi elapsed dalam detik. Session --since memiliki start dari input dan status aktif tanpa field selesai.
- **FR-007**: --since memerlukan tidak ada session aktif; jangan menawarkan switch atau mengubah session aktif existing melalui flow backdate.
- **FR-008**: Sebelum insert, kedua flow menolak overlap kandidat dengan seluruh session existing lintas tiket/repository. Batas rentang bersifat awal inklusif/akhir eksklusif; active existing memiliki akhir terbuka.
- **FR-009**: Pemeriksaan active/overlap, pembuatan tiket bila perlu, dan insert session harus atomik. Konflik atau kegagalan sebelum commit tidak menulis data pekerjaan parsial.
- **FR-010**: Tidak mengubah/menghapus session existing atau memindahkan note/commit existing. Tidak menambahkan activity START/STOP baru; timeline mengikuti projection session fitur 07.
- **FR-011**: Hasil sukses exit 0 dan menampilkan key, judul session serta waktu aktual yang disimpan; completed menampilkan durasi. Invalid input, time, active/overlap conflict, storage/cancel/output error exit non-zero dengan pesan sesuai.
- **FR-012**: Help/version termasuk help session/start tidak membuat/membuka config/database. Flow manual tidak memerlukan prompt, AI, jaringan, atau Git capture.
- **FR-013**: Session baru memakai penyimpanan pengguna yang sama, tetap kompatibel dengan note/stop/commit, dashboard dan timeline existing.

## Key Entities

- **Local wall time**: HH:mm pengguna yang diinterpretasikan pada tanggal/zona lokal command, hanya jika menghasilkan satu instant yang sah.
- **Manual completed session**: Rentang selesai dengan tiket, judul, start/end serta durasi eksplisit.
- **Backdated active session**: Session aktif dengan waktu start yang diberikan pengguna.
- **Session range conflict**: Session existing yang beririsan dengan kandidat; identitas, tiket dan waktunya menjadi konteks error.

## Success Criteria

- **SC-001**: Developer mencatat pekerjaan yang telah selesai melalui satu command tanpa prompt atau perubahan waktu otomatis.
- **SC-002**: Contoh 09:00–11:00 menghasilkan tepat satu completed session dengan durasi 7.200 detik; dashboard dan timeline menampilkan kontribusi yang sama.
- **SC-003**: --since 09:00 pada now 10:30 menghasilkan active elapsed 5.400 detik; stop/note/commit berikutnya tetap bekerja dengan session tersebut.
- **SC-004**: Seluruh fixture invalid/future/ambiguous time, overlap, active conflict dan failure pre-commit tidak menambah atau mengubah tiket/session/aktivitas.
- **SC-005**: Pada dua command dengan rentang overlap yang dijalankan bersamaan, maksimal satu penambahan sukses dan tidak ada dua session aktif.
- **SC-006**: Tidak ada waktu atau atribusi yang ditentukan AI, teks commit, jumlah perubahan kode, atau jarak antar aktivitas.

## Dependencies and Assumptions

- Fitur 01–03 menyediakan config, tiket, schema, session dan transaksi; fitur 04–07 membaca entity tersebut. Tidak diperlukan perubahan schema atau integrasi eksternal.
- Goal hanya menyebut HH:mm: versi ini membatasi input ke hari lokal saat ini. Flag tanggal/full timestamp dan lintas hari ditunda.
- Penolakan overlap merupakan keputusan produk untuk mencegah double counting fitur 07. Pemeriksaan berlaku pada dua flow manual baru; tidak memperbaiki overlap historis atau mengubah semantics ordinary start/stop.
- --since tidak menjalankan switch confirmation karena stop retroaktif dapat bertabrakan dengan note/commit yang sudah tersimpan. Ordinary start tanpa --since tetap menyediakan switch existing.
- PRD 21 menyebut penawaran attach unsessioned activity sebagai opsional. Fitur ini tidak menyertakannya agar penambahan rentang tidak mengubah atribusi evidence tanpa operasi terpisah.
- **Persyaratan implementasi dari goal**: TDD RED → GREEN → REFACTOR pada setiap perubahan/penambahan kode application dan domain. Gate coverage existing dijabarkan pada plan/quickstart.

## Out of Scope

Edit/hapus/merge session, tanggal historis/lintas tengah malam, full timestamp/offset flags, force-overlap, override session aktif, attachment/reassignment evidence, recovery gap otomatis, durasi perkiraan AI, summary dan sinkronisasi eksternal.
