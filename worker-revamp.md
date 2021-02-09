# Worker revamp

## Goals
1. Eliminate double sending
2. Elegantly handle addition/removal of instances
3. Minimize lost emails


## Consistent Hashing Approach
Distributed consistent hashing
What if each worker inserted its ID into some table, and there was a stored procedure on the DB that assigned each email to a worker using consistent hashing.
Can the consistent hashing itself be run in a distributed manner?
Ex: Each worker runs the distributed hashing algo, reads the DB to get the list of available workers. Each node sends a heartbeat to the DB periodically, just to update its UpdatedAt field.
Records whose UpdatedAt falls behind some threshold get pruned, and their associated hashing members are removed.
Problem: What happens if one node doesn't know a node is dead yet? And do the nodes assign a specific worker to an email, or just a key? Just a key.
So when emails are created, they're assigned a consistent hashing key.
When nodes fetch emails, they fetch all "queued" emails and then call LocateKey to see if they're supposed to send them. If they aren't, they discard the email. 
Take the remaining emails and begin sending them. Before sending the given email, make an UPDATE query to update the status of the email with this email's ID and status "queued" to "processing". If zero affected rows are returned, it means some other worker is sending the email (ex: due to a transient difference in the member-set), so discard the email, the other worker is sending it.

If a worker were to die while processing some email, then the user wouldn't receive it. In this instance, the user would press the "resend" button, which would update the email record's status to "queued" so that it could be dequeued by some other worker.
Since staus is only updated when a worker dies, email loss would be minimal. A dead instance would only result in losing one email. This could be tuned for more losses but higher throughput by updating the statuses to "processing" in a batch before beginning to send any.


# Golang queueing client
- Golang library which implements queueing behaviour on top of SQL DB (provider-agnostic, ontop of say sqlx)

# Motivations
- Use cases which benefit from queuing semantics without needing a full blown Kafka cluster
- Ex: email sending and other async tasks associated with web services
- Enable message queue/worker pool functionality simply by integrating a Golang package
- Another ex use case: notification sending in abtestmain, separation of concerns. Request handlers write message to DB and move on, handling of that message can be handled elsewhere
without needing to clutter this code or modify it later
- Yes, we can send dispatch email/notification delivery in a goroutine, but then we need to put all of this event handling code in our request handlers. Better to just report the message/event and not have to know/care how it is used
- Give the option to run workers on separate instances, scale them independently, not consume server resources with async tasks unrelated to serving requests
- Searched for such a library and couldn't find it, prevent anyone else from having to implement queuing ontop of their SQL DB again!

# Goals
- Provide message queue/worker pool functionality without needing to add/maintain separate piece of infra
- Effortlessly and transparently scale background workers up with number of instances without "double-processing" tasks (ie Kratos mailer bug)
- Get message queue/worker pool functionality without needing to configure/maintain a Redis/RabbitMQ/Kafka instance
- Efficiently handle dynamic scaling of worker pool
- Work ontop of (most) popular SQL DBs without any added configuration on the DB side, everything handled in library
- Work at scale
- If message/event processing is a core piece of your service, yeah you should probably consider some dedicated infra. But if you just happen to need to process some messages,
like web service which emits notifications, you shouldn't need to integrate a whole AWS service (pay for it) or manage more infra
- Cost efficiency, ie resource efficiency
- Minimize DB load
	- How expensive is a SELECT and maybe an INSERT query, though, really?
	- Could potentially add 3 extra queries to each request (but asynchronously..., still, tripling DB load. Maybe this is the tradeoff)
	- What's your concern? Taking up cluster bandwidth? 

# Guarantees
- Since motivating use case is email sending, we want to provide "at most once" guarantee
- Need efficient way to ensure that a message isn't processed twice
- What we know:
1. Messages can't be statically assigned to a node at push-time, since that node could die and then these messages would be in limbo
2. We can't overload DB with a ton of requests

