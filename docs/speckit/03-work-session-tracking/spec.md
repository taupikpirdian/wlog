# Feature Specification: Work Session Tracking

**Feature**: `03-work-session-tracking`  
**Status**: Implemented  
**Sumber**: `goal`, `../../BRD.md` bagian 6–7, `../../PRD.md` bagian 9–10, 12, 37, dan `../../ERD.md`

## Ringkasan

Developer dapat memulai pekerjaan menggunakan `wl start` atau `wl s`, lalu mengakhirinya menggunakan `wl stop` atau `wl x`. Session menyimpan tiket, judul, repository opsional, waktu mulai/selesai, serta durasi yang dihitung dari timestamp aktual. Hanya satu session boleh aktif untuk seluruh penyimpanan lokal pengguna, termasuk ketika command dijalankan dari repository berbeda.

## User Scenarios and Acceptance

### P1 — Memulai pekerjaan

Sebagai developer, saya ingin memulai pencatatan pekerjaan dengan satu command agar waktu kerja dapat diukur.

1. Given tidak ada session aktif, when `wl s OOT-3751 "Fix tax calculation"`, then session dibuat dengan tiket tersebut, judul, waktu mulai aktual, dan status `ACTIVE`.
2. Given tiket belum terdaftar, when start berhasil, then satu record tiket dibuat menggunakan kemampuan fitur 02 dan session merujuk ID tiket itu.
3. Given tiket sudah ada, when start berhasil, then ID tiket yang sama digunakan tanpa mengubah judul master tiket.
4. Given command dijalankan dalam repository Git, then lokasi root repository dicatat sebagai konteks session.
5. Given command dijalankan di luar repository atau konteks Git tidak tersedia, then session tetap dapat dimulai dengan repository kosong.
6. Given key tidak valid atau judul kosong setelah whitespace dipangkas, then command gagal tanpa membuat tiket/session baru.

### P1 — Menyelesaikan pekerjaan

Sebagai developer, saya ingin menghentikan session agar rentang waktu dan durasinya tersimpan.

1. Given satu session aktif, when `wl x`, then session menjadi `COMPLETED`, `ended_at` terisi, dan durasi berasal dari selisih timestamp mulai/selesai.
2. Given tidak ada session aktif, when stop, then command mengembalikan error yang menjelaskan cara memulai session, tanpa mengubah data.
3. Given session selesai, when stop diulang, then record yang sudah selesai tidak diubah.
4. Given waktu selesai lebih awal daripada waktu mulai karena perubahan jam perangkat, then stop gagal dengan pesan koreksi jam dan session tetap aktif.
5. Given session dimulai sebelum tengah malam dan dihentikan sesudahnya, then durasi memakai seluruh selisih waktu, tanpa dipotong pada pergantian tanggal.

### P1 — Mengganti session aktif

Sebagai developer, saya ingin memberi persetujuan sebelum session lama dihentikan ketika mulai mengerjakan tiket berikutnya.

1. Given session aktif, when start baru, then CLI menampilkan tiket, judul, waktu mulai, elapsed time, dan meminta konfirmasi.
2. Given jawaban `y`, `yes`, atau Enter pada prompt `[Y/n]`, then session lama diselesaikan dan session baru dimulai dengan satu timestamp pergantian yang sama.
3. Given jawaban `n` atau `no`, then start dibatalkan dengan exit sukses dan tidak ada perubahan data.
4. Given jawaban tidak dikenal, then CLI meminta jawaban ulang tanpa mengubah data.
5. Given input noninteraktif atau EOF tanpa jawaban, then command gagal dengan pesan bahwa session masih aktif, tanpa perubahan data.
6. Given session aktif berubah ketika pengguna menjawab prompt, then command gagal dan meminta pengguna mengulang; session yang berbeda tidak boleh ditutup tanpa persetujuan baru.
7. Given start baru gagal setelah pengguna menyetujui pergantian, then seluruh perubahan dibatalkan dan session lama tetap aktif.

## Functional Requirements

