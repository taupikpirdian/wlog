# Feature Specification: Work Activity Notes

**Feature**: `04-work-activity-notes`  
**Status**: Implemented  
**Sumber**: `goal.md`, `../../BRD.md` bagian 4.1, 8, 23, dan 28; `../../PRD.md` bagian 8, 11, 28, 34, dan 38; `../../ERD.md`

## Ringkasan

Developer dapat mencatat investigasi, temuan, support QA, meeting, dan pekerjaan non-coding melalui `wl note` atau `wl n`. Catatan mengikuti tiket dan session aktif, menyimpan teks pengguna serta waktu pencatatan aktual, dan menjadi evidence untuk timeline dan rangkuman pada fitur berikutnya. Menambahkan note tidak mengubah durasi session.

## User Scenarios and Testing

### P1 — Mencatat aktivitas pada session aktif

Sebagai developer, saya ingin menyimpan konteks pekerjaan dengan satu command agar detail penting tidak terlupakan.

**Independent test**: Mulai session `OOT-3668`, jalankan `wl n "Check Splunk logs"`, dan periksa bahwa satu catatan tersimpan pada tiket dan session tersebut.

1. Given session `OOT-3668` aktif, when `wl n "Check Splunk logs"`, then satu aktivitas `NOTE` tersimpan dengan teks tersebut, tiket dan session aktif, serta waktu pencatatan aktual; CLI menampilkan `✓ Note added to OOT-3668`.
2. Given session aktif, when `wl note "Found response mismatch"`, then hasil setara dengan alias `n`.
3. Given session aktif, when tiga command pada goal dijalankan, then tiga catatan tersimpan secara terpisah pada session yang sama, tanpa perubahan waktu mulai, status, waktu selesai, atau durasi session.
4. Given teks berisi Unicode, tanda kutip, atau newline di dalam satu argumen, then isi tetap tersimpan setelah whitespace awal/akhir dipangkas; isi di tengah tidak diubah atau diringkas.
5. Given note yang sama dijalankan dua kali dengan sengaja, then dua aktivitas tersimpan; kesamaan teks bukan alasan deduplikasi.

### P1 — Mencegah catatan tanpa session

Sebagai developer, saya ingin mengetahui ketika session belum dimulai agar catatan tidak kehilangan konteks tiket.

**Independent test**: Pada penyimpanan tanpa session aktif, jalankan `wl n "Support QA retest"`; pastikan command gagal tanpa membuat catatan, tiket, atau session.

1. Given tidak ada session aktif, when note dengan teks valid dijalankan, then exit non-zero, informasi bahwa session belum aktif, dan saran `wl s <ticket> "<title>"` ditampilkan.
2. Given session terakhir sudah selesai, when note dijalankan, then catatan tidak dilekatkan ke session selesai atau tiket terakhir; perilaku sama dengan tidak ada session aktif.
3. Given input kosong, whitespace-only, tanpa argumen, atau lebih dari satu argumen, then command gagal dengan informasi validasi dan tidak membuat aktivitas.

### P1 — Menjaga atribusi ketika command bersamaan

Sebagai developer, saya ingin note tetap terkait dengan session yang dipilih untuk command tersebut ketika proses lain mengganti atau menghentikan session.

**Independent test**: Tahan proses note setelah membaca session A, hentikan/ganti A lewat proses lain, lalu lanjutkan note; pastikan error dan tidak ada catatan baru.

1. Given note telah memilih session A, when proses lain menghentikan atau mengganti A sebelum penyimpanan, then note gagal dengan konflik dan saran mengulang; note tidak dipindahkan otomatis ke session B.
2. Given note tersimpan terlebih dahulu lalu session dihentikan, then note tetap terhubung ke session yang kini selesai dan tidak menghalangi stop.
3. Given dua note dijalankan bersamaan dan session tetap aktif, then keduanya tersimpan sekali masing-masing tanpa menimpa satu sama lain.
4. Given penyimpanan gagal atau dibatalkan sebelum berhasil, then tidak ada aktivitas parsial dan session/tiket tidak berubah.

### P2 — Mencatat tanpa ketergantungan repository

Sebagai developer, saya ingin mencatat pekerjaan non-coding dari direktori mana pun.

1. Given session aktif, when command dijalankan di luar Git atau Git tidak tersedia, then note tetap berhasil.
2. Given command dijalankan dari repository yang berbeda, then note tetap mengikuti session aktif global; konteks repository mengikuti session tersebut, bila tersedia.
3. Given session melewati tengah malam, then note tetap melekat pada session aktif dan memakai tanggal/waktu aktual pencatatan, tanpa membuka session baru.

## Edge Cases

- Waktu perangkat sebelum waktu mulai session: tolak pencatatan, sarankan koreksi jam, dan pertahankan session tanpa perubahan. Waktu yang sama dengan waktu mulai diperbolehkan.
- Note menyebut key tiket lain dalam teks: simpan sebagai teks; atribusi tetap mengikuti tiket session aktif.
- Timestamp dua note sama: keduanya tetap tersimpan sebagai aktivitas berbeda; identitas aktivitas dapat dipakai sebagai urutan tambahan oleh pembaca timeline.
- Input melalui pipe tidak dibaca sebagai isi note; teks wajib diberikan sebagai satu argumen. Command tidak memerlukan prompt interaktif.
- Kegagalan menampilkan output setelah penyimpanan berhasil tidak menghapus note. Retry manual dapat membuat catatan kedua; fitur ini tidak menyediakan idempotency token.

