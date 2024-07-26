package command

import (
	"context"
	"errors"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/markliederbach/min.go/client"
	"github.com/urfave/cli/v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	FixtureSyncerCommandRapidApiBaseUrl = "rapid-api-base-url"
	FixtureSyncerCommandRapidApiKey     = "rapid-api-key"
	FixtureSyncerCommandRapidApiHost    = "rapid-api-host"
	FixtureSyncerCommandRapidApiLeague  = "rapid-api-league"
	FixtureSyncerCommandRapidApiTeam    = "rapid-api-team"
	FixtureSyncerCommandClearDb         = "clear-db"
)

const (
	DBFixtureNotifyStateNew      = "NEW"
	DBFixtureNotifyStateKickoff  = "KICKOFF"
	DBFixtureNotifyStateHalftime = "HALFTIME"
	DBFixtureNotifyStateSecond   = "SECOND"
	DBFixtureNotifyStateFulltime = "FULLTIME"
)

type FixtureSyncerCommand struct {
	DB *gorm.DB
}

func NewFixtureSyncerCommand(db *gorm.DB) *FixtureSyncerCommand {
	return &FixtureSyncerCommand{
		DB: db,
	}
}

type FixtureSyncerCommandArgs struct {
	RapidApiBaseUrl    string
	RapidApiKey        string
	RapidApiHostHeader string
	RapidApiLeagueId   string
	RapidApiTeamId     string
	ClearDb            bool
}

func NewFixtureSyncerCommandArgs(c *cli.Command) *FixtureSyncerCommandArgs {
	return &FixtureSyncerCommandArgs{
		RapidApiBaseUrl:    c.String(FixtureSyncerCommandRapidApiBaseUrl),
		RapidApiKey:        c.String(FixtureSyncerCommandRapidApiKey),
		RapidApiHostHeader: c.String(FixtureSyncerCommandRapidApiHost),
		RapidApiLeagueId:   c.String(FixtureSyncerCommandRapidApiLeague),
		RapidApiTeamId:     c.String(FixtureSyncerCommandRapidApiTeam),
		ClearDb:            c.Bool(FixtureSyncerCommandClearDb),
	}
}

