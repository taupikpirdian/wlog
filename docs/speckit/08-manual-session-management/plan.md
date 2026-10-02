# Implementation Plan: Manual Session Management

**Feature**: `08-manual-session-management`

**Status**: Implemented and verified

## Technical Context

| Area | Rancangan |
| --- | --- |
| Runtime | Go/Cobra/SQLite existing, tanpa dependency baru |
| Domain | `domain/session/manual.go`: wall time validation/resolution, candidate range, overlap rule dan errors |
| Application | `application/session/manual.go`: manual/backdated usecases dengan port atomik, clock/location/repository injeksi |
| Infrastructure | `infrastructure/storage/manual_sessions.go`: extend SQLiteSessionStore dengan insert manual atomik dan BEGIN IMMEDIATE |
| Delivery | `delivery/cli/manual_sessions.go`: AddManualSessionCommands, command session dan --since/-s; factory terpisah/lazy |
| Composition | Wiring `cmd/wlog/main.go` memakai loader/store existing, time.Now/time.Local dan repository reader best effort |
| Schema | work_sessions/tickets existing, tidak ada migration atau activity mutation |
| Verification | Unit TDD domain/application, concurrency dua koneksi, CLI args/help/error, regression 01–07 |

## Project Context and Workflow

Repo tidak memiliki `.specify/`, active template, constitution, extension hooks, atau setup-plan.sh. Ikuti layout docs/speckit fitur sebelumnya pada path pengguna; tidak perlu branch baru atau instalasi tooling. Research memakai goal, BRD/PRD serta source lokal. Artefak ini adalah desain, bukan bukti implementasi.

## Architecture and Composition

Domain session memiliki aturan syntactic HH:mm, resolve tanggal/zona lokal secara unik, batas now, positive completed interval, dan overlap. Reuse ValidateTitle, TicketKey pattern, Session, serta Complete untuk perhitungan durasi bila sesuai; pertahankan completed/active fields yang dipakai fitur lain.

Application manual service berisi usecases CreateCompleted dan StartSince. Constructor menerima manual persistence port, configured key pattern, clock, location dan optional repository reader. Application membaca now tepat sekali, meminta domain membentuk candidate dan mengirim candidate beserta observedAt ke operasi store atomik. Kembalikan hasil yang sudah memiliki session/ticket ID setelah commit. Application tidak menjalankan SQL atau melakukan estimasi waktu.

Tambahkan interface manual secara terpisah dari Store/SessionService existing agar ordinary start/stop serta fake tests tidak dipaksa memiliki operasi baru. SQLiteSessionStore boleh mengimplementasikan kedua interface. Start command memilih factory manual hanya jika flag --since Changed; nilai --since kosong eksplisit harus gagal, bukan dianggap ordinary start. Manual session command memakai factory manual yang sama. Root wiring mengikuti pola additive command/factory existing; help/version dan invalid argument tidak membuka dependency.

Tidak ada prompt pada dua flow manual. --since menolak existing active tanpa memanggil confirmSwitch; ordinary start tanpa flag tetap memakai confirmation existing. Pakai safeText dari delivery fitur 07 untuk output title/key/conflict agar multiline/control tidak merusak terminal. Reuse dailyDuration untuk presentasi durasi agar contoh 2h sesuai goal; detik tersimpan tetap utuh.

## Local Time Resolution

- Validate input persis HH:mm, angka 00–23/00–59, tanpa whitespace luar atau format alternatif.
- Ambil tanggal kalender dari observedAt dalam location; seconds/nanoseconds input nol.
- Resolver wajib mendeteksi zero/multiple instant untuk wall time pada tanggal tersebut; jangan hanya menerima hasil time.Date/time.Parse yang menormalisasi gap atau memilih satu offset fold.
- Gap/fold menghasilkan error domain. Unique from/to/since dikonversi ke UTC. Kandidat future ditolak terhadap observedAt; from < to untuk completed, since <= observedAt untuk active.
- Durasi computed berdasarkan elapsed UTC, bukan subtract angka jam. Unit fixture menguji gap/fold dan rentang unambiguous yang melintasi perubahan offset.

Rancangan resolver dapat mengevaluasi kandidat instant dalam day window dan mencocokkan kembali tanggal/jam lokal; strategi final harus dibuktikan melalui DST fixtures. Tidak perlu menambahkan date/offset API atau dependency timezone baru. Bila test/runtime tidak memiliki zoneinfo, gunakan fixture timezone data atau tzdata standard Go untuk test.

## Atomic Persistence

