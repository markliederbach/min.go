package command

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/markliederbach/min.go/client"
)

type RefreshThreadsTokenCommand struct{}

func NewRefreshThreadsTokenCommand() *RefreshThreadsTokenCommand {
	return &RefreshThreadsTokenCommand{}
}

type RefreshThreadsTokenCommandArgs struct {
	ApiKey         string
	ThreadsBaseUrl string
}

func NewRefreshThreadsTokenCommandArgs(c *cli.Command) *RefreshThreadsTokenCommandArgs {
	return &RefreshThreadsTokenCommandArgs{
		ApiKey:         c.String(MatchCommandThreadsApiKey),
		ThreadsBaseUrl: c.String(MatchCommandThreadsBaseUrl),
	}
}

func (c *RefreshThreadsTokenCommand) ToCliCommand() *cli.Command {
	return &cli.Command{
		Name:  "refresh-threads-token",
		Usage: "Refresh the Threads API access token",
		Flags: []cli.Flag{
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
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			args := NewRefreshThreadsTokenCommandArgs(c)
			client, err := client.NewThreadsClient(ctx, client.ThreadsClientOptions{
				APIKey:            args.ApiKey,
				BaseUrl:           args.ThreadsBaseUrl,
				SkipUserIdPrefill: true,
			})
			if err != nil {
				return err
			}
			resp, err := client.RefreshToken(ctx)
			if err != nil {
				return err
			}
			// print marshaled response in json
			jsonResp, err := json.Marshal(resp)
			if err != nil {
				return err
			}
			fmt.Println(string(jsonResp))
			return nil
		},
	}
}
