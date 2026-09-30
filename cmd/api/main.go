package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
    fb "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/firebase"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/routes"
    "github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
    "github.com/robfig/cron/v3"
)

func main() {
    // 1. Load configuration from .env
    cfg := config.LoadConfig()

    // 2. Connect to PostgreSQL
    database.ConnectDatabase(cfg)

    // 2b. Connect to Redis (optional - caching disabled if REDIS_URL unset)
    database.ConnectRedis(cfg)

    // 3. Run auto-migration to create tables (dev/debug only;
    // In production (GIN_MODE=release), schema changes must go through
    // versioned migrations in migrations/ using the migrate CLI.
    if cfg.GinMode != gin.ReleaseMode {
        database.AutoMigrate()
    }

    // 3b. Apply small production-safe schema patches even when full
    // AutoMigrate is skipped (GIN_MODE=release) - see EnsureProductionSchemaPatches.
    database.EnsureProductionSchemaPatches()
	database.EnsureInvoiceSchemaPatches()

    // 4. Initialize Firebase (push notifications)
    fb.InitFirebase(cfg.FirebaseCredentialsPath)

    // 5. Setup daily push notification schedule (IST)
    loc, err := time.LoadLocation("Asia/Kolkata")
    if err != nil {
        log.Printf("Failed to load IST timezone, using server local time: %v", err)
        loc = time.Local
    }
    c := cron.New(cron.WithLocation(loc))
    c.AddFunc("0 11 * * *", func() {
        services.SendPushToAll("Quick Delivery", "\U0001F6D2 Everything you need, delivered to your doorstep in minutes.")
    })
    c.AddFunc("0 15 * * *", func() {
        services.SendPushToAll("Fresh Deals", "\U0001F389 Fresh deals are waiting for you. Don't miss out!")
    })
    // Sweep expired cart reservations every 2 minutes. Reservations also
    // self-expire lazily (checked inline on every reserve), but this catches
    // holds nobody happens to re-check, so rows don't pile up forever.
    c.AddFunc("*/2 * * * *", func() {
        services.CleanupExpiredReservations()
    })
    // Sweep delivery assignments whose acceptance window has passed without
    // a response, mark them expired, and automatically try the next
    // eligible partner. Runs every minute so a silent partner never stalls
    // an order for much longer than the configured timeout.
    c.AddFunc("* * * * *", func() {
        services.ExpireStaleAssignments()
    })
    c.Start()

    // 6. Setup Gin router
    gin.SetMode(cfg.GinMode)
    router := gin.Default()

    // 7. Register all routes
    routes.SetupRoutes(router)

    // 8. Start server, with graceful shutdown on SIGINT/SIGTERM so in-flight
    // requests (e.g. a handover or invoice generation mid-transaction) get a
    // chance to finish instead of being cut off when a container is stopped.
    srv := &http.Server{
        Addr:    ":" + cfg.Port,
        Handler: router,
    }

    go func() {
        log.Printf("Server starting on port %s", cfg.Port)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Failed to start server: %v", err)
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        log.Fatalf("Server forced to shut down: %v", err)
    }
    log.Println("Server exited cleanly")
}


