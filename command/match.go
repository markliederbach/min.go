package command

import (
	"context"
	"errors"
	"log/slog"

	"github.com/markliederbach/min.go/client"
	"github.com/urfave/cli/v3"
	"gorm.io/gorm"
)

const (
	MatchCommandRapidApiBaseUrl  = "rapid-api-base-url"
	MatchCommandRapidApiKey      = "rapid-api-key"
	MatchCommandRapidApiHost     = "rapid-api-host"
	MatchCommandRapidApiLeague   = "rapid-api-league"
	MatchCommandRapidApiTeam     = "rapid-api-team"
	MatchCommandKickoffThreshold = "kickoff-threshold"
	MatchCommandClearDb          = "clear-db"
	MatchCommandThreadsApiKey    = "threads-api-key"
	MatchCommandThreadsBaseUrl   = "threads-base-url"
	MatchCommandUsername         = "threads-username"
)

type MatchCommandArgs struct {
	RapidApiBaseUrl    string
	RapidApiKey        string
	RapidApiHostHeader string
	RapidApiLeagueId   string
	RapidApiTeamId     string
	KickoffThreshold   int
	ClearDb            bool
	ThreadsApiKey      string
	ThreadsBaseUrl     string
	ThreadsUsername    string
}

type MatchCommand struct {
	DB *gorm.DB
}

func NewMatchCommand(db *gorm.DB) *MatchCommand {
	return &MatchCommand{
		DB: db,
	}
}

func NewMatchCommandArgs(c *cli.Command) *MatchCommandArgs {
	return &MatchCommandArgs{
		RapidApiBaseUrl:    c.String(MatchCommandRapidApiBaseUrl),
		RapidApiKey:        c.String(MatchCommandRapidApiKey),
		RapidApiHostHeader: c.String(MatchCommandRapidApiHost),
		RapidApiLeagueId:   c.String(MatchCommandRapidApiLeague),
		RapidApiTeamId:     c.String(MatchCommandRapidApiTeam),
		KickoffThreshold:   int(c.Int(MatchCommandKickoffThreshold)),
		ClearDb:            c.Bool(MatchCommandClearDb),
		ThreadsApiKey:      c.String(MatchCommandThreadsApiKey),
		ThreadsBaseUrl:     c.String(MatchCommandThreadsBaseUrl),
		ThreadsUsername:    c.String(MatchCommandUsername),
	}
}