1. Validasi input produk sebelum mutation; ambil now sekali dan bentuk candidate lewat domain.
2. Adapter memperoleh connection dan BEGIN IMMEDIATE menggunakan pola writeTransaction existing; writer lock didapat sebelum membaca existing sessions.
3. Baca semua session beserta start/end/status, parse timestamp memakai parser SQLite existing agar RFC3339Nano dan legacy CURRENT_TIMESTAMP kompatibel. Malformed timestamp/range gagal, tanpa skip row.
4. Untuk StartSince, recheck tidak ada ACTIVE. Untuk keduanya, bandingkan candidate dengan setiap existing interval melalui aturan domain di bawah lock. Pre-read opsional application hanya membantu pesan; tidak menjadi dasar final write.
5. Jika valid, INSERT tiket ON CONFLICT DO NOTHING, resolve ticket ID, lalu INSERT session. Semua SQL parameter-bound. Jangan mengubah title tiket existing.
6. Completed menyimpan ended_at dan duration_seconds; active menyimpan keduanya null. Simpan UTC RFC3339Nano. created_at/updated_at memakai observedAt, bukan backdated started_at, agar audit creation tidak dipalsukan.
7. Commit sebelum result sukses. Conflict/error/cancellation pre-commit memakai rollback cleanup existing yang tetap berjalan saat context command dibatalkan. Tidak ada activity update atau existing-session completion pada operasi ini.

Active interval diperlakukan [start,+infinity), termasuk candidate --since; completed interval [start,end). Completed nol durasi existing tidak overlap. Candidate manual completed wajib positive. Adjacency start==other.end atau end==other.start sah. Overlap check global berdasarkan database, tidak dibatasi tiket/repository.

Concurrent ordinary start setelah manual commit tetap mengikuti semantics fitur 03. Desain ini tidak mengklaim mendeteksi semua overlap yang dibuat ordinary start dengan jam perangkat keliru atau data pihak luar di kemudian hari; yang dijamin adalah validasi atomik saat flow manual menambahkan candidate.

## Schema Compatibility

Tidak ada kolom source/estimated atau type activity baru. Intent waktu manual terdapat pada input operasi; Session tetap format existing. Dashboard/timeline membaca start/end tanpa perubahan. Note/commit yang tidak terkait session tetap tidak terkait, sehingga manual addition hanya menambah tracked time serta START/STOP projection.

## Constitution Check

Tidak ada constitution formal; gunakan arsitektur repo dan goal.

- [x] Domain tidak bergantung pada Cobra/SQL/filesystem/AI.
- [x] Application mengorkestrasi domain dan port, dengan clock/location injeksi.
- [x] Infrastructure mengimplementasikan transaksi/SQL/parsing; domain menentukan overlap.
- [x] Root layer layout existing dipertahankan, tanpa internal.
- [x] Dua flow manual atomik tanpa prompt atau transaksi selama interaksi pengguna.
- [x] TDD domain/application wajib; 100% statement coverage executable package existing dipertahankan.
- [x] Post-design: single-active, ordinary switch, note/commit attribution dan read projections fitur 07 tetap kompatibel.

## Phase 0 — Decisions

Lihat [research.md](research.md). Tidak ada keputusan desain terbuka; scope goal dan perbedaan dengan opsi PRD dicatat eksplisit.

## Phase 1 — Artifacts

- [data-model.md](data-model.md): range, wall time, persisted fields dan invariant.
- [contracts/cli.md](contracts/cli.md): command/flag/output/exit serta error semantics.
- [quickstart.md](quickstart.md): acceptance/TDD/concurrency/regression guide.
- [checklists/requirements.md](checklists/requirements.md): hasil review spesifikasi.

## Implementation Sequence

1. RED tests domain HH:mm, current local date, unique resolve/future/gap/fold, range and overlap; GREEN lalu REFACTOR.
2. RED application tests untuk create completed/start since, one clock, key/title, optional repository, persistence/conflict/cancel; GREEN lalu REFACTOR.
3. Implement store manual dengan integration fixtures transaksi, timestamp compatibility, overlap/adjacency/active, rollback dan concurrent writers.
4. Tambah command/flag serta factory routing tests; verifikasi required flags dan syntax errors sebelum dependency, since Changed, safe output dan close/output error.
5. Wire lazy factory dan regression ordinary start/switch, stop, note, capture serta dashboard/today.
6. Jalankan coverage/race/vet/build dan isolated smoke; perbarui README serta status/bukti quickstart setelah implementasi selesai.

## Risks and Verification

| Risiko | Mitigasi / verifikasi |
| --- | --- |
| HH:mm diam-diam menjadi kemarin/future | Same local date, reject future/from>=to, clock deterministik |
| Gap/fold memilih waktu yang tidak dimaksud | Reject zero/multiple instant, DST fixture |
| Double counting | Half-open global overlap, check di bawah writer lock |
| Backdated switch merusak evidence session lama | --since tanpa switch; active conflict tanpa mutation |
| Concurrent insert setelah overlap precheck | BEGIN IMMEDIATE sebelum read/validation/insert |
| Timestamp legacy menyebabkan range SQL salah | Parse existing timestamps, compare instant melalui domain |
| Ticket dibuat tetapi session gagal | Satu transaction, failure-trigger rollback fixture |
| Implicit reattachment | Tidak ada UPDATE activity; assert session_id before/after |
| Retry sesudah output error | Document persisted success boundary dan overlap/active response pada retry |
