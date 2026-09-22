-- 017: shares_outstanding_pit — point-in-time share counts from SEC EDGAR
-- (Phase 2 §3.2).
--
-- WHY
--
-- The §3.2 gates test market cap. Market cap was TODAY's value, applied to
-- every historical row back to 2016. That is lookahead in the selection of the
-- candidate set itself, not merely in a feature: which historical setups were
-- ever studied, and which bucket each landed in, depended on what the company
-- later became.
--
-- Measured on a 71-symbol sample (Phase 2 §3.2.1): **20.1% of historical
-- bar-days change bucket** under a point-in-time cap, and **12.1% of
-- gate-passing candidates**. 2.51% of bar-days were excluded from the universe
-- altogether by today's cap while being legitimate market-bucket candidates on
-- the day — those setups are not in the dataset at all, and no re-weighting
-- recovers them.
--
-- THE AS-OF RULE, WHICH IS THE WHOLE POINT OF THE TABLE
--
-- Join on `filed_date`, NEVER on `period_end`. The period end precedes the
-- filing by weeks, so joining on it would credit us with knowing a share count
-- before it was public — replacing one lookahead with a subtler one. Both
-- columns are stored so the distinction stays auditable, and the primary key is
-- on `filed_date` to make the correct join the natural one.
--
-- MULTI-CLASS
--
-- 19.7% of sampled symbols report several facts sharing one `filed_date`, one
-- per share class. Rows are stored per (symbol, filed_date) with the classes
-- SUMMED, and `multi_class` set true so those symbols can be excluded in a
-- sensitivity check. Summing all classes and multiplying by the traded class's
-- price is the standard market-cap approximation; it is an approximation
-- because non-traded classes may carry different economics, which is why the
-- flag exists rather than the decision being buried.

CREATE TABLE IF NOT EXISTS shares_outstanding_pit (
    symbol      TEXT        NOT NULL,
    filed_date  DATE        NOT NULL,
    shares      DOUBLE PRECISION NOT NULL,

    cik         TEXT        NOT NULL,
    period_end  DATE,
    form        TEXT,
    multi_class BOOLEAN     NOT NULL DEFAULT false,
    class_count INTEGER     NOT NULL DEFAULT 1,
    source      TEXT        NOT NULL DEFAULT 'sec_edgar_companyconcept',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (symbol, filed_date)
);

CREATE INDEX IF NOT EXISTS sop_asof ON shares_outstanding_pit (symbol, filed_date DESC);

COMMENT ON TABLE shares_outstanding_pit IS
'Point-in-time common shares outstanding from SEC EDGAR
(dei:EntityCommonStockSharesOutstanding via the XBRL companyconcept API).
Consumed by gate v2 as raw_close[t] x shares(filed_date <= t). Both factors must
be UNADJUSTED: multiplying an adjusted price by an unadjusted share count is
wrong by the cumulative split factor, which is 10-100x for reverse-split penny
names.';

COMMENT ON COLUMN shares_outstanding_pit.filed_date IS
'The date the fact became PUBLIC. This is the only column an as-of join may use.
Joining on period_end instead would use information weeks before it was
available, which is the lookahead this table exists to remove.';

COMMENT ON COLUMN shares_outstanding_pit.period_end IS
'The date the fact is ABOUT. Stored for auditing only. Never join on it.';

COMMENT ON COLUMN shares_outstanding_pit.multi_class IS
'True when the filing reported more than one share class on this filed_date and
`shares` is their SUM. Set so the gate-v2 sensitivity check can exclude these
symbols: summing classes and pricing them all at the traded class''s price is
the standard approximation, not a fact.';

-- Coverage is deliberately NOT complete, and the gate must treat absence as
-- absence. A symbol-day with no filing on or before t is UNMEASURABLE: gate v2
-- fails it with market_cap_pit_unavailable rather than falling back to today's
-- value, because a fallback would reintroduce exactly the leak being removed
-- for precisely the rows where it cannot be checked.
