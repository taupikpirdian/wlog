# Feature Specification: Dashboard & Today Timeline

**Feature**: `07-dashboard-today-timeline`

**Created**: 2026-10-02

**Status**: Implemented and verified

**Sumber**: [goal.md](goal.md), BRD bagian 14–15, PRD bagian 8, 20, 34, serta fitur 01–06.

## Ringkasan

Developer menjalankan `wl` untuk melihat session aktif, durasi berjalan, dan total waktu hari ini per tiket. `wl today` menampilkan urutan START, NOTE, COMMIT, dan STOP hari ini sehingga pekerjaan dapat ditinjau sebelum membuat worklog. Keduanya memakai data lokal tanpa jaringan, capture baru, atau perubahan catatan pekerjaan.

## User Scenarios and Testing

### P1 — Melihat pekerjaan saat ini

Sebagai developer, saya ingin mengetahui session aktif dan waktu yang sudah dicatat melalui satu command.

**Independent test**: Siapkan beberapa session selesai dan satu session aktif dengan waktu acuan tetap; jalankan `wl` dan cocokkan durasi serta total per tiket.

1. Given session OOT-3751 dimulai 09:14 dan waktu sekarang 10:56, when `wl`, then tampilkan key, judul session, Started 09:14, dan Duration 1h 42m.
2. Given beberapa session untuk tiket yang sama, then gabungkan kontribusi hari ini dalam satu baris tiket, termasuk waktu berjalan session aktif.
3. Given tidak ada session aktif tetapi ada pekerjaan hari ini, then tampilkan No active session dan ringkasan pekerjaan yang sudah selesai.
4. Given penyimpanan belum berisi pekerjaan, then tampilkan No active session, No tracked sessions today, dan Total 0m; command berhasil.
5. Given command dijalankan di direktori lain atau di luar Git, then hasil menggunakan seluruh data pengguna yang sama.

### P1 — Meninjau timeline hari ini

Sebagai developer, saya ingin membaca kejadian sesuai waktu agar urutan pekerjaan mudah ditelusuri.

**Independent test**: Siapkan dua session, note, dan commit hari ini dengan waktu tetap; jalankan `wl today` dan cocokkan setiap baris dengan sumbernya.

1. Given session dimulai 09:00, note 09:15, commit 10:12, dan selesai 11:30, then tampilkan START, NOTE, COMMIT, STOP dalam urutan tersebut, dengan key tiket pada masing-masing baris.
2. Given session kedua dimulai 13:00, then START dan note session kedua muncul setelah kejadian pagi, tanpa pengelompokan yang mengubah kronologi global.
3. Given session masih aktif, then tidak ada STOP buatan; total menggunakan waktu acuan command.
4. Given tidak ada kejadian hari ini, then tampilkan tanggal, No activities today, dan Total tracked yang sesuai. Session yang dimulai kemarin dapat memberi total positif tanpa START hari ini.
5. Given kejadian memiliki timestamp sama, then urutan tetap deterministik pada pembacaan ulang data yang sama.

### P1 — Membatasi durasi pada hari lokal

Sebagai developer, saya ingin total hari ini menghitung hanya waktu yang berada di tanggal lokal hari ini.

**Independent test**: Pada zona Asia/Jakarta, session kemarin 23:30 sampai hari ini 00:30 menyumbang 30m; session aktif dari kemarin 23:00 sampai sekarang 01:00 menyumbang 1h hari ini dan Duration aktif 2h.

1. Given session melintasi tengah malam, then total hari ini hanya menghitung irisan session dengan hari ini; Duration aktif tetap seluruh waktu sejak mulai.
2. Given kejadian tepat tengah malam awal hari, then masuk hari ini; tepat tengah malam hari berikutnya tidak masuk.
3. Given perubahan offset zona lokal, then batas hari mengikuti tanggal kalender lokal, bukan asumsi setiap hari selalu 24 jam.
4. Given waktu sekarang sebelum start session aktif atau rentang session tidak valid, then gagal dengan pesan data/waktu tidak valid tanpa menampilkan durasi negatif atau mengubah data.