func (f *FixtureSyncerCommand) ToCliCommand() *cli.Command {
	return &cli.Command{
		Name:  "fixture-syncer",
		Usage: "Sync fixtures from the Rapid API",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        FixtureSyncerCommandRapidApiBaseUrl,
				Usage:       "Rapid API base URL",
				Required:    false,
				Value:       "https://api-football-v1.p.rapidapi.com/v3",
				DefaultText: "https://api-football-v1.p.rapidapi.com/v3",
				Sources:     cli.EnvVars("RAPID_API_BASE_URL"),
			},
			&cli.StringFlag{
				Name:     FixtureSyncerCommandRapidApiKey,
				Usage:    "Rapid API key",
				Required: true,
				Sources:  cli.EnvVars("RAPID_API_KEY"),
			},
			&cli.StringFlag{
				Name:     FixtureSyncerCommandRapidApiHost,
				Usage:    "Rapid API host header",
				Required: false,
				Value:    "api-football-v1.p.rapidapi.com",
				Sources:  cli.EnvVars("RAPID_API_HOST"),
			},
			&cli.StringFlag{
				Name:        FixtureSyncerCommandRapidApiLeague,
				Usage:       "Rapid API league ID",
				Required:    false,
				Value:       "489",
				DefaultText: "489",
				Sources:     cli.EnvVars("RAPID_API_LEAGUE"),
			},
			&cli.StringFlag{
				Name:        FixtureSyncerCommandRapidApiTeam,
				Usage:       "Rapid API team ID",
				Required:    false,
				Value:       "9025",
				DefaultText: "9025",
				Sources:     cli.EnvVars("RAPID_API_TEAM"),
			},
			&cli.BoolFlag{
				Name:     FixtureSyncerCommandClearDb,
				Usage:    "Clear the database before syncing",
				Required: false,
				Value:    false,
				Sources:  cli.EnvVars("CLEAR_DB"),
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			args := NewFixtureSyncerCommandArgs(c)
			return runOnce(ctx, args)
		},
		Commands: []*cli.Command{
			{
				Name:  "cron",
				Usage: "Run the fixture syncer in a cron schedule",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "schedule",
						Usage:       "Cron schedule",
						Required:    false,
						Value:       "*/2 * * * *",
						DefaultText: "*/2 * * * *",
						Sources:     cli.EnvVars("CRON_FIXTURE_SCHEDULE"),
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					args := NewFixtureSyncerCommandArgs(c)
					schedule := c.String("schedule")
					scheduler, err := gocron.NewScheduler()
					if err != nil {
						return err
					}

					defer func() { _ = scheduler.Shutdown() }()

					_, err = scheduler.NewJob(
						gocron.CronJob(schedule, false),
						gocron.NewTask(runOnce, ctx, args),
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

func runOnce(ctx context.Context, args *FixtureSyncerCommandArgs) error {
	db, err := gorm.Open(sqlite.Open("./state.db"), &gorm.Config{})
	if err != nil {
		return err
	}
	rapidClient, err := client.NewRapidClient(
		ctx,
		client.RapidClientOptions{
			APIKey:     args.RapidApiKey,
			BaseUrl:    args.RapidApiBaseUrl,
			HostHeader: args.RapidApiHostHeader,
			LeagueId:   args.RapidApiLeagueId,
			TeamId:     args.RapidApiTeamId,
		},
	)
	if err != nil {
		return err
	}

	// Clear the database if requested
	if args.ClearDb {
		if err := db.Exec("DELETE FROM database_fixtures").Error; err != nil {
			return err
		}
	}

	// Get all recent fixtures from Rapid API
	recentFixtures, err := rapidClient.GetFixturesByStatus(ctx, client.RapidFixtureStatusAllRecent)
	if err != nil {
		return err
	}

	// for each fixture, check if it exists in the database
	// if it doesn't, add it. If it does, update it.
	for _, fixture := range recentFixtures {
		dbFixture := client.DatabaseFixture{}
		if err := db.Where("id = ?", fixture.Fixture.ID).First(&dbFixture).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := db.Create(&client.DatabaseFixture{
				ID:           fixture.Fixture.ID,
				Date:         fixture.Fixture.Date,
				Timezone:     fixture.Fixture.Timezone,
				Status:       fixture.Fixture.Status.Short,
				Elapsed:      fixture.Fixture.Status.Elapsed,
				LeagueID:     fixture.League.ID,
				LeagueName:   fixture.League.Name,
				HomeTeamID:   fixture.Teams.Home.ID,
				AwayTeamID:   fixture.Teams.Away.ID,
				HomeTeamName: fixture.Teams.Home.Name,
				AwayTeamName: fixture.Teams.Away.Name,
				HomeScore:    fixture.Goals.Home,
				AwayScore:    fixture.Goals.Away,
				NotifyState:  DBFixtureNotifyStateNew,
			}).Error; err != nil {
				return err
			}
			continue
		}

		if err := db.Model(&dbFixture).Updates(client.DatabaseFixture{
			Date:      fixture.Fixture.Date,
			Timezone:  fixture.Fixture.Timezone,
			Status:    fixture.Fixture.Status.Short,
			Elapsed:   fixture.Fixture.Status.Elapsed,
			HomeScore: fixture.Goals.Home,
			AwayScore: fixture.Goals.Away,
		}).Error; err != nil {
			return err
		}
	}

	// For each database fixture older than 24 hours, delete it
	if err := db.Where("date < ?", time.Now().Add(-24*time.Hour)).Delete(&client.DatabaseFixture{}).Error; err != nil {
		return err
	}

	return nil
}
