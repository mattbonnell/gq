# gq

[![Go Reference](https://pkg.go.dev/badge/github.com/mattbonnell/gq.svg)](https://pkg.go.dev/github.com/mattbonnell/gq)
[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/mattbonnell/gq)](https://github.com/mattbonnell/gq)
[![Go Report Card](https://goreportcard.com/badge/github.com/mattbonnell/gq)](https://goreportcard.com/report/github.com/mattbonnell/gq)

gq is a multi-producer, multi-consumer message queue which can be backed your project's SQL database.
Consumers and Producers are non-blocking and concurrent by default, and can be tuned to fit your use case.
It delivers a scalable, performant message queue without needing to integrate and maintain dedicated infrastructure to support it.

## User Guide
### Definitions
- Message queue: A First In, First Out (FIFO) queue which can be pushed to and popped from by multiple agents simultaneously.
- Client: The Client maintains the connection to the database, and sets up the required database tables if they don't yet exist
- Message: A Message is a byte slice (`[]byte`). This allows for maximum flexibility, as you can marshal data of any type into a byte slice 
using [proto](https://pkg.go.dev/google.golang.org/protobuf/proto#Marshal), [json](https://golang.org/pkg/encoding/json/#Marshal), or any other binary format.
- Producer: Producers are used to push messages onto the queue through their `Push(message []byte)` method.
- Consumer: Consumers pops messages from the queue and process them using a user-provided callback of the form `func process(message []byte) error`.

### Usage

#### Creating a new Client
To create a new Client, call `gq.NewClient(db *sql.DB, driverName string)`:
```go
db, err := sql.Open("mysql", "user:password@...")
client, err := gq.NewClient(db, "mysql")
```

#### Creating a new Producer
To create a new Producer, call `gq.Client.NewProducer(ctx context.Context, opts *gq.ProducerOptions)`:
```go
producer, err := client.NewProducer(ctx, nil)
```
Passing `nil` for the options will create the Producer with the default configuration.
#### Pushing messages
To push a message, marshal your message and then pass it to the Producer's `Push` method. Here's an example using a Protobuf encoding:
```go
email := &pb.EmailMessage{
	To: "receiver@example.com",
	From: "sender@example.com",
	Body: "..."
}
msg, err := proto.Marshal(email)
producer.Push(msg)
```
`gq.Producer.Push` is non-blocking, making it safe to use in your request handlers. Behind the scenes, `Push` sends the message onto a channel and returns. A message-pushing
goroutine which starts when the Producer is instantiated receives the message off the channel and pushes it onto the queue.


#### Creating a new Consumer
To create a new Consumer, call `gq.Client.NewConsumer(ctx context.Context, process func(message []byte) error, opts *gq.ConsumerOptions)`:
```go
sendEmail := func(message []byte) error {
	email := &pb.EmailMessage{}
	if err := proto.Unmarshal(message, email); err != nil {
		return fmt.Errorf("Failed to parse email: %s", err)
	}
	if err := SendEmail(email); err != nil {
		return fmt.Errorf("Failed to send email: %s", err)
	}
	return nil
}
consumer, err := client.NewConsumer(ctx, sendEmail, nil)
```
Passing `nil` for the options will create the Consumer with the default configuration.
The Consumer will start asynchronously pulling and processing messages immediately. Messages which return error from the process function will be
requeued and retried a configurable number of times (3 by default).

### Documentation
For detailed documentation, including more advanced Producer/Consumer configuration, refer to the [go-docs](pkg.go.dev/mattbonnell/gq).

## Supported SQL DBs backends
gq currently supports MySQL, Postgres, and CockroachDB.

## Benchmarking against Redis
Results obtained using [mq_benchmark](github.com/mattbonnell/mq_benchmark)
### Throughput
```bash
❯ go run main.go redis false 10000 10000
2021/02/27 20:46:43 Begin redis test
2021/02/27 20:46:43 Received 10000 messages in 199.981995 ms
2021/02/27 20:46:43 Received 50004.500000 per second
2021/02/27 20:46:43 Sent 10000 messages in 200.138000 ms
2021/02/27 20:46:43 Sent 49965.523438 per second
2021/02/27 20:46:43 End redis test
❯ go run main.go gq false 10000 10000
2021/02/27 20:46:50 Begin gq test
2021/02/27 20:46:50 Sent 10000 messages in 14.238000 ms
2021/02/27 20:46:50 Sent 702345.812500 per second
2021/02/27 20:46:50 Received 10000 messages in 170.003998 ms
2021/02/27 20:46:50 Received 58822.144531 per second
2021/02/27 20:46:50 End gq test
```
### Latency
```bash
❯ go run main.go redis true 10000 10000
2021/02/27 20:48:49 Begin redis test
2021/02/27 20:48:49 Sent 10000 messages in 66.870003 ms
2021/02/27 20:48:49 Sent 149543.890625 per second
2021/02/27 20:48:49 Mean latency for 10000 messages: 0.153254 ms
2021/02/27 20:48:49 End redis test
❯ go run main.go gq true 10000 10000
2021/02/27 20:48:36 Begin gq test
2021/02/27 20:48:36 Sent 10000 messages in 12.475000 ms
2021/02/27 20:48:36 Sent 801603.187500 per second
2021/02/27 20:48:36 Mean latency for 10000 messages: NaN ms
2021/02/27 20:48:36 End gq test
```
