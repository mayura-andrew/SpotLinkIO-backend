package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/mayura-andrew/SpotLinkIO-backend/internal/data"
	"github.com/mayura-andrew/SpotLinkIO-backend/internal/jsonlog"
	"github.com/mayura-andrew/SpotLinkIO-backend/internal/mailer"
	"golang.org/x/oauth2"
)

const version = "1.0.0"

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  string
	}
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	oauth struct {
		googleClientID     string
		googleClientSecret string
		redirectURI        string
	}
	frontendURL string
	qr          struct {
		storageDir string
	}
	cors struct {
		trustedOrigins []string
	}
}

type application struct {
	config            config
	logger            *jsonlog.Logger
	models            data.Models
	mailer            mailer.Mailer
	wg                sync.WaitGroup
	googleOauthConfig *oauth2.Config
}

func main() {
	var cfg config

	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production|testing)")

	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("DB_DSN"), "PostgreSQL DSN")

	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max connection idle time")

	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")
	flag.StringVar(&cfg.smtp.host, "smtp-host", os.Getenv("SMTPHOST"), "SMTP host")
	flag.StringVar(&cfg.frontendURL, "frontend-url", os.Getenv("FRONTEND_URL"), "Frontend URL")

	envSMTPPort := os.Getenv("SMTPPORT")

	if envSMTPPort == "" {
		envSMTPPort = "587"
		fmt.Println("SMTPPORT is not set. Defaulting to 587")
	}

	intSMTPPort, err := strconv.Atoi(envSMTPPort)
	if err != nil {
		fmt.Println("SMTPPORT is not a number. Defaulting to 587")
		intSMTPPort = 587
	}

	flag.IntVar(&cfg.smtp.port, "smtp-port", intSMTPPort, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", os.Getenv("SMTPUSERNAME"), "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", os.Getenv("SMTPPASS"), "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", os.Getenv("SMTPSENDER"), "SMTP sender")
	flag.Parse()

	// Add OAuth config
	flag.StringVar(&cfg.oauth.googleClientID, "oauth-google-client-id", os.Getenv("GOOGLE_CLIENT_ID"), "Google OAuth Client ID")
	flag.StringVar(&cfg.oauth.googleClientSecret, "oauth-google-client-secret", os.Getenv("GOOGLE_CLIENT_SECRET"), "Google OAuth Client Secret")
	flag.StringVar(&cfg.oauth.redirectURI, "oauth-redirect-url", os.Getenv("GOOGLE_REDIRECT_URI"), "OAuth Redirect URL")

	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})

	cfg.cors.trustedOrigins = append(cfg.cors.trustedOrigins, "http://localhost:5173", "http://localhost:3000")

	cfg.qr.storageDir = "./qr_images"
	flag.Parse()

	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)
	if logger == nil {
		panic("Logger is not initialized")
	}
	db, err := openDB(cfg)
	if err != nil {
		logger.PrintFatal(err, nil)
	}

	defer db.Close()

	logger.PrintInfo("database connection pool established", nil)

	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
		mailer: mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
	}

	app.initGoogleOAuth()

	err = app.serve()
	if err != nil {
		logger.PrintFatal(err, nil)
	}

}

func openDB(cfg config) (*sql.DB, error) {

	dsn := cfg.db.dsn
	if !strings.Contains(dsn, "sslmode=") {
		if strings.Contains(dsn, "?") {
			dsn += "&sslmode=disable"
		} else {
			dsn += "?sslmode=disable"
		}
	}
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.db.maxOpenConns)
	db.SetMaxIdleConns(cfg.db.maxIdleConns)

	duration, err := time.ParseDuration(cfg.db.maxIdleTime)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxIdleTime(duration)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	return db, nil
}
