# Feature Specification: Git Commit Capture

**Feature**: `05-git-commit-capture`  
**Status**: Implemented  
**Sumber**: `goal.md`; BRD bagian 4.2, 5, 9–12, 23–24; PRD bagian 14–19, 32–34, 39; ERD work_activities

## Ringkasan

Developer dapat menjalankan `wl git` untuk mencatat commit terbaru di repository aktif sebagai evidence pekerjaan. Catatan mencakup repository, branch bila tersedia, commit hash, message, daftar file berubah, statistik perubahan, waktu, dan tiket hasil deteksi. Commit tidak hilang ketika session belum dimulai atau tiketnya berbeda dari session aktif. Capture tidak membuat durasi pekerjaan dan tidak membuat commit Git gagal.

## User Scenarios and Testing

### P1 — Capture commit yang sesuai dengan session

Sebagai developer, saya ingin commit menjadi evidence pada tiket/session pekerjaan tanpa menyalin informasi Git secara manual.

**Independent test**: Mulai session OOT-3751, buat commit `OOT-3751 fix tax calculation` pada repository test, lalu jalankan `wl git`.

1. Given commit terbaru menyebut OOT-3751 dan session OOT-3751 aktif, when capture, then satu aktivitas GIT_COMMIT tersimpan pada tiket/session tersebut dengan hash dan metadata commit.
2. Given commit message memiliki subject dan body, then seluruh message disimpan; tiket pertama yang cocok di seluruh message menjadi tiket commit.
3. Given commit memiliki file berubah, then file dan statistik perubahan tersimpan sesuai konfigurasi capture, tanpa membaca perubahan yang belum committed sebagai evidence.
4. Given HEAD bergerak ketika metadata dibaca, then seluruh data commit tetap merujuk satu hash yang dipilih pada awal capture; informasi branch yang tidak lagi dapat dipastikan tidak dipakai sebagai fallback tiket.

### P1 — Capture tanpa session dan ticket mismatch

Sebagai developer, saya ingin evidence coding tetap tersimpan ketika lupa memulai session atau bekerja pada tiket lain.

**Independent test**: Capture commit OOT-3751 tanpa session, lalu capture commit baru OOT-3751 ketika session OOT-3668 aktif.

1. Given commit memiliki tiket dan tidak ada session aktif, then aktivitas tersimpan pada tiket tersebut tanpa session dan diberi penanda konseptual UNSESSIONED.
2. Given commit menyebut OOT-3751 dan session OOT-3668 aktif, then aktivitas tetap tersimpan pada OOT-3751 tanpa session; tampilkan warning ticket mismatch.
3. Given capture tanpa session atau mismatch, then tidak ada session baru, penghentian session lama, atau perubahan durasi otomatis.
4. Given commit tidak menyebut tiket, then gunakan tiket session aktif; jika tidak ada session aktif, gunakan tiket pertama pada branch; jika tidak ditemukan juga, simpan sebagai UNASSIGNED tanpa tiket/session.

### P1 — Capture ulang aman

Sebagai developer, saya ingin command atau hook yang berjalan ulang tidak menggandakan evidence.

**Independent test**: Jalankan `wl git` dua kali pada commit yang sama, termasuk dari subdirektori repository.

1. Given pasangan repository dan commit hash sudah tersimpan, then tampilkan `✓ Commit already captured.` dengan exit 0 tanpa row baru atau perubahan atribusi.
2. Given dua capture commit yang sama berjalan bersamaan, then tepat satu aktivitas baru tersimpan; proses lainnya mendapat hasil already captured.
3. Given session atau branch berubah setelah capture pertama, then capture ulang tidak memindahkan commit ke tiket/session baru.
4. Given hash sama berada di dua checkout dengan root path berbeda, then masing-masing repository boleh memiliki satu capture.

### P2 — Metadata parsial dan kondisi Git khusus

Sebagai developer, saya ingin capture tetap berguna ketika sebagian informasi tambahan tidak tersedia.

