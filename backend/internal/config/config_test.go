package config

import (
	"testing"
	"time"
)

// clearEnv removes every variable Load reads so the test sees pure defaults.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PORT", "DB_PATH", "USER_AGENT", "CORS_ALLOW_ORIGIN", "AI_SUMMARY_API_KEY",
		"AI_SUMMARY_MODEL", "AI_SUMMARY_BASE_URL", "AI_SUMMARY_PROVIDER",
		"ADMIN_USERNAME", "ADMIN_PASSWORD",
		"ADMIN_TOKEN_SECRET", "SCRAPE_MAX_PER_SOURCE", "SCRAPE_MAX_INSERT_PER_SOURCE",
		"SCRAPE_TIMEOUT_SECONDS",
		"SCRAPE_MIN_CONTENT_CHARS", "SCRAPE_DELAY_MIN_SECONDS", "SCRAPE_DELAY_MAX_SECONDS",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.UserAgent == "" {
		t.Error("UserAgent empty, want default NeuralwireBot dev UA")
	}
	if cfg.ScrapeMaxPerSource != 5 {
		t.Errorf("ScrapeMaxPerSource = %d, want 5", cfg.ScrapeMaxPerSource)
	}
	if cfg.ScrapeMaxInsertPerSource != 5 {
		t.Errorf("ScrapeMaxInsertPerSource = %d, want 5", cfg.ScrapeMaxInsertPerSource)
	}
	if cfg.ScrapeTimeout != 15*time.Second {
		t.Errorf("ScrapeTimeout = %v, want 15s", cfg.ScrapeTimeout)
	}
	if cfg.ScrapeMinContentChars != 500 {
		t.Errorf("ScrapeMinContentChars = %d, want 500", cfg.ScrapeMinContentChars)
	}
	if cfg.ScrapeDelayMin != 1*time.Second {
		t.Errorf("ScrapeDelayMin = %v, want 1s", cfg.ScrapeDelayMin)
	}
	if cfg.ScrapeDelayMax != 2*time.Second {
		t.Errorf("ScrapeDelayMax = %v, want 2s", cfg.ScrapeDelayMax)
	}
	if cfg.AdminUsername != "admin" || cfg.AdminPassword != "admin123" {
		t.Errorf("admin defaults = %q/%q, want admin/admin123", cfg.AdminUsername, cfg.AdminPassword)
	}
}

func TestLoadRespectsExplicitEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("SCRAPE_MIN_CONTENT_CHARS", "1200")
	t.Setenv("SCRAPE_TIMEOUT_SECONDS", "7")
	t.Setenv("SCRAPE_MAX_PER_SOURCE", "3")
	t.Setenv("SCRAPE_MAX_INSERT_PER_SOURCE", "7")
	t.Setenv("SCRAPE_DELAY_MIN_SECONDS", "2")
	t.Setenv("SCRAPE_DELAY_MAX_SECONDS", "4")
	t.Setenv("USER_AGENT", "NeuralwireBot/1.0 (+https://neuralwire.com)")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.UserAgent != "NeuralwireBot/1.0 (+https://neuralwire.com)" {
		t.Errorf("UserAgent = %q, want custom UA", cfg.UserAgent)
	}
	if cfg.ScrapeMinContentChars != 1200 {
		t.Errorf("ScrapeMinContentChars = %d, want 1200", cfg.ScrapeMinContentChars)
	}
	if cfg.ScrapeTimeout != 7*time.Second {
		t.Errorf("ScrapeTimeout = %v, want 7s", cfg.ScrapeTimeout)
	}
	if cfg.ScrapeMaxPerSource != 3 {
		t.Errorf("ScrapeMaxPerSource = %d, want 3", cfg.ScrapeMaxPerSource)
	}
	if cfg.ScrapeMaxInsertPerSource != 7 {
		t.Errorf("ScrapeMaxInsertPerSource = %d, want 7", cfg.ScrapeMaxInsertPerSource)
	}
	if cfg.ScrapeDelayMin != 2*time.Second {
		t.Errorf("ScrapeDelayMin = %v, want 2s", cfg.ScrapeDelayMin)
	}
	if cfg.ScrapeDelayMax != 4*time.Second {
		t.Errorf("ScrapeDelayMax = %v, want 4s", cfg.ScrapeDelayMax)
	}
}

func TestLoadRejectsBadEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("SCRAPE_MIN_CONTENT_CHARS", "abc")
	if _, err := Load(); err == nil {
		t.Error("Load with invalid SCRAPE_MIN_CONTENT_CHARS succeeded, want error")
	}
}
