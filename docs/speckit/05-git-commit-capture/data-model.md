# Data Model: Git Commit Capture

## Git Commit Activity

Gunakan work_activities existing dengan type GIT_COMMIT; tidak ada migration.

| Field | Nilai | Aturan |
| --- | --- | --- |
| id | Activity ID hasil INSERT | Unik per row |
| ticket_id | Tiket terdeteksi atau NULL | NULL untuk UNASSIGNED; tidak membuat tiket literal UNASSIGNED |
| session_id | Matching active session atau NULL | NULL untuk mismatch/no-session/unassigned |
| type | GIT_COMMIT | Bukan NOTE, UNSESSIONED, atau UNASSIGNED enum baru |
| description | NULL | Full message ada di commit_message; tidak merangkum otomatis |
| repository | Canonical absolute worktree root | Wajib pada capture; bagian identity |
| branch | Capture-time branch atau NULL | Detached HEAD/unavailable/HEAD berubah → NULL |
| commit_hash | Full opaque hash Git | Wajib; jangan mengunci model pada short hash/algoritma tertentu |
| commit_message | Full subject+body atau NULL | Empty message yang terukur boleh string kosong |
| changed_files | JSON array path, [] atau NULL | [] = terukur tanpa perubahan; NULL = disabled/unavailable |
| insertions | Total baris teks atau NULL | 0 = terukur tanpa baris masuk; NULL = disabled/unavailable |
| deletions | Total baris teks atau NULL | 0 = terukur tanpa baris keluar; NULL = disabled/unavailable |
| metadata | Versioned JSON | Provenance, attribution/enrichment status, warning codes |
| created_at | Committer time UTC RFC3339Nano | Fallback capture time bila timestamp unavailable, ditandai |

SQLite kolom stats nullable walau memiliki default 0; tulis NULL eksplisit untuk data yang tidak dikumpulkan. Path relatif terhadap root, case/whitespace tetap dipertahankan. Sorting lexicographic byte path + dedup path membuat array stabil. Daftar file dapat mencakup .env* sebagai nama; statistik kontennya tidak dihitung.

## Metadata v1

```json
{
  "version": 1,
  "source": "git-capture",
  "captured_at": "2026-10-01T03:16:00Z",
  "time_source": "commit",
  "ticket_source": "MESSAGE",
  "capture_status": {
    "message": "collected",
    "timestamp": "collected",
    "branch": "collected",
    "changed_files": "collected",
    "diff_stat": "collected"
  },
  "diff_basis": "first_parent",
  "rename_mode": "delete_add",
  "diff_stat_scope": "non_env_text_files",
  "binary_files": [],
  "warnings": []
}
```

- time_source: commit/capture_fallback; captured_at clock application selalu tersedia.
- ticket_source: MESSAGE/ACTIVE_SESSION/BRANCH/UNASSIGNED.
- Status: collected/disabled/unavailable; branch juga dapat detached atau unstable. Detached normal tidak menambah warning.
- diff_basis: empty_tree untuk root, first_parent untuk lainnya; not_collected bila kedua enrichment disabled dan unavailable bila parent context tidak dapat dibaca. Tidak menyimpan stats multi-parent terpisah.
- binary_files: path hasil numstat ketika stat collected; NULL bila statistik tidak tersedia/dinonaktifkan. Nilai [] berarti tidak ditemukan binary pada scope statistik.
- warnings: stable codes, misalnya ticket_mismatch, unassigned, message_unavailable, timestamp_unavailable, branch_unavailable, branch_unstable, changed_files_unavailable, diff_stat_unavailable, full_diff_unsupported. Metadata tidak menyimpan stderr subprocess mentah, environment, isi .env, atau patch.

Metadata mengenali capture source command; tanpa flag dari hook, tidak menyatakan source=post-commit. Fitur 06 hanya memanggil command ini.

## Attribution Matrix

| Message key | Active session | Branch key | Ticket source/tujuan | Session |
| --- | --- | --- | --- | --- |
| A | A | Apa pun | MESSAGE/A | Active A |
| A | B | Apa pun | MESSAGE/A + mismatch warning | NULL |
| A | Tidak ada | Apa pun | MESSAGE/A | NULL |
| Tidak ada | A | B/none | ACTIVE_SESSION/A | Active A |
| Tidak ada | Tidak ada | B | BRANCH/B | NULL |
| Tidak ada | Tidak ada | Tidak ada | UNASSIGNED/NULL | NULL |

Message unavailable mengikuti baris tanpa message key dengan warning. Deteksi branch hanya jika branch valid/stabil tersedia. Tiap sumber teks memilih first match sesuai pattern fitur 02.

## Invariants and Relationships

1. Repository dan commit_hash nonempty; pair identik maksimal satu row lewat idx_work_activities_repo_commit.
2. Jika session_id terisi, session ACTIVE pada write dan ticket_id sama dengan tiket session.
3. Jika ticket_id NULL, session_id juga NULL. Session NULL tidak berarti commit memiliki durasi nol; commit tidak punya durasi.
4. Tiket otomatis dibuat tanpa title bila belum ada; master title/updated_at existing tidak diubah.
5. Duplicate tidak memperbarui row, metadata, tiket, session, atau captured_at original.
6. New ticket+activity hanya ada setelah transaksi berhasil; rollback tidak meninggalkan tiket baru.
7. Optional enrichment unavailable berbeda dari hasil kosong/nol yang terukur.
8. Timestamp commit tidak memulai, menutup, atau mengoreksi session.

## Outcomes

- Captured: row baru, key/session optional, warnings dari hasil yang benar-benar tersimpan.
- AlreadyCaptured: identitas sudah ada, bukan row baru; output sukses, tanpa atribusi/enrichment ulang.
- Failed: error sebelum sukses, tidak ada mutation parsial. Error output setelah COMMIT tidak menghapus row; retry menemukan AlreadyCaptured.

UNSESSIONED dan UNASSIGNED merupakan label konseptual dari nullability, bukan status/type schema baru. Session COMPLETED tetap boleh dirujuk oleh commit yang terhubung saat session masih aktif.
