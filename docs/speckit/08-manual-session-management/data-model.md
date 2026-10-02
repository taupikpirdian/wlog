# Data Model: Manual Session Management

Tidak ada tabel/kolom baru. Rancangan berikut memperluas operasi session existing; nama tipe/metode adalah desain, bukan API Go final.

## LocalWallTime

Value HH:mm dengan hour 0–23/minute 0–59, seconds/nanoseconds nol. Format strict, tanpa whitespace atau tanggal tambahan. Resolve memakai observedAt local date dan location; tepat satu matching instant diperlukan. Zero match = nonexistent, multiple match = ambiguous. Keduanya invalid; tidak normalize/estimate.

## Manual Request

| Field | Completed flow | StartSince flow |
| --- | --- | --- |
| TicketKey | Satu argumen, configured pattern | Argumen existing start |
| Title | Flag --title wajib dan nonempty setelah trim | Argumen title existing |
| From/To | HH:mm wajib, unique resolve, from < to <= observedAt | Tidak tersedia |
| Since | Tidak tersedia | HH:mm wajib bila flag Changed, since <= observedAt |
| ObservedAt | One actual command clock | One actual command clock |
| Location | Zona lokal perangkat | Zona lokal perangkat |
| Repository | Optional current repository best effort | Optional current repository best effort |

## Session Candidate and Persisted Fields

| work_sessions field | Completed manual | Backdated active |
| --- | --- | --- |
| id | Generated saat insert | Generated saat insert |
| ticket_id | Existing/new ticket ID | Existing/new ticket ID |
| title | Validated user title | Validated user title |
| repository | Optional source repository | Optional source repository |
| started_at | Resolved From UTC | Resolved Since UTC |
| ended_at | Resolved To UTC | NULL |
| duration_seconds | floor((To - From) dalam detik) | NULL |
| status | COMPLETED | ACTIVE |
| created_at/updated_at | ObservedAt UTC | ObservedAt UTC |

Timestamp serialisasi memakai format UTC existing; resolver mempertahankan instant walau angka jam lokal memiliki offset berbeda. Tidak ada field AI estimate/source baru. ticket title existing tidak diubah oleh session title.

## Range and Overlap

Completed interval `[start,end)`, positive untuk kandidat manual. Existing zero-duration completed interval kosong dan tidak overlap. Active interval `[start,+infinity)`, termasuk kandidat StartSince.

Untuk dua nonempty interval A dan B:

```text
overlap jika A.start < B.end dan B.start < A.end
end active dianggap +infinity
```

End == other.start merupakan adjacency yang sah. Perbandingan memakai instant, tidak string timestamp atau tampilan HH:mm. Overlap global tanpa filter tiket/repository.

StartSince juga selalu memerlukan tidak ada active existing, meskipun since sama dengan observedAt. Completed baru tidak mengubah active existing; boleh berada seluruhnya sebelum active start.

## Persistence Outcome

Success mengembalikan Session dengan ID/TicketID valid setelah commit. Conflict memuat existing session ID, key, start/end/status untuk pesan CLI, tanpa raw SQL/internal detail. Invalid existing range/timestamp menghasilkan error data; tidak memilih waktu pengganti.

Tidak ada state transition pada session existing. Entity baru langsung COMPLETED atau ACTIVE; StartSince kemudian dapat Complete lewat stop existing. Tidak ada update activity atau implicit relink.

## Invariants

1. Input pengguna menjadi start/end yang tersimpan setelah unique local-time resolution, tanpa estimasi.
2. Completed manual positive dan selesai tidak melewati observedAt; StartSince tidak future.
3. Active/overlap check dan insert berada dalam satu serialized write transaction.
4. Tiket baru rollback bila session insert gagal; judul master existing tetap utuh.
5. Tidak ada session existing yang selesai/diedit/dihapus oleh flow manual.
6. Activity attribution/timestamps tetap utuh, termasuk evidence dengan tanggal/rentang cocok.
7. Projection dashboard/timeline existing memakai session baru tanpa event persistence tambahan.
8. Creation timestamp merekam waktu pencatatan, bukan waktu mulai yang dibackdate.
