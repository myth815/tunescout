package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddress   string
	APIKey          string
	UserAgent       string
	LogLevel        slog.Level
	MaxUploadBytes  int64
	ProviderTimeout time.Duration
	SearchTimeout   time.Duration
	RequestTimeout  time.Duration

	MusicBrainzBaseURL string
	ITunesBaseURL      string
	LRCLIBBaseURL      string
	AudiusBaseURL      string
	JamendoBaseURL     string
	JamendoClientID    string
	AcoustIDBaseURL    string
	AcoustIDAPIKey     string
	AudDBaseURL        string
	AudDAPIToken       string
}

func FromEnv() Config {
	return Config{
		ListenAddress:      env("TUNESCOUT_LISTEN", ":8080"),
		APIKey:             strings.TrimSpace(os.Getenv("TUNESCOUT_API_KEY")),
		UserAgent:          env("TUNESCOUT_USER_AGENT", "TuneScout/0.1 (+https://github.com/myth815/tunescout)"),
		LogLevel:           logLevel(env("TUNESCOUT_LOG_LEVEL", "info")),
		MaxUploadBytes:     int64(envInt("TUNESCOUT_MAX_UPLOAD_MB", 32)) * 1024 * 1024,
		ProviderTimeout:    envDuration("TUNESCOUT_PROVIDER_TIMEOUT", 12*time.Second),
		SearchTimeout:      envDuration("TUNESCOUT_SEARCH_TIMEOUT", 25*time.Second),
		RequestTimeout:     envDuration("TUNESCOUT_REQUEST_TIMEOUT", 45*time.Second),
		MusicBrainzBaseURL: strings.TrimRight(env("MUSICBRAINZ_BASE_URL", "https://musicbrainz.org"), "/"),
		ITunesBaseURL:      strings.TrimRight(env("ITUNES_BASE_URL", "https://itunes.apple.com"), "/"),
		LRCLIBBaseURL:      strings.TrimRight(env("LRCLIB_BASE_URL", "https://lrclib.net"), "/"),
		AudiusBaseURL:      strings.TrimRight(env("AUDIUS_BASE_URL", "https://api.audius.co"), "/"),
		JamendoBaseURL:     strings.TrimRight(env("JAMENDO_BASE_URL", "https://api.jamendo.com"), "/"),
		JamendoClientID:    strings.TrimSpace(os.Getenv("JAMENDO_CLIENT_ID")),
		AcoustIDBaseURL:    strings.TrimRight(env("ACOUSTID_BASE_URL", "https://api.acoustid.org"), "/"),
		AcoustIDAPIKey:     strings.TrimSpace(os.Getenv("ACOUSTID_API_KEY")),
		AudDBaseURL:        strings.TrimRight(env("AUDD_BASE_URL", "https://api.audd.io"), "/"),
		AudDAPIToken:       strings.TrimSpace(os.Getenv("AUDD_API_TOKEN")),
	}
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func logLevel(value string) slog.Level {
	switch strings.ToLower(value) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
