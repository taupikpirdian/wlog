# Research and Decisions: Git Hook Installation

**Status**: Complete  
**Basis**: goal.md, BRD/PRD, fitur 05, arsitektur existing, dan dokumentasi resmi Git. Dokumen merupakan desain; tidak mengklaim implementasi hook/tests sudah selesai.

## 1. Command dan batas fitur

**Decision**: Nama utama install-hooks dari goal; alias install-hook mengikuti BRD/PRD. Tanpa argumen/prompt, hanya post-commit. Instalasi tidak menjalankan wl git atau membuka config/SQLite.

**Rationale**: Mempertahankan kedua bentuk sumber tanpa membuat command multi-hook yang belum diminta. Pemasangan file dan capture evidence merupakan usecase terpisah.

**Alternatives considered**: Singular saja, pemasangan otomatis saat startup, atau pre-commit sekaligus. Tidak dipilih karena goal/batas scope.

## 2. Lokasi efektif Git

**Decision**: Resolve dari worktree root melalui `rev-parse --path-format=absolute --git-path hooks/post-commit`, bersama common-dir inspection. Jangan mengasumsikan .git directory. Default linked worktree hooks berada dalam metadata bersama. [Git repository layout](https://git-scm.com/docs/gitrepository-layout#Documentation/gitrepository-layout.txt-hooks), [Git rev-parse](https://git-scm.com/docs/git-rev-parse)

**Rationale**: Linked worktree .git dapat berupa file; instalasi perlu target yang benar dan scope yang dapat dipahami pengguna.

**Alternatives considered**: Join root/.git/hooks, mengubah config agar hook berada di worktree, atau hardcode path absolut pada wrapper. Tidak dipilih.

## 3. Custom/shared hook path

**Decision**: Jika core.hooksPath disetel pada scope mana pun, termasuk /dev/null, gagal unsupported sebelum mutation. Default shared linked worktree tetap didukung dan scope ditampilkan.

**Rationale**: core.hooksPath dapat menunjuk path relatif/absolut atau lokasi bersama banyak repository; /dev/null menonaktifkan hooks. Instalasi pada lokasi tersebut dapat melampaui repository yang diminta atau mengganggu manager. [Git config: core.hooksPath](https://git-scm.com/docs/git-config#Documentation/git-config.txt-corehooksPath)

**Alternatives considered**: Mengikuti seluruh custom path otomatis, override core.hooksPath lokal, atau flag force. Tidak diperlukan goal; integrasi manual mempertahankan konfigurasi pengguna.

**Source check**: Git path resolver memperhitungkan hooksPath/common-dir; perilaku tersebut juga terlihat pada [source resmi Git path.c](https://github.com/git/git/blob/v2.43.0/path.c#L383-L399). Penolakan custom path adalah keputusan scope produk, bukan keterbatasan Git.

## 4. Runtime hook dan failure isolation

**Decision**: POSIX wrapper menjalankan original supported hook sebagai proses, lalu `wl git >/dev/null 2>&1 || true`. Return status original, atau 0 tanpa original. wl berasal dari PATH runtime; tidak ada path wl/repository yang dibekukan.

**Rationale**: Post-commit tidak menerima argumen, berjalan setelah commit, dan tidak menentukan outcome commit; executable hook serta cwd worktree disiapkan oleh Git. [Git hooks](https://git-scm.com/docs/githooks)

**Alternatives considered**: Background capture, source original, set -e, mengembalikan status wl, atau menelan error original. Tidak dipilih agar original semantics dan failure isolation jelas. Wrapper tetap forward "$@" untuk manual invocation/testing.

## 5. Existing arbitrary hook

**Decision**: Copy original regular file byte-identical dan permission-identical ke reserved sibling, lalu pasang wrapper. Hook semula nonexecutable tidak diaktifkan. Jangan append shell ke interpreter arbitrary.

**Rationale**: PRD meminta append aman; append setelah exit/unreachable body tidak menjamin capture, dan hook Python/binary tidak menerima shell text. Process chaining menjalankan original exit/non-zero tanpa memutus wrapper.

**Alternatives considered**: Selalu overwrite, append berdasarkan shebang, source backup, atau integrasi sebelum original body. Dapat merusak program existing atau mengubah scope shell/env.

**Compatibility limit**: Pemanggilan backup mengubah argv0/$0. Script bergantung pada basename atau manager yang merawat target memerlukan integrasi manual. Output existing-hook install menyebut backup; tidak menjanjikan universal compatibility.

## 6. Ownership dan idempotency

**Decision**: Versioned exact wrapper marker/template menyertakan original-presence/executable flag dan digest. Marker saja tidak membuktikan ownership. Valid wrapper+backup → AlreadyInstalled; valid wrapper tanpa execute → RepairMode. Modified/malformed wrapper, missing/modified backup, symlink/nonregular, atau reserved collision → conflict.

**Rationale**: Rerun tidak boleh menggandakan integration, overwrite perubahan pengguna, atau menganggap file arbitrary milik installer.

**Alternatives considered**: Search substring wl git, marker-only trust, append setiap run, atau overwrite backup. Tidak melindungi preservation.

## 7. Atomic publish dan crash boundary

**Decision**: Per-target exclusive lock, same-directory temporary files, no-clobber backup publish, snapshot recheck, kemudian atomic rename wrapper sebagai commit point. Original target tidak dipindahkan sebelum wrapper tersedia. Cleanup hanya file milik operasi yang masih sesuai identity/digest.

**Rationale**: Menghindari partial target/gap hook tanpa menjanjikan filesystem transaction untuk semua file. Crash pre-publish dapat meninggalkan reserved backup/lock; command berikutnya conflict dan menawarkan recovery manual.

**Alternatives considered**: Truncate/write target langsung, rename original dahulu, unconditional backup overwrite, atau menghapus stale lock otomatis. Tidak dipilih karena risiko kehilangan original atau race.

**Concurrency limit**: Lock mengoordinasikan wl installer. Editor external tidak wajib mengikuti lock; perubahan yang terdeteksi ditolak, tetapi tidak ada jaminan compare-and-swap terhadap semua external writer.

## 8. TDD dan coverage

**Decision**: Implementasi domain/application memakai RED → GREEN → REFACTOR dan fake dependencies, dengan 100% statement coverage pada seluruh package executable. Coverage gate termasuk package existing, bukan hanya fitur hook.

**Rationale**: Goal menambahkan TDD secara eksplisit dan pengguna sebelumnya meminta seluruh application/domain tercover 100%. Fixture Git/filesystem melengkapi unit test untuk perilaku platform.

**Alternatives considered**: Test setelah implementasi, hanya happy-path, atau mengandalkan integration coverage. Tidak memenuhi requirement pengguna.

## 9. Security/scope operations

**Decision**: Tidak ada shell interpolation user data saat install, tidak mengeksekusi hook existing untuk inspeksi, tidak mengikuti symlink target, dan tidak mengganti config Git. Copy hanya file hook yang memang menjadi target preservation.

**Rationale**: Installer memerlukan filesystem changes terbatas pada default hook directory. Eksekusi hook baru terjadi saat Git menjalankannya setelah commit.

**Alternatives considered**: Menjalankan existing hook sebagai probe, membaca environment/source lain, atau chmod shared ancestors. Tidak diperlukan.
