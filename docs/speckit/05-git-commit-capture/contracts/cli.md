# CLI Contract: Git Commit Capture

## Command and Scope

```sh
wl git
```

Tanpa argumen posisi, alias tambahan, prompt, commit selector, atau history scan. Capture satu HEAD terpilih dari non-bare worktree aktif, termasuk ketika command dijalankan dari subdirektori. Jangan memasang atau mengubah hook; fitur 06 melakukan instalasi.

## Success: Matching Session

```text
✓ Commit captured under OOT-3751.
```

Exit 0. Activity terhubung dengan active session OOT-3751 pada saat persistence. Tiket ditentukan oleh prioritas message → active session → branch → unassigned.

## Success: No Session

```text
✓ Commit captured under OOT-3751 (UNSESSIONED).
```

Exit 0; ticket_id terisi, session_id NULL. Tidak ada waktu/durasi yang dihasilkan dari commit.

## Success with Mismatch Warning

Stdout:

```text
✓ Commit captured under OOT-3751 (UNSESSIONED).
```

Stderr setelah commit storage berhasil:

```text
⚠ Ticket mismatch

Active session : OOT-3668
Commit ticket  : OOT-3751

Commit saved under OOT-3751.
```

Exit 0. Warning tidak mengganti tiket atau menutup session aktif.

## Success: Unassigned

Stdout:

```text
✓ Commit captured (UNASSIGNED).
```

Stderr:

```text
⚠ No ticket detected. Commit saved without a ticket or session.
```

Exit 0, ticket_id/session_id NULL. UNASSIGNED bukan tiket master.

## Already Captured

```text
✓ Commit already captured.
```

Exit 0 tanpa mutation atau warning atribusi baru. Berlaku pada pair repository+hash. Capture lama tidak di-update ketika session/branch/config berubah.

## Partial Metadata

Capture dengan identity lengkap tetapi optional metadata unavailable tetap memakai output sukses sesuai atribusi. Tambahkan warning stderr yang menyebut field tidak tersedia; jangan cetak isi message/file/env atau stderr subprocess mentah sebagai error log.

Detached HEAD normal menghasilkan branch NULL tanpa warning. Capture-full-diff flag true memberi warning `Full diff capture is not available in this feature; commit metadata was saved.` Flag false tidak memberi warning. Disabled stats/files bukan error.

## Failure and Exit Status

| Kondisi | Exit | Perilaku |
| --- | --- | --- |
| Captured/AlreadyCaptured | 0 | Output sukses setelah hasil persistence |
| Mismatch/unassigned/optional metadata unavailable | 0 | Sukses dan warning stderr setelah storage commit |
| Argumen tambahan | Non-zero | Usage git; tanpa dependency access |
| Git tidak tersedia/capability offline tidak didukung | Non-zero | Pesan Git/capability actionable; tanpa write |
| Di luar worktree, bare, atau belum ada commit | Non-zero | Jelaskan kebutuhan repository dengan commit; tanpa activity/tiket |
| Root/hash gagal dibaca | Non-zero | Capture failed; tanpa identity palsu |
| Config/ticket pattern invalid | Non-zero | Pesan konfigurasi; tanpa activity baru |
| Context dibatalkan/storage read-lock-INSERT-COMMIT gagal | Non-zero | Capture failed; rollback yang belum committed; jangan cetak sukses |
| Output gagal setelah COMMIT | Non-zero | Row tetap tersimpan; retry aman melalui dedup |
| git --help/help git/help/version umum | 0 | Tanpa Git, config, atau storage |

Untuk kegagalan manual storage, pesan utama `⚠ Worklog capture failed.` diikuti alasan aman. Tidak ada command yang mengubah commit Git. Hook fitur 06 menangani non-zero agar proses git commit tidak gagal.

## Configuration and Privacy Contract

- capture_changed_files dan capture_diff_stat independen; NULL + status disabled/unavailable berbeda dari []/0 yang terukur.
- File/stat berasal dari commit terpilih vs empty tree/root atau parent pertama/non-root, termasuk merge.
- Rename tampil sebagai path lama dihapus dan path baru ditambah; binary tidak menambah total baris teks.
- Tidak membaca isi .env*, patch/source diff, working-tree changes, atau memanggil layanan jaringan/AI. Nama file .env* boleh tersimpan, scope statistik mengecualikan kontennya.
- Branch NULL pada detached/unstable HEAD; fallback branch tidak tersedia dalam keadaan tersebut.
- Tidak mengubah format/aturan command start/stop/note fitur sebelumnya.
