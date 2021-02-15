# Implementation notes

## Database driver
1. In order to be DB agnostic, need to have the client code import the driver and supply the `sql.Open()` arguments (driver name, connection string)
2. All `gq` code should be driver agnostic, only relying on `database/sql` idioms (and maybe sqlx)
3. How to have driver-agnostic column types? Maybe this is default, need to keep reading
