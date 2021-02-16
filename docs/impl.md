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
1. If a func name includes `Query`, it is designed to ask questions of the database, and will return a set of rows, even if it's empty.
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

### Modifying data/working with transactions
#### Statements that modify data
1. Use `Exec()`, preferably with a prepared statement, to accomplish an `INSERT`, `UPDATE`, `DELETE`, or another statement that doesn't return rows.
2. Executing a statement returns an `sql.Result` object which gives access to some statement metadata: `LastInsertId()` (last inserted ID) and `RowsAffected` (number of rows affected)
3. Should NEVER use `Query` for a statement which doesn't return rows: the returned `sql.Rows` will reserve a connection until the `sql.Rows` is closed.

#### Transactions
1. In Go, a transaction is essentially an object which reserves a connection to the datastore.
2. One-to-one mapping between operations which can be performed on an `sql.DB` and an `sql.Tx`, with the difference being that they are all performed on the same connection.
3. If you need to work with multiple statements that modify connection state (ex creating temporary tables, setting variables, changing connection options such as char sets or
timeouts), you need a Tx, to ensure they are all executed on the same transaction.

## Handling errors
1. Almost all operations with `database/sql` types return an error as the last value. You should always check these values, and never ignore them.

### Errors from QueryRow()
1. Need to handle the special case where the error returned is `sql.ErrNoRows`, as this isn't usually erroneous from the application code's perspective, it's just the only way for the caller to
distinguish whether `QueryRow` returned a row or not

## Working with null values
1. Try to avoid them whenever possible

## Caveats
### Multiple statement support
1. `database/sql` has no explicit multi-statement support, so behavior when executing multi-statements is backend-dependent.
2. Can't batch statements in a transaction since each statement needs to be executed serially, and the resources in the result, such as Row or Rows, must be scanned or closed so the underlying
connection is free for the next statement to use. For ex, in a non-transaction context, you could execute a query, loop over the rows and execute additional queries within the loop, since 
these inner queries can make use of other connections. In a transaction, you have only one connection, which is busy until the initial query is closed, so this doesn't work (deadlock?).

## sqlx

### prepared statement rebind
1. You can use the `sqlx.DB.Rebind(string) string` function with the `?` bindvar syntax to get a query which is suitable for excution on the current database type. This will be key
for acheiving backend-agnosticism.

### Query
1. Treat Rows returned from a Query like a database cursor, rather than a materialized list of results.
2. Iterating via `Next()` is a good way to bound memory usage, as only one row is scanned at a time.

### QueryX
1. QueryX behaves exactly as Query does, but it returns a `sqlx.Rows`, which has extended scanning capabilities, like `StructScan`, `SliceScan`, and `MapScan`
2. QueryRowX is the equivalent for QueryRow; it retuns a `sqlx.Row`.

### Get and Select
1. Get and Select are time saving extensions to the handle types. They combine query execution and scanning into one operation.
2. Get is used for fetching a single result and scanning it, while Select is used for fetching a slice of results.
3. Select can save a lot of boilerplate, but it is semantically different from Queryx, since it loads the entire result set into memory at once. If the result set is not bounded to some reasonable
size, it might be best to use the classic Queryx/StructScan iteration instead.

### Query helpers
#### "In" Queries
1. `sqlx.In` is a helper which processes IN queries and expands any bindvars which correspond to a slice in the arguments to to the length of that slice, and then appends those slice elements
to a new argslist. You can then pass this query to `sqlx.DB.Rebind()` to replace the default `?` bindvars with ones suitable for your backend. 
Check out the [example](https://jmoiron.github.io/sqlx/#inQueries)

#### Named Queries
1. Allow you to use a bindvar syntax which refers to the names of struct fields or map keys to bind variables in a query, rather than having to refer to everything positionally.
2. There are three extra query verbs related to named queries: `NamedQuery`, `NamedExec`, `NamedStmt` (for prepared statements)

#### Advanced scanning
1. Can scan into structs with embedded structs, so pieces of DB tables can be reused (eg ID, created timestamp)
2. Can scan into slices and maps

### Custom types
1. `sql.Scanner` allows you to use custom types in a Scan().
2. `driver.Valuer`allows you to use custom types in a Query/QueryRow/Exec


















































