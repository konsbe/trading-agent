-- 024: user watchlists, written by momentum-api's watchlist endpoints.
--
-- The one write path next to the read-only scanner API
-- (docs/MOMENTUM_SCANNER_API.md §2.5). It stores which symbols a user chose to
-- follow; it never touches the scanner's tables.
--
-- OWNER
--
-- owner_sub is the identity provider's subject for the signed-in user. There is
-- no auth yet, so every row is written with owner_sub NULL: the single shared
-- "unauthenticated" watchlist. Once auth exists, a signed-in user's rows carry
-- their subject and they see only those; NULL rows stay the unauthenticated
-- list. owner_sub is never guessed or defaulted to a placeholder string, so
-- "nobody signed in" cannot be confused with a real account named that way.
--
-- One row per (owner, symbol). NULL owners compare as distinct in a plain
-- UNIQUE constraint, so uniqueness is enforced on COALESCE(owner_sub, '').

CREATE TABLE IF NOT EXISTS watchlist_items (
    id          BIGSERIAL    PRIMARY KEY,
    owner_sub   TEXT         NULL,
    symbol      TEXT         NOT NULL,
    added_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT watchlist_items_symbol_upper CHECK (symbol = upper(symbol) AND symbol <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS watchlist_items_owner_symbol
    ON watchlist_items (COALESCE(owner_sub, ''), symbol);

CREATE INDEX IF NOT EXISTS watchlist_items_owner_added
    ON watchlist_items (owner_sub, added_at DESC);