func (m *MatchCommand) ToCliCommand() *cli.Command {
	return &cli.Command{
		Name:  "match",
		Usage: "Retrieve updates from Forward Madison FC matches and publish to the Fediverse",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        MatchCommandRapidApiBaseUrl,
				Usage:       "Rapid API base URL",
				Required:    false,
				Value:       "https://api-football-v1.p.rapidapi.com/v3",
				DefaultText: "https://api-football-v1.p.rapidapi.com/v3",
				Sources:     cli.EnvVars("RAPID_API_BASE_URL"),
			},
			&cli.StringFlag{
				Name:     MatchCommandRapidApiKey,
				Usage:    "Rapid API key",
				Required: true,
				Sources:  cli.EnvVars("RAPID_API_KEY"),
			},
			&cli.StringFlag{
				Name:     MatchCommandRapidApiHost,
				Usage:    "Rapid API host header",
				Required: false,
				Value:    "api-football-v1.p.rapidapi.com",
				Sources:  cli.EnvVars("RAPID_API_HOST"),
			},
			&cli.StringFlag{
				Name:        MatchCommandRapidApiLeague,
				Usage:       "Rapid API league ID",
				Required:    false,
				Value:       "489",
				DefaultText: "489",
				Sources:     cli.EnvVars("RAPID_API_LEAGUE"),
			},
			&cli.StringFlag{
				Name:        MatchCommandRapidApiTeam,
				Usage:       "Rapid API team ID",
				Required:    false,
				Value:       "9025",
				DefaultText: "9025",
				Sources:     cli.EnvVars("RAPID_API_TEAM"),
			},
			&cli.IntFlag{
				Name:        MatchCommandKickoffThreshold,
				Usage:       "Kickoff threshold in minutes",
				Required:    false,
				Value:       10,
				DefaultText: "10",
				Sources:     cli.EnvVars("KICKOFF_THRESHOLD"),
			},
			&cli.BoolFlag{
				Name:        MatchCommandClearDb,
				Usage:       "Clear the database before running the command",
				Required:    false,
				Value:       false,
				DefaultText: "false",
				Sources:     cli.EnvVars("CLEAR_DB"),
			},
			&cli.StringFlag{
				Name:     MatchCommandThreadsApiKey,
				Usage:    "Threads API key",
				Required: true,
				Sources:  cli.EnvVars("THREADS_API_KEY"),
			},
			&cli.StringFlag{
				Name:        MatchCommandThreadsBaseUrl,
				Usage:       "Threads base URL",
				Required:    false,
				Value:       "https://graph.threads.net",
				DefaultText: "https://graph.threads.net",
				Sources:     cli.EnvVars("THREADS_BASE_URL"),
			},
			&cli.StringFlag{
				Name:        MatchCommandUsername,
				Usage:       "Threads username",
				Required:    false,
				Value:       "mingos.updates",
				DefaultText: "mingos.updates",
				Sources:     cli.EnvVars("THREADS_USERNAME"),
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			slog.InfoContext(ctx, "running match command")
			args := NewMatchCommandArgs(c)

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

			// postResponse, err := threadsClient.CreateTextPost(ctx, "This is a test\n#HelloWorld")
			_, err = threadsClient.GetUserId(ctx)
			if err != nil {
				return err
			}

			rapidClient, err := client.NewRapidClient(
				ctx,
				client.RapidClientOptions{
					APIKey:     args.RapidApiKey,
					HostHeader: args.RapidApiHostHeader,
					BaseUrl:    args.RapidApiBaseUrl,
					LeagueId:   args.RapidApiLeagueId,
					TeamId:     args.RapidApiTeamId,
				},
			)
			if err != nil {
				return err
			}

			if args.ClearDb {
				slog.InfoContext(ctx, "clearing database")
				err := m.DB.Exec("DELETE FROM event_infos").Error
				if err != nil {
					return err
				}
				err = m.DB.Exec("DELETE FROM fixture_infos").Error
				if err != nil {
					return err
				}
			}

			recentFixtures, err := rapidClient.GetFixturesByStatus(ctx, client.RapidFixtureStatusAllRecent)
			if err != nil {
				return err
			}

			if len(recentFixtures) == 0 {
				return errors.New("no in progress fixtures found")
			}

			kickoffFixtures, err := m.getNewKickoffFixtures(ctx, recentFixtures, args.KickoffThreshold)
			if err != nil {
				return err
			}

			for _, fixture := range kickoffFixtures {
				slog.InfoContext(ctx, "new fixture", "fixture", fixture)
			}

			return nil
		},
	}
}

func (m *MatchCommand) getNewKickoffFixtures(ctx context.Context, fixtures []client.FixtureInfo, kickoffThreshold int) ([]client.FixtureInfo, error) {
	kickoffFixtures := []client.FixtureInfo{}

	for _, fixture := range fixtures {
		err := m.DB.Where("id = ?", fixture.Fixture.ID).First(&fixture).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				slog.ErrorContext(ctx, "error querying for fixture", err)
				return kickoffFixtures, err
			}
			// Add new fixture to DB
			slog.InfoContext(ctx, "fixture does not exist, creating")
			err := m.DB.Create(&fixture).Error
			if err != nil {
				return kickoffFixtures, err
			}
			if fixture.Fixture.Status.Short == string(client.RapidFixtureStatusKickoff) && fixture.Fixture.Status.Elapsed <= kickoffThreshold {
				kickoffFixtures = append(kickoffFixtures, fixture)
			}
		}
		// slog.InfoContext(ctx, "fixture exists, updating")
		// err = m.DB.Save(&fixture).Error
		// if err != nil {
		// 	return kickoffFixtures, err
		// }
	}
	return kickoffFixtures, nil
}

func (m *MatchCommand) getNewEvents(ctx context.Context, rapidClient *client.RapidClientImpl, fixtureId int) ([]client.EventInfo, error) {
	events, err := rapidClient.GetMatchEvents(ctx, fixtureId)
	if err != nil {
		return []client.EventInfo{}, err
	}

	newEvents := []client.EventInfo{}

	for _, event := range events {
		err := m.DB.Where(
			"fixture_id = ? AND time_elapsed = ? AND time_extra = ? AND type = ? AND player_id = ?",
			event.FixtureID, event.Time.Elapsed, event.Time.Extra, event.Type, event.Player.ID,
		).First(&event).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.ErrorContext(ctx, "error querying for event", err)
			continue
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.InfoContext(ctx, "event does not exist, creating")
			err := m.DB.Create(&event).Error
			if err != nil {
				slog.ErrorContext(ctx, "error creating event", err)
				continue
			}
			newEvents = append(newEvents, event)
		}
	}

	return newEvents, nil
}
