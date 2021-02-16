package sqlite3

const (
	message = `
CREATE TABLE message (
	id INTEGER PRIMARY KEY,
    	status INTEGER,
	payload BLOB,
	consumer_id INTEGER,
	created DATETIME DEFAULT CURRENT_TIMESTAMP
);`
	messageIdStatus = `
CREATE UNIQUE INDEX message_id_status
ON message (id, status);
`
	messageIdConsumer = `
CREATE UNIQUE INDEX message_id_consumer
ON message (id, consumer_id);
`

	consumer = `
CREATE TABLE consumer (
	id INTEGER PRIMARY KEY,
	created DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated DATETIME DEFAULT CURRENT_TIMESTAMP
);`

	consumerCreatedUpdated = `
CREATE INDEX consumer_created_updated
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
