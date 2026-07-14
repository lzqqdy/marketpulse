package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_mysqlRedisDefaults(t *testing.T) {
	path := filepath.Join("..", "..", "config", "config.example.yaml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MySQL.Enabled || cfg.Redis.Enabled {
		t.Fatalf("persistence should be disabled by default: mysql=%v redis=%v", cfg.MySQL.Enabled, cfg.Redis.Enabled)
	}
	if cfg.MySQL.Host != "127.0.0.1" || cfg.MySQL.Port != 3306 || cfg.MySQL.Database != "marketpulse" {
		t.Fatalf("mysql defaults: %+v", cfg.MySQL)
	}
	if cfg.MySQL.MaxOpenConns != 20 || cfg.MySQL.ConnMaxLifetime != time.Hour {
		t.Fatalf("mysql pool defaults: %+v", cfg.MySQL)
	}
	if cfg.Redis.Addr != "127.0.0.1:6379" || cfg.Redis.PoolSize != 10 {
		t.Fatalf("redis defaults: %+v", cfg.Redis)
	}
	dsn := cfg.MySQL.DSN()
	if dsn != "root@tcp(127.0.0.1:3306)/marketpulse?parseTime=true&loc=Local&charset=utf8mb4" {
		t.Fatalf("mysql dsn: %s", dsn)
	}
}

func TestLoad_mysqlRedisEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `
app:
  mode: debug
symbols:
  - BTC
mysql:
  enabled: false
  host: 10.0.0.1
redis:
  enabled: false
  addr: 10.0.0.2:6380
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MARKETPULSE_MYSQL_ENABLED", "true")
	t.Setenv("MARKETPULSE_MYSQL_PASSWORD", "s3cret")
	t.Setenv("MARKETPULSE_MYSQL_DATABASE", "mp_prod")
	t.Setenv("MARKETPULSE_REDIS_ENABLED", "1")
	t.Setenv("MARKETPULSE_REDIS_ADDR", "redis.internal:6379")
	t.Setenv("MARKETPULSE_REDIS_DB", "2")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MySQL.Enabled || cfg.MySQL.Password != "s3cret" || cfg.MySQL.Database != "mp_prod" {
		t.Fatalf("mysql env: %+v", cfg.MySQL)
	}
	if !cfg.Redis.Enabled || cfg.Redis.Addr != "redis.internal:6379" || cfg.Redis.DB != 2 {
		t.Fatalf("redis env: %+v", cfg.Redis)
	}
}

func TestValidate_mysqlEnabledRequiresHost(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{Mode: "debug"},
		Symbols: []string{"BTC"},
		MySQL: MySQLConfig{
			Enabled:  true,
			Host:     "",
			Database: "marketpulse",
		},
	}
	cfg.applyDefaults()
	// applyDefaults fills host; clear for this case
	cfg.MySQL.Host = ""
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error when mysql.enabled and host empty")
	}
}

func TestValidate_redisEnabledRequiresAddr(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{Mode: "debug"},
		Symbols: []string{"BTC"},
		Redis: RedisConfig{
			Enabled: true,
			Addr:    "",
		},
	}
	cfg.applyDefaults()
	cfg.Redis.Addr = ""
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error when redis.enabled and addr empty")
	}
}
