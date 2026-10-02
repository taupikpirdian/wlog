# Feature Specification: Git Hook Installation

**Feature**: `06-git-hook-installation`  
**Status**: Implemented and verified  
**Sumber**: goal.md; BRD bagian 9, 20, 24; PRD bagian 13, 31–34; fitur 05 Git Commit Capture

## Ringkasan

Developer dapat menjalankan `wl install-hooks` untuk mengaktifkan capture commit otomatis pada repository lokal. Integrasi post-commit menjalankan `wl git` sesudah commit. Kegagalan worklog tidak mengubah hasil commit, dan hook yang sudah dimiliki pengguna tidak dihapus atau ditimpa tanpa salinan utuh. Instalasi ulang tidak menggandakan integrasi.

## User Scenarios and Testing

### P1 — Mengaktifkan capture otomatis

Sebagai developer, saya ingin memasang integrasi satu kali agar commit berikutnya tercatat tanpa menjalankan `wl git` manual.

**Independent test**: Pada repository test tanpa post-commit, jalankan install-hooks lalu buat commit bertiket; pastikan hook executable dan tepat satu activity tercatat.

1. Given worktree Git lokal dan tidak ada post-commit, when `wl install-hooks`, then hook terpasang pada lokasi default efektif Git, executable, dan output menyebut repository serta path hook.
2. Given command dijalankan dari subdirektori, then target sama dengan instalasi dari root repository.
3. Given repository belum memiliki commit, then instalasi tetap berhasil; integrasi dijalankan pada commit pertama.
4. Given linked worktree memakai lokasi hook default bersama, then instalasi berlaku pada seluruh worktree repository tersebut; command menampilkan scope tersebut. Saat hook berjalan, capture mengikuti worktree tempat commit dilakukan.

### P1 — Mempertahankan hook yang sudah ada

Sebagai developer, saya ingin aktivitas post-commit yang sudah ada tetap berjalan setelah memasang worklog.

**Independent test**: Siapkan hook existing yang menulis sentinel lalu exit; install integrasi dan commit. Pastikan sentinel tetap dibuat, bytes/permission original disimpan, dan wl capture tetap dijalankan.

1. Given regular executable post-commit existing, then simpan salinan original utuh tanpa overwrite backup, dan pasang wrapper yang menjalankan original sebagai proses terpisah sebelum wl git.
2. Given original mengembalikan non-zero atau memanggil exit, then wl git tetap dicoba; hasil wrapper mempertahankan status original, tanpa dipengaruhi kegagalan capture.
3. Given original tidak executable, then original tetap disimpan dengan permission yang sama dan tidak dijalankan otomatis; wrapper baru menjalankan capture saja.
4. Given hook adalah symlink/nonregular, backup reserved sudah ada, atau file managed berubah/tidak lengkap, then gagal tanpa menimpa file pengguna dan beri petunjuk penyelesaian manual.
5. Given hook bergantung pada basename/$0 original atau dikelola sistem hook lain, then gunakan integrasi manual; chaining backup tidak menjanjikan kompatibilitas universal. Instalasi dengan hook existing menampilkan path original agar perubahan cara pemanggilan dapat ditinjau.

### P1 — Instalasi ulang dan kegagalan aman

Sebagai developer, saya ingin command dapat diulang tanpa duplikasi atau kehilangan hook lama.

**Independent test**: Install dua kali; pastikan satu managed integration. Simulasikan kegagalan publish dan pastikan post-commit existing tetap utuh.

1. Given managed wrapper lengkap dan sesuai versi/template, then instalasi ulang exit 0 sebagai already installed; tidak menambah blok, backup, atau hook lain.
2. Given wrapper managed valid tetapi permission executable hilang, then perbaiki permission saja dan laporkan repaired.
3. Given dua installer berjalan bersamaan, then maksimal satu publish; proses lain mendapat busy/retry atau already installed tanpa duplicate backup.
4. Given read/write/permission/verification/publish gagal sebelum sukses, then original target tetap utuh, tidak ada wrapper parsial, dan perubahan milik operasi yang gagal dibersihkan bila aman.
5. Given proses berhenti setelah backup dibuat tetapi sebelum wrapper dipublish, then original masih menjadi post-commit; percobaan berikutnya melaporkan reserved-backup conflict untuk recovery manual, bukan overwrite otomatis.

### P1 — Capture gagal tanpa mengganggu commit

Sebagai developer, saya ingin Git commit tetap berhasil walau wl tidak tersedia atau penyimpanan worklog bermasalah.

1. Given wl tersedia pada PATH Git process, then wrapper menjalankan `wl git` tepat sekali per invocation, tanpa input interaktif.
2. Given wl tidak tersedia, capture error/non-zero, atau database tidak dapat dipakai, then hasil original hook tetap dipertahankan dan commit tidak dibatalkan karena capture.
3. Given capture mengeluarkan warning/output, then stdout/stderr capture dibuang; output original tidak dibuang.
4. Given capture dipanggil ulang untuk commit sama, then deduplikasi fitur 05 tetap berlaku; installer tidak menambah aturan dedup baru.

## Edge Cases

