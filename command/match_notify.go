package command

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-co-op/gocron/v2"
	"github.com/markliederbach/min.go/client"
	"github.com/urfave/cli/v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	KickoffThreshold       = "kickoff-threshold"
	ThreadsDryRun          = "threads-dry-run"
	ThreadsApiKey          = "threads-api-key"
	ThreadsBaseUrl         = "threads-base-url"
	ThreadsUsername        = "threads-username"
	ThreadsPostLimitPerRun = "threads-post-limit"
)

type MatchNotifyCommand struct{}

func NewMatchNotifyCommand() *MatchNotifyCommand {
	return &MatchNotifyCommand{}
}

type MatchNotifyCommandArgs struct {
	KickoffThreshold int
	ThreadsDryRun    bool
	ThreadsApiKey    string
	ThreadsBaseUrl   string
	ThreadsUsername  string
	ThreadsPostLimit int
}

func NewMatchNotifyCommandArgs(c *cli.Command) *MatchNotifyCommandArgs {
	return &MatchNotifyCommandArgs{
		KickoffThreshold: int(c.Int(KickoffThreshold)),
		ThreadsDryRun:    c.Bool(ThreadsDryRun),
		ThreadsApiKey:    c.String(ThreadsApiKey),
		ThreadsBaseUrl:   c.String(ThreadsBaseUrl),
		ThreadsUsername:  c.String(ThreadsUsername),
		ThreadsPostLimit: int(c.Int(ThreadsPostLimitPerRun)),
	}
}

func (m *MatchNotifyCommand) ToCliCommand() *cli.Command {
	return &cli.Command{
		Name:  "match-notify",
		Usage: "Send out threads posts based on match events",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:        KickoffThreshold,
				Usage:       "Kickoff threshold in minutes",
				Required:    false,
				Value:       10,
				DefaultText: "10",
				Sources:     cli.EnvVars("KICKOFF_THRESHOLD"),
			},
			&cli.BoolFlag{
				Name:        ThreadsDryRun,
				Usage:       "Dry run mode for Threads",
				Required:    false,
				Value:       false,
				DefaultText: "false",
				Sources:     cli.EnvVars("THREADS_DRY_RUN"),
			},
			&cli.StringFlag{
				Name:     ThreadsApiKey,
				Usage:    "Threads API key",
				Required: true,
				Sources:  cli.EnvVars("THREADS_API_KEY"),
			},
			&cli.StringFlag{
				Name:        ThreadsBaseUrl,
				Usage:       "Threads base URL",
				Required:    false,
				Value:       "https://graph.threads.net",
				DefaultText: "https://graph.threads.net",
				Sources:     cli.EnvVars("THREADS_BASE_URL"),
			},
			&cli.StringFlag{
				Name:        ThreadsUsername,
				Usage:       "Threads username",
				Required:    false,
				Value:       "mingos.updates",
				DefaultText: "mingos.updates",
				Sources:     cli.EnvVars("THREADS_USERNAME"),
			},
			&cli.IntFlag{
				Name:        ThreadsPostLimitPerRun,
				Usage:       "Threads post limit per run",
				Required:    false,
				Value:       5,
				DefaultText: "5",
				Sources:     cli.EnvVars("THREADS_POST_LIMIT_PER_RUN"),
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			args := NewMatchNotifyCommandArgs(c)
			return runMatchNotifyOnce(ctx, args)
		},
		Commands: []*cli.Command{
			{
				Name:  "cron",
				Usage: "Run the match notifier in a cron schedule",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "schedule",
						Usage:       "Cron schedule",
						Required:    false,
						Value:       "*/1 * * * *",
						DefaultText: "*/1 * * * *",
						Sources:     cli.EnvVars("CRON_MATCH_NOTIFY_SCHEDULE"),
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					args := NewMatchNotifyCommandArgs(c)
					schedule := c.String("schedule")
					scheduler, err := gocron.NewScheduler()
					if err != nil {
						return err
					}

					defer func() { _ = scheduler.Shutdown() }()

					_, err = scheduler.NewJob(
						gocron.CronJob(schedule, false),
						gocron.NewTask(runMatchNotifyOnce, ctx, args),
						gocron.WithSingletonMode(gocron.LimitModeReschedule),
					)
					if err != nil {
						return err
					}

					scheduler.Start()

					// Block forever
					select {}
				},
			},
		},
	}
}

