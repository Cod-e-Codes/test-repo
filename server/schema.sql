CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	message_id INTEGER DEFAULT 0,
	sender TEXT,
	content TEXT,
	created_at DATETIME,
	is_encrypted BOOLEAN DEFAULT 0,
	encrypted_data BLOB,
	nonce BLOB,
	recipient TEXT
);

CREATE TABLE IF NOT EXISTS user_message_state (
	username TEXT PRIMARY KEY,
	last_message_id INTEGER NOT NULL DEFAULT 0,
	last_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_messages_message_id ON messages(message_id);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_user_message_state_username ON user_message_state(username);

-- Ban history tracking table
CREATE TABLE IF NOT EXISTS ban_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL,
	banned_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	unbanned_at DATETIME,
	banned_by TEXT NOT NULL,
	INDEX(username, banned_at)
);

-- Create indexes for ban history performance
CREATE INDEX IF NOT EXISTS idx_ban_history_username ON ban_history(username);
CREATE INDEX IF NOT EXISTS idx_ban_history_banned_at ON ban_history(banned_at);
CREATE INDEX IF NOT EXISTS idx_ban_history_unbanned_at ON ban_history(unbanned_at);
