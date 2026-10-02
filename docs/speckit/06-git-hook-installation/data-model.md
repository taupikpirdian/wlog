# Data Model: Git Hook Installation

## Repository Hook Location

| Field | Makna | Validation |
| --- | --- | --- |
| WorktreeRoot | Root canonical worktree untuk inspector/output | Non-bare Git worktree |
| CommonDirectory | Git shared metadata directory | Resolve oleh Git; jangan asumsi root/.git |
| HookPath | Default effective post-commit absolute path | Resolve dari root, bukan custom hooksPath |
| Scope | repository / shared-worktrees | Scope output, tidak menentukan capture repository runtime |
| CustomHooksPath | Effective config set/value | Set → unsupported; jangan mutation |

Repository boleh unborn. Installer tidak membutuhkan commit hash/ticket/session.

## Managed Files

| Path | Isi/ownership | Aturan |
| --- | --- | --- |
| post-commit | Wrapper versioned milik wl setelah publish | Exact template validation, executable |
| post-commit.wlog-original | Bytes original hook bila ada | No-clobber; mode original; tidak auto-rewrite |
| post-commit.wlog.lock | Exclusive lock milik invocation | Busy bila existing; no automatic stale deletion |
| Temporary siblings | Wrapper/original staging | Unique name, same directory; hanya invocation pemilik yang cleanup |

Tidak ada SQLite row, config.yaml mutation, manifest database, atau hook lain. New hook tanpa original tidak membuat original backup. Reserved-path collision walaupun target tidak ada tetap conflict.

## Hook Snapshot

Snapshot target/backup menyimpan:

- Presence dan regular-file type; lstat membedakan symlink.
- Bytes/size, SHA-256 digest, permission rwx, original executable flag.
- File identity yang tersedia pada platform untuk recheck/cleanup ownership.
- Managed marker/version dan expected template integrity.

Domain memakai value snapshot, bukan file handle atau os.FileMode dependency. Adapter mengonversi hasil filesystem ke value domain. Digest adalah integrity identifier, bukan enkripsi; jangan mencetak isi original hook pada output.

## Managed Header and Wrapper Policy

Version 1 memiliki exact marker serta metadata original-present, original-was-executable, dan digest original. Tidak menyimpan path absolute wl/worktree; original reserved basename tetap di directory wrapper.

Wrapper contract:

1. Initial original status 0.
2. Bila original semula executable, panggil backup sebagai process dengan args/stdin/cwd/env yang tersedia; simpan status, termasuk non-zero.
3. Selalu coba wl git dengan stdout/stderr capture dibuang dan error ditahan.
4. Exit status original, atau 0 tanpa executable original.

Wrapper tidak menggunakan source original, set -e, atau background capture. Original-nonexec tetap tidak dijalankan, walau permission backup kemudian berubah. Missing backup saat runtime tidak boleh membuat wl failure memblokir commit; installer mendeteksi kondisi tidak lengkap pada rerun dan tidak auto-repair content.

## Install States and Outcomes

| State inspected under lock | Plan/outcome | Mutation |
| --- | --- | --- |
| Target absent, reserved backup absent | New → Installed | Publish wrapper only |
| Target regular unmanaged, backup absent | PreserveExisting → Installed | Copy original + publish wrapper |
| Valid managed target/backup, executable | AlreadyInstalled | No content/mode change |
| Valid managed target/backup, wrapper nonexec | Repaired | Wrapper mode only |
| Custom path, symlink/nonregular, reserved collision | Failed | None |
| Managed marker/version/template/backup invalid | Failed | None |
| Lock existing | Busy | None |
| Snapshot changed before publish | Conflict | No overwrite target; ownership-aware cleanup |
| Crash before wrapper publish | Incomplete reserved state | Original target stays; recovery manual |

No automatic overwrite/upgrade/delete of modified managed files. Unsupported version produces conflict requiring review, not silent migration.

## Invariants

1. Installed integration attempts wl git once per invocation, after supported original.
2. Original bytes and permission preserved at install; original never deleted before publish.
3. Worklog exit/output does not replace original status/output.
4. Duplicate install does not add wrapper, block, or backup.
5. Post-commit publication is whole-file atomic; target never contains partial wrapper.
6. Backup is no-clobber and cleanup never deletes preexisting/user-modified files.
7. Custom hooksPath/config remains unchanged.
8. Installer does not execute hook/capture or create worklog data.
9. Wrapper capture context is runtime Git cwd, supporting all default linked worktrees.

## Commit Boundary

Atomic rename of prepared wrapper is content publish point. Before it, original target remains unchanged; backup/temp/lock may exist. After it, integration is installed even if directory sync, cleanup, or output later fails. Permission repair has its own commit point when the mode change succeeds. Rerun inspects actual files and reports installed, repair, or conflict; it does not blindly retry writes.
