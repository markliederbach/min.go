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

func NewThreadsClient(ctx context.Context, options ThreadsClientOptions) (*ThreadsClientImpl, error) {
	client := &ThreadsClientImpl{
		Options:    options,
		HttpClient: http.DefaultClient,
	}
	if !options.SkipUserIdPrefill {
		if err := client.ensureUserId(ctx); err != nil {
			return &ThreadsClientImpl{}, err
		}
	}
	return client, nil
}

func (c *ThreadsClientImpl) RefreshToken(ctx context.Context) (RefreshTokenResponse, error) {
	params := url.Values{}
	params.Set("grant_type", "th_refresh_token")
	resp, err := c.call(ctx, http.MethodGet, "/refresh_access_token", params)
	if err != nil {
		return RefreshTokenResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return RefreshTokenResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response RefreshTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return RefreshTokenResponse{}, err
	}

	return response, nil
}

func (c *ThreadsClientImpl) GetUserId(ctx context.Context) (GetUserIdResponse, error) {
	params := url.Values{}
	params.Set("fields", "id,username")

	resp, err := c.call(ctx, http.MethodGet, "/v1.0/me", params)
	if err != nil {
		return GetUserIdResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return GetUserIdResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response GetUserIdResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return GetUserIdResponse{}, err
	}
	return response, nil
}

func (c *ThreadsClientImpl) CreateTextPost(ctx context.Context, content string) (CreateTextPostResponse, error) {
	params := url.Values{}
	params.Set("media_type", "TEXT")
	params.Set("text", content)

	resp, err := c.call(ctx, http.MethodPost, fmt.Sprintf("/v1.0/%s/threads", c.userId), params)
	if err != nil {
		return CreateTextPostResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return CreateTextPostResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var containerResponse CreateTextPostContainerResponse
	if err := json.NewDecoder(resp.Body).Decode(&containerResponse); err != nil {
		return CreateTextPostResponse{}, err
	}

	if err := c.waitForMediaContainer(ctx, containerResponse.ID); err != nil {
		return CreateTextPostResponse{}, err
	}

	containerPublishResponse, err := c.publishContainer(ctx, containerResponse.ID)
	if err != nil {
		return CreateTextPostResponse{}, err
	}

	return CreateTextPostResponse{ID: containerPublishResponse.ID}, nil
}

func (c *ThreadsClientImpl) publishContainer(ctx context.Context, containerId string) (PublishMediaContainerResponse, error) {
	if err := c.ensureUserId(ctx); err != nil {
		return PublishMediaContainerResponse{}, err
	}

	params := url.Values{}
	params.Set("creation_id", containerId)

	resp, err := c.call(ctx, http.MethodPost, fmt.Sprintf("/v1.0/%s/threads_publish", c.userId), params)
	if err != nil {
		return PublishMediaContainerResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return PublishMediaContainerResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response PublishMediaContainerResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return PublishMediaContainerResponse{}, err
	}

	return response, nil
}

func (c *ThreadsClientImpl) ensureUserId(ctx context.Context) error {
	if c.userId == "" {
		userId, err := c.GetUserId(ctx)
		if err != nil {
			return err
		}
		c.userId = userId.ID
	}
	return nil
}

func (c *ThreadsClientImpl) waitForMediaContainer(ctx context.Context, containerId string) error {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return errors.New("timeout waiting for media container")
		case <-ticker.C:
			status, err := c.GetMediaContainerStatus(ctx, containerId)
			if err != nil {
				return err
			}

			switch status.Status {
			case ThreadsContainerStatusInProgress:
				slog.DebugContext(ctx, "container in progress")
			case ThreadsContainerStatusFinished:
				return nil
			default:
				return fmt.Errorf("unexpected container status: %s", status.Status)
			}
		}
	}
}

func (c *ThreadsClientImpl) GetMediaContainerStatus(ctx context.Context, containerId string) (GetMediaContainerStatusResponse, error) {
	params := url.Values{}
	params.Set("fields", "status,error_message")

	resp, err := c.call(ctx, http.MethodGet, fmt.Sprintf("/v1.0/%s", containerId), params)
	if err != nil {
		return GetMediaContainerStatusResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return GetMediaContainerStatusResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response GetMediaContainerStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return GetMediaContainerStatusResponse{}, err
	}
	return response, nil
}

func (c *ThreadsClientImpl) call(ctx context.Context, method, uri string, params url.Values) (*http.Response, error) {
	parsedUrl, err := url.Parse(c.Options.BaseUrl)
	if err != nil {
		return &http.Response{}, err
	}

	parsedUrl.Path = path.Join(parsedUrl.Path, uri)

	params.Set("access_token", c.Options.APIKey)
	parsedUrl.RawQuery = params.Encode()

	slog.DebugContext(ctx, "calling %s %s", method, parsedUrl.String())

	var req *http.Request
	switch method {
	case http.MethodGet:
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, parsedUrl.String(), nil)
		if err != nil {
			return &http.Response{}, err
		}
	case http.MethodPost:
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, parsedUrl.String(), nil)
		if err != nil {
			return &http.Response{}, err
		}
	default:
		return &http.Response{}, fmt.Errorf("unsupported method: %s", method)
	}

	var resp *http.Response
	backoff := time.Second
	for {
		resp, err = c.HttpClient.Do(req)
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
