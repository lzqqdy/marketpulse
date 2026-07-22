CREATE TABLE IF NOT EXISTS portfolio_settings (
  user_id BIGINT NOT NULL PRIMARY KEY,
  principal_cny DECIMAL(18,2) NOT NULL DEFAULT 0,
  principal_usdt DECIMAL(18,8) NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS portfolio_holdings (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  asset_type VARCHAR(16) NOT NULL,
  symbol VARCHAR(32) NOT NULL,
  quantity DECIMAL(24,8) NOT NULL DEFAULT 0,
  target_price DECIMAL(24,8) NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE KEY uk_portfolio_holdings_user_asset (user_id, asset_type, symbol),
  KEY idx_portfolio_holdings_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS portfolio_snapshots (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  date DATE NOT NULL,
  kind VARCHAR(16) NOT NULL DEFAULT 'daily',
  total_value DECIMAL(18,8) NOT NULL DEFAULT 0,
  total_value_cny DECIMAL(18,2) NOT NULL DEFAULT 0,
  daily_profit DECIMAL(18,2) NOT NULL DEFAULT 0,
  daily_profit_rate DECIMAL(18,8) NOT NULL DEFAULT 0,
  total_profit DECIMAL(18,2) NOT NULL DEFAULT 0,
  total_profit_rate DECIMAL(18,8) NOT NULL DEFAULT 0,
  asset_detail JSON NULL,
  source VARCHAR(16) NOT NULL DEFAULT 'system',
  created_at DATETIME NOT NULL,
  UNIQUE KEY uk_portfolio_snapshots_user_date_kind (user_id, date, kind),
  KEY idx_portfolio_snapshots_user_date (user_id, date),
  KEY idx_portfolio_snapshots_user_kind_date (user_id, kind, date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
