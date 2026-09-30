PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT (datetime('now'))
	);
INSERT INTO goose_db_version VALUES(1,0,1,'2026-09-30 18:33:15');
INSERT INTO goose_db_version VALUES(2,20250424200609,1,'2026-09-30 18:33:15');
INSERT INTO goose_db_version VALUES(3,20250515105448,1,'2026-09-30 18:33:15');
INSERT INTO goose_db_version VALUES(4,20250624000000,1,'2026-09-30 18:33:15');
INSERT INTO goose_db_version VALUES(5,20250627000000,1,'2026-09-30 18:33:15');
INSERT INTO goose_db_version VALUES(6,20250810000000,1,'2026-09-30 18:33:15');
INSERT INTO goose_db_version VALUES(7,20250812000000,1,'2026-09-30 18:33:15');
INSERT INTO goose_db_version VALUES(8,20260127000000,1,'2026-09-30 18:33:15');
INSERT INTO goose_db_version VALUES(9,20260902000000,1,'2026-09-30 18:33:15');
INSERT INTO goose_db_version VALUES(10,20260912000000,1,'2026-09-30 18:33:15');
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    parent_session_id TEXT,
    title TEXT NOT NULL,
    message_count INTEGER NOT NULL DEFAULT 0 CHECK (message_count >= 0),
    prompt_tokens  INTEGER NOT NULL DEFAULT 0 CHECK (prompt_tokens >= 0),
    completion_tokens  INTEGER NOT NULL DEFAULT 0 CHECK (completion_tokens>= 0),
    cost REAL NOT NULL DEFAULT 0.0 CHECK (cost >= 0.0),
    updated_at INTEGER NOT NULL,  -- Unix timestamp in seconds
    created_at INTEGER NOT NULL   -- Unix timestamp in seconds
