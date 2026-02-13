package config

import (
	"os"
	"time"
)

type Config struct {
	Server       ServerConfig
	Database     DatabaseConfig
	JWT          JWTConfig
	OAuth        OAuthConfig
	Cloudinary   CloudinaryConfig
	Location     LocationConfig
	Payment      PaymentConfig
	LiberecMpesa LiberecMpesaConfig
}

type ServerConfig struct {
	Port         string
	Env          string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	DSN             string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
	Issuer        string
}

type OAuthConfig struct {
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
}

type CloudinaryConfig struct {
	CloudName string
	APIKey    string
	APISecret string
}

type LocationConfig struct {
	MapUpdateIntervalSec int
	LocationFuzzMeters   float64
	MinAge               int
}

type PaymentConfig struct {
	WebhookSecret string
	PaymentExpiry time.Duration
}

// LiberecMpesaConfig for M-Pesa STK via TheLiberec Card API
type LiberecMpesaConfig struct {
	BaseURL        string
	Email          string
	Password       string
	WebhookBaseURL string // e.g. https://yourdomain.com - callback will be WebhookBaseURL + /api/v1/webhooks/mpesa
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         "8099",
			Env:          "development",
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		Database: DatabaseConfig{
			DSN:             "joelwasike:@Webuye2021@tcp(localhost:3306)/metchi?charset=utf8mb4&parseTime=True&loc=Local",
			MaxIdleConns:    10,
			MaxOpenConns:    100,
			ConnMaxLifetime: time.Hour,
		},
		JWT: JWTConfig{
			AccessSecret:  "change-me-in-production",
			RefreshSecret: "change-me-refresh",
			AccessExpiry:  15 * time.Minute,
			RefreshExpiry: 168 * time.Hour,
			Issuer:        "metchi",
		},
		OAuth: OAuthConfig{
			GoogleClientID:     "your-google-client-id.apps.googleusercontent.com",
			GoogleClientSecret: "your-google-client-secret",
			GoogleRedirectURL:  "metchi.theliberec.com/app/api/v1/auth/google/callback",
		},
		Cloudinary: CloudinaryConfig{
			CloudName: "dcrdv2jcz",
			APIKey:    "781537668289137",
			APISecret: "0pwGloCz0wgOE_W2aORNsB-KF2g",
		},
		Location: LocationConfig{
			MapUpdateIntervalSec: 5,
			LocationFuzzMeters:   100,
			MinAge:               18,
		},
		Payment: PaymentConfig{
			WebhookSecret: "",
			PaymentExpiry: 30 * time.Minute,
		},
		LiberecMpesa: func() LiberecMpesaConfig {
			webhookBase := "https://metchi.theliberec.com"
			if v := os.Getenv("MPESA_WEBHOOK_BASE_URL"); v != "" {
				webhookBase = v
			}
			return LiberecMpesaConfig{
				BaseURL:        "https://card-api.theliberec.com",
				Email:          "metchi@gmail.com",
				Password:       "joelwasike",
				WebhookBaseURL: webhookBase,
			}
		}(),
	}
}
