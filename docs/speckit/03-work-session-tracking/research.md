# Research: Work Session Tracking

## Waktu dan durasi

**Decision**: Simpan timestamp UTC dengan offset eksplisit, tampilkan timezone lokal, dan hitung detik penuh dari timestamp yang disimpan.

**Rationale**: Mendukung pergantian tanggal dan timezone tanpa menggantungkan durasi pada teks jam. Goal melarang AI menentukan durasi.

**Alternatives**: Pembulatan ke menit dapat menambah waktu yang tidak dikerjakan; menghitung hanya dari jam lokal dapat salah pada perubahan tanggal/offset.

## Single active session

**Decision**: Aturan berlaku per database pengguna. Konfirmasi pergantian diikat pada ID session, lalu perubahan dilakukan dalam satu transaksi dengan timestamp pergantian yang sama.

**Rationale**: BRD/PRD meminta hanya satu session aktif dan persetujuan sebelum menutup session lama. Prompt dapat berlangsung cukup lama sehingga snapshot perlu diperiksa lagi.

**Alternatives**: Stop lalu start dalam dua transaksi dapat kehilangan session aktif jika start gagal. Membuka transaksi selama prompt dapat menghalangi command lain.

## Repository context

**Decision**: Catat path root Git saat start sebagai metadata opsional; kegagalan deteksi dianggap konteks tidak tersedia.

**Rationale**: Goal meminta repository, sedangkan pekerjaan support, meeting, atau investigasi tetap perlu dicatat di luar repository.

**Alternatives**: Mewajibkan Git membatasi session non-coding; mencatat direktori kerja sebagai repository dapat keliru bila direktori bukan repo.

## Batas recovery dan note

**Decision**: Fitur 03 menyediakan start/stop waktu aktual. Note mengikuti fitur 04; `--since`, rentang manual, dan koreksi waktu mengikuti fitur recovery.

**Rationale**: Goal fitur 03 mengunci pencatatan aktual dan single active session. PRD memisahkan recovery ke prioritas P2.

## Durasi nol

**Decision**: Session di bawah satu detik menyimpan durasi nol.

**Rationale**: Membulatkan minimum ke satu detik akan mengubah durasi aktual. Keputusan ini memperjelas pengecualian terhadap contoh PRD AC-03 yang menyebut `> 0`.