func runMatchNotifyOnce(ctx context.Context, args *MatchNotifyCommandArgs) error {
	db, err := gorm.Open(sqlite.Open("./state.db"), &gorm.Config{})
	if err != nil {
		return err
	}
	threadsClient, err := client.NewThreadsClient(
		ctx,
		client.ThreadsClientOptions{
			APIKey:   args.ThreadsApiKey,
			BaseUrl:  args.ThreadsBaseUrl,
			Username: args.ThreadsUsername,
		},
	)
	if err != nil {
		return err
	}

	return notifyMatchPeriods(ctx, args, db, threadsClient)
}

func notifyMatchPeriods(ctx context.Context, args *MatchNotifyCommandArgs, db *gorm.DB, threadsClient *client.ThreadsClientImpl) error {
	var err error
	postsSent := 0
	postsSent, err = notifyKickoffPeriod(ctx, args, db, threadsClient, postsSent)
	if err != nil {
		return err
	}
	postsSent, err = notifyFulltimePeriod(ctx, args, db, threadsClient, postsSent)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "threads published", "posts", postsSent)
	return nil
}

func notifyKickoffPeriod(ctx context.Context, args *MatchNotifyCommandArgs, db *gorm.DB, threadsClient *client.ThreadsClientImpl, postsSent int) (int, error) {
	kickoffFixtures := []client.DatabaseFixture{}
	if err := db.Where(
		"status = ? AND notify_state = ? AND elapsed <= ?",
		client.RapidFixtureStatusKickoff,
		DBFixtureNotifyStateNew,
		args.KickoffThreshold,
	).Order("date DESC").Order("elapsed").Find(&kickoffFixtures).Error; err != nil {
		return 0, err
	}
	for _, dbFixture := range kickoffFixtures {
		if postsSent >= args.ThreadsPostLimit {
			break
		}
		message := fmt.Sprintf(
			"KICKOFF\n\n%s vs %s",
			dbFixture.HomeTeamName,
			dbFixture.AwayTeamName,
		)
		if args.ThreadsDryRun {
			fmt.Println(message)
		} else {
			_, err := threadsClient.CreateTextPost(
				ctx,
				message,
			)
			if err != nil {
				return postsSent, err
			}
			// update the database to mark the match as notified
			if err := db.Model(&dbFixture).Update("notify_state", DBFixtureNotifyStateKickoff).Error; err != nil {
				return postsSent, err
			}

		}
		postsSent++
	}
	return postsSent, nil
}

func notifyFulltimePeriod(ctx context.Context, args *MatchNotifyCommandArgs, db *gorm.DB, threadsClient *client.ThreadsClientImpl, postsSent int) (int, error) {
	fulltimeFixtures := []client.DatabaseFixture{}
	if err := db.Where(
		"status = ? AND notify_state != ?",
		client.RapidFixtureStatusFulltime,
		DBFixtureNotifyStateFulltime,
	).Order("date DESC").Find(&fulltimeFixtures).Error; err != nil {
		return 0, err
	}
	for _, dbFixture := range fulltimeFixtures {
		if postsSent >= args.ThreadsPostLimit {
			break
		}
		message := fmt.Sprintf(
			"FT: %s %d - %d %s",
			dbFixture.HomeTeamName,
			dbFixture.HomeScore,
			dbFixture.AwayScore,
			dbFixture.AwayTeamName,
		)
		if args.ThreadsDryRun {
			fmt.Println(message)
		} else {
			_, err := threadsClient.CreateTextPost(
				ctx,
				message,
			)
			if err != nil {
				return postsSent, err
			}
			// update the database to mark the match as notified
			if err := db.Model(&dbFixture).Update("notify_state", DBFixtureNotifyStateFulltime).Error; err != nil {
				return postsSent, err
			}

		}
		postsSent++
	}

	return postsSent, nil
}
