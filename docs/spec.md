# gq spec

## Description
gq is a Go package which implements a multi-producer, multi-consumer message queue ontop of a vendor-agnostic SQL storage backend.
It delivers a scalable message queue without needing to integrate/maintain additional infrastructure to support it.

## Definitions
- Consumer: A consumer is a queue client which pops messages from the queue.
- Producer: A producer is a queue client which pushes messages onto the queue.
- Queue: A set of SQL DB tables through which the queue is maintained and listeners coordinate.

## Concepts

### Client

#### Initialization

The gq client is initialized by passing it DB connection parameters.

### Producers

#### Initialization

Producers are initialized by calling the client's NewProducer method and passing it a push batch size (1 by default).

The push batch size defines how many messages a producer should gather before pushing them onto the queue.
Larger batch size means fewer database queries and thus less overhead, while a smaller batch size means more queries but lower latency.

#### Enqueuing messages

Messages are pushed onto the queue by calling the Producer's `Push` method, supplying a message payload and optionally a deadline by which to successfully process the message.

If this deadline passes and the message has yet to be successfully processed, it should be abandoned (in `at most once` mode; see **Message delivery guarantees** below)

### Consumers

#### Initialization

Consumers are initialized by calling the client's NewConsumer method and passing it a pop batch size (10 by default).

The pop batch size defines how many messages a consumer should pop from the queue at once.

Larger queue size means fewer database queries and thus less overhead, but a greater chance of losing messages if a consumer fails, while a smaller batch size means more queries but less risk.


#### Registration

After consumers are initialized, they register themselves with the queue by inserting a record into the Consumers table.

After registering, the consumer receives its ID, which is its index in the birth order of all active consumers.

#### Heartbeats

For the duration of their life, consumers send a "heartbeat" to the DB on a regular basis to keep their Consumers record refreshed.

Consumers that miss a number of consecutive heartbeats are considered to have died, and must re-register to start dequeuing messages again.

With every H heartbeats, a given consumer marks any consumers who've missed more than D heartbeats as having died (deletes their record), and checks if it itself is still alive or if it
needs to register again.

#### Dequeuing messages

After confirming its liveliness (and potentially re-registering), the consumer Pops the oldest batch of messages which meet the following conditions:

1. message.ID % number of active consumers = consumer.ID
2. message.status = "queued"
3. message.deadline = null or message.deadline < current time

After receiving the batch of messages which meet these conditions, the consumer attempts to update the status of these records to "consumed" and assign their consumer UUID to the record's "consumer" column.
In this update query, the consumer includes a WHERE condition that the status must be "queued".

If the number of rows updated is less than the number of messages in the batch, it means that one or more of the messages have already been consumed by another consumer.
The consumer makes an additional select query for messages whose ID is in the original batch,and whose "consumer" column matches the consumer's UUID. This allows the consumer
to adjust its batch to include only those which it was able to claim.

#### Processing messages

The client registers a `Process` callback with the consumer, which specifies how each message should be processed. The consumer iterates over its batch, passing each message's payload
to this function. If the function returns successfully, the consumer updates the message's status to "processed". If the function returns error, the consumer checks the message deadline. If it has passed, it updates the message's status
to "failed". If it has not yet passed, it resets its status to "queued".

## Message delivery guarantees
### At most once
gq can be configured to offer an `at most once` delivery guarantee. In this mode, after the deadline expires, the message is marked "failed" and won't be retried.
For example, in an email verification context, the deadline can be used to ensure that if the user doesn't obtain the email within some set period, if they request another, they won't receive the original message at some later point.

### At least once
gq can also be configured to offer an `at least once` delivery guarantee for use cases were message delivery is essential. In `at least once`, when a message's deadline expires while its status is "consumed",
other consumers are free to claim it and attempt to deliver it. In this mode, message deduplication is the client's responsibility.
