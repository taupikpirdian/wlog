# Implementation Plan: Git Hook Installation

**Feature**: `06-git-hook-installation`  
**Status**: Implemented and verified

## Technical Context

| Area | Rancangan |
| --- | --- |
| CLI | Cobra install-hooks, alias install-hook, NoArgs, lazy factory |
| Domain | domain/hook: install state, snapshot validation, marker/template policy, outcomes/errors |
| Application | application/hook: repository inspection, plan/recheck/apply melalui port, TDD |
| Git/filesystem | infrastructure/githook: location adapter, lock, snapshot, copy/temp/publish/cleanup |
| Delivery | delivery/cli/hooks.go: args, outcome, paths/scope/error messages |
| Wiring | cmd/wlog: cwd, repository inspector dan hook store; tidak menginisialisasi config/SQLite |
| Platform | macOS/Linux, POSIX /bin/sh wrapper, wl dari runtime PATH |
| Test gate | 100% statement coverage application/domain, race detector, filesystem dan real Git fixtures |

## Project Workflow and Constitution Check

Repo tidak memiliki .specify/template/constitution/extensions atau setup-plan.sh. Artefak mengikuti layout docs/speckit sebelumnya tanpa perpindahan branch/tooling initialization.

- [x] Domain tidak mengakses os/exec, filesystem, SQL, atau Cobra.
- [x] Application bergantung pada repository/hook-store port; tidak membuat adapter konkret.
- [x] File operations, Git command runner, mode/lock/snapshot/recheck berada di infrastructure.
- [x] Domain menentukan expected managed content dan install outcome; delivery memformat output.
- [x] TDD dan 100% statement coverage meliputi domain/application baru serta package existing.
- [x] Tidak ada internal, schema migration, config mutation, atau SQLite initialization.
- [x] Target hook shared linked worktree dinyatakan; custom hooksPath ditolak tanpa mutation.
- [x] Review setelah desain memenuhi preservation, idempotency, dan failure isolation.

## Repository Resolution

Inspector menggunakan exec.CommandContext dengan argument arrays/deadline, tanpa shell. Resolve root dan Git common directory. Baca effective core.hooksPath (semua config scopes), termasuk nilai kosong/disabled; bila set, hasil unsupported sebelum mutation. Tidak mengubah Git config.

Dari root, resolve `git rev-parse --path-format=absolute --git-path hooks/post-commit`; canonicalize/validate metadata directory. Jangan membentuk root/.git/hooks secara manual. Unborn HEAD tetap valid karena install tidak membaca commit. Bare/nonrepository ditolak.

Default hooks linked worktree berada pada shared common metadata; hasil inspector menyebut scope shared-worktrees bila target common hook area. Capture wrapper tidak memakai root yang dibekukan, sehingga wl git membaca cwd worktree actual yang disiapkan Git.

Symlink/nonregular target/backup/lock ditolak melalui lstat. Validasi hook directory/ancestors sebelum write; jangan mengikuti symlink yang mengalihkan write target. Git common-dir yang sah tetap merupakan lokasi metadata target, termasuk bila berada di luar root worktree.

## Managed Wrapper and Original Preservation

Domain membentuk POSIX wrapper version 1 dengan exact ownership marker, flag original-was-executable, dan SHA-256 digest bytes original bila ada. Marker saja tidak cukup untuk menganggap file milik wl; validasi exact template+original state.

New hook tanpa original menjalankan capture dan exit 0. Existing hook: backup sibling `post-commit.wlog-original`, byte-identical, mode original. Wrapper menjalankan backup sebagai proses terpisah hanya bila semula executable, forward args dan stdin/cwd/environment, simpan status tanpa set -e, jalankan capture terlepas dari status original, lalu return status original. Jangan source backup atau menambahkan shell code ke body hook arbitrary.

Wrapper menentukan directory dirinya pada runtime; backup berada di directory yang sama. Semua path dikutip. Tidak menanam absolute repo path atau path wl. Tidak ada command substitution atas teks hook pengguna. Capture redirection berlaku hanya pada wl, bukan pada original process; tidak memakai background task.

Existing nonexecutable tetap dicopy tetapi flag tidak menjalankannya. New wrapper mode 0755; wrapper pengganti existing memakai permission rwx original ditambah owner-execute tanpa memperluas group/other permissions. Original backup mode sama. Valid managed wrapper nonexecutable diperbaiki mode tanpa rewrite bytes.

Keterbatasan chaining: backup dipanggil dengan argv0/$0 berupa nama backup. Jangan menjanjikan kompatibilitas universal bagi script yang bergantung pada basename. Hook-manager/custom-path integration berada di luar scope; existing integration output menyebut backup untuk peninjauan/manual integration.

## Atomic Publish and Concurrency

