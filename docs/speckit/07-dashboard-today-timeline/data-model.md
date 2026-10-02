# Data Model: Dashboard & Today Timeline

Read projections saja, tanpa tabel atau state transition baru. Nama tipe adalah rancangan, bukan API Go final.

## Persisted Sources

| Source | Fields yang diperlukan | Mapping |
| --- | --- | --- |
| tickets | id, ticket_key | Key untuk session/activity; nullable join activity |
| work_sessions | id, ticket_id, title, started_at, ended_at, status | Active view, durasi, START/STOP |
| work_activities | id, ticket_id, session_id, type, description, commit_message, commit_hash, created_at | NOTE/COMMIT, kategori dan attribution |

`duration_seconds` tetap tersimpan sesuai fitur 03, tetapi projection menghitung clipping dari timestamp. Tidak ada query Git atau perubahan metadata capture. TicketKey activity berasal dari ticket_id sendiri, bukan inferensi session/message.

## DayWindow

Fields: LocalDate, Location, Start, End, Now. Start adalah midnight lokal tanggal Now; End midnight kalender hari berikutnya. Event termasuk jika `Start <= EventTime < End`. Durasi dibatasi Now; event future hari ini tetap ditampilkan sebagai evidence.

## DailySnapshot

Fields: Sessions dan Activities dari satu transaction. Session harus memiliki key, timestamp parsed, dan status valid. Completed wajib end >= start; active tanpa end dan start <= Now; maksimal satu active. Error parse/invariant menghasilkan error usecase tanpa partial result.

Activity memiliki ID, nullable TicketKey/SessionID, type NOTE/GIT_COMMIT, timestamp, description/message/hash opsional. Null session/ticket sah untuk commit fitur 05. Relasi tidak diubah atau ditebak ulang.

## Session Contribution

Untuk setiap session:

```text
effectiveEnd = min(endedAt jika completed, Now)
effectiveEnd = Now jika active
from = max(startedAt, DayWindow.Start)
until = min(effectiveEnd, DayWindow.End)
seconds = 0 jika until <= from
seconds = floor((until - from) dalam detik) selainnya
```

Active elapsed dihitung terpisah sebagai floor((Now - startedAt) dalam detik), tidak diclip pada midnight. Tidak melakukan write ke ended_at/duration_seconds/status.

## TicketDailySummary

Fields: TicketKey dan TrackedSeconds. Sertakan tiket dengan irisan session nonempty, boundary event hari ini, atau activity hari ini; termasuk session nol durasi yang memiliki event. Sum session contribution per key; sort key ascending. TotalTrackedSeconds adalah sum kontribusi seluruh session sebelum formatting.

## TimelineEvent

Fields: Time, Kind, TicketKey nullable, Text, SourceKind, SourceID, CommitHash optional.

| Kind | Source | Time | Text |
| --- | --- | --- | --- |
| START | Session | StartedAt | Key wajib; tidak membutuhkan judul pada baris |
| NOTE | Activity NOTE | CreatedAt | Description |
| COMMIT | Activity GIT_COMMIT | CreatedAt | CommitMessage atau `(no commit message)` |
| STOP | Completed session | EndedAt | Key wajib |

Ordering tuple: full Time ascending, kind rank START=0/NOTE=1/COMMIT=2/STOP=3, SourceID ascending. Source ID tidak dicampur sebagai identitas global; kombinasi SourceKind/ID/Kind mengidentifikasi event. Dua note dengan teks/waktu sama tetap berbeda.

## DashboardView and TodayView

DashboardView: DayWindow, optional ActiveSessionView, TicketSummaries, TotalTrackedSeconds, UnsessionedEvents, UnassignedEvents. Kedua kategori difilter hari ini dan memakai event ordering yang sama.

TodayView: DayWindow, Events, TotalTrackedSeconds. Tidak mengelompokkan per tiket atau menampilkan event kemarin untuk melengkapi session lintas hari.

## Invariants

1. Semua durasi nonnegative dan berasal dari session saja.
2. Setiap source event dalam hari muncul tepat sekali pada timeline.
3. Active session tidak menghasilkan STOP.
4. Per command hanya satu Now dan satu snapshot.
5. Unassigned dashboard tidak berulang di Unsessioned.
6. Sanitasi/formatting hanya mengubah view text, bukan data sumber.
7. Session/aktivitas/tiket tidak berubah selama pembacaan; bootstrap schema tetap kontrak fitur 01.
