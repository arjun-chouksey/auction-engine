CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name TEXT NOT NULL CHECK (length(trim(full_name)) > 0),
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('BUYER', 'SUPPLIER')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_lower_uq ON users (lower(email));

CREATE TABLE rfqs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reference_code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    buyer_id UUID NOT NULL REFERENCES users(id),
    currency_code CHAR(3) NOT NULL DEFAULT 'INR',
    service_date DATE NOT NULL,
    bid_start_at TIMESTAMPTZ NOT NULL,
    initial_close_at TIMESTAMPTZ NOT NULL,
    current_close_at TIMESTAMPTZ NOT NULL,
    forced_close_at TIMESTAMPTZ NOT NULL,
    trigger_window_minutes INTEGER NOT NULL CHECK (trigger_window_minutes > 0),
    extension_minutes INTEGER NOT NULL CHECK (extension_minutes > 0),
    trigger_type TEXT NOT NULL CHECK (
        trigger_type IN ('ANY_BID', 'ANY_RANK_CHANGE', 'L1_CHANGE')
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT rfq_nonempty_name CHECK (length(trim(name)) > 0),
    CONSTRAINT rfq_nonempty_reference CHECK (length(trim(reference_code)) > 0),
    CONSTRAINT rfq_initial_times CHECK (bid_start_at < initial_close_at),
    CONSTRAINT rfq_force_after_initial CHECK (initial_close_at < forced_close_at),
    CONSTRAINT rfq_current_bounded CHECK (
        current_close_at >= initial_close_at
        AND current_close_at <= forced_close_at
    )
);

CREATE TABLE bids (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    rfq_id UUID NOT NULL REFERENCES rfqs(id),
    supplier_id UUID NOT NULL REFERENCES users(id),
    idempotency_key UUID NOT NULL,
    carrier_name TEXT NOT NULL CHECK (length(trim(carrier_name)) > 0),
    freight_minor BIGINT NOT NULL CHECK (freight_minor >= 0),
    origin_minor BIGINT NOT NULL CHECK (origin_minor >= 0),
    destination_minor BIGINT NOT NULL CHECK (destination_minor >= 0),
    total_minor BIGINT GENERATED ALWAYS AS
        (freight_minor + origin_minor + destination_minor) STORED,
    transit_time_hours INTEGER NOT NULL CHECK (transit_time_hours > 0),
    quote_valid_until DATE NOT NULL,
    submitted_at TIMESTAMPTZ NOT NULL,
    UNIQUE (rfq_id, supplier_id, idempotency_key),
    CONSTRAINT positive_bid_total CHECK (total_minor > 0)
);

CREATE TABLE auction_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    rfq_id UUID NOT NULL REFERENCES rfqs(id),
    bid_id BIGINT NULL REFERENCES bids(id),
    actor_id UUID NULL REFERENCES users(id),
    event_type TEXT NOT NULL CHECK (
        event_type IN ('RFQ_CREATED', 'BID_ACCEPTED', 'AUCTION_EXTENDED')
    ),
    event_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX rfqs_buyer_created_idx ON rfqs (buyer_id, created_at DESC);
CREATE INDEX rfqs_created_idx ON rfqs (created_at DESC, id);
CREATE INDEX bids_rfq_supplier_latest_idx
    ON bids (rfq_id, supplier_id, submitted_at DESC, id DESC);
CREATE INDEX bids_rfq_chronological_idx
    ON bids (rfq_id, submitted_at DESC, id DESC);
CREATE INDEX bids_rfq_price_idx
    ON bids (rfq_id, total_minor ASC, submitted_at ASC, id ASC);
CREATE INDEX events_rfq_history_idx
    ON auction_events (rfq_id, created_at DESC, id DESC);
