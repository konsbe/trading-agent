-- 018: redefine the Phase 2 lockbox as a REGION rather than a fixed row list.
--
-- WHY THE ROW LIST NO LONGER WORKS
--
-- The lockbox was reserved as 1,783 specific gate-passing rows. Gate v2
-- (point-in-time market cap) changes WHICH symbol-days pass §3.2, so under the
-- new gates that row list is neither a superset nor a subset of the candidates
-- in its own window. Keeping it would produce two bad options:
--
--   * exclude exactly those 1,783 rows -> new gate-v2 candidates inside the
--     window leak into the training data;
--   * re-derive rows and call it the same lockbox -> the manifest hash no
--     longer describes the data, so nothing can be verified.
--
-- WHY REDEFINING IS SAFE HERE, AND WOULD NOT BE LATER
--
-- The guarantee a lockbox provides is "no decision has been informed by these
-- outcomes". That holds for the REGION, not merely for the row list: no report,
-- threshold search or model fit has looked at any outcome inside
-- 2025-03-28 .. 2026-03-27 for a non-pilot symbol. The region was excluded
-- wholesale from every report produced so far.
--
-- So the region can be restated without weakening anything, and membership can
-- be re-derived mechanically under any gate version. **This is a one-time
-- correction made possible by the fact that the window has never been
-- inspected.** Once an outcome inside it has been seen, no redefinition is
-- legitimate, and the §4.3 rule stands: a modified model is tested on forward
-- sessions that accrue after the reservation date, not on a re-cut lockbox.
--
-- WHAT THE REGION IS
--
--   date   in [2025-03-28, 2026-03-27]   (unchanged)
--   symbol NOT IN momentum_pilot_cohort  (unchanged)
--
-- Identical boundaries to the original reservation. Only the unit of membership
-- changes: a region, evaluated per query, instead of a frozen row list.

CREATE TABLE IF NOT EXISTS phase2_lockbox_region (
    region_key  TEXT PRIMARY KEY,
    start_date  DATE        NOT NULL,
    end_date    DATE        NOT NULL,
    exclude_cohort TEXT     NOT NULL,
    reserved_at TIMESTAMPTZ NOT NULL,
    definition  TEXT        NOT NULL,
    supersedes  TEXT
);

COMMENT ON TABLE phase2_lockbox_region IS
'Phase 2 §4.3 lockbox, defined as a REGION (date window x symbols outside the
in-sample pilot cohort). Membership is re-derived from this definition under
whatever gate version is in force, so a change to the gates cannot silently
move the boundary of the held-out data. Supersedes the fixed row list in
phase2_lockbox, which is retained for audit.';

COMMENT ON COLUMN phase2_lockbox_region.exclude_cohort IS
'Name of the cohort in momentum_cohort_manifest whose symbols are excluded from
the region -- the in-sample pilot. Named rather than inlined so the exclusion
and its content hash cannot drift apart.';

INSERT INTO phase2_lockbox_region
    (region_key, start_date, end_date, exclude_cohort, reserved_at, definition, supersedes)
VALUES (
    'phase2_lockbox_v2',
    '2025-03-28',
    '2026-03-27',
    'phase1_pilot_450',
    '2026-09-21 00:00:00+00',
    'Region lockbox: every candidate whose date falls in [2025-03-28, 2026-03-27] '
    'AND whose symbol is not in momentum_pilot_cohort, under whatever gate version '
    'is in force. Boundaries identical to the 2026-09-21 row-list reservation; only '
    'the unit of membership changed, because gate v2 alters which symbol-days pass '
    'Phase 1 section 3.2. Safe because no outcome inside this window has ever been '
    'inspected -- the region was excluded wholesale from every report to date.',
    'phase2_lockbox (row list, 1783 rows, sha256 '
    '104cce0836a12af00242b31de3df5b53175febf8b5cf1024e40f74792feb495c)'
)
ON CONFLICT (region_key) DO NOTHING;

-- The region's own hash is over its DEFINITION, not over member rows, because
-- the membership is intentionally derived. A hash over rows would reintroduce
-- exactly the brittleness this migration removes.
INSERT INTO momentum_cohort_manifest
    (cohort_key, symbol_count, content_hash, definition)
SELECT 'phase2_lockbox_region',
       0,
       md5(region_key || '|' || start_date || '|' || end_date || '|' || exclude_cohort),
       definition
FROM phase2_lockbox_region WHERE region_key = 'phase2_lockbox_v2'
ON CONFLICT (cohort_key) DO NOTHING;

COMMENT ON TABLE phase2_lockbox IS
'SUPERSEDED 2026-09-21 by phase2_lockbox_region. Retained as the audit record of
the original 2026-09-21 reservation: 1,783 gate-v1 rows, 1,037 symbols, sha256
104cce0836a12af00242b31de3df5b53175febf8b5cf1024e40f74792feb495c. Do NOT use
for exclusion -- gate v2 changes which symbol-days pass, so this row list no
longer covers its own window. Exclusion goes through the region.';
