# Research: Dashboard & Today Timeline

**Date**: 2026-10-02

**Metode**: Analisis sumber dan implementasi lokal; keputusan desain berikut belum merupakan hasil benchmark atau pengujian implementasi.

## 1. Chronology versus ticket grouping

**Decision**: `wl today` memakai chronology global; setiap event menyertakan tiket. Dashboard mengagregasi per tiket.

**Rationale**: [goal.md](goal.md) menampilkan morning session lalu afternoon session berurutan. PRD 20 memakai grouping per tiket, sehingga goal spesifik pengguna menjadi prioritas. Tambahan tanggal/total memenuhi konteks BRD 15 dan PRD 20.

**Alternatives considered**: Grouping default atau menambah flag. Grouping mengubah contoh goal; flag memperluas scope.

## 2. Local day and active duration

**Decision**: Zona lokal perangkat, kalender midnight sampai midnight berikutnya, satu now per command. Active Duration seluruh elapsed; Today hanya bagian dalam hari ini.

**Rationale**: Config existing pada `application/bootstrap/config.go` tidak memiliki timezone. Session source menyimpan UTC; local conversion merupakan projection. Clipping mencegah seluruh session lintas hari masuk total hari ini.

**Alternatives considered**: UTC day, fixed Asia/Jakarta, atau memasukkan seluruh session berdasarkan start date. Ketiganya dapat menyimpangkan arti hari lokal pengguna.

## 3. Event sources and evidence time

**Decision**: START/STOP dari work_sessions; NOTE/GIT_COMMIT dari work_activities, dengan label tampilan COMMIT. Waktu activity menggunakan created_at, bukan captured_at metadata.

**Rationale**: Migration 001 membatasi activity type pada NOTE/GIT_COMMIT; start/stop tidak disimpan sebagai activity. Fitur 05 menyimpan waktu commit pada created_at dan capture provenance pada metadata. Projection tidak membutuhkan migration atau duplicate event rows.

**Alternatives considered**: Membuat START/STOP persistent atau memakai captured_at. Menambah data serta mengubah semantik timeline existing.

## 4. Tracked time and activity categories

**Decision**: Sum clipped session seconds, lalu floor ke menit untuk presentasi. Aktivitas bertiket tanpa session masuk Unsessioned; tanpa tiket masuk Unassigned saja. Activity-only ticket mendapat 0m.

**Rationale**: PRD 8 meminta kategori tersebut. Note dan commit adalah evidence, bukan rentang waktu. Domain session existing memakai integer duration seconds.

**Alternatives considered**: Menaksir waktu dari jarak antar commit atau menghitung unassigned dua kategori. Estimasi tidak memiliki dasar session dan duplikasi menyulitkan review.

## 5. Consistent read and timestamp compatibility

**Decision**: Snapshot port dan satu read transaction untuk session/activity. Baseline membaca seluruh records lalu memparse timestamp melalui parser existing dan filter di domain.

**Rationale**: `infrastructure/storage/sessions.go` memakai RFC3339Nano; migration defaults memakai CURRENT_TIMESTAMP. Filter string langsung berisiko berbeda dengan instant chronology. Snapshot menghindari active session stale digabung dengan stop yang baru tersimpan.

**Alternatives considered**: Query terpisah tanpa transaction, lexical range query, schema normalization. Baseline mengutamakan kompatibilitas tanpa migration; performa harus diukur sesuai plan.

## 6. Startup, output, and verification

**Decision**: Ganti root readiness output dengan dashboard, reuse lazy config/storage initialization, tampilkan teks evidence satu baris aman, dan lakukan TDD domain/application.

**Rationale**: `delivery/cli/root.go` saat ini menampilkan wl ready, sedangkan goal/PRD meminta dashboard. Factory existing pada `cmd/wlog/main.go` menyediakan pola close DB dan deferred access. Goal secara eksplisit mewajibkan TDD; fitur 06 telah menetapkan coverage gate 100% executable domain/application.

**Alternatives considered**: Mempertahankan readiness sebagai output default, bootstrap dua kali, membuka DB di constructor command, atau test setelah implementasi. Tidak memenuhi goal atau lazy help behavior.
