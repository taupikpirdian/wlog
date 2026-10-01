# Research and Decisions: Git Commit Capture

**Status**: Complete  
**Basis**: goal, BRD/PRD/ERD, migration 001, ticket pattern/service, session transaction helper, activity NOTE implementation, dan dokumentasi resmi Git. Tidak ada implementasi/benchmark fitur 05 yang diklaim oleh dokumen ini.

## 1. Capture satu HEAD immutable

**Decision**: Resolve root dan `git rev-parse --verify HEAD^{commit}` sekali; gunakan hash untuk pembacaan selanjutnya. Repository identity adalah canonical absolute worktree root. Bare/unborn/nonrepository tidak menghasilkan activity.

**Rationale**: Menghindari gabungan metadata dua commit bila HEAD berubah. Root canonical menyamakan pemanggilan dari subdirektori/symlink. Rev-parse mendukung verifikasi revision sebagai commit. [Git rev-parse](https://git-scm.com/docs/git-rev-parse)

**Alternatives considered**: Membaca HEAD terpisah pada setiap command, memakai basename repository, atau scan seluruh history. Tidak memenuhi identity stabil dan scope satu commit.

## 2. Message, waktu, dan branch

**Decision**: Message lengkap memakai `%B`; committer timestamp memakai `%cI`; parents memakai `%P`. Format metadata dipisah NUL dan parser menjaga body message. `symbolic-ref --quiet --short HEAD` memberi branch capture-time; detached HEAD normal dengan NULL. Jika HEAD tidak lagi sama setelah branch dibaca, branch dihilangkan dengan warning. Format message/time mengacu pada [Git log](https://git-scm.com/docs/git-log); branch pada [Git symbolic-ref](https://git-scm.com/docs/git-symbolic-ref).

**Rationale**: Message body dapat mengandung key; branch bukan properti immutable commit. Created_at menggunakan commit time untuk timeline; captured_at clock application untuk provenance/fallback.

**Alternatives considered**: Subject saja, short hash, author timestamp, atau string HEAD sebagai branch. Pilihan tersebut menghilangkan informasi atau membuat konteks palsu.

## 3. Prioritas tiket dan attachment

**Decision**: Ekstraksi first-match KeyPattern fitur 02: message → active session → branch → UNASSIGNED. Active session diberikan kepada pure attribution resolver di dalam transaksi persistence. Ticket sama menghasilkan session attachment, mismatch menghasilkan session NULL + warning.

**Rationale**: Mengikuti BRD bagian 5 dan PRD 15–18. Pure resolver mempertahankan aturan di domain/application sekaligus memilih active session yang masih berlaku saat write.

**Alternatives considered**: Branch sebelum session, tiket dari repository name, snapshot session saat mulai subprocess, atau mengembalikan konflik seperti NOTE. Git capture harus mempertahankan evidence dan tidak membutuhkan persetujuan session; atribusi atomik pada write dipilih daripada kehilangan commit saat session berubah.

**Boundary**: Global session tidak dibatasi repository. Manual capture HEAD lama dapat terhubung ke session bertiket sama; ini evidence association, bukan inferensi rentang/durasi. Relink historis tidak termasuk fitur ini.

## 4. Deduplikasi dan transaksi

**Decision**: Identity `(repository, commit_hash)` mengikuti ERD dan unique index migration 001. Lookup existing terlebih dahulu di BEGIN IMMEDIATE; jika ada, return AlreadyCaptured tanpa resolver, ticket INSERT, atau UPDATE. New ticket dan activity INSERT satu transaksi.

**Rationale**: PRD hash-only bertentangan dengan schema existing. Repository+hash mencegah satu checkout menekan evidence checkout lain, dan no-update membuat retry stabil walau session/config berubah.

**Alternatives considered**: Migration hash unique global, application-only precheck, atau upsert activity untuk enrichment/attachment. Mengubah semantics existing, tidak melindungi concurrency, atau memindahkan evidence capture pertama.

## 5. Statistik perubahan deterministik

**Decision**: Root dibandingkan dengan tree kosong melalui --root; non-root/merge memakai parent pertama eksplisit. Gunakan name-only dan numstat NUL output, disable rename detection, dan tetapkan Myers/no-indent-heuristic. Binary -/- tidak menyumbang jumlah baris teks. Format/opsi merujuk [Git diff-tree](https://git-scm.com/docs/git-diff-tree).

**Rationale**: Kontrak sederhana dapat diuji tanpa gabungan statistik multi-parent. No-renames menghindari heuristic mahal; rename muncul sebagai old deleted + new added. NUL menjaga filename berkarakter whitespace.

**Alternatives considered**: Combined merge diff, jumlah seluruh parent, shortstat parsing human-readable, atau -M rename detection. Tidak dipilih untuk menghindari duplikasi, locale-dependent output, dan model rename tambahan.

## 6. Optional metadata dan konfigurasi

**Decision**: Root/hash wajib; message, time, branch, files/stat optional dengan status collected/disabled/unavailable. Missing message melanjutkan fallback tiket berikutnya dengan warning; missing time menggunakan clock capture. Empty commit yang terukur berbeda dari enrichment unavailable.

**Rationale**: PRD 32 meminta data yang tersedia tetap disimpan. NULL dan status JSON membedakan tidak diketahui dari 0/[] yang benar-benar terukur.

**Alternatives considered**: Membatalkan semua capture karena stats gagal, menyimpan defaults nol tanpa status, atau mengisi ulang duplicate. Pilihan tersebut kehilangan evidence atau membuat data menyesatkan.

**Config boundary**: Changed-files/diff-stat flags independen. Full diff di luar goal fitur 05; flag true memberi warning unsupported, bukan pengumpulan patch tersembunyi. AI flag tidak memicu panggilan AI.

## 7. Offline dan privasi

**Decision**: Gunakan argument array dan read-only commit object operations; disable external diff/textconv, lazy fetch, serta replacement objects. Attributes untuk enrichment diambil dari commit dengan --attr-source. Periksa capability Git; jangan memakai fallback yang menghilangkan proteksi. Opsi global offline/attributes tersedia di [Git command documentation](https://git-scm.com/docs/git).

**Rationale**: Capture tidak boleh membaca working file sebagai evidence, mengeksekusi configured content helper, atau melakukan fetch otomatis. Pathspec exclude keluarga .env* diterapkan sebelum numstat; nama path tetap boleh tercatat. Source diff tidak disimpan/dikirim.

**Alternatives considered**: Plain working-tree git diff, shell interpolation, capture patch default, atau filtering env sesudah diff dibaca. Tidak sesuai scope commit dan privasi BRD/PRD.

**Compatibility boundary**: Ketidaktersediaan offline capability menjadi error actionable; ketidaktersediaan safe optional enrichment menghasilkan warning/NULL. Test mendeteksi unsupported options dengan fake runner, bukan mengasumsikan semua versi Git memiliki kemampuan sama.

## 8. Output, failure, dan hook

**Decision**: New capture atau AlreadyCaptured exit 0; mismatch/unassigned/partial metadata warning ke stderr setelah commit. Essential Git/storage/config errors exit non-zero. Help tidak menyentuh dependency. Feature 06 menjalankan wl git dan menahan error agar commit tidak gagal.

**Rationale**: Pemanggilan manual harus menunjukkan kegagalan capture; hook menentukan kebijakan integrasinya. Tidak ada operasi write Git di command ini.

**Alternatives considered**: Semua failure exit 0, memasang hook sebagai side effect, atau warning sebelum storage sukses yang menyatakan commit telah tersimpan. Tidak dipilih.

## 9. Performance

**Decision**: Target PRD <500 ms pada executable, initialized local DB/repository biasa tanpa contention. Metadata budget 2 detik bukan SLA; optional timeout menyimpan partial data, cancellation pengguna tidak menulis. Writer wait mengikuti SQLite timeout existing.

**Rationale**: Git object parsing di luar transaksi mengurangi lock time; full diff/rename detection tidak diperlukan goal.

**Alternatives considered**: Menahan writer lock selama subprocess, mengganti data dengan estimasi ketika timeout, atau menyatakan target tercapai tanpa benchmark. Tidak dipilih.
