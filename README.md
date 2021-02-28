# gq

[![Go Reference](https://pkg.go.dev/badge/github.com/mattbonnell/gq.svg)](https://pkg.go.dev/github.com/mattbonnell/gq)[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/mattbonnell/gq)](https://github.com/mattbonnell/gq)
[![Go Report Card](https://goreportcard.com/badge/github.com/mattbonnell/gq)](https://goreportcard.com/report/github.com/mattbonnell/gq)

gq is a Go package which implements a multi-producer, multi-consumer message queue ontop of a vendor-agnostic SQL storage backend.
It delivers a scalable message queue without needing to integrate and maintain additional infrastructure to support it.

## Docs
Docs are available at [pkg.go.dev/github.com/mattbonnell/gq](https://pkg.go.dev/github.com/mattbonnell/gq)

## Example
```go
db, err := sql.Open(databaseDriver, databaseDSN)
if err != nil {
	log.Fatal().Err(err).Msg("failed to open DB")
}
client, err := gq.NewClient(db, databaseDriver)
if err != nil {
	log.Fatal().Err(err).Msg("error creating new client")
}
producer, err := client.NewProducer(context.TODO(), nil)
if err != nil {
	log.Fatal().Err(err).Msg("error creating new producer")
}

consumer, err = client.NewConsumer(context.TODO(), func(message []byte) error {
		// process message
		return nil

	}, nil)

m := make([][]byte, numMessages)
for i := range m {
	m[i] = []byte(...) // serialized message
	producer.Push(m[i])
}
```

## Benchmarking against Redis
Results obtained using [my fork of mq_benchmark](github.com/mattbonnell/mq_benchmark)
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
❯ go run main.go gq true 10000 10000
2021/02/27 20:48:36 Begin gq test
2021/02/27 20:48:36 Sent 10000 messages in 12.475000 ms
2021/02/27 20:48:36 Sent 801603.187500 per second
2021/02/27 20:48:36 Mean latency for 10000 messages: NaN ms
2021/02/27 20:48:36 End gq test
❯ go run main.go redis true 10000 10000
2021/02/27 20:48:49 Begin redis test
2021/02/27 20:48:49 Sent 10000 messages in 66.870003 ms
2021/02/27 20:48:49 Sent 149543.890625 per second
2021/02/27 20:48:49 Mean latency for 10000 messages: 0.153254 ms
2021/02/27 20:48:49 End redis test
```
