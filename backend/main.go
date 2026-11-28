// Package main is the entry point for the App Suite server. It parses the
// configuration flags, initializes the SQLite database, creates the live events'
// hub, and starts the HTTP server, which serves the API and the SPA embedded in
// the binary (internal/web).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"app-suite/internal/api"
	"app-suite/internal/live"
	"app-suite/internal/logging"
	"app-suite/internal/server"
	"app-suite/internal/store"
	"app-suite/internal/web"
)

// main keeps the process's exit status truthful. run does the real work and
// returns an error for anything fatal; a non-zero exit is what tells systemd's
// Restart=on-failure that this was a crash and not a clean stop. Exiting here
// rather than inside run means run's deferred cleanup (db.Close, the log closer)
// has already completed by the time the process goes away.
func main() {
	// `app-suite openapi` prints the API's OpenAPI document, which
	// frontend/src/api/openapi.json commits (task gen).
	if len(os.Args) == 2 && os.Args[1] == "openapi" {
		if err := openapi(); err != nil {
			fmt.Fprintln(os.Stderr, "openapi:", err)
			os.Exit(1)
		}
		return
	}
	// `app-suite admin -db <path> <username>` makes or resets an admin on the host
	// (admin.go).
	if len(os.Args) >= 2 && os.Args[1] == "admin" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		status := adminCommand(ctx, os.Args[2:], os.Stdin, os.Stdout, os.Stderr)
		stop()
		os.Exit(status)
	}
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "/opt/app-suite/data/database.sqlite", "SQLite database path")
	dataDir := flag.String("data", "/opt/app-suite/data", "Data folder: the uploads (images/, fonts/, carrd/) and the plugin repository (plugin/), which the server serves itself (fonts/ only through its tokenized links)")
	corsOrigins := flag.String("cors-origins", "", "Comma-separated CORS allowlist of cross-origin sites (env APPSUITE_CORS_ORIGINS; empty = same-origin only, no CORS headers)")
	turnstileSecret := flag.String("turnstile-secret", "", "Cloudflare Turnstile secret key (env APPSUITE_TURNSTILE_SECRET; empty = login bot check disabled)")
	turnstileSiteKey := flag.String("turnstile-sitekey", "", "Cloudflare Turnstile public site key (env APPSUITE_TURNSTILE_SITEKEY)")
	logFile := flag.String("log-file", "", "Rotating JSON log file path (daily midnight rotation); empty = stdout only")
	webhookOverride := flag.String("webhook-override", "", "Redirect EVERY outbound Discord post to this webhook URL (env APPSUITE_WEBHOOK_OVERRIDE; dev safety when running on a copy of live data)")
	webhookDryRun := flag.Bool("webhook-dry-run", false, "Log outbound Discord posts instead of sending them (env APPSUITE_WEBHOOK_DRY_RUN=1); wins over -webhook-override")
	cspReportOnly := flag.Bool("csp-report-only", false, "Send the Content-Security-Policy as report-only: browsers report what it would block and block nothing (env APPSUITE_CSP_REPORT_ONLY=1)")
	ownersFlag := flag.String("owners", "", "Comma-separated usernames of the owner accounts, which nobody deactivates, demotes, renames or deletes (env APPSUITE_OWNERS)")
	flag.Parse()

	// Logging: install the JSON slog handler before anything logs. Always writes
	// to stdout (journald); also to the rotating file when -log-file names one (the
	// systemd unit does). On a file-sink failure we still log to stdout and warn.
	logCloser, logErr := logging.Setup(*logFile, slog.LevelInfo)
	if logCloser != nil {
		defer func() { _ = logCloser.Close() }() // at exit: the log is where a failure would go
	}
	if logErr != nil {
		slog.Warn("file logging disabled; using stdout only", "path", *logFile, "error", logErr)
	}

	// CORS allowlist: flag > env. Normally empty - the SPA and API are
	// same-origin in both prod (one server behind the proxy) and dev (Vite proxies /api).
	originsCSV := *corsOrigins
	if originsCSV == "" {
		originsCSV = os.Getenv("APPSUITE_CORS_ORIGINS")
	}
	var allowedOrigins []string
	if originsCSV != "" {
		allowedOrigins = strings.Split(originsCSV, ",")
	}

	// The process context: canceled by SIGINT/SIGTERM, or by run itself when the
	// server fails, so everything started under it - the migrations below, then the
	// background loops - stops on either. Created before the database opens so a
	// signal that lands mid-migration stops the migration between statements rather
	// than killing the process outright. Until stopSignals runs at exit, a second
	// signal is swallowed rather than killing the graceful shutdown part-way.
	sigCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	ctx, cancel := context.WithCancel(sigCtx)
	defer cancel()

	db, err := store.New(ctx, *dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("close database", "error", err)
		}
	}()

	// Cloudflare Turnstile login bot check: flag > env. Disabled when no secret.
	tsSecret := *turnstileSecret
	if tsSecret == "" {
		tsSecret = os.Getenv("APPSUITE_TURNSTILE_SECRET")
	}
	tsSiteKey := *turnstileSiteKey
	if tsSiteKey == "" {
		tsSiteKey = os.Getenv("APPSUITE_TURNSTILE_SITEKEY")
	}

	// Signing in with Discord: the Discord application's id and OAuth client secret,
	// from the env file, under the APPSUITE_ prefix every other variable carries. Off
	// unless both are set.
	discordClientID := strings.TrimSpace(os.Getenv("APPSUITE_DISCORD_APPLICATION_ID"))
	discordClientSecret := strings.TrimSpace(os.Getenv("APPSUITE_DISCORD_CLIENT_SECRET"))

	// The SPA, from the build embedded at compile time. A binary built without it
	// stops here rather than serving a site with no pages.
	site, err := web.New(web.Dist())
	if err != nil {
		return err
	}
	// The policy can be made report-only from the env file, without a deploy, while
	// something it blocks in production is fixed.
	site.ReportOnly = *cspReportOnly || os.Getenv("APPSUITE_CSP_REPORT_ONLY") == "1"
	if site.ReportOnly {
		slog.Warn("CSP REPORT-ONLY: the Content-Security-Policy is reported, not enforced")
	}

	hub := live.New()
	srv := server.New(ctx, db, hub, *dataDir, allowedOrigins)
	srv.SetSite(site)
	srv.SetTurnstile(tsSecret, tsSiteKey)
	srv.SetDiscordSignIn(discordClientID, discordClientSecret)
	srv.SetLogFile(*logFile) // GET /api/v1/logs tails this file

	// The owner accounts: flag > env. Each is made on the host first (`app-suite admin`).
	ownersCSV := *ownersFlag
	if ownersCSV == "" {
		ownersCSV = os.Getenv("APPSUITE_OWNERS")
	}
	var owners []string
	for _, name := range strings.Split(ownersCSV, ",") {
		if name = strings.TrimSpace(name); name != "" {
			owners = append(owners, name)
		}
	}
	srv.SetOwners(owners)
	if len(owners) > 0 {
		slog.Info("owner accounts named", "owners", owners)
	} else {
		slog.Warn("no owner accounts named (-owners, APPSUITE_OWNERS); any admin may demote or delete another")
	}

	// Outbound-Discord safety valves. A development server is normally pointed at a
	// COPY OF THE LIVE DATABASE, whose announcement types, book clubs, affiliates and
	// tea rooms all carry real webhook URLs - so without one of these, simply starting
	// the process lets the announcement scheduler post to production channels. Both
	// are off unless asked for, so production behaviour is unchanged; both are logged
	// loudly at startup, because "did that actually post?" must never be a guess.
	hookOverride := *webhookOverride
	if hookOverride == "" {
		hookOverride = os.Getenv("APPSUITE_WEBHOOK_OVERRIDE")
	}
	hookDryRun := *webhookDryRun || os.Getenv("APPSUITE_WEBHOOK_DRY_RUN") == "1"
	srv.SetWebhookPolicy(hookOverride, hookDryRun)
	switch {
	case hookDryRun:
		slog.Warn("DISCORD DRY RUN: outbound posts are suppressed and only logged")
	case hookOverride != "":
		slog.Warn("DISCORD OVERRIDE: every outbound post is redirected to the override webhook")
	}

	// Live log tail: each JSON log line goes to the admins' event streams as a log
	// event. Skipped entirely while no admin is listening, and lossy so a burst can't
	// end anyone's stream. The line is parsed at once and copied out of, so the
	// logger's buffer is not kept.
	logging.SetTailSink(api.LogTail(hub))
	if tsSecret != "" {
		slog.Info("Cloudflare Turnstile bot check enabled on the login form")
	} else {
		slog.Warn("Cloudflare Turnstile not configured; login bot check disabled")
	}
	if discordClientID != "" && discordClientSecret != "" {
		slog.Info("Discord sign-in enabled")
	} else {
		slog.Warn("Discord sign-in not configured (APPSUITE_DISCORD_APPLICATION_ID, APPSUITE_DISCORD_CLIENT_SECRET); it is off")
	}

	httpServer := &http.Server{
		Addr:    *addr,
		Handler: srv,
		// Timeouts guard against slow-client / idle-socket resource exhaustion.
		// Deliberately NO WriteTimeout: an event stream is a long-lived answer a
		// global write deadline would sever; each event's write sets its own deadline
		// instead. ReadHeaderTimeout bounds the slow-headers (Slowloris) attack;
		// ReadTimeout bounds the full request read; IdleTimeout reaps kept-alive
		// connections.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Background scheduler: posts due announcement embeds to Discord. Runs under the
	// process context, canceled on shutdown, so the goroutine exits cleanly.
	// schedDone is closed when it has fully returned, so shutdown can wait for an
	// in-flight sweep before closing the DB.
	schedDone := make(chan struct{})
	go func() {
		defer close(schedDone)
		srv.RunAnnouncementScheduler(ctx)
	}()

	// Automatic bingo-draw scheduler: draws a number on the admin-chosen interval
	// while a game has "auto" switched on. Shares the shutdown-cancelled context so
	// it stops drawing before the database closes.
	autoDone := make(chan struct{})
	go func() {
		defer close(autoDone)
		srv.RunAutoDrawScheduler(ctx)
	}()

	// Tea-room lock scheduler: unlocks rooms whose set expiry has passed and tells
	// the admin clients (the in-game plugin alerts its operator). Same shared
	// context, so it stops sweeping before the database closes.
	roomLockDone := make(chan struct{})
	go func() {
		defer close(roomLockDone)
		srv.RunTeaRoomLockScheduler(ctx)
	}()

	// Gpose photo sweeper: deletes the gposes staff uploaded once their day is over.
	// Same shared context, so it stops sweeping before the database closes.
	gposePhotoDone := make(chan struct{})
	go func() {
		defer close(gposePhotoDone)
		srv.RunGposePhotoScheduler(ctx)
	}()

	// A failed ListenAndServe (e.g. the port is already in use) must run the SAME
	// graceful-shutdown path below rather than exiting on the spot - otherwise the
	// deferred cleanup (db.Close, scheduler cancel) is skipped and in-flight state
	// can be left inconsistent. Send the error to a buffered channel the select
	// awaits, then return it so the process still exits non-zero.
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("App Suite API server starting", "addr", *addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Non-nil only when the server itself failed. A signal-driven stop is a clean
	// exit; a bind failure is not, and must not report success for a backend that
	// never started.
	var fatal error
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received, shutting down gracefully...")
	case err := <-serverErr:
		slog.Error("server failed, shutting down", "error", err)
		fatal = err
	}
	cancel() // stop the background loops

	// Give in-flight requests 10 seconds to complete.
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	// Wait for the announcement scheduler to finish any in-flight sweep before
	// the deferred db.Close(): a Discord post that already succeeded must get its
	// cursor advanced (MarkAnnouncementPosted), or it re-posts on the next boot.
	// Bounded by the shutdown deadline so a hung post can't block forever.
	select {
	case <-schedDone:
	case <-shutdownCtx.Done():
		slog.Warn("announcement scheduler did not stop within the shutdown deadline")
	}

	// Likewise wait for the auto-draw scheduler to return so an in-flight draw
	// finishes writing before db.Close(). Bounded by the same shutdown deadline.
	select {
	case <-autoDone:
	case <-shutdownCtx.Done():
		slog.Warn("auto-draw scheduler did not stop within the shutdown deadline")
	}

	// And the tea-room lock sweeper, so an unlock it has already written is not
	// interrupted part-way by db.Close(). Bounded by the same shutdown deadline.
	select {
	case <-roomLockDone:
	case <-shutdownCtx.Done():
		slog.Warn("tea-room lock scheduler did not stop within the shutdown deadline")
	}

	// And the gpose photo sweeper, so a photo it has cleared has its file deleted too.
	select {
	case <-gposePhotoDone:
	case <-shutdownCtx.Done():
		slog.Warn("gpose photo scheduler did not stop within the shutdown deadline")
	}

	// End every event stream before shutting the HTTP server down: an open stream is
	// a request its shutdown would otherwise wait on until its deadline.
	hub.Close()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	slog.Info("server stopped")
	return fatal
}

// openapi prints the API's OpenAPI document.
func openapi() error {
	spec, err := api.Spec()
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(spec)
	return err
}
