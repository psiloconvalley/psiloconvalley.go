package main

import (
	"log"
	"net/http"
	"os"
	"time"
	"context"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/justinas/nosurf"

	"psiloconvalley/internal/app"
	"psiloconvalley/internal/auth"
	"psiloconvalley/internal/handlers"
	"psiloconvalley/internal/pdf"
	"psiloconvalley/internal/router"
)

func main() {
	// 1. Load Environment Variables
	_ = godotenv.Load()

	// 2. Initialize Database (PostgreSQL)
	app.InitDB()
	defer app.CloseDB()

	// 3. Initialize External Services
	pdf.Init()
	defer pdf.Shutdown()
	auth.InitGoogleOAuth()

	// 4. Construct Core Layers
	application := app.NewApp(app.DB)
	httpHandlers := handlers.NewHandlers(application)
	baseRouter := router.NewRouter(httpHandlers)

	// 5. Apply Global CSRF Protection
	csrfHandler := nosurf.New(baseRouter)
	csrfHandler.ExemptPath("/stripe/webhook")
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   os.Getenv("RAILWAY_ENVIRONMENT") != "",
		SameSite: http.SameSiteLaxMode,
	})
	csrfHandler.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[SECURITY] CSRF Rejection: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		http.Error(w, "Forbidden — invalid or missing CSRF token", http.StatusForbidden)
	}))

	// 6. Configure HTTP Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      csrfHandler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

		// ── Scheduler Engine ─────────────────────────────────────────────
	// Runs in a goroutine alongside the HTTP server.
	// Cancelled gracefully when the process receives SIGINT or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go application.Scheduler.Start(ctx)

	log.Printf("🚀 PsiloConValley Operating on :%s [CSRF ENABLED]", port)
	log.Fatal(srv.ListenAndServe())	


}
