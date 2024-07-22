package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/markliederbach/min.go/client"
	"github.com/markliederbach/min.go/command"
	"github.com/urfave/cli/v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	_ "github.com/mattn/go-sqlite3"
)

var (
	// Version is automatically overridden at compile time
	Version = "latest"
	AppName = "min.go"
)

func init() {
	cli.VersionPrinter = func(cmd *cli.Command) {
		fmt.Printf("%s %s\n", cmd.Root().Name, cmd.Root().Version)
	}

	// read log level from LOG_LEVEL env var
	logLevel := logLevel(os.Getenv("LOG_LEVEL"))
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)
}

func initDb() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("./state.db"), &gorm.Config{})
	if err != nil {
		slog.Error("failed to connect database", "error", err)
		panic("failed to connect database")
	}
	db.AutoMigrate(&client.EventInfo{}, &client.FixtureInfo{})
	return db
}

func main() {
	db := initDb()
	cmd := cli.Command{
		Name:    "Min.go",
		Version: Version,
		Authors: []any{
			"Mark Liederbach",
		},
		Usage: "Retrieve updates from Forward Madison FC matches and publish to the Fediverse",
		Commands: []*cli.Command{
			command.NewMatchCommand(db).ToCliCommand(),
			command.NewRefreshThreadsTokenCommand().ToCliCommand(),
		},
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func logLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
