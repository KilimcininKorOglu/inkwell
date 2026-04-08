package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds global environment variable configuration.
// IMAP settings are now stored per-domain in the database.
type Config struct {
	// Database
	DBHost     string
	DBName     string
	DBUser     string
	DBPassword string

	// Web
	Port          string
	AdminUser     string
	AdminPassword string

	// Polling
	FetchInterval int // seconds

	// Encryption
	EncryptionKey string // 32-byte hex-encoded key for AES-256-GCM
}

// Load reads environment variables and returns a Config with defaults.
func Load() *Config {
	return &Config{
		DBHost:        getEnv("DB_HOST", "db"),
		DBName:        getEnv("DB_NAME", "dmarc"),
		DBUser:        getEnv("DB_USER", "dmarcuser"),
		DBPassword:    getEnv("DB_PASSWORD", "dmarcpass"),
		Port:          getEnv("PORT", "8080"),
		AdminUser:     getEnv("ADMIN_USER", ""),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),
		FetchInterval: getEnvInt("FETCH_INTERVAL", 300),
		EncryptionKey: getEnv("ENCRYPTION_KEY", ""),
	}
}

// DSN returns the MySQL/MariaDB connection string for GORM.
func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&charset=utf8mb4&loc=Local",
		c.DBUser, c.DBPassword, c.DBHost, c.DBName)
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			if i < 60 { // minimum 1 minute to prevent CPU exhaustion
				i = 60
			}
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key, defaultVal string) bool {
	if v := os.Getenv(key); v != "" {
		return v == "true" || v == "1" || v == "yes"
	}
	return defaultVal == "true" || defaultVal == "1" || defaultVal == "yes"
}