# Looking forward
- Develop specific plugins/companion libraries to handle common use cases, like email dispatch, Slack notif delivery, forwarding event on to centralized event management service or queue
- Topics (ok let's relax)


# Design
- Distributed consistent hashing approach
- Each worker runs consistent hashing algo
- Push() assigns key to message
- Pull() either:
1. Retrieves messages in worker's key range, or retrieves batch of messages and only processes those which belong to it (TBD)
- Each worker emits a "heartbeat" to DB intermittently to inform other workers that it's still alive, this is how # of workers is tracked
- Each worker queries DB intermittently to check how many workers are alive and recomputes consistent hashing
- If new worker has been added, add it to "hash ring"
- If worker has died, remove it from "hash ring"
- Encode payloads using protocol buffers for minimal serialization overhead
- Naive solution:
	- Have some "state" column, set to "queued" by default. 
	- Before Pull() client yields a message, it tries to update its state to "processed". If the message's status is already "processed", client discards the message
	- Downsides: each message triggers an additional DB query. So two queries per messages (Push, and then status compare and swap, assuming Pulls are batched and so Pull query cost is amortized)
	- Could minimize conflicts by employing distributed consistent hashing algo and then having workers try to update the status of all of "their" messages in one query.
		1. If affected rows count = # of messages, proceed
		2. If affected rows count < # of messages, means another worker has claimed some of these messages
			a) either we're out of date with # of workers, or they are
			b) they've already claimed those msgs and are currently processing them, nothing we can do about that
			c) so what do we do with our batch? 
			d) can't throw it out or we lose all un-processed messages which we just marked as processed
	- Have workers check "alive workers" table and update their hash rings before making each pull
	- This ensures they have most up to date workers list before determining assignments, minimizes chance of conflicts
	- Have new workers wait until all other workers acknowledge it before it can pull messages, so that they don't conflict with them (can have some counter column, or acknowledgements table)
- If we don't want to lose any msgs, could offer option to write claimed msgs to /tmp file so that if server crashes, msgs can be recovered (if persistent storage is present)
- Assign each message to a partition (random number from large range), and give it some "consumed" state/column
- When a new consumer comes online, it registers with the "consumers" table and is somehow assigned a range of partitions (or just use id as key)
- How to assign this range? Take non-consumed messages where id % number of workers = worker's "birth order"
- worker's "birth order" is its position in the worker records of alive workers sorted by "created" time
- all workers can agree on their birth order by looking at the worker table
	- Cache these messages and then mark them as consumed.
	- Insert into 'consumed log' table where message ID is unique
	- Discard messages which aren't returned, they've already been consumed. Can join on this table to check if message already consumed on pulls
	- Or just do UPDATE while returning affected rows, so we know which messages hadn't already been consumed. Only process these messages
		- Doesn't work on MySQL (WHYYY)
		- Would need to do something like UPDATE, including worker ID, if there are conflicts, SELECT messages with IDs from batch, discard messages with a diff worker ID
		- Additional query, but no table locking :)
		- Conflicts should be rare, so extra queries should be rare
		- Having separate "consumed" table mapping message IDs to worker IDs would be cleaner, would necessitate JOINing to get consumption status tho. But could have covering index on "consumed" table so this would be super cheap
		- Ok so "consumption_log" table it is
	- There, no doubles. Easy
- If new worker is added?
	- Messages already marked consumed unnaffected. 
	- ~~New worker can't pull messages until all other workers acknowledge it (prevents conflicts)~~ (will still have conflicts as these other workers notice new worker at diff times)
	- Conflicts just mean one extra query over the worker's next batch, not a big deal lol. Cost of this extra query is amortized over batch size
	- When workers notice new worker, update their total worker count
	- Workers start pulling msgs in their newly assigned partitions
- If worker terminates gracefully
	- It finishes processing its current batch, and then marks itself dead (sets its heartbeat to NULL or some super old value?)
- If worker dies (stops updating its "heartbeat" record)
	- when workers query workers table before pulling, they'll only select workers who've emitted a heartbeat in the last epsilon seconds (heartbeat period * K?), and use count as # of workers
	- worst case, some messages don't get consumed in the time it takes for workers to notice that a worker has died (few seconds at most, can be tuned by heartbeat period, K value)
	- Once worker notices a worker has died, it updates its worker count
	- On some interval, workers prune dead workers from the worker table (in a batch amortize cost)
- In short, will likely have some "conflicts" when a worker is added/removed, but this will result in at most n extra queries where n is the number of workers, not a big deal

## Queries
1. Heartbeat
2. Liveliness check
3. Pull() (batch fetch messages which *should* belong to calling worker)
4a. Insert "consumed" records
4b. (if conflicts) fetch messages with id in Pull() batch and current workers ID (basically messages which were successfully consumed by this worker)
5. Periodic dead workers prune

## Notes
- No table locking!
- All message fetching is amortized over batch size!
- Conflict minimization! (depending on heartbeat frequency)
- Conflicts handled by one extra batch query!

## Follow-up
- Some caching-to-disk strategy to avoid losing any messages
- Add some API call to tell consumer you've processed message, so it can update some log pointer. Basically replay message stream from where processor left off so as not to lose messages (worry about this after)