, summary_message_id TEXT, todos TEXT);
INSERT INTO sessions VALUES('e49948d6-3e9d-41d4-ab68-872b8ebf8b8e',NULL,'The command printed fixture.',8,150,6,0.00045799999999999997,1790793195,1790793195,NULL,NULL);
CREATE TABLE files (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    path TEXT NOT NULL,
    content TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,  -- Unix timestamp in seconds
    updated_at INTEGER NOT NULL,  -- Unix timestamp in seconds
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE,
    UNIQUE(path, session_id, version)
);
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    parts TEXT NOT NULL default '[]',
    model TEXT,
    created_at INTEGER NOT NULL,  -- Unix timestamp in seconds
    updated_at INTEGER NOT NULL,  -- Unix timestamp in seconds
    finished_at INTEGER, provider TEXT, is_summary_message INTEGER DEFAULT 0 NOT NULL, prism_model_id TEXT, prism_model_name TEXT, prism_hypercredit_savings REAL, prism_dollar_savings REAL,  -- Unix timestamp in seconds
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);
INSERT INTO messages VALUES('b003b99b-5e60-4b63-b4ea-cfcb9d41a375','e49948d6-3e9d-41d4-ab68-872b8ebf8b8e','user','[{"type":"text","data":{"text":"run echo fixture please"}},{"type":"finish","data":{"reason":"stop","time":0}}]','',1790793195,1790793195,NULL,NULL,0,NULL,NULL,NULL,NULL);
INSERT INTO messages VALUES('acf1eed6-806f-4cba-a686-69730c6ce9b7','e49948d6-3e9d-41d4-ab68-872b8ebf8b8e','assistant','[{"type":"tool_call","data":{"id":"call_1","name":"bash","input":"{\"command\":\"echo fixture\",\"description\":\"Print fixture\"}","provider_executed":false,"finished":true}},{"type":"finish","data":{"reason":"tool_use","time":1790793195}}]','fake-model',1790793195,1790793195,1790793195,'fake',0,NULL,NULL,NULL,NULL);
INSERT INTO messages VALUES('63f2bce7-afed-4ff8-9542-a404815315ed','e49948d6-3e9d-41d4-ab68-872b8ebf8b8e','tool','[{"type":"tool_result","data":{"tool_call_id":"call_1","name":"bash","content":"fixture\n\n\n\u003ccwd\u003e/home/user/work/proj\u003c/cwd\u003e","data":"","mime_type":"","metadata":"{\"start_time\":1790793195328,\"end_time\":1790793195429,\"output\":\"fixture\\n\",\"description\":\"Print fixture\",\"working_directory\":\"/home/user/work/proj\"}","is_error":false}},{"type":"finish","data":{"reason":"stop","time":0}}]','',1790793195,1790793195,NULL,NULL,0,NULL,NULL,NULL,NULL);
INSERT INTO messages VALUES('c1a33b53-1b9a-418d-9357-556a8c6227b9','e49948d6-3e9d-41d4-ab68-872b8ebf8b8e','assistant','[{"type":"text","data":{"text":"The command printed fixture."}},{"type":"finish","data":{"reason":"end_turn","time":1790793195}}]','fake-model',1790793195,1790793195,1790793195,'fake',0,NULL,NULL,NULL,NULL);
INSERT INTO messages VALUES('103def9b-4748-46f0-85e7-ee7033dc5b7c','e49948d6-3e9d-41d4-ab68-872b8ebf8b8e','user','[{"type":"text","data":{"text":"and once more"}},{"type":"finish","data":{"reason":"stop","time":0}}]','',1790793195,1790793195,NULL,NULL,0,NULL,NULL,NULL,NULL);
INSERT INTO messages VALUES('55c27572-e2aa-4f17-8c0f-ba49ac6e2f24','e49948d6-3e9d-41d4-ab68-872b8ebf8b8e','assistant','[{"type":"tool_call","data":{"id":"call_1","name":"bash","input":"{\"command\":\"echo fixture\",\"description\":\"Print fixture\"}","provider_executed":false,"finished":true}},{"type":"finish","data":{"reason":"tool_use","time":1790793195}}]','fake-model',1790793195,1790793195,1790793195,'fake',0,NULL,NULL,NULL,NULL);
INSERT INTO messages VALUES('871830d8-6907-473c-b187-25b4000e1f5c','e49948d6-3e9d-41d4-ab68-872b8ebf8b8e','tool','[{"type":"tool_result","data":{"tool_call_id":"call_1","name":"bash","content":"fixture\n\n\n\u003ccwd\u003e/home/user/work/proj\u003c/cwd\u003e","data":"","mime_type":"","metadata":"{\"start_time\":1790793195486,\"end_time\":1790793195586,\"output\":\"fixture\\n\",\"description\":\"Print fixture\",\"working_directory\":\"/home/user/work/proj\"}","is_error":false}},{"type":"finish","data":{"reason":"stop","time":0}}]','',1790793195,1790793195,NULL,NULL,0,NULL,NULL,NULL,NULL);
INSERT INTO messages VALUES('d05c5265-0ab9-4c12-b111-658b5d3231da','e49948d6-3e9d-41d4-ab68-872b8ebf8b8e','assistant','[{"type":"text","data":{"text":"The command printed fixture."}},{"type":"finish","data":{"reason":"end_turn","time":1790793195}}]','fake-model',1790793195,1790793195,1790793195,'fake',0,NULL,NULL,NULL,NULL);
CREATE TABLE read_files (
    session_id TEXT NOT NULL CHECK (session_id != ''),
    path TEXT NOT NULL CHECK (path != ''),
    read_at INTEGER NOT NULL,  -- Unix timestamp in seconds when file was last read
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE,
    PRIMARY KEY (path, session_id)
);
PRAGMA writable_schema=ON;
CREATE TABLE IF NOT EXISTS sqlite_sequence(name,seq);
DELETE FROM sqlite_sequence;
INSERT INTO sqlite_sequence VALUES('goose_db_version',10);
CREATE TRIGGER update_sessions_updated_at
AFTER UPDATE ON sessions
BEGIN
UPDATE sessions SET updated_at = strftime('%s', 'now')
WHERE id = new.id;
END;
CREATE TRIGGER update_files_updated_at
AFTER UPDATE ON files
BEGIN
UPDATE files SET updated_at = strftime('%s', 'now')
WHERE id = new.id;
END;
CREATE TRIGGER update_messages_updated_at
AFTER UPDATE ON messages
BEGIN
UPDATE messages SET updated_at = strftime('%s', 'now')
WHERE id = new.id;
END;
CREATE TRIGGER update_session_message_count_on_insert
AFTER INSERT ON messages
BEGIN
UPDATE sessions SET
    message_count = message_count + 1
WHERE id = new.session_id;
END;
CREATE TRIGGER update_session_message_count_on_delete
AFTER DELETE ON messages
BEGIN
UPDATE sessions SET
    message_count = message_count - 1
WHERE id = old.session_id;
END;
CREATE INDEX idx_files_session_id ON files (session_id);
CREATE INDEX idx_files_path ON files (path);
CREATE INDEX idx_messages_session_id ON messages (session_id);
CREATE INDEX idx_sessions_created_at ON sessions (created_at);
CREATE INDEX idx_messages_created_at ON messages (created_at);
CREATE INDEX idx_files_created_at ON files (created_at);
CREATE INDEX idx_read_files_session_id ON read_files (session_id);
CREATE INDEX idx_read_files_path ON read_files (path);
CREATE INDEX idx_messages_role_created_at ON messages (role, created_at);
PRAGMA writable_schema=OFF;
COMMIT;
