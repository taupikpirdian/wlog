SpecKit 02 — Ticket Management
Fokus pada tiket sebagai agregator aktivitas.
OOT-3751
    ├── sessions
    ├── notes
    ├── commits
    └── AI generations

Scope:
Parse ticket:
[A-Z][A-Z0-9]+-[0-9]+

Create ticket
Find ticket
Upsert ticket
Extract ticket from commit message

Contoh:
fix: [OOT-3751][taupik.pirdian@salt.co.id][SP29] fix tax calculation -> ticket_key
fix: [OOT-3751][taupik.pirdian@salt.co.id] fix tax calculation -> ticket_key
fix: [taupik.pirdian@salt.co.id][OOT-3751] fix tax calculation -> ticket_key
fix: [taupik.pirdian@salt.co.id][ORB-3751] fix tax calculation -> ticket_key

Menghasilkan:
1. OOT-3751
2. OOT-3751
3. OOT-3751
4. ORB-3751