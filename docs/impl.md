# Implementation notes

[`database/sql` guide](http://go-database-sql.org/)

## Database driver
1. In order to be DB agnostic, need to have the client code import the driver and supply the `sql.Open()` arguments (driver name, connection string)
2. All `gq` code should be driver agnostic, only relying on `database/sql` idioms (and maybe sqlx)
3. How to have driver-agnostic column types? Maybe this is default, need to keep reading

## sql.DB object
1. Designed to be long-lived
2. Create it once and pass it around as needed
3. Manages pool of DB connections

## queries/statements
1. If a func name includes `Query`, it is designed to ask questions of the database, and will return a set of rows, even if its empty.
2. Statements that don't return rows should not use `Query` functions; they should use `Exec()`.

### Handling result sets
1. Need to call `Close()` on the results set to free up the underlying connection. Easiest way is to `defer` a call to close. But if fetching multiple result sets in
one function (eg querying in a loop), deferring calls to Close is a bad as you'll accumulate busy connections. The connection should be closed as soon as you're done with it,
so in this case, you should call Close explicitly when done with each result set.

2. Remember to check for error after a loop over the result set, in case loop was exited early due to some error. Don't assume that loop ended because all rows in the result set 
were successfully iterated over.

### Scan
`Scan()` performs type conversions for you based on the type of the supplied pointers; the type of the DB columns is irrelevant.

### Preparing satements
Under the hood, `db.Query()` prepares, executes and closes a prepared statement (three round-trips to the DB)
Using prepared statements with placeholders can save on a lot of round trips (n-1 for n similar queries)
Usual DB connection communication flow is:
1. Client sends prepared statement
2. DB responds with statement ID
3. Client makes queries using statement ID and values for placeholders
Thus, prepared statements are bound to a given connection.
`database/sql` manages a pool of connections for you. A `Stmt` will remember its underlying connection, but doesn't reserve it, so it's possible that a given statement's connection becomes
busy or closed in between queries. If this happens, the Stmt will get another connection from the pool and re-prepares the statement. High-concurrency usage of the DB can therefore lead to
a lot of statement re-preparation, affecting performance.

### Prepared statements and transactions
When you create a `Tx`, it is bound to one and only one connection.
Therefore, statements prepared in a Tx can't be used separately from it, and prepared statements created on a DB can't be used within a transaction, because they will be bound to a different
connection.

There is a `Tx.Stmt()` method which basically copies a stmt prepared outside a Tx into the Tx. The behavior and implementation are inefficient as the stmt is re-prepared each time it 
is executed, and so it's not advisable to use this.
