package mysql

const (
	message = `CREATE TABLE IF NOT EXISTS message (
	id INT AUTO_INCREMENT PRIMARY KEY,
	payload BLOB NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	ready_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	retries INT DEFAULT 0,
	INDEX ready_at (ready_at ASC)
);`
)

var Schema = []string{message}
