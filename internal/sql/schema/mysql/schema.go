package mysql

const (
	message = `
CREATE TABLE IF NOT EXISTS message (
	id INT AUTO_INCREMENT PRIMARY KEY,
    	status TINYINT UNSIGNED,
	payload BLOB,
	consumer_id INT,
	created TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	INDEX id_status (id, status),
	INDEX id_consumer (id, consumer),
);`

	consumer = `
CREATE TABLE IF NOT EXISTS consumer (
	id INT AUTO_INCREMENT
	created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	INDEX create_update (created, updated)
);`
)

var Schema = []string{message, consumer}