1. Inspect/validate scope dan target; tidak menjalankan original hook atau wl git selama install.
2. Acquire per-target reserved lock dengan exclusive creation; conflict/busy keluar tanpa mutation. Stale lock tidak dihapus otomatis.
3. Baca ulang target/backup di bawah lock, kumpulkan file identity/digest/mode; domain menentukan New, PreserveExisting, AlreadyInstalled, RepairMode, atau Conflict.
4. Untuk perubahan content, buat wrapper temporary pada directory target; write penuh, sync, mode tepat, close. Untuk original, copy ke temporary dan verify bytes/mode, lalu publish backup secara no-clobber (misalnya hard-link temp ke nama reserved, kemudian unlink temp).
5. Recheck target file identity, content digest, mode, serta lokasi terhadap snapshot sebelum publish. Perubahan terdeteksi → conflict tanpa overwrite target.
6. Publish wrapper dengan rename atomic dari same-directory temp ke post-commit. Original target tetap ada hingga rename; jangan memindahkan original terlebih dahulu yang membuat gap tanpa hook.
7. Setelah publish berhasil, kembalikan Installed; commit point content adalah rename. Untuk RepairMode, commit point adalah perubahan mode yang berhasil; error sesudahnya juga dilaporkan sebagai post-change failure. Directory sync dapat dipakai untuk durability; failure sesudah commit point jangan mengklaim target tetap unchanged.
8. Cleanup temp/lock milik operasi. Jika failure sebelum rename, hapus backup yang baru dibuat hanya jika identity/digest masih milik operasi; bila berubah/cleanup gagal, pertahankan dan laporkan path recovery. Tidak menghapus reserved file preexisting.

Atomic rename melindungi target dari partial content. Tidak ada atomic transaction untuk semua sibling files; crash sebelum publish dapat meninggalkan backup/lock tetapi original target tetap utuh. Crash sesudah publish menghasilkan managed wrapper+backup siap pakai, mungkin dengan lock yang perlu inspeksi manual.

Advisory lock mengoordinasikan installer wl; external editor yang tidak memakai lock tetap dapat berlomba setelah final check. Dokumentasikan batas ini; tidak ada force-overwrite otomatis. Tests membuktikan conflict untuk perubahan yang terdeteksi serta ownership-aware cleanup.

## Application Ports and Outcomes

Repository inspector mengembalikan root/common-dir/hook-path/scope. Hook store memberi operasi yang mengunci, membaca snapshot, memanggil pure domain planning, dan menerapkan plan dengan revalidation; tidak menahan lock saat Git subprocess berjalan.

Application mengoordinasikan inspector/store dan meneruskan cancellation. Hasil memuat status Installed/AlreadyInstalled/Repaired, repository, hook path, original backup optional, dan scope. Typed errors meliputi unsupported location, conflict, busy, malformed managed state, serta adapter errors dengan cause dipertahankan.

Tidak membuka storage worklog atau membaca config.yaml; post-commit akan memanggil fitur 05 lazily ketika commit benar-benar terjadi.

## TDD and Verification Gates

Ikuti RED → GREEN → REFACTOR untuk setiap business/application behavior sebelum implementation code ditulis. Test menggunakan fake inspector/store; constructor/orchestration, valid state, validation/error, cancellation, dependency failure, dan idempotency/error return semua tercakup. Jangan memakai integration coverage sebagai pengganti unit tests kedua layer.

Gate:

```sh
go test -race -coverprofile=coverage.out ./application/... ./domain/...
go tool cover -func=coverage.out
go test -race ./...
go vet ./...
go build -o ./wl ./cmd/wlog
```

Seluruh executable function/package domain/application harus 100% statement coverage. Package doc.go tanpa statements tidak memiliki target numerik. Fixture filesystem/real Git melengkapi unit tests untuk publish/mode/rollback dan runtime hook behavior; regression fitur 01–05 wajib lulus.

## Phase 0 / Phase 1 Artifacts

- [research.md](research.md): sumber resmi Git, keputusan scope/wrapper/atomicity.
- [data-model.md](data-model.md): filesystem entities, reserved paths, state transitions.
- [contracts/cli.md](contracts/cli.md): command/output/exit dan runtime contract.
- [quickstart.md](quickstart.md): validasi terisolasi, termasuk real commit saat capture gagal.
- [checklists/requirements.md](checklists/requirements.md): quality review.

## Implementation Sequence

1. RED domain state/template/preservation/idempotency tests; GREEN domain rules; REFACTOR.
2. RED application inspection/apply/outcome/error/cancellation tests; GREEN service/ports; REFACTOR.
3. Implement Git location resolver dan filesystem adapter dengan lock/temp/no-clobber backup/atomic publish.
4. Tambahkan command, alias, lazy factory dan main wiring tanpa config/database access.
5. Verifikasi executable wrapper, original exit/stdout/mode, missing/failing wl, shared worktrees, path quoting, rollback dan collision races.
6. Verifikasi 100% coverage, race/vet/build serta acceptance guide; perbarui README/status saat implementasi selesai.

## Risks

| Risiko | Mitigasi |
| --- | --- |
| Arbitrary hook syntax/exit | Process chaining, byte-identical backup; tidak append/source arbitrary program |
| Backup argv0 berbeda | Batas kompatibilitas eksplisit, custom manager/manual integration scope |
| Shared/custom hooks scope | Tolak core.hooksPath; report shared default linked worktree |
| Crash multi-file publish | Copy backup dahulu tanpa gap target, commit point rename, reserved-state conflict/recovery manual |
| External mutation | Snapshot/file identity/digest/mode recheck; tidak mengklaim lock universal |
| wl tidak ada pada GUI PATH | Runtime name lookup, fail-open capture, installer output menyebut PATH requirement |
| Destructive cleanup | Hapus hanya temporary/lock/backup yang dibuat operasi dan masih cocok ownership snapshot |