1. Given detached HEAD, then capture tetap berhasil dengan branch kosong; deteksi tiket dari message/session tetap berlaku.
2. Given root commit, merge commit, empty commit, binary file, atau rename, then capture menghasilkan metadata yang konsisten dengan kontrak perbandingan commit tanpa menggandakan perubahan merge.
3. Given sebagian metadata tambahan gagal dibaca, then simpan identitas commit dan data yang tersedia dengan warning; nilai yang tidak diketahui tidak dinyatakan sebagai hasil nol yang terukur.
4. Given tidak berada di repository, Git tidak tersedia, repository bare, atau belum ada commit, then command gagal non-zero tanpa activity/tiket baru.
5. Given storage gagal sebelum capture berhasil, then tidak ada data parsial atau tiket baru yang tertinggal; commit Git yang sudah ada tidak diubah.

## Edge Cases

- Prioritas tiket: message → session aktif → branch → UNASSIGNED. Message yang menyebut tiket lain selalu mengalahkan session/branch.
- Session aktif berlaku global per database; kesesuaian tiket menentukan attachment, tanpa syarat root repository session harus sama.
- Session dipilih secara atomik saat penyimpanan. Bila session berubah selama metadata Git dikumpulkan, gunakan session aktif saat penyimpanan; tidak memakai snapshot lama dari awal command.
- Capture adalah pencatatan HEAD terpilih, bukan impor history. Commit timestamp dapat berada sebelum session aktif; attachment mengikuti tiket/session saat capture dan tidak menjadi klaim durasi historis.
- Commit timestamp menjadi waktu aktivitas; waktu capture disimpan terpisah sebagai provenance. Jika timestamp tidak tersedia, gunakan waktu capture dan tandai fallback.
- File binary tidak memberi jumlah baris teks; statistik numerik hanya menjumlahkan data teks yang tersedia. Rename dihitung sebagai penghapusan path lama dan penambahan path baru pada versi ini.
- Nama file dengan spasi, tab, newline, atau Unicode tetap dipertahankan tanpa pemotongan berdasarkan baris.
- Isi `.env` dan keluarga `.env*` tidak dibaca/dikumpulkan. Nama file boleh tercatat; statistik isi file tersebut dikecualikan dan scope statistik ditandai.
- Full diff tidak dikumpulkan pada fitur ini. Setting `capture_full_diff=true` menghasilkan warning bahwa pengumpulan tersebut belum tersedia; capture metadata tetap berjalan.

## Functional Requirements

- **FR-001**: Sediakan `wl git` tanpa argumen posisi, tanpa prompt, dan tanpa pemasangan hook otomatis.
- **FR-002**: Capture satu commit yang merupakan HEAD pada awal pembacaan; semua metadata commit menggunakan identitas yang sama.
- **FR-003**: Simpan aktivitas bertipe GIT_COMMIT dengan repository, full commit hash, message bila tersedia, branch opsional, waktu, serta metadata file/statistik sesuai konfigurasi.
- **FR-004**: Gunakan `ticket.pattern` dari konfigurasi dan perilaku first-match fitur 02 untuk ekstraksi message/branch; prioritas deteksi mengikuti message → session → branch → UNASSIGNED.
- **FR-005**: Jika tiket hasil deteksi sama dengan tiket session aktif pada penyimpanan, hubungkan aktivitas dengan session tersebut. Jika berbeda, simpan pada tiket commit tanpa session dan tampilkan warning.
- **FR-006**: Tanpa session, commit bertiket tetap tersimpan sebagai UNSESSIONED; commit tanpa tiket tersimpan sebagai UNASSIGNED. Tidak membuat key tiket palsu bernama UNASSIGNED.
- **FR-007**: Pastikan tiket yang terdeteksi tersedia tanpa menimpa judul master; pembuatan tiket dan activity harus berhasil bersama atau tidak diterapkan.
- **FR-008**: Deduplikasi berlaku pada pasangan repository dan full commit hash, termasuk proses bersamaan. Capture ulang exit 0 tanpa update row/tiket atau atribusi ulang.
- **FR-009**: Capture tidak mengubah session, rentang waktu, durasi, repository Git, branch, index, atau isi commit.
- **FR-010**: Metadata tambahan yang tidak tersedia atau dinonaktifkan disimpan sebagai tidak diketahui/tidak dikumpulkan dengan status yang jelas; capture tetap berhasil bila repository dan commit identity diketahui.
- **FR-011**: Kegagalan identity Git, argumen, konfigurasi, pembatalan, atau storage menghasilkan exit non-zero tanpa data parsial. Warning mismatch/unassigned/metadata parsial setelah penyimpanan berhasil tetap exit 0.
- **FR-012**: `git.capture_changed_files` dan `git.capture_diff_stat` menentukan pengumpulan file/statistik. Full diff/isi source code tidak dikumpulkan; tidak membaca isi `.env*` dan tidak memanggil AI atau layanan jaringan.
- **FR-013**: Bantuan `wl git --help`, help umum, serta version tersedia tanpa mengakses Git/config/storage.
- **FR-014**: Failure capture tidak menjalankan operasi yang membatalkan commit Git. Penanganan exit code pada post-commit hook merupakan kontrak fitur 06.

