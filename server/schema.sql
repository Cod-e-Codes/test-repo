CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	sender TEXT,
	content TEXT,
	created_at DATETIME,
	is_encrypted BOOLEAN DEFAULT 0,
	encrypted_data BLOB,
	nonce BLOB,
	recipient TEXT
);