### P2 — Melihat evidence di luar session

Sebagai developer, saya ingin commit yang tidak terkait session tetap terlihat tanpa dianggap sebagai waktu kerja.

1. Given commit memiliki tiket tetapi tidak memiliki session, then dashboard menampilkannya pada Unsessioned dan timeline tetap menyertakan COMMIT dengan tiket tersebut.
2. Given commit tidak memiliki tiket, then dashboard menampilkannya pada Unassigned dan timeline memakai label UNASSIGNED.
3. Given tiket hanya memiliki aktivitas tanpa session, then baris tiket Today bernilai 0m; aktivitas tidak menambah total tracked.
4. Given commit tertangkap sekarang tetapi waktu kejadiannya kemarin, then tidak masuk hari ini. Waktu capture tidak menggantikan waktu kejadian yang tersimpan.

## Edge Cases

- Session mulai kemarin dan berhenti hari ini: hanya STOP muncul hari ini; START kemarin tidak disalin. Key pada STOP menjaga konteks.
- Session berdurasi nol tetap menghasilkan START/STOP bila kejadiannya hari ini, dengan kontribusi 0m.
- NOTE/COMMIT tampil berdasarkan waktu kejadiannya sendiri, bukan tanggal mulai session terkait.
- Commit yang tidak memiliki pesan menampilkan `(no commit message)`; hash pendek tersedia sebagai identitas tambahan.
- Judul/note/pesan multiline dan karakter kontrol ditampilkan aman sebagai satu baris; isi tersimpan tidak diubah.
- Semua detik dijumlahkan sebelum format menit. Dua session 40 detik menghasilkan 1m, bukan 0m; kurang dari satu menit ditampilkan 0m.
- Pembacaan bersamaan dengan start/stop/note/capture memakai satu snapshot yang konsisten; perubahan sesudah snapshot terlihat pada command berikutnya.
- Storage/cancellation/output error menghasilkan non-zero. Jangan menyatakan data kosong bila pembacaan gagal.

## Functional Requirements

- **FR-001**: `wl` tanpa argumen menampilkan dashboard, menggantikan output readiness fitur 01; inisialisasi otomatis yang aman tetap tersedia.
- **FR-002**: Tampilkan key, judul session, waktu mulai lokal, dan durasi seluruh session aktif; tanpa session aktif tampilkan status eksplisit.
- **FR-003**: Tampilkan satu baris per tiket yang memiliki kontribusi session atau aktivitas hari ini, serta total tracked dari session saja. Urutkan key tiket menaik.
- **FR-004**: Durasi harian adalah irisan rentang session dengan hari lokal hingga waktu acuan command; session aktif berakhir sementara pada waktu acuan tanpa disimpan sebagai selesai.
- **FR-005**: `wl today` menerima nol argumen dan menampilkan tanggal serta kejadian hari ini dalam urutan kronologis global.
- **FR-006**: START/STOP berasal dari waktu mulai/selesai session; NOTE/COMMIT dari aktivitas tersimpan. Tampilkan waktu HH:mm, jenis, tiket atau UNASSIGNED, dan isi NOTE/COMMIT.
- **FR-007**: Filter kejadian pada rentang awal hari inklusif sampai awal hari berikutnya eksklusif. Waktu event tersimpan dipertahankan walau berada setelah waktu acuan; durasi hanya dihitung sampai waktu acuan.
- **FR-008**: Kejadian dengan waktu sama diurutkan START, NOTE, COMMIT, STOP, lalu identitas sumber menaik; timestamp penuh dipakai sebelum format HH:mm.
- **FR-009**: Tampilkan Unsessioned untuk aktivitas bertiket tanpa session dan Unassigned untuk aktivitas tanpa tiket; aktivitas tanpa tiket masuk Unassigned saja agar tidak berulang pada dashboard.
- **FR-010**: Aktivitas bukan sumber durasi; tiket dengan aktivitas saja bernilai 0m. Total dihitung dari detik sebelum pembulatan tampilan ke menit.
- **FR-011**: Kedua command memakai zona lokal perangkat dan satu waktu acuan per eksekusi, seluruh data pengguna lintas repository, serta snapshot pembacaan yang konsisten.
- **FR-012**: Tidak menambah/mengubah session, tiket, atau aktivitas. Tidak menjalankan Git, AI, sinkronisasi, atau prompt. Inisialisasi config/schema mengikuti fitur 01.
- **FR-013**: Data kosong exit 0; argumen tidak valid, storage error, cancellation, data/waktu invalid, dan output error exit non-zero dengan pesan sesuai.
- **FR-014**: Help/version tersedia tanpa membuka atau membuat config/database. Teks tampilan satu baris tidak boleh menjalankan karakter kontrol terminal.