## Functional Requirements

- **FR-001**: Sediakan `wl note "<description>"` dengan alias `n`; keduanya menerima tepat satu argumen deskripsi.
- **FR-002**: Pangkas whitespace awal/akhir deskripsi, tolak hasil kosong, dan pertahankan isi lainnya tanpa klasifikasi atau perubahan otomatis.
- **FR-003**: Note harus memiliki tiket dan session aktif yang sama. Pengguna tidak memilih tiket/session melalui argumen tambahan.
- **FR-004**: Simpan satu aktivitas bertipe `NOTE` untuk setiap command yang berhasil, dengan deskripsi dan waktu pencatatan aktual.
- **FR-005**: Tanpa session aktif, gagal tanpa membuat catatan, tiket, atau session; tampilkan cara memulai session.
- **FR-006**: Validasi ulang bahwa session yang dipilih masih aktif saat penyimpanan. Perubahan session menghasilkan konflik tanpa atribusi ulang otomatis atau write parsial.
- **FR-007**: Menambahkan note tidak mengubah tiket, judul session, status, rentang waktu, atau durasi session.
- **FR-008**: Catatan mengikuti session aktif global pada penyimpanan pengguna, tanpa bergantung pada direktori tempat command dijalankan.
- **FR-009**: Konteks repository note mengikuti session aktif bila tersedia; Git dan jaringan tidak diperlukan.
- **FR-010**: Waktu pencatatan tidak boleh lebih awal dari waktu mulai session; error tidak mengubah data yang sudah ada.
- **FR-011**: Sukses menghasilkan exit 0 dan konfirmasi tiket tujuan. Error argumen, validasi, tidak ada session, konflik, atau penyimpanan menghasilkan exit non-zero dengan pesan yang sesuai.
- **FR-012**: Help/version, termasuk bantuan note, tersedia tanpa membuat atau membuka penyimpanan.
- **FR-013**: Note bukan sumber durasi pekerjaan dan tidak memicu AI, Git capture, atau pengiriman data ke layanan lain.

## Key Entities

- **Note activity**: Catatan teks bertipe `NOTE`, dengan identitas, tiket, session, waktu pencatatan, dan konteks repository opsional.
- **Active work session**: Satu rentang pekerjaan aktif global yang menentukan tiket dan session tujuan note.
- **Ticket**: Identitas pekerjaan dari session aktif; note tidak membuat atau mengganti identitas tiket.

## Success Criteria

- **SC-001**: Developer dapat mencatat aktivitas non-coding melalui satu command, tanpa prompt atau argumen tiket tambahan.
- **SC-002**: Setiap note yang berhasil memiliki tiket yang sama dengan session tujuannya; tidak ada note tanpa session yang dibuat oleh fitur ini.
- **SC-003**: Input tidak valid, tidak ada session, konflik, dan kegagalan penyimpanan sebelum sukses tidak menambah catatan atau mengubah session/tiket.
- **SC-004**: Dua pencatatan yang berhasil menghasilkan dua aktivitas utuh, termasuk bila teks atau waktu pencatatannya sama.
- **SC-005**: Developer menerima konfirmasi note dalam kurang dari 200 ms pada penggunaan lokal biasa setelah inisialisasi, tanpa antrean proses lain, sesuai target PRD bagian 34.
- **SC-006**: Note dapat ditambahkan di luar repository dan tanpa jaringan selama ada session aktif.

## Dependencies and Assumptions

- Fitur 01 menyediakan konfigurasi dan penyimpanan; fitur 02 menyediakan identitas tiket; fitur 03 menyediakan session aktif global.
- `goal.md` menyebut alias `n`; nama lengkap `note` mengikuti PRD bagian 8 dan 11.
- Pemangkasan whitespace luar, pemeriksaan jam, dan konflik snapshot melengkapi sumber agar input serta atribusi konsisten. Jam mundur antar-note yang masih setelah start tidak dikoreksi otomatis; timestamp aktual dipertahankan.
- Note mengambil repository dari session, bukan dari direktori command. Branch dan detail commit tidak dikumpulkan untuk note pada fitur ini.
- Catatan dengan isi sama boleh berulang. Tidak ada batas panjang produk tambahan pada fitur ini; kegagalan penyimpanan tetap dilaporkan tanpa memotong isi.
- AC-05 PRD menuliskan `wl n` tanpa isi sebagai contoh tanpa session. Dengan kontrak tepat satu argumen, bentuk tersebut menghasilkan error argumen; pengujian tanpa session memakai deskripsi valid.

## Out of Scope

Edit/hapus/list note, pemilihan tiket manual, note tanpa session, backdating, koreksi durasi, klasifikasi aktivitas, Git commit capture/hook, dashboard/timeline, AI summary/description, dan sinkronisasi eksternal.
