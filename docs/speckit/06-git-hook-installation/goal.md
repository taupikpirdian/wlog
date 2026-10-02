SpecKit 06 — Git Hook Installation
Implement:
wl install-hooks

Menginstall:
.git/hooks/post-commit

yang menjalankan:
wl git

Hook tidak boleh menyebabkan Git commit gagal jika wl error.
wl git >/dev/null 2>&1 || true

Setelah ini flow-nya sudah:
git commit
    ↓
post-commit
    ↓
wl git
    ↓
SQLite

Notes:
Gunakan TDD ketika implement code untuk folder:
- application
- domain