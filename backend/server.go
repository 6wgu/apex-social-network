package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"social-network/pkg/db/initial"
	"social-network/pkg/db/sqlite"
	"social-network/pkg/handlers"
	"social-network/pkg/mail"
	"social-network/pkg/middleware"
	"social-network/pkg/otp"
	"social-network/pkg/session"
	"social-network/pkg/ws"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	envFile := mail.LoadDotEnv(".")
	migrateFlag := flag.String("migrate", "up", `"up" runs pending migrations then starts the server; "down" rolls back all migrations and exits`)
	flag.Parse()

	var (
		port       = env("PORT", "8080")
		dbPath     = env("DB_PATH", "data/social-network.db")
		uploadDir  = env("UPLOAD_DIR", "uploads")
		migrations = env("MIGRATIONS_PATH", "file://pkg/db/migrations/sqlite")
		origins    = ws.ParseOrigins(env("FRONTEND_ORIGIN", "http://localhost:3000,http://127.0.0.1:3000"))
	)

	// A fresh clone starts from the shipped database and its images.
	if err := initial.Install(dbPath, uploadDir); err != nil {
		log.Fatalf("install starting database: %v", err)
	}
	db, err := sqlite.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	switch *migrateFlag {
	case "down":
		if err := sqlite.MigrateDown(db, migrations); err != nil {
			log.Fatal(err)
		}
		log.Println("all migrations rolled back")
		return
	case "up":
		if err := sqlite.MigrateUp(db, migrations); err != nil {
			log.Fatal(err)
		}
		log.Println("migrations applied")
	default:
		log.Fatalf("unknown -migrate value %q (use up or down)", *migrateFlag)
	}

	// Sessions that expired while the server was down are useless; drop them.
	if err := session.DeleteExpired(db); err != nil {
		log.Fatalf("delete expired sessions: %v", err)
	}
	// COOKIE_SECURE=1 when the site is served over HTTPS; off for http://localhost.
	session.SecureCookies = os.Getenv("COOKIE_SECURE") == "1"

	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		log.Fatalf("create upload dir: %v", err)
	}

	mailCfg := mail.FromEnv()
	mailer := mail.New(mailCfg)
	if envFile != "" {
		log.Printf("email: settings loaded from %s", envFile)
	} else {
		log.Println("email: no .env file found in this folder or its parents")
	}
	if mailer.Enabled() {
		log.Printf("email: sending through %s:%d as %s", mailCfg.Host, mailCfg.Port, mailCfg.User)
		go func() {
			if err := mailer.Check(); err != nil {
				log.Printf("email: Gmail login FAILED: %v", err)
				return
			}
			log.Println("email: Gmail login OK")
		}()
	} else {
		log.Println("email: console mode (SMTP_USER and SMTP_PASS are empty), emails are printed here")
	}

	app := &handlers.App{DB: db, UploadDir: uploadDir, Hub: ws.NewHub(origins...), Mailer: mailer,
		OTP: otp.New(otp.SQLStore{DB: db}, mailer, otp.FromConfig(mailCfg)),
		// Nothing but /api/me, the email code and logout until the email is verified.
		RequireVerified: true}
	handler := middleware.Chain(app.Routes(),
		middleware.Recover,
		middleware.Logger,
		middleware.CORS(origins...),
	)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       120 * time.Second, // long enough for a 50 MB image upload
		WriteTimeout:      120 * time.Second, // and to send a 50 MB GIF to a slow viewer
		IdleTimeout:       2 * time.Minute,
	}

	// Stop on Ctrl+C or "docker stop": finish the requests in flight, then
	// return so the deferred db.Close() runs.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("backend listening on :%s", port)
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		db.Close()
		log.Fatalf("server stopped: %v", err)
	case <-ctx.Done():
		log.Println("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}
}
