# ipfs-ks-postgres

> An implementation of [the keystore interface](https://github.com/ipfs/go-ipfs-keystore) for PostgreSQL that uses the [pgx](https://github.com/jackc/pgx) PostgreSQL driver.

## Install

```sh
go get github.com/4everland/ipfs-ks-postgres
```

## Usage

Ensure a database is created and a table exists that has the following structure (replacing `table_name` with the name of the table the datastore will use - by default this is `blocks`):

```sql
CREATE TABLE IF NOT EXISTS table_name (name TEXT NOT NULL PRIMARY KEY, data BYTEA)
```

Import and use in your application:

```go
package main

import (
	"context"
	pgks "github.com/4everland/ipfs-ks-postgres"
)

const (
	connString = "postgresql://user:pass@host:12345/database?sslmode=require"
	tableName  = "keys" // (default)
)

func main() {
	ks, err := pgks.NewKeystore(context.Background(), connString, pgks.Table(tableName))
	if err != nil {
		panic(err)
	}
}
```

## License

[MIT](LICENSE)