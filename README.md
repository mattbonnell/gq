# gq

[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/mattbonnell/gq)](https://github.com/mattbonnell/gq)
[![Go Report Card](https://goreportcard.com/badge/github.com/mattbonnell/gq)](https://goreportcard.com/report/github.com/mattbonnell/gq)

gq is a Go package which implements a multi-producer, multi-consumer message queue ontop of a vendor-agnostic SQL storage backend.
It delivers a scalable message queue without needing to integrate and maintain additional infrastructure to support it.

## Benchmarking against Redis
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
