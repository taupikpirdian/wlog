# Research: Manual Session Management

**Date**: 2026-10-02

**Metode**: Analisis goal, BRD/PRD dan source lokal; tidak ada dependency baru atau klaim pengujian implementasi.

## 1. Two additive operations

**Decision**: Manual completed command dan backdated active flag; ordinary start/stop tetap tersedia.

**Rationale**: [goal.md](goal.md) meminta kedua contoh, PRD 21 menetapkan completed session dan PRD 9 menyediakan --since/-s. `application/session/service.go` saat ini hanya mendukung start-now/stop-now sehingga manual usecases baru diperlukan.

**Alternatives considered**: Edit session existing atau completed-only. Tidak memenuhi dua flow goal dan berisiko mengubah evidence yang telah ada.

## 2. Day, format, future and timezone

**Decision**: HH:mm strict pada tanggal/zona lokal saat eksekusi, one now, reject future dan gap/fold; completed from < to.

**Rationale**: Goal tidak menyediakan tanggal/offset. Config existing belum memiliki timezone; fitur 07 memakai zona lokal. Reject menjaga input eksplisit dan larangan estimasi, tanpa mengarang yesterday rollover atau memilih instant ambigu.

**Alternatives considered**: Infer kemarin, normalize DST gap, choose fold offset, atau menambah flags date/RFC3339. Memperluas scope atau menentukan waktu tanpa input pengguna.

## 3. Overlap and active session

**Decision**: Reject overlap global pada flow manual, boundary adjacency valid, active interval akhir terbuka. --since harus tanpa active; tidak ada retroactive switch.

**Rationale**: Dashboard fitur 07 menjumlahkan contribution session; overlap akan terhitung dua kali. Single-active constraint ada pada migration 001. Switch existing menyelesaikan session lama pada waktu start baru; backdate dapat meletakkan end sebelum note/commit existing.

**Alternatives considered**: Allow overlaps, force flag, atau otomatis trim/split. Menambah double counting atau mengubah waktu yang dipilih pengguna. Ordinary start tetap semantics lama; fitur ini tidak memperbaiki history existing.

## 4. Evidence attachment

**Decision**: Tidak reattach note/commit otomatis dan tidak ada attach prompt.

**Rationale**: PRD 21 menyebut activity attachment sebagai penawaran opsional; goal hanya meminta pembuatan session dengan waktu manual. Atribusi capture fitur 05 memakai rule sendiri, sehingga waktu yang cocok belum menjadi izin untuk mengubah session_id.

**Alternatives considered**: Auto-attach berdasarkan ticket/range atau prompt Y/n. Memperluas scope mutasi dan membutuhkan aturan tambahan untuk mismatch, timestamp boundaries serta concurrency activity.

## 5. Transaction and schema

**Decision**: Reuse schema work_sessions/tickets serta writeTransaction BEGIN IMMEDIATE; check active/overlap dan ticket+session insert dalam satu transaction. created_at memakai waktu pencatatan, start/end memakai input.

**Rationale**: `infrastructure/storage/sessions.go` telah menyediakan serialized writer dan rollback cleanup saat cancellation. Migration 001 mendukung COMPLETED/ACTIVE dan nullable end/duration tanpa perubahan. Fitur 07 memakai timestamp untuk START/STOP projection, bukan activity persistent.

**Alternatives considered**: Precheck di luar lock, migration source/estimated, atau persistent START/STOP activity. Precheck dapat berlomba; migration/activity duplikat tidak diperlukan untuk goal.

## 6. Verification

**Decision**: TDD untuk domain/application, 100% executable statement coverage gate existing, serta SQLite/CLI regression fixtures terisolasi.

**Rationale**: Goal secara eksplisit mewajibkan TDD. Fitur 07 mempertahankan coverage gate semua executable domain/application; manual logic menambah timezone/range branches yang perlu test deterministik.

**Alternatives considered**: Test setelah implementasi atau hanya contoh happy path. Tidak memenuhi goal dan tidak membuktikan race/time/conflict behavior.
