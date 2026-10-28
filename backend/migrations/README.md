# Database migrations

Versioned `*.up.sql` and `*.down.sql` files belong in this directory. The root
`Makefile` wires the pinned `golang-migrate` runner so migrations can be applied
with `make migrate-up` once the first migration exists.