- core.hooksPath disetel, termasuk local relative/custom/shared path atau /dev/null: installer tidak mengubah konfigurasi atau directory tersebut; gagal dengan penjelasan integrasi manual.
- Bare repository/nonrepository/Git unavailable: installer gagal tanpa file atau database baru.
- PATH saat commit berbeda dari terminal instalasi: wrapper memakai nama wl, bukan absolute executable yang dibekukan; capture mungkin terlewat tetapi commit tetap aman. Output instalasi menyebut kebutuhan wl pada PATH Git process.
- Hook original dapat mengubah environment/cwd atau memakai set -e: eksekusi sebagai proses terpisah menjaga kondisi wrapper. Stdin/args/cwd/environment diteruskan; tidak menjamin stdin tetap belum dikonsumsi setelah original berjalan.
- Stale installer lock tidak dihapus otomatis; tampilkan path lock untuk pemeriksaan manual. Tidak ada force/uninstall command pada fitur ini.
- File diubah tool lain saat instalasi: verifikasi ulang sebelum publish dan fail bila perubahan terdeteksi. Lock hanya mengoordinasikan installer wl, bukan semua editor/proses eksternal.
- Error output setelah publish tidak membatalkan integrasi yang sudah terpasang; rerun dapat memverifikasi status installed.

## Functional Requirements

- **FR-001**: Sediakan `wl install-hooks`, alias `install-hook`, tanpa argumen posisi/prompt. Keduanya memasang hanya integrasi post-commit.
- **FR-002**: Validasi worktree Git non-bare dan resolve default effective hook path melalui Git, termasuk subdirektori dan linked worktree; tidak mengasumsikan .git selalu directory.
- **FR-003**: core.hooksPath yang disetel ditangani sebagai unsupported custom integration tanpa perubahan file/config. Lokasi default bersama linked worktree didukung dengan scope yang ditampilkan.
- **FR-004**: Hook menjalankan `wl git >/dev/null 2>&1 || true`; capture tidak mengubah status original hook atau hasil commit.
- **FR-005**: Preserve original regular hook bytes dan permission dalam backup reserved; original executable dijalankan sebagai proses, original nonexecutable tidak diaktifkan.
- **FR-006**: Wrapper executable tidak menggandakan capture, mengubah working files/refs/index, atau menjalankan capture pada saat instalasi.
- **FR-007**: Instalasi ulang valid bersifat idempotent; permission repair diperbolehkan tanpa rewriting bytes atau backup.
- **FR-008**: Tolak symlink/nonregular target, reserved-file collision, malformed/modified managed integration, dan scope custom path sebelum mutation.
- **FR-009**: Publish target secara utuh setelah verifikasi snapshot existing; koordinasikan concurrent installer dan cleanup file sementara milik operasi tersebut. Jangan menghapus backup/file pihak lain ketika conflict.
- **FR-010**: New/already-installed/repaired exit 0; invalid argument/repository, unsupported scope, busy/conflict, cancellation, dan filesystem/Git errors exit non-zero dengan pesan sesuai.
- **FR-011**: Help/version dan argument validation tidak mengakses Git/filesystem/config/storage. Install tidak memerlukan atau membuat SQLite/config worklog.
- **FR-012**: Eksekusi capture memakai wl dari PATH Git process, tidak bergantung pada path checkout tempat installer dijalankan; linked worktree capture mengikuti cwd Git hook.
- **FR-013**: Implementasi domain/application memakai TDD, fake dependency, dan 100% statement coverage pada seluruh package executable, meneruskan persyaratan pengguna dan goal.

## Key Entities

- **Repository hook location**: Worktree root, Git common directory, default post-commit path, dan scope single/shared-worktrees.
- **Managed integration**: Wrapper post-commit dengan marker/version dan template yang dapat diverifikasi.
- **Original hook backup**: Salinan hook existing, bytes/permission original, dan digest untuk ownership/integrity verification.
- **Install outcome**: Installed, already installed, repaired, atau failed tanpa publish parsial.

## Success Criteria

- **SC-001**: Developer mengaktifkan capture commit otomatis melalui satu command tanpa konfigurasi database tambahan.
- **SC-002**: Instalasi ulang menghasilkan satu integrasi wl per invocation hook, tanpa duplicate wrapper/backup.
- **SC-003**: Original hook yang didukung tetap menjalankan perilakunya dan tersedia sebagai salinan utuh setelah instalasi.
- **SC-004**: Semua skenario capture missing/error/non-zero tidak mengubah hasil commit akibat worklog; original hook output/status tetap terjaga.
- **SC-005**: Semua skenario instalasi gagal sebelum publish mempertahankan post-commit existing dan tidak menghasilkan file target parsial.
- **SC-006**: Domain/application unit tests mencapai 100% statement coverage dan regression fitur 01–05 tetap lulus.

## Dependencies and Source Decisions

- wl git fitur 05 sudah tersedia; scope instalasi berbeda dari metadata capture. Tidak ada schema migration.
- Goal memakai plural install-hooks, BRD/PRD memakai singular install-hook; alias menyediakan kompatibilitas keduanya.
- PRD meminta append aman pada hook existing. Desain memilih chaining wrapper+backup agar interpreter arbitrary, exit, dan set -e tidak membuat append rusak/tidak terjangkau. Tidak menjanjikan identitas argv0/$0 original tetap sama.
- Backup reserved dan wrapper hanya dimiliki integrasi wl. Existing user file di reserved path adalah conflict, bukan izin overwrite.
- Capture failure ditahan wrapper; original hook failure tidak ditelan. Git post-commit sendiri tidak menentukan keberhasilan commit yang sudah selesai.
- Tidak ada jaminan capture tersedia dari GUI yang tidak memiliki wl pada PATH; failure tetap tidak merusak commit.

## Out of Scope

Uninstall/force/automatic recovery stale lock, custom core.hooksPath/hook-manager integration, bare repositories, pre-commit/review hooks, Windows shell support, pengaturan PATH otomatis, background jobs, log capture failure persisten, schema/data migration, dan perubahan perilaku capture fitur 05.
