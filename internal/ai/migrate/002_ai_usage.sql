CREATE TABLE IF NOT EXISTS ai_usage_daily (
  user_id BIGINT NOT NULL,
  day DATE NOT NULL,
  chat_count INT NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, day)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
