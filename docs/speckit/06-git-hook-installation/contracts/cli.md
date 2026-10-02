# CLI Contract: Git Hook Installation

## Command

```sh
wl install-hooks
wl install-hook
```

Main command plural; singular compatibility alias. NoArgs, tanpa prompt atau force/uninstall flags. Memasang post-commit saja pada default Git hook directory.

## New Install

```text
✓ Git integration installed

Repository: /path/to/project
Hook: /path/to/git/hooks/post-commit

Commits will automatically be captured when wl is available on the Git process PATH.
```

Exit 0. Path actual dapat berbeda dari root/.git karena linked worktree. Tidak membuat config/database worklog atau melakukan capture sekarang.

Jika original regular hook existing dipreserve, tambahkan:

```text
Original hook preserved: /path/to/git/hooks/post-commit.wlog-original
The original hook runs from this backup path; hooks that depend on their filename require manual integration.
```

Default linked worktree shared scope:

```text
Scope: all worktrees sharing this repository's default hooks directory.
```

## Rerun / Permission Repair

```text
✓ Git integration already installed
Hook: /path/to/git/hooks/post-commit
```

atau:

```text
✓ Git hook executable permission repaired
Hook: /path/to/git/hooks/post-commit
```

Exit 0. Idempotency memerlukan complete/exact managed wrapper dan original backup bila ada; tidak sekadar mendeteksi substring wl git.

## Errors and Exit Status

| Kondisi | Exit | Perilaku |
| --- | --- | --- |
| Installed/AlreadyInstalled/Repaired | 0 | Tampilkan outcome dan lokasi |
| Argumen tambahan | Non-zero | Error + usage, tanpa dependency access |
| Git unavailable/nonrepository/bare | Non-zero | Kebutuhan local worktree, tanpa mutation |
| core.hooksPath set/disabled | Non-zero | Custom integration unsupported; sarankan menambahkan safe capture command lewat mekanisme pengguna, tanpa config change |
| Target/backup/lock symlink atau nonregular | Non-zero | Unsafe/unsupported file type, tanpa mengikuti link |
| Reserved backup atau malformed/modified managed state | Non-zero | Conflict dan path yang perlu ditinjau; jangan overwrite |
| Installer lock existing | Non-zero | Busy, path lock, dan saran retry/inspect; tidak auto-delete lock |
| Snapshot berubah sebelum publish | Non-zero | Conflict, original terbaru tidak ditimpa bila perubahan terdeteksi |
| Write/mode/verify/publish gagal pre-publish | Non-zero | Target original utuh; cleanup file milik invocation atau report recovery path |
| Sync/cleanup/output gagal post-publish | Non-zero | Jelaskan integration dapat sudah terpasang; rerun untuk verifikasi |
| Cancellation | Non-zero | Jangan publish sesudah cancellation terdeteksi; cleanup sesuai ownership |
| Help/version termasuk help install-hooks | 0 | Tanpa Git, config, storage, atau hook filesystem |

Error tidak mencetak raw isi hook. Tidak ada install partial yang dilaporkan sukses.

## Runtime Contract

Core capture command wrapper:

```sh
wl git >/dev/null 2>&1 || true
```

- Existing supported executable hook berjalan dahulu sebagai proses terpisah; stdin/args/cwd/env diteruskan. Hook semula nonexec tidak diaktifkan.
- Wrapper mempertahankan original exit status/output; capture exit/output tidak mengubahnya. Tanpa original executable, status wrapper 0.
- Missing wl, storage error, atau metadata capture error ditahan. Tidak ada prompt atau background task.
- wl diambil dari runtime PATH; wrapper tidak mengubah PATH atau membekukan repo/executable path.
- Hook dijalankan Git setelah commit; install tidak menjalankan hook. Capture dedup/error/attribution tetap kontrak fitur 05.

Chaining backup mengubah argv0/$0, sehingga arbitrary filename-sensitive hook/manager tidak dijamin kompatibel. Custom hooksPath ditolak; pengguna mengintegrasikan command safe di atas melalui manager/custom hook miliknya.
