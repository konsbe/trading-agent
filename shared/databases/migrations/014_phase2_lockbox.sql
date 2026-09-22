-- 014: the Phase 2 lockbox (spec §4.3).
--
-- WHAT IT IS
--
-- A set of candidate rows reserved BEFORE any evaluation was run, which no
-- report, no threshold search and no model fit may read. It is evaluated
-- exactly once, on the final pre-registered model, and the result stands
-- whatever it is.
--
-- WHY IT HAS TO EXIST BEFORE THE FIRST NUMBER IS LOOKED AT
--
-- Phase 1 revised the score three times against the same 218 candidates. Every
-- revision was defensible on its own and the result was still in-sample, because
-- "held out" cannot be established retroactively: once a number has been seen it
-- has informed a decision, including the decision not to change anything. The
-- only defence is reserving the data first, which is why this migration runs
-- before the post-upgrade report rather than alongside it.
--
-- DEFINITION (fixed at reservation time, recorded in the manifest row)
--
--   complete labels, AND within the most recent 12 months of complete-label
--   dates, AND the symbol is NOT in momentum_pilot_cohort.
--
-- The pilot exclusion is what makes the lockbox genuinely unseen. Pilot symbols
-- are in-sample for score v2 at every date, including dates the pilot never
-- evaluated, because the weights were chosen partly from how those symbols
-- behave. Recency matters for the other half: the most recent complete-label
-- window is the closest available stand-in for the forward data the strategy
-- would actually meet.

CREATE TABLE IF NOT EXISTS phase2_lockbox (
    symbol      TEXT        NOT NULL,
    ts          DATE        NOT NULL,
    bucket      TEXT,
    reserved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (symbol, ts)
);

COMMENT ON TABLE phase2_lockbox IS
'Phase 2 §4.3 lockbox: candidate rows reserved before any evaluation and
excluded from every report, threshold search and model fit. Read only behind an
explicit --final-lockbox-evaluation flag, exactly once, with each use logged
into §4.3 of the Phase 2 spec. A modified model does NOT get a second
evaluation here -- it must be tested on sessions that accrue after the
reservation date, because a set that has been measured against twice is a
validation set, not a lockbox.';

COMMENT ON COLUMN phase2_lockbox.ts IS
'The candidate date (the gate-passing session), stored as a DATE because
candidates are daily bars. Together with symbol it is the row key that every
dataset query excludes.';

-- The manifest row lands in momentum_cohort_manifest (migration 013) so that
-- both frozen sets -- the in-sample pilot and the held-out lockbox -- are
-- hashed the same way and in the same place. They are two halves of one
-- question: which rows may a given number have been computed from?