## Key Entities

- **Day window**: Tanggal lokal, zona, awal/akhir hari, dan waktu acuan command.
- **Active session view**: Identitas tiket, judul session, waktu mulai dan elapsed penuh.
- **Ticket daily summary**: Key dan detik kontribusi semua session terkait dalam hari tersebut.
- **Timeline event**: Waktu, jenis, tiket opsional, sumber session/activity, dan teks evidence.
- **Daily snapshot**: Data session serta aktivitas konsisten untuk satu pembacaan.

## Success Criteria

- **SC-001**: Developer melihat status aktif dan total per tiket melalui satu command tanpa prompt.
- **SC-002**: Seluruh fixture normal, lintas hari, batas tengah malam, dan perubahan offset menghasilkan jumlah detik sesuai irisan rentang kerja; note/commit menambah nol detik.
- **SC-003**: Setiap kejadian hari ini muncul tepat sekali pada timeline, dengan urutan deterministik dan atribusi sesuai sumber; tidak ada STOP buatan.
- **SC-004**: Pada penyimpanan lokal yang sudah diinisialisasi tanpa antrean proses lain, dashboard selesai kurang dari 200 ms dan timeline kurang dari 500 ms, sesuai target PRD; validasi mencatat perangkat dan ukuran fixture.
- **SC-005**: Semua skenario pembacaan sukses maupun gagal mempertahankan data pekerjaan yang tersimpan; hasil kosong tidak menggantikan error.

## Dependencies and Assumptions

- Fitur 01 menyediakan config/schema, 02 tiket, 03 session, 04 note, 05–06 evidence commit. Judul dashboard menggunakan judul session, bukan judul tiket yang dapat kosong.
- Goal menjadi acuan kronologi global `wl today`, menggantikan contoh grouping per tiket PRD 20. Ringkasan per tiket tetap tersedia pada dashboard.
- Total dan kategori Unsessioned/Unassigned melengkapi contoh minimal goal sesuai PRD 8. Kategori kosong dihilangkan.
- Zona lokal perangkat dipilih karena konfigurasi saat ini belum memiliki timezone; tidak ada flag tanggal/zona tambahan.
- Durasi ditampilkan `0m`, `45m`, `1h 42m`; pembulatan ke bawah untuk presentasi saja. Total memakai jumlah detik mentah sehingga dapat berbeda dari penjumlahan angka menit pada baris.
- Snapshot dan validasi rentang melengkapi sumber agar hasil tidak mencampur keadaan sebelum/sesudah stop. Timestamp future event tetap ditampilkan pada tanggalnya sebagai evidence tersimpan.
- **Persyaratan implementasi dari goal**: gunakan TDD RED → GREEN → REFACTOR pada setiap penambahan/perubahan kode `application` dan `domain`. Gate coverage yang diteruskan dari fitur 06 dijabarkan pada plan/quickstart.

## Out of Scope

Grouping/flag/filter/pagination timeline, pemilihan tanggal historis, live refresh, export JSON, edit/hapus/assign activity, manual session fitur 08, koreksi timestamp, Git capture baru, AI summary, dan layanan eksternal.
