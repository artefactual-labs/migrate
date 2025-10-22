package rootcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
	"github.com/stephenafamo/bob"
	"go.temporal.io/sdk/client"
	_ "modernc.org/sqlite"

	"github.com/artefactual-labs/migrate/internal/application"
	"github.com/artefactual-labs/migrate/internal/database/migrations"
	"github.com/artefactual-labs/migrate/internal/storage_service"
)

type RootConfig struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	Flags   *ff.FlagSet
	Command *ff.Command

	loggerOnce sync.Once
	logger     *slog.Logger

	appOnce sync.Once
	app     *application.App
	appErr  error
}

func New(stdin io.Reader, stdout, stderr io.Writer) *RootConfig {
	cfg := &RootConfig{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}

	cfg.Flags = ff.NewFlagSet("migrate")

	cfg.Command = &ff.Command{
		Name:      "migrate",
		Usage:     "migrate <SUBCOMMAND> ...",
		ShortHelp: "Digital preservation helper commands.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}

	return cfg
}

func (cfg *RootConfig) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(cfg.Stdout, ffhelp.Command(cfg.Command))
		return ff.ErrHelp
	}
	return errors.New("missing command")
}

func (cfg *RootConfig) Logger() *slog.Logger {
	cfg.loggerOnce.Do(func() {
		handler := slog.NewTextHandler(cfg.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
		cfg.logger = slog.New(handler)
	})
	return cfg.logger
}

func (cfg *RootConfig) App(ctx context.Context) (*application.App, error) {
	cfg.appOnce.Do(func() {
		app, err := cfg.initApp(ctx)
		if err != nil {
			cfg.appErr = err
			return
		}
		cfg.app = app
	})

	return cfg.app, cfg.appErr
}

func (cfg *RootConfig) initApp(ctx context.Context) (*application.App, error) {
	config, path, err := application.LoadConfig()
	if err != nil {
		return nil, err
	}

	cfg.Logger().Info("Loaded config.", slog.String("path", path))

	db, err := initDatabase(ctx, config.Database.SQLite.Path)
	if err != nil {
		return nil, err
	}

	logger := cfg.Logger()

	apiCfg := config.StorageService.API
	storageClient := storage_service.NewAPI(http.DefaultClient, apiCfg.URL, apiCfg.Username, apiCfg.APIKey)

	temporalClient, err := client.Dial(client.Options{
		Namespace: config.Temporal.Namespace,
		HostPort:  config.Temporal.Address,
		Logger:    logger,
	})
	if err != nil {
		return nil, fmt.Errorf("dial temporal: %w", err)
	}
	return application.New(logger, db, config, temporalClient, storageClient), nil
}

func initDatabase(ctx context.Context, datasource string) (db bob.DB, err error) {
	if datasource == "" {
		return db, fmt.Errorf("sqlite path not configured")
	}

	dir := filepath.Dir(datasource)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return db, fmt.Errorf("ensure sqlite directory %q: %w", dir, err)
		}
	}

	db, err = bob.Open("sqlite", datasource)
	if err != nil {
		return db, fmt.Errorf("open sqlite db: %w", err)
	}

	if err = db.PingContext(ctx); err != nil {
		return db, fmt.Errorf("ping db: %w", err)
	}

	var file []byte
	file, err = migrations.FS.ReadFile("schema.sql")
	if err != nil {
		return db, fmt.Errorf("read schema.sql: %w", err)
	}

	if _, err = db.ExecContext(ctx, string(file)); err != nil {
		return db, fmt.Errorf("exec schema.sql: %w", err)
	}

	return db, nil
}