- **FR-001**: Sediakan `wl start <ticket> "<title>"` dengan alias `s`, serta `wl stop` dengan alias `x`.
- **FR-002**: Start menerima tepat dua argumen; stop tidak menerima argumen.
- **FR-003**: Validasi seluruh key menggunakan `ticket.pattern` dari konfigurasi fitur 01/02. Pangkas whitespace judul dan tolak judul kosong.
- **FR-004**: Start menyimpan tiket, judul session, repository opsional, waktu mulai aktual, dan status `ACTIVE`; waktu selesai/durasi belum diisi.
- **FR-005**: Hanya satu session aktif diperbolehkan secara global per database. Aturan tetap berlaku untuk command bersamaan.
- **FR-006**: Start pada session aktif membutuhkan konfirmasi sebelum ada perubahan data, termasuk bila tiket baru sama dengan tiket session aktif.
- **FR-007**: Pergantian session bersifat atomik: menyelesaikan session lama, memastikan tiket baru, dan membuat session baru harus berhasil bersama.
- **FR-008**: Konfirmasi berlaku hanya untuk ID session yang ditampilkan. Perubahan session aktif selama prompt membatalkan tindakan.
- **FR-009**: Stop hanya mengubah session berstatus `ACTIVE`, mengisi waktu selesai dan durasi, lalu menetapkan status `COMPLETED`.
- **FR-010**: Durasi dalam detik adalah selisih timestamp aktual, dibulatkan ke bawah ke detik penuh. Session kurang dari satu detik boleh berdurasi nol; durasi negatif ditolak.
- **FR-011**: Durasi tidak ditaksir oleh AI, commit, judul, atau pembulatan tampilan.
- **FR-012**: Start tetap tersedia tanpa Git atau repository; kegagalan membaca konteks repository tidak membatalkan pencatatan pekerjaan.
- **FR-013**: Error validasi, storage, dan konflik session menghasilkan exit non-zero; pembatalan eksplisit pengguna menghasilkan exit 0.
- **FR-014**: Tidak dibutuhkan koneksi internet untuk start/stop.

## Key Entities

- **Work session**: Rentang pekerjaan dengan satu tiket, judul, konteks repository opsional, dan status `ACTIVE` atau `COMPLETED`.
- **Ticket**: Identitas lokal fitur 02 yang digunakan bersama oleh seluruh session untuk Jira key yang sama.
- **Durasi**: Detik penuh yang diturunkan dari timestamp mulai dan selesai, bukan estimasi.

## Success Criteria

- **SC-001**: Developer dapat memulai dan menghentikan pekerjaan masing-masing dengan satu command.
- **SC-002**: Tidak pernah tersimpan lebih dari satu session aktif, termasuk pada dua start yang bersamaan.
- **SC-003**: Durasi semua session selesai sama dengan selisih timestamp yang disimpan dalam detik penuh.
- **SC-004**: Pembatalan, konflik konfirmasi, dan kegagalan pergantian tidak mengubah session sebelumnya atau meninggalkan tiket baru yang tidak digunakan.
- **SC-005**: Operasi lokal start/stop mencapai target PRD di bawah 200 ms pada database lokal biasa, tidak termasuk waktu pengguna menjawab prompt atau menunggu lock proses lain.
- **SC-006**: Start/stop dapat digunakan di luar repository dan tanpa jaringan.

## Assumptions and Source Decisions

- Fitur ini menyediakan start/stop dengan waktu aktual. `--since`/`-s`, pembuatan rentang manual, dan koreksi waktu dari BRD/PRD bagian recovery dijadwalkan sebagai fitur pemulihan waktu.
- Judul session berbeda dari judul master tiket. Start hanya memastikan tiket ada, sehingga tidak menimpa judul master dengan judul kegiatan.
- Repository dicatat sebagai path root Git lokal; field kosong diperbolehkan.
- PRD AC-03 menyebut durasi `> 0`; untuk session yang selesai dalam kurang dari satu detik, spesifikasi ini memakai `0` agar tidak mengarang tambahan waktu.
- Default prompt `[Y/n]` berarti Enter mengonfirmasi hanya pada terminal interaktif. EOF/noninteraktif tidak dianggap persetujuan.

## Out of Scope

Note (fitur 04), capture commit, dashboard root, timeline harian, koreksi waktu, pengaitan activity lama, dan AI summary dikerjakan pada fitur masing-masing.
