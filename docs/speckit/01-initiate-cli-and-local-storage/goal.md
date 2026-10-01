SpecKit 01 — Initialize CLI & Local Storage
Goal: menghasilkan fondasi aplikasi yang sudah bisa dijalankan sebagai wl.
Scope:
- Initialize Go module
- Setup Cobra CLI
- SQLite connection
- ~/.worklog/
- worklog.db
- config.yaml
- Database migration
- Base repository pattern
- wl --version
- wl --help

Acceptance utama:
wl --version
wl --help

dan otomatis tersedia:
~/.worklog/
├── config.yaml
└── worklog.db