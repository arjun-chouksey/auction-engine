# Database migrations

Versioned `*.up.sql` and `*.down.sql` files live in this directory. The root
`Makefile` wires the pinned `golang-migrate` runner:

```bash
make migrate-up
make migrate-down
```

`000001_initial_schema` creates the users, RFQs, immutable bids, and auction
events tables with their approved constraints and indexes.

