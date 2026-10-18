PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT (datetime('now'))
	);
INSERT INTO goose_db_version VALUES(1,0,1,'2026-10-18 14:41:18');
INSERT INTO goose_db_version VALUES(2,20250424200609,1,'2026-10-18 14:41:18');
INSERT INTO goose_db_version VALUES(3,20250515105448,1,'2026-10-18 14:41:18');
INSERT INTO goose_db_version VALUES(4,20250624000000,1,'2026-10-18 14:41:18');
INSERT INTO goose_db_version VALUES(5,20250627000000,1,'2026-10-18 14:41:18');
INSERT INTO goose_db_version VALUES(6,20250810000000,1,'2026-10-18 14:41:18');
INSERT INTO goose_db_version VALUES(7,20250812000000,1,'2026-10-18 14:41:18');
INSERT INTO goose_db_version VALUES(8,20260127000000,1,'2026-10-18 14:41:18');
INSERT INTO goose_db_version VALUES(9,20260902000000,1,'2026-10-18 14:41:18');
INSERT INTO goose_db_version VALUES(10,20260912000000,1,'2026-10-18 14:41:18');
INSERT INTO goose_db_version VALUES(11,20260921000000,1,'2026-10-18 14:41:18');
INSERT INTO goose_db_version VALUES(12,20260928000000,1,'2026-10-18 14:41:18');
INSERT INTO goose_db_version VALUES(13,20260928000001,1,'2026-10-18 14:41:18');
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
, summary_message_id TEXT, todos TEXT, channel TEXT);
INSERT INTO sessions VALUES('76c6dd61-4a2a-4858-b158-5cc88c4ef649',NULL,'The command printed fixture.',8,150,6,0.000458,1792334478,1792334478,NULL,NULL,NULL);
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
INSERT INTO messages VALUES('cf195639-d455-4a4f-92ea-731dc7688130','76c6dd61-4a2a-4858-b158-5cc88c4ef649','user','[{"type":"text","data":{"text":"run echo fixture please"}},{"type":"finish","data":{"reason":"stop","time":0}}]','',1792334478,1792334478,NULL,NULL,0,NULL,NULL,NULL,NULL);
INSERT INTO messages VALUES('2e5eb5eb-5555-44b7-8581-115623748b2b','76c6dd61-4a2a-4858-b158-5cc88c4ef649','assistant','[{"type":"tool_call","data":{"id":"call_1","name":"bash","input":"{\"command\":\"echo fixture\",\"description\":\"Print fixture\"}","provider_executed":false,"finished":true}},{"type":"finish","data":{"reason":"tool_use","time":1792334478}}]','fake-model',1792334478,1792334478,1792334478,'fake',0,NULL,NULL,NULL,NULL);
INSERT INTO messages VALUES('1f927332-7f2c-4201-b2b0-9685c42cfb4a','76c6dd61-4a2a-4858-b158-5cc88c4ef649','tool','[{"type":"tool_result","data":{"tool_call_id":"call_1","name":"bash","content":"fixture\n\n\n\u003ccwd\u003e/home/user/work/proj\u003c/cwd\u003e","data":"","mime_type":"","metadata":"{\"start_time\":1792334478162,\"end_time\":1792334478263,\"output\":\"fixture\\n\",\"description\":\"Print fixture\",\"working_directory\":\"/home/user/work/proj\"}","is_error":false}},{"type":"finish","data":{"reason":"stop","time":0}}]','',1792334478,1792334478,NULL,NULL,0,NULL,NULL,NULL,NULL);
INSERT INTO messages VALUES('540c05de-72d4-41ba-a67e-f64a8f7ef679','76c6dd61-4a2a-4858-b158-5cc88c4ef649','assistant','[{"type":"text","data":{"text":"The command printed fixture."}},{"type":"finish","data":{"reason":"end_turn","time":1792334478}}]','fake-model',1792334478,1792334478,1792334478,'fake',0,NULL,NULL,NULL,NULL);
INSERT INTO messages VALUES('5cc52f75-ae6f-4b4e-b187-d34fb8ee2208','76c6dd61-4a2a-4858-b158-5cc88c4ef649','user','[{"type":"text","data":{"text":"and once more"}},{"type":"finish","data":{"reason":"stop","time":0}}]','',1792334478,1792334478,NULL,NULL,0,NULL,NULL,NULL,NULL);
INSERT INTO messages VALUES('ebf3064c-1381-4efb-a339-bdba2b919e60','76c6dd61-4a2a-4858-b158-5cc88c4ef649','assistant','[{"type":"tool_call","data":{"id":"call_1","name":"bash","input":"{\"command\":\"echo fixture\",\"description\":\"Print fixture\"}","provider_executed":false,"finished":true}},{"type":"finish","data":{"reason":"tool_use","time":1792334478}}]','fake-model',1792334478,1792334478,1792334478,'fake',0,NULL,NULL,NULL,NULL);
INSERT INTO messages VALUES('13508045-187d-47a2-a9c1-ceaba607d27c','76c6dd61-4a2a-4858-b158-5cc88c4ef649','tool','[{"type":"tool_result","data":{"tool_call_id":"call_1","name":"bash","content":"fixture\n\n\n\u003ccwd\u003e/home/user/work/proj\u003c/cwd\u003e","data":"","mime_type":"","metadata":"{\"start_time\":1792334478317,\"end_time\":1792334478418,\"output\":\"fixture\\n\",\"description\":\"Print fixture\",\"working_directory\":\"/home/user/work/proj\"}","is_error":false}},{"type":"finish","data":{"reason":"stop","time":0}}]','',1792334478,1792334478,NULL,NULL,0,NULL,NULL,NULL,NULL);
INSERT INTO messages VALUES('97113384-160a-4210-aa5f-da5b4cdb5fbb','76c6dd61-4a2a-4858-b158-5cc88c4ef649','assistant','[{"type":"text","data":{"text":"The command printed fixture."}},{"type":"finish","data":{"reason":"end_turn","time":1792334478}}]','fake-model',1792334478,1792334478,1792334478,'fake',0,NULL,NULL,NULL,NULL);
CREATE TABLE read_files (
    session_id TEXT NOT NULL CHECK (session_id != ''),
    path TEXT NOT NULL CHECK (path != ''),
    read_at INTEGER NOT NULL,  -- Unix timestamp in seconds when file was last read
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE,
    PRIMARY KEY (path, session_id)
);
CREATE TABLE mcp_disabled_servers (
    name TEXT PRIMARY KEY
);
CREATE TABLE mcp_enabled_servers (
    name TEXT PRIMARY KEY
);
PRAGMA writable_schema=ON;
CREATE TABLE IF NOT EXISTS sqlite_sequence(name,seq);
DELETE FROM sqlite_sequence;
INSERT INTO sqlite_sequence VALUES('goose_db_version',13);
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
