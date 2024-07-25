package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"time"
)

func NewRapidClient(ctx context.Context, options RapidClientOptions) (*RapidClientImpl, error) {
	return &RapidClientImpl{
		Options:    options,
		HttpClient: http.DefaultClient,
	}, nil
}

func (r *RapidClientImpl) GetSeasonTeams(ctx context.Context, year string) ([]TeamVenue, error) {
	params := url.Values{}
	params.Set("league", r.Options.LeagueId)
	params.Set("season", year)

	resp, err := r.call(ctx, http.MethodGet, "/teams", params)
	if err != nil {
		return []TeamVenue{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return []TeamVenue{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response GetTeamsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return []TeamVenue{}, err
	}

	return response.Response, nil
}

func (r *RapidClientImpl) GetFixturesByStatus(ctx context.Context, status RapidFixtureStatus) ([]FixtureInfo, error) {
	params := url.Values{}
	params.Set("status", string(status))
	params.Set("season", time.Now().Format("2006"))
	params.Set("date", time.Now().Format("2006-01-02"))
	// params.Set("league", r.Options.LeagueId)
	// params.Set("team", r.Options.TeamId)

	resp, err := r.call(ctx, http.MethodGet, "/fixtures", params)
	if err != nil {
		return []FixtureInfo{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return []FixtureInfo{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response GetFixturesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return []FixtureInfo{}, err
	}

	return response.Response, nil
}

func (r *RapidClientImpl) GetMatchEvents(ctx context.Context, fixtureId int) ([]EventInfo, error) {
	params := url.Values{}
	params.Set("fixture", fmt.Sprintf("%d", fixtureId))

	resp, err := r.call(ctx, http.MethodGet, "/fixtures/events", params)
	if err != nil {
		return []EventInfo{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return []EventInfo{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response GetEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return []EventInfo{}, err
	}

	for i := range response.Response {
		response.Response[i].FixtureID = fixtureId
	}

	return response.Response, nil
}

func (r *RapidClientImpl) call(ctx context.Context, method, uri string, params url.Values) (*http.Response, error) {
	parsedUrl, err := url.Parse(r.Options.BaseUrl)
	if err != nil {
		return &http.Response{}, err
	}

	parsedUrl.Path = path.Join(parsedUrl.Path, uri)
	parsedUrl.RawQuery = params.Encode()

	slog.InfoContext(ctx, "calling %s %s", method, parsedUrl.String())

	var req *http.Request
	switch method {
	case http.MethodGet:
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, parsedUrl.String(), nil)
		if err != nil {
			return &http.Response{}, err
		}
	// case http.MethodPost:
	// 	req, err = http.NewRequestWithContext(ctx, http.MethodPost, parsedUrl.String(), nil)
	// 	if err != nil {
	// 		return &http.Response{}, err
	// 	}
	default:
		return &http.Response{}, fmt.Errorf("unsupported method: %s", method)
	}

	req.Header.Set("X-RapidAPI-Key", r.Options.APIKey)
	req.Header.Set("X-RapidAPI-Host", r.Options.HostHeader)

	var resp *http.Response
	backoff := time.Second
	for {
		resp, err = r.HttpClient.Do(req)
		if err != nil {
			return &http.Response{}, err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			if backoff > 5*time.Minute {
				return resp, errors.New(resp.Status)
			}

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return resp, ctx.Err()
			}

			backoff = time.Duration(float64(backoff) * 1.5)
			continue
		}

		break
	}
	return resp, nil
}
