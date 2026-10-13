ALTER TABLE portfolio_snapshots
  ADD COLUMN valued_at_ms BIGINT NULL,
  ADD COLUMN has_daily_baseline BOOLEAN NULL;
