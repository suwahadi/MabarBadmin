package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/suwahadi/mabarbadmin/internal/config"
	"github.com/suwahadi/mabarbadmin/internal/content"
	"github.com/suwahadi/mabarbadmin/internal/database"
	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
	"github.com/suwahadi/mabarbadmin/internal/handler"
	"github.com/suwahadi/mabarbadmin/internal/media"
	"github.com/suwahadi/mabarbadmin/internal/scheduler"
	"github.com/suwahadi/mabarbadmin/internal/service"
	"github.com/suwahadi/mabarbadmin/internal/settings"
	"github.com/suwahadi/mabarbadmin/web"
)

var version = "dev"

const usage = `Usage: mabar <command> [flags]

Commands:
  serve                     start the web server (runs migrations first)
  migrate up|down|status    manage database migrations
  seed [--demo]             insert default settings and pages (idempotent)
  create-admin              create the admin account
  version                   print the version
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(ctx, log)
	case "migrate":
		dir := "up"
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		err = withDB(ctx, func(svc *service.Service) error { return database.Migrate(ctx, svc.Pool, dir) }, log)
	case "seed":
		fs := flag.NewFlagSet("seed", flag.ExitOnError)
		demo := fs.Bool("demo", false, "also insert demo venues, sessions, and participants")
		_ = fs.Parse(os.Args[2:])
		err = withDB(ctx, func(svc *service.Service) error { return seed(ctx, svc, *demo) }, log)
	case "create-admin":
		err = withDB(ctx, func(svc *service.Service) error { return createAdmin(ctx, svc, os.Args[2:]) }, log)
	case "version":
		fmt.Println(version)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newService(ctx context.Context, cfg config.Config, log *slog.Logger) (*service.Service, error) {
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	q := sqlc.New(pool)
	return service.New(pool, settings.NewStore(q), media.NewService(cfg.UploadDir, log), log), nil
}

func withDB(ctx context.Context, fn func(*service.Service) error, log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := newService(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer svc.Pool.Close()
	return fn(svc)
}

func serve(ctx context.Context, log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, err := newService(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer svc.Pool.Close()
	if err := database.Migrate(ctx, svc.Pool, "up"); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := os.MkdirAll(cfg.UploadDir, 0o750); err != nil {
		return err
	}
	app, err := handler.New(cfg, svc, log, version, web.Templates, web.Static)
	if err != nil {
		return err
	}
	go (&scheduler.Scheduler{Svc: svc, Log: log}).Run(ctx)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.ListenAddr, "version", version)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	log.Info("shutting down")
	return srv.Shutdown(shutdown)
}

func seed(ctx context.Context, svc *service.Service, demo bool) error {
	if err := database.Migrate(ctx, svc.Pool, "up"); err != nil {
		return err
	}
	if err := settings.Seed(ctx, svc.Q, nil); err != nil {
		return err
	}
	if err := content.SeedPages(ctx, svc.Q); err != nil {
		return err
	}
	fmt.Println("Default settings and pages are in place.")
	if demo {
		if err := seedDemo(ctx, svc); err != nil {
			return err
		}
		fmt.Println("Demo data inserted. Demo member password: demo12345")
	}
	return nil
}

func createAdmin(ctx context.Context, svc *service.Service, args []string) error {
	fs := flag.NewFlagSet("create-admin", flag.ExitOnError)
	name := fs.String("name", "", "display name")
	phone := fs.String("phone", "", "WhatsApp number, e.g. 0812-3456-7890")
	stdin := fs.Bool("password-stdin", false, "read the password from standard input")
	_ = fs.Parse(args)

	in := bufio.NewReader(os.Stdin)
	ask := func(label string, cur *string) error {
		if *cur != "" {
			return nil
		}
		fmt.Print(label)
		line, err := in.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		*cur = strings.TrimSpace(line)
		return nil
	}
	if err := ask("Nama admin: ", name); err != nil {
		return err
	}
	if err := ask("No WA admin: ", phone); err != nil {
		return err
	}
	var password string
	if *stdin || !term.IsTerminal(int(os.Stdin.Fd())) {
		line, err := in.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		password = strings.TrimRight(line, "\r\n")
	} else {
		fmt.Print("Password: ")
		p1, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return err
		}
		fmt.Print("Ulangi password: ")
		p2, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return err
		}
		if string(p1) != string(p2) {
			return errors.New("password tidak sama")
		}
		password = string(p1)
	}
	if err := database.Migrate(ctx, svc.Pool, "up"); err != nil {
		return err
	}
	u, err := svc.CreateUser(ctx, service.SignupInput{Name: *name, Phone: *phone, Password: password, Level: "advanced", Role: "admin"})
	if err != nil {
		return err
	}
	fmt.Printf("Admin %s (%s) dibuat.\n", u.Name, u.Phone)
	return nil
}
