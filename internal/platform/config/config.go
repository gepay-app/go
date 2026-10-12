package config

import (
	"fmt"
	"github.com/caarlos0/env/v11"
	"strings"
	"time"
)

func Load() (*App, error) {
	var app App
	if err := env.Parse(&app); err != nil {
		return nil, err
	}
	if err := app.validate(); err != nil {
		return nil, err
	}
	return &app, nil
}

type App struct {
	Environment string `env:"APP_ENV"  envDefault:"development"`
	Name        string `env:"APP_NAME" envDefault:"app"`
	Port        int    `env:"APP_PORT" envDefault:"8080"`
	LogLevel    string `env:"APP_LOG_LEVEL"`
	LogFormat   string `env:"APP_LOG_FORMAT"`
	// CORSOrigins enables CORS when set, e.g. APP_CORS_ORIGINS=http://localhost:3000,https://app.example.com
	CORSOrigins []string `env:"APP_CORS_ORIGINS"`

	Database Database `envPrefix:"DB_"`
	Redis    Redis    `envPrefix:"REDIS_"`
	Firebase Firebase `envPrefix:"FIREBASE_"`
}
type Database struct {
	URL             string        `env:"URL"` // wajib
	MaxConns        int32         `env:"MAX_CONNS" envDefault:"25"`
	MinConns        int32         `env:"MIN_CONNS" envDefault:"2"`
	ConnMaxLifetime time.Duration `env:"CONN_MAX_LIFETIME"  envDefault:"30m"`
	ConnMaxIdleTime time.Duration `env:"CONN_MAX_IDLE_TIME" envDefault:"10m"`
}

type Redis struct {
	Addr     string `env:"ADDR" envDefault:"localhost:6379"`
	Username string `env:"USERNAME"`
	Password string `env:"PASSWORD"`
	DB       int    `env:"DB"   envDefault:"0"`
}

type Firebase struct {
	CredentialsFile string `env:"CREDENTIALS_FILE"` // wajib, mis. secrets/service-account.json
	ProjectID       string `env:"PROJECT_ID"`       // opsional; kosong = dari file
}

func (app *App) validate() error {
	if strings.TrimSpace(app.Database.URL) == "" {
		return fmt.Errorf("DB_URL is required")
	}
	if app.Port <= 0 || app.Port > 65535 {
		return fmt.Errorf("APP_PORT must be between 1 and 65535, got %d", app.Port)
	}
	switch strings.ToLower(strings.TrimSpace(app.LogLevel)) {
	case "", "debug", "info", "warn", "warning", "error":
	default:
		return fmt.Errorf("APP_LOG_LEVEL must be one of debug|info|warn|error (or empty for auto), got %q", app.LogLevel)
	}
	switch strings.ToLower(strings.TrimSpace(app.LogFormat)) {
	case "", "json", "text":
	default:
		return fmt.Errorf("APP_LOG_FORMAT must be one of json|text (or empty for auto), got %q", app.LogFormat)
	}
	if strings.TrimSpace(app.Firebase.CredentialsFile) == "" {
		return fmt.Errorf("FIREBASE_CREDENTIALS_FILE is required")
	}
	return nil
}
