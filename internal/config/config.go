package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL   string
	ListenAddr    string
	UploadDir     string
	BaseURL       string
	SessionSecret []byte
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		ListenAddr:  envOr("LISTEN_ADDR", "127.0.0.1:8080"),
		UploadDir:   envOr("UPLOAD_DIR", "./data/uploads"),
		BaseURL:     strings.TrimRight(envOr("BASE_URL", "http://localhost:8080"), "/"),
	}
	secret := os.Getenv("SESSION_SECRET")
	var errs []error
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if len(secret) < 32 {
		errs = append(errs, errors.New("SESSION_SECRET must be at least 32 characters"))
	}
	if u, err := url.Parse(cfg.BaseURL); err != nil || u.Scheme == "" || u.Host == "" {
		errs = append(errs, fmt.Errorf("BASE_URL %q is not an absolute URL", cfg.BaseURL))
	}
	cfg.SessionSecret = []byte(secret)
	return cfg, errors.Join(errs...)
}

func (c Config) SecureCookies() bool {
	return strings.HasPrefix(c.BaseURL, "https://")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
