package sqlite3

const (
	message = `
CREATE TABLE IF NOT EXISTS message (
	id INTEGER PRIMARY KEY,
    	status INTEGER,
	payload BLOB,
	consumer_id INTEGER,
	created DATETIME DEFAULT CURRENT_TIMESTAMP
);`
	messageIdStatus = `
CREATE INDEX IF NOT EXISTS message_id_status
ON message (id, status);
`
	messageIdConsumer = `
CREATE INDEX IF NOT EXISTS message_id_consumer
ON message (id, consumer_id);
`

	consumer = `
CREATE TABLE IF NOT EXISTS consumer (
	id INTEGER PRIMARY KEY,
	created DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated DATETIME DEFAULT CURRENT_TIMESTAMP
);`

	consumerCreatedUpdated = `
CREATE INDEX IF NOT EXISTS consumer_created_updated
ON consumer (created, updated);
`
)

var Schema = []string{
	message,
	messageIdStatus,
	messageIdConsumer,
	consumer,
	consumerCreatedUpdated,
}
