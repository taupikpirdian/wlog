CREATE TABLE tickets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_key TEXT NOT NULL UNIQUE,
    title TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE work_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    repository TEXT,
    started_at TEXT NOT NULL,
    ended_at TEXT,
    duration_seconds INTEGER,
    status TEXT NOT NULL CHECK(status IN ('ACTIVE', 'COMPLETED')),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(ticket_id) REFERENCES tickets(id)
);

CREATE TABLE work_activities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id INTEGER,
    session_id INTEGER,
    type TEXT NOT NULL CHECK(type IN ('NOTE', 'GIT_COMMIT')),
    description TEXT,
    repository TEXT,
    branch TEXT,
    commit_hash TEXT,
    commit_message TEXT,
    changed_files TEXT,
    insertions INTEGER DEFAULT 0,
    deletions INTEGER DEFAULT 0,
    metadata TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(ticket_id) REFERENCES tickets(id),
    FOREIGN KEY(session_id) REFERENCES work_sessions(id)
);

CREATE TABLE ai_generations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id INTEGER,
    type TEXT NOT NULL CHECK(type IN (
        'WORKLOG_SUMMARY',
        'TICKET_DESCRIPTION',
        'CODE_REVIEW'
    )),
    content TEXT NOT NULL,
    generated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(ticket_id) REFERENCES tickets(id)
);

CREATE INDEX idx_work_sessions_ticket ON work_sessions(ticket_id);
CREATE INDEX idx_work_sessions_started_at ON work_sessions(started_at);
CREATE INDEX idx_work_activities_ticket ON work_activities(ticket_id);
CREATE INDEX idx_work_activities_session ON work_activities(session_id);
CREATE INDEX idx_work_activities_created_at ON work_activities(created_at);
CREATE UNIQUE INDEX idx_work_activities_repo_commit
    ON work_activities(repository, commit_hash)
    WHERE commit_hash IS NOT NULL;
CREATE UNIQUE INDEX idx_single_active_session
    ON work_sessions(status)
    WHERE status = 'ACTIVE';
