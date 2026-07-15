CREATE TABLE IF NOT EXISTS alert_rules (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  asset_type VARCHAR(32) NOT NULL,
  symbol VARCHAR(64) NOT NULL,
  field VARCHAR(32) NOT NULL DEFAULT 'price',
  rule_type TINYINT UNSIGNED NOT NULL,
  params JSON NOT NULL,
  channels JSON NOT NULL,
  frequency VARCHAR(16) NOT NULL,
  interval_minutes INT UNSIGNED NOT NULL DEFAULT 10,
  set_price DECIMAL(24,8) NOT NULL,
  status VARCHAR(16) NOT NULL,
  last_triggered_at BIGINT UNSIGNED NULL,
  trigger_count INT UNSIGNED NOT NULL DEFAULT 0,
  created_at BIGINT UNSIGNED NOT NULL,
  updated_at BIGINT UNSIGNED NOT NULL,
  is_deleted TINYINT NOT NULL DEFAULT 0,
  KEY idx_alert_rules_user (user_id, is_deleted, status),
  KEY idx_alert_rules_symbol (asset_type, symbol, status, is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS alert_deliveries (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  rule_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  asset_type VARCHAR(32) NOT NULL,
  symbol VARCHAR(64) NOT NULL,
  rule_type TINYINT UNSIGNED NOT NULL,
  channel VARCHAR(16) NOT NULL,
  trigger_value DECIMAL(24,8) NOT NULL,
  title VARCHAR(256) NOT NULL,
  body VARCHAR(1024) NOT NULL,
  status VARCHAR(16) NOT NULL,
  error_msg VARCHAR(512) NOT NULL DEFAULT '',
  created_at BIGINT UNSIGNED NOT NULL,
  KEY idx_alert_deliveries_user (user_id, created_at),
  KEY idx_alert_deliveries_rule (rule_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