## Key Entities

- **Git commit evidence**: Aktivitas dengan identitas repository+hash, message, waktu commit, branch capture-time, serta metadata perubahan opsional.
- **Ticket attribution**: Tiket dan sumber deteksinya: MESSAGE, ACTIVE_SESSION, BRANCH, atau UNASSIGNED.
- **Session attachment**: Relasi opsional dengan session aktif yang memiliki tiket sama saat capture disimpan.
- **Capture outcome**: Captured atau already captured, dengan peringatan bila evidence baru memiliki metadata parsial, mismatch, atau tidak bertiket.

## Success Criteria

- **SC-001**: Satu command menghasilkan satu evidence commit yang dapat dipakai fitur timeline/rangkuman selanjutnya.
- **SC-002**: Seluruh kasus matching, no-session, mismatch, fallback, dan unassigned menghasilkan atribusi sesuai prioritas yang ditentukan.
- **SC-003**: Capture berulang/bersamaan menghasilkan maksimal satu evidence per pasangan repository+hash dan tidak mengubah capture pertama.
- **SC-004**: Evidence tetap tersimpan tanpa session dan tidak menambah atau mengubah durasi pekerjaan.
- **SC-005**: Capture lokal biasa selesai dalam kurang dari 500 ms setelah inisialisasi, tanpa lock contention, sesuai PRD bagian 34; ukur hasil implementasi sebelum menyatakan target tercapai.
- **SC-006**: Capture bekerja tanpa jaringan, tidak mengubah Git commit, dan tidak mengumpulkan isi `.env*` atau source diff.

## Dependencies and Source Decisions

- Fitur 01–04 menyediakan konfigurasi, schema, ticket pattern, session aktif global, dan activity storage; fitur 06 memasang hook.
- PRD bagian 19 menyebut hash unique global, tetapi ERD dan migration 001 memakai repository+hash. Spesifikasi memilih skema existing agar checkout berbeda tidak saling menghilangkan evidence; tidak diperlukan migrasi.
- Repository adalah canonical absolute worktree root; subdirektori/symlink mengarah pada identitas root yang sama. Checkout/worktree dengan root berbeda dianggap repository capture berbeda.
- Root commit dibandingkan dengan tree kosong; commit lain termasuk merge dibandingkan dengan parent pertama. Rename tidak dikenali sebagai jenis terpisah.
- Timestamp memakai committer time, bukan author time; branch merepresentasikan konteks capture, bukan properti immutable commit.
- Kegagalan message berarti ekstraksi sumber tersebut tidak tersedia; gunakan prioritas sumber tersedia berikutnya dan warning. Detached HEAD bukan error metadata branch.
- CLI dapat gagal setelah write committed karena error output; rerun tetap aman berkat deduplikasi. Tidak ada retry yang memodifikasi capture lama.

## Out of Scope

Install/uninstall hook, history import atau commit selector, remote fetch/push, Jira API, full diff, edit/relink activity, time recovery, dashboard/timeline, AI classification/summary, dan deduplikasi lintas checkout.
