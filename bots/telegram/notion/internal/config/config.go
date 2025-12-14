package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	TelegramToken            string `yaml:"telegram_token"`
	TelegramDownloadMaxBytes int64  `yaml:"telegram_download_max_bytes"`

	NotionToken     string `yaml:"notion_token"`
	NotionDatabase  string `yaml:"notion_database_id"`
	NotionTitleProp string `yaml:"notion_title_prop"`

	S3Endpoint        string `yaml:"s3_endpoint"`
	S3Region          string `yaml:"s3_region"`
	S3Bucket          string `yaml:"s3_bucket"`
	S3AccessKeyID     string `yaml:"s3_access_key_id"`
	S3SecretAccessKey string `yaml:"s3_secret_access_key"`
	S3PublicBaseURL   string `yaml:"s3_public_base_url"`
	S3KeyPrefix       string `yaml:"s3_key_prefix"`
	S3ForcePathStyle  bool   `yaml:"s3_force_path_style"`

	MaxImageBytes     int64 `yaml:"max_image_bytes"`
	ImgJPEGMinQuality int   `yaml:"img_jpeg_min_quality"`
}

func Load(configPath string) (Config, error) {
	cfg := defaultsFromEnv()

	if strings.TrimSpace(configPath) != "" {
		if b, err := os.ReadFile(configPath); err == nil {
			expanded := os.ExpandEnv(string(b))
			_ = yaml.Unmarshal([]byte(expanded), &cfg)
		}
	}

	applyEnvOverrides(&cfg)
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func defaultsFromEnv() Config {
	return Config{
		TelegramToken:            os.Getenv("TELEGRAM_TOKEN"),
		TelegramDownloadMaxBytes: 30 * 1024 * 1024,

		NotionToken:     os.Getenv("NOTION_TOKEN"),
		NotionDatabase:  os.Getenv("NOTION_DATABASE_ID"),
		NotionTitleProp: firstNonEmpty(os.Getenv("NOTION_TITLE_PROP"), "Name"),

		S3Endpoint:        os.Getenv("S3_ENDPOINT"),
		S3Region:          firstNonEmpty(os.Getenv("S3_REGION"), "auto"),
		S3Bucket:          os.Getenv("S3_BUCKET"),
		S3AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
		S3SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		S3PublicBaseURL:   os.Getenv("S3_PUBLIC_BASE_URL"),
		S3KeyPrefix:       firstNonEmpty(os.Getenv("S3_KEY_PREFIX"), "telegram-notes"),
		S3ForcePathStyle:  parseBool(os.Getenv("S3_FORCE_PATH_STYLE")),

		MaxImageBytes:     4_900_000,
		ImgJPEGMinQuality: 45,
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("TELEGRAM_TOKEN")); v != "" {
		cfg.TelegramToken = v
	}
	if v := strings.TrimSpace(os.Getenv("NOTION_TOKEN")); v != "" {
		cfg.NotionToken = v
	}
	if v := strings.TrimSpace(os.Getenv("NOTION_DATABASE_ID")); v != "" {
		cfg.NotionDatabase = v
	}
	if v := strings.TrimSpace(os.Getenv("NOTION_TITLE_PROP")); v != "" {
		cfg.NotionTitleProp = v
	}
	if v := strings.TrimSpace(os.Getenv("S3_ENDPOINT")); v != "" {
		cfg.S3Endpoint = v
	}
	if v := strings.TrimSpace(os.Getenv("S3_REGION")); v != "" {
		cfg.S3Region = v
	}
	if v := strings.TrimSpace(os.Getenv("S3_BUCKET")); v != "" {
		cfg.S3Bucket = v
	}
	if v := strings.TrimSpace(os.Getenv("S3_ACCESS_KEY_ID")); v != "" {
		cfg.S3AccessKeyID = v
	}
	if v := strings.TrimSpace(os.Getenv("S3_SECRET_ACCESS_KEY")); v != "" {
		cfg.S3SecretAccessKey = v
	}
	if v := strings.TrimSpace(os.Getenv("S3_PUBLIC_BASE_URL")); v != "" {
		cfg.S3PublicBaseURL = v
	}
	if v := strings.TrimSpace(os.Getenv("S3_KEY_PREFIX")); v != "" {
		cfg.S3KeyPrefix = v
	}
	if v := strings.TrimSpace(os.Getenv("S3_FORCE_PATH_STYLE")); v != "" {
		cfg.S3ForcePathStyle = parseBool(v)
	}

	if v := strings.TrimSpace(os.Getenv("MAX_IMAGE_BYTES")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxImageBytes = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("TG_DOWNLOAD_MAX_BYTES")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.TelegramDownloadMaxBytes = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("IMG_JPEG_MIN_QUALITY")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ImgJPEGMinQuality = n
		}
	}
}

func validate(cfg Config) error {
	var missing []string
	if cfg.TelegramToken == "" {
		missing = append(missing, "telegram_token")
	}
	if cfg.NotionToken == "" {
		missing = append(missing, "notion_token")
	}
	if cfg.NotionDatabase == "" {
		missing = append(missing, "notion_database_id")
	}
	if cfg.S3Endpoint == "" {
		missing = append(missing, "s3_endpoint")
	}
	if cfg.S3Bucket == "" {
		missing = append(missing, "s3_bucket")
	}
	if cfg.S3AccessKeyID == "" {
		missing = append(missing, "s3_access_key_id")
	}
	if cfg.S3SecretAccessKey == "" {
		missing = append(missing, "s3_secret_access_key")
	}
	if cfg.S3PublicBaseURL == "" {
		missing = append(missing, "s3_public_base_url")
	}
	if len(missing) > 0 {
		return errors.New("missing required config: " + strings.Join(missing, ", "))
	}
	if cfg.ImgJPEGMinQuality < 1 || cfg.ImgJPEGMinQuality > 100 {
		return fmt.Errorf("img_jpeg_min_quality out of range: %d", cfg.ImgJPEGMinQuality)
	}
	if cfg.MaxImageBytes <= 0 {
		return errors.New("max_image_bytes must be positive")
	}
	return nil
}

func firstNonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func parseBool(v string) bool {
	s := strings.TrimSpace(strings.ToLower(v))
	return s == "1" || s == "true" || s == "yes" || s == "y" || s == "on"
}
