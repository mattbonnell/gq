package sqlite3

const (
	message = `
CREATE TABLE message (
	id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    	status TINYINT UNSIGNED,
	payload BLOB,
	consumer_id INT UNSIGNED,
	created TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	INDEX id_status (id, status),
	INDEX id_consumer (id, consumer),
);`

	consumer = `
CREATE TABLE consumer (
	id INT UNSIGNED AUTO_INCREMENT
	created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	INDEX create_update (created, updated)
);`
)

var Schema = []string{message, consumer}
