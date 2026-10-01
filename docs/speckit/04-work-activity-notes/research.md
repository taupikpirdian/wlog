# Research and Decisions: Work Activity Notes

**Status**: Complete  
**Basis**: goal, BRD/PRD/ERD, schema migration 001, serta application/session dan SQLiteSessionStore fitur 03. Review desain concurrency dilakukan terhadap kode lokal; dokumen ini tidak mengklaim benchmark atau implementasi fitur 04 sudah dijalankan.

## 1. Command dan atribusi

**Decision**: `wl note "<description>"`, alias `n`, mengikuti session aktif global serta tiketnya. Tanpa session aktif, gagal tanpa write.

**Rationale**: Goal menetapkan active ticket/session; PRD bagian 11 mendefinisikan nama lengkap, alias, dan error. Note adalah context capture selama session.

**Alternatives considered**: Memilih tiket dari teks, menerima ticket flag, membuat session otomatis, atau menyimpan note unsessioned. Semuanya memperluas scope goal dan mengaburkan atribusi.

## 2. Snapshot dan writer transaction

**Decision**: Application memilih snapshot aktif; adapter memverifikasi ulang ID, ticket ID, dan started_at dalam `BEGIN IMMEDIATE` sebelum INSERT. Jika snapshot berubah, gagal konflik tanpa retry yang memilih tujuan baru.

**Rationale**: Fitur 03 sudah memakai writer transaction untuk start/switch/stop. FK memastikan relasi ada, tetapi tidak membuktikan session masih ACTIVE. Recheck di dalam transaksi menjaga NOTE dari session yang sudah dihentikan setelah pembacaan awal.

**Alternatives considered**: Read lalu insert tanpa transaksi; hanya memeriksa FK; selalu memakai active session terbaru saat insert. Pendekatan tersebut dapat menambahkan note pada session selesai atau mengganti tiket tujuan tanpa keputusan command.

**Concurrency outcomes**: Note menang lebih dahulu → note tersimpan, lalu stop/switch dapat selesai. Stop/switch menang setelah snapshot dibaca → note gagal. Perubahan sebelum snapshot dibaca → note memilih session aktif baru. Dua note pada session yang tetap aktif boleh sama-sama berhasil.

## 3. Description dan pengulangan

**Decision**: Trim whitespace luar, tolak kosong, pertahankan whitespace internal, Unicode, newline, dan tanda kutip. Tidak ada deduplikasi berdasarkan teks/timestamp atau batas panjang produk tambahan.

**Rationale**: Catatan harus mempertahankan evidence asli. Catatan sama dapat menggambarkan aktivitas berulang. SQL parameter binding menangani teks sebagai data.

**Alternatives considered**: Menggabungkan argumen tak berquote, normalisasi semua whitespace, pemotongan otomatis, deduplikasi teks, atau klasifikasi AI. Tidak diperlukan oleh goal dan dapat menghilangkan konteks.

## 4. Waktu dan clock

**Decision**: Clock diinjeksi ke application; created_at dibaca setelah snapshot terpilih dan ditulis UTC RFC3339Nano. Domain menolak waktu sebelum start session dan memperbolehkan waktu sama dengan start.

**Rationale**: Menjaga note di dalam rentang awal pekerjaan dan konsisten dengan clock validation fitur 03. Timestamp aktual tidak diganti berdasarkan durasi atau perkiraan.

**Alternatives considered**: Default CURRENT_TIMESTAMP, backdating, menggeser note ke timestamp terakhir, atau menerima timestamp sebelum start. Default database mengurangi kendali clock test; koreksi/backdate merupakan scope recovery.

**Boundary**: Clock yang mundur antar-note tetapi masih setelah start tidak ditolak atau dikoreksi. Pembaca timeline kelak dapat mengurutkan created_at lalu ID untuk timestamp sama; urutan waktu tidak dijanjikan sebagai urutan command ketika jam berubah.

## 5. Repository dan metadata

**Decision**: Copy repository nullable dari session terpilih. Jangan menjalankan Git. Branch, commit fields, changed_files, dan metadata tetap NULL; insertions/deletions default 0.

**Rationale**: Note non-coding harus tersedia dari direktori mana pun. Session aktif global menjadi sumber konteks; Git capture merupakan fitur 05.

**Alternatives considered**: Mengambil current Git root/branch, mengisi metadata environment, atau menjadikan Git syarat note. Tidak diperlukan dan menambah I/O serta ambiguitas konteks.

## 6. Schema dan lapisan

**Decision**: Gunakan work_activities existing tanpa migration. Note memiliki tiket/session/description wajib pada entity/usecase; nullable schema tetap dipertahankan untuk Git activity. Letakkan package activity di domain/application, SQL di infrastructure, CLI di delivery, dan wiring di cmd/wlog.

**Rationale**: Relasi dan kolom NOTE sudah tersedia. Kode session/ticket fitur 02/03 sudah mengikuti arsitektur berlapis di root.

**Alternatives considered**: Tabel notes terpisah, NOT NULL global, domain mengakses SQL, atau folder internal. Tidak sesuai schema dan preferensi layout pengguna.

## 7. Error, cleanup, dan performa

**Decision**: Error input menampilkan usage; error no-active memberi saran start; konflik memberi saran retry. Sukses hanya dicetak setelah commit. Gunakan cleanup rollback saat context command batal dan tutup database per command.

**Rationale**: Menghindari pesan sukses palsu atau koneksi tertinggal. Busy timeout 5 detik sudah tersedia pada OpenDatabase; target PRD <200 ms berlaku pada operasi lokal biasa, tanpa lock wait dan cold initialization.

**Alternatives considered**: Silent drop, implicit retry ke session lain, transaksi lebih panjang, atau klaim performa tanpa pengukuran. Semuanya tidak memenuhi atribusi/observability yang diinginkan.

**Limitation**: Error output setelah commit tidak membatalkan data yang telah tersimpan. Retry pengguna dapat membuat note kedua. Tepat satu note berlaku untuk tiap invocation sukses, bukan exactly-once lintas retry.
