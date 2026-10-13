package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"social-network/pkg/db/sqlite"
	"social-network/pkg/handlers"
	"social-network/pkg/middleware"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	migrateFlag := flag.String("migrate", "up", `"up" runs pending migrations then starts the server; "down" rolls back all migrations and exits`)
	flag.Parse()

	var (
		port       = env("PORT", "8080")
		dbPath     = env("DB_PATH", "data/social-network.db")
		uploadDir  = env("UPLOAD_DIR", "uploads")
		migrations = env("MIGRATIONS_PATH", "file://pkg/db/migrations/sqlite")
		origin     = env("FRONTEND_ORIGIN", "http://localhost:3000")
	)

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

	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		log.Fatalf("create upload dir: %v", err)
	}

	app := &handlers.App{DB: db, UploadDir: uploadDir}
	handler := middleware.Chain(app.Routes(),
		middleware.Recover,
		middleware.Logger,
		middleware.CORS(origin),
	)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("backend listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}
