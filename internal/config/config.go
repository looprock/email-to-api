package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	// Database Configuration
	Database struct {
		Driver   string
		Path     string // For SQLite
		Host     string // For PostgreSQL
		Port     int    // For PostgreSQL
		User     string // For PostgreSQL
		Password string // For PostgreSQL
		Name     string // For PostgreSQL
		SSLMode  string // For PostgreSQL
	}

	// Admin Server Configuration
	AdminServer struct {
		Host string
		Port int
	}

	// Mail Server Configuration
	MailServer struct {
		Host          string
		Port          int
		Domain        string
		ReceiveMethod string
		MaxEmailSize  int64
		MaxRetries    int
		RetryDelay    int
		SMTPHost      string
		SMTPPort      int
	}

	// Mailgun Configuration (optional)
	Mailgun struct {
		APIKey      string
		Domain      string
		FromAddress string
		SiteDomain  string
	}
}

// LoadConfig loads the configuration from environment variables and config files
func LoadConfig() (*Config, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Read config file
	v.SetConfigName("config")            // name of config file (without extension)
	v.SetConfigType("yaml")              // type of config file
	v.AddConfigPath(".")                 // current directory
	v.AddConfigPath("$HOME/.emailtoapi") // home directory
	v.AddConfigPath("/etc/emailtoapi/")  // system directory

	// Read config file (if exists)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found - that's ok, we'll use env vars and defaults
	}

	// Environment variables
	v.SetEnvPrefix("EMAILTOAPI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Map legacy env vars for backward compatibility
	mapLegacyEnvVars(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Database defaults
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.path", "emailtoapi.db")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.name", "emailtoapi")
	v.SetDefault("database.sslmode", "disable")

	// Admin server defaults
	v.SetDefault("adminserver.host", "0.0.0.0")
	v.SetDefault("adminserver.port", 8080)

	// Mail server defaults
	v.SetDefault("mailserver.host", "0.0.0.0")
	v.SetDefault("mailserver.port", 25)
	v.SetDefault("mailserver.receivemethod", "smtp")
	v.SetDefault("mailserver.maxemailsize", 10*1024*1024) // 10MB
	v.SetDefault("mailserver.maxretries", 10)
	v.SetDefault("mailserver.retrydelay", 5)
	v.SetDefault("mailserver.smtphost", "0.0.0.0")
	v.SetDefault("mailserver.smtpport", 2525)

	// Mailgun defaults
	v.SetDefault("mailgun.site_domain", "")
}

// mapLegacyEnvVars maps old environment variable names to new configuration paths
func mapLegacyEnvVars(v *viper.Viper) {
	// Map legacy DB variables
	if val := v.GetString("DB_DRIVER"); val != "" {
		v.Set("database.driver", val)
	}
	if val := v.GetString("DB_PATH"); val != "" {
		v.Set("database.path", val)
	}
	if val := v.GetString("DB_HOST"); val != "" {
		v.Set("database.host", val)
	}
	if val := v.GetString("DB_PORT"); val != "" {
		v.Set("database.port", val)
	}
	if val := v.GetString("DB_USER"); val != "" {
		v.Set("database.user", val)
	}
	if val := v.GetString("DB_PASSWORD"); val != "" {
		v.Set("database.password", val)
	}
	if val := v.GetString("DB_NAME"); val != "" {
		v.Set("database.name", val)
	}
	if val := v.GetString("DB_SSLMODE"); val != "" {
		v.Set("database.sslmode", val)
	}

	// Map legacy mail server variables
	if val := v.GetString("MAILREADER_DOMAIN"); val != "" {
		v.Set("mailserver.domain", val)
	}
	if val := v.GetString("MAILREADER_SMTP_HOST"); val != "" {
		v.Set("mailserver.smtphost", val)
	}
	if val := v.GetString("MAILREADER_SMTP_PORT"); val != "" {
		v.Set("mailserver.smtpport", val)
	}
	if val := v.GetString("MAILREADER_MAX_EMAIL_SIZE"); val != "" {
		v.Set("mailserver.maxemailsize", val)
	}
	if val := v.GetString("MAILREADER_MAX_RETRIES"); val != "" {
		v.Set("mailserver.maxretries", val)
	}
	if val := v.GetString("MAILREADER_RETRY_DELAY"); val != "" {
		v.Set("mailserver.retrydelay", val)
	}
	if val := v.GetString("MAILREADER_RECEIVE_METHOD"); val != "" {
		v.Set("mailserver.receivemethod", val)
	}

	// Map legacy Mailgun variables
	if val := v.GetString("MAILGUN_API_KEY"); val != "" {
		v.Set("mailgun.apikey", val)
	}
	if val := v.GetString("MAILGUN_DOMAIN"); val != "" {
		v.Set("mailgun.domain", val)
	}
	if val := v.GetString("MAILGUN_FROM_ADDRESS"); val != "" {
		v.Set("mailgun.fromaddress", val)
	}
	if val := v.GetString("MAILGUN_SITE_DOMAIN"); val != "" {
		v.Set("mailgun.site_domain", val)
	}
}
