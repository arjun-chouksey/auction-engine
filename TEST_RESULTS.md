# Verification results

Observed on 2026-10-28 using the pinned Docker Compose, Go, PostgreSQL, Node, and frontend toolchain in this repository.

## Mandatory matrix

| IDs | Result | Evidence |
|---|---|---|
| RFQ-01 | PASS | `TestAuthenticationAndRFQAPI/buyer_creates_valid_RFQ` |
| RFQ-02, RFQ-03 | PASS | RFQ invalid-order and non-positive-window HTTP subtests plus schema constraints |
| RFQ-04 | PASS | Duplicate-reference HTTP subtest |
| AUTH-01 | PASS | Buyer-create and supplier-forbidden HTTP subtests |
| AUTH-02 | PASS | `TestBidSubmissionRankingAndHistory`; buyer rejected and supplier accepted |
| AUTH-03 | PASS | Missing/invalid token HTTP and SSE tests |
| BID-01, BID-02 | PASS | Three-supplier bid integration flow, including supplier improvement while not L1 |
| BID-03 | PASS | Equal quote returns `BID_NOT_IMPROVED`; bid/event totals remain unchanged |
| BID-04 | PASS | Latest-only ranking and immutable price-sorted five-row history assertions |
| BID-05 | PASS | Integration timestamp tie plus deterministic domain supplier-ID tie test |
| BID-06, BID-07 | PASS | Negative, zero, overflow, transit, carrier, and validity rejection cases with zero side effects |
| BID-08, BID-09 | PASS | Replay returns the same bid; changed payload returns `IDEMPOTENCY_KEY_REUSED` |
| TIME-01–TIME-06 | PASS | Nanosecond/exact-boundary unit tests and post-lock database-time integration test |
| EXT-01–EXT-08 | PASS | Table-driven trigger-policy and trigger-window unit tests plus policy integration tests |
| EXT-09–EXT-12 | PASS | Repeated/capped extension, closed auction, and replay-after-close tests |
| STATUS-01, STATUS-02 | PASS | Natural versus forced-cap status unit tests |
| RACE-01–RACE-04 | PASS | Concurrent suppliers, same supplier, same key, extensions, and post-lock close tests; race detector passed |
| EVENT-01 | PASS | Bid/extension event ordering and structured metadata integration assertions |
| EVENT-02 | PASS | Rejected bid produces no event or hub signal; service emits only after successful non-replay commit |
| EVENT-03 | PASS | Browser parser handles chunked SSE and reconnects after clean/error termination; focus/polling refetch remains enabled |
| UI-01 | PARTIAL | Buyer create and supplier role screens were exercised visibly; three-supplier ranking/history is integration-tested, but one uninterrupted three-browser-session walkthrough was not recorded |
| UI-02 | PASS | Browser displayed explicit `Asia/Calcutta` labels while API/database timestamps remained RFC3339/UTC |
| UI-03 | PARTIAL | Database persistence, detail refetch, reconnect, and polling are verified; a visible refresh immediately after a real extension was not recorded |

## Commands and observed results

- `make backend-lint` — PASS (`gofmt` clean and `go vet ./...` clean).
- `make backend-test` — PASS (all Go package tests).
- `make backend-integration-test` — PASS against real PostgreSQL; all integration tests passed.
- `CGO_ENABLED=1 go test -race -tags=integration -count=1 ./tests/integration` in the disposable integration container — PASS in 8.133s.
- `npm run lint` — PASS.
- `npm test` — PASS, 5 files and 11 tests.
- `npm run build` — PASS, production Vite bundle generated.
- `docker compose up -d --build` plus frontend, proxied health, and supplier-login requests — PASS after bounded startup retry; UI/API/DB running and API/DB healthy.

## Explicit boundaries

This evidence is not a production load test, penetration test, formal accessibility certification, multi-replica SSE test, or multi-region reliability test. See `README.md` for product limitations.
