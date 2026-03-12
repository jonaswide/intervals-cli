package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jonaswide/intervals-cli/internal/api/gen"
	"github.com/jonaswide/intervals-cli/internal/httpx"
)

const athleteID = "0"

type Config struct {
	BaseURL   string
	Timeout   time.Duration
	Verbose   bool
	Stderr    io.Writer
	UserAgent string
}

type Client struct {
	api      *gen.ClientWithResponses
	authMode string
}

type ActivityListOptions struct {
	Oldest  string
	Newest  *string
	RouteID *int64
	Limit   *int32
	Fields  []string
}

type ActivitySearchOptions struct {
	Query string
	Limit *int32
}

type ActivityUploadOptions struct {
	FilePath      string
	Name          *string
	Description   *string
	DeviceName    *string
	ExternalID    *string
	PairedEventID *int32
}

type ActivityStreamsOptions struct {
	Types           []string
	IncludeDefaults *bool
}

type ActivityBestEffortsOptions struct {
	Stream           string
	DurationSec      *int32
	DistanceM        *float32
	Count            *int32
	MinValue         *float32
	ExcludeIntervals *bool
	StartIndex       *int32
	EndIndex         *int32
}

type EventListOptions struct {
	Oldest   *string
	Newest   *string
	Category []string
	Limit    *int32
}

type WellnessListOptions struct {
	Oldest *string
	Newest *string
	Fields []string
}

type APIError struct {
	StatusCode int
	Body       []byte
}

func (e *APIError) Error() string {
	msg := strings.TrimSpace(string(e.Body))
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	var payload map[string]any
	if err := json.Unmarshal(e.Body, &payload); err == nil {
		for _, key := range []string{"message", "error", "detail"} {
			if s, ok := payload[key].(string); ok && s != "" {
				msg = s
				break
			}
		}
	}
	return fmt.Sprintf("api error (%d): %s", e.StatusCode, msg)
}

func NewClient(cfg Config) (*Client, error) {
	editor, mode, err := resolveAuth(cfg.UserAgent)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
		Transport: httpx.RetryTransport{
			Base:    http.DefaultTransport,
			Verbose: cfg.Verbose,
			Stderr:  cfg.Stderr,
		},
	}
	api, err := gen.NewClientWithResponses(cfg.BaseURL,
		gen.WithHTTPClient(httpClient),
		gen.WithRequestEditorFn(editor),
	)
	if err != nil {
		return nil, err
	}
	return &Client{api: api, authMode: mode}, nil
}

func (c *Client) AuthStatus(ctx context.Context) (any, error) {
	whoami, err := c.WhoAmI(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"auth_mode": c.authMode,
		"athlete":   whoami,
	}, nil
}

func (c *Client) WhoAmI(ctx context.Context) (any, error) {
	return c.AthleteProfile(ctx)
}

func (c *Client) AthleteGet(ctx context.Context) (any, error) {
	resp, err := c.api.GetAthleteWithResponse(ctx, athleteID)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) AthleteProfile(ctx context.Context) (any, error) {
	resp, err := c.api.GetAthleteProfileWithResponse(ctx, athleteID)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) AthleteTrainingPlan(ctx context.Context) (any, error) {
	resp, err := c.api.GetAthleteTrainingPlanWithResponse(ctx, athleteID)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) ActivitiesList(ctx context.Context, opts ActivityListOptions) (any, error) {
	params := &gen.ListActivitiesParams{
		Oldest:  opts.Oldest,
		Newest:  opts.Newest,
		RouteId: opts.RouteID,
		Limit:   opts.Limit,
	}
	if len(opts.Fields) > 0 {
		params.Fields = &opts.Fields
	}
	resp, err := c.api.ListActivitiesWithResponse(ctx, athleteID, params)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) ActivitiesSearch(ctx context.Context, opts ActivitySearchOptions) (any, error) {
	params := &gen.SearchForActivitiesParams{Q: opts.Query, Limit: opts.Limit}
	resp, err := c.api.SearchForActivitiesWithResponse(ctx, athleteID, params)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) ActivitiesUpload(ctx context.Context, opts ActivityUploadOptions) (any, error) {
	body, contentType, err := buildMultipartFile("file", opts.FilePath)
	if err != nil {
		return nil, err
	}
	params := &gen.UploadActivityParams{
		Name:          opts.Name,
		Description:   opts.Description,
		DeviceName:    opts.DeviceName,
		ExternalId:    opts.ExternalID,
		PairedEventId: opts.PairedEventID,
	}
	resp, err := c.api.UploadActivityWithBodyWithResponse(ctx, athleteID, params, contentType, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK, http.StatusCreated)
}

func (c *Client) ActivityGet(ctx context.Context, id string) (any, error) {
	resp, err := c.api.GetActivityWithResponse(ctx, id, nil)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) ActivityStreams(ctx context.Context, id string, opts ActivityStreamsOptions) (any, error) {
	params := &gen.GetActivityStreamsParams{
		IncludeDefaults: opts.IncludeDefaults,
	}
	if len(opts.Types) > 0 {
		params.Types = &opts.Types
	}
	resp, err := c.api.GetActivityStreamsWithResponse(ctx, id, "", params)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) ActivityIntervals(ctx context.Context, id string) (any, error) {
	resp, err := c.api.GetIntervalsWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) ActivityBestEfforts(ctx context.Context, id string, opts ActivityBestEffortsOptions) (any, error) {
	params := &gen.FindBestEffortsParams{
		Stream:           opts.Stream,
		Duration:         opts.DurationSec,
		Distance:         opts.DistanceM,
		Count:            opts.Count,
		MinValue:         opts.MinValue,
		ExcludeIntervals: opts.ExcludeIntervals,
		StartIndex:       opts.StartIndex,
		EndIndex:         opts.EndIndex,
	}
	resp, err := c.api.FindBestEffortsWithResponse(ctx, id, params)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) ActivityDownload(ctx context.Context, id, kind string) ([]byte, error) {
	switch kind {
	case "original":
		resp, err := c.api.DownloadActivityFileWithResponse(ctx, id)
		if err != nil {
			return nil, err
		}
		return decodeBinary(resp.StatusCode(), resp.Body, http.StatusOK)
	case "fit":
		resp, err := c.api.DownloadActivityFitFileWithResponse(ctx, id, nil)
		if err != nil {
			return nil, err
		}
		return decodeBinary(resp.StatusCode(), resp.Body, http.StatusOK)
	case "gpx":
		resp, err := c.api.DownloadActivityGpxFileWithResponse(ctx, id, nil)
		if err != nil {
			return nil, err
		}
		return decodeBinary(resp.StatusCode(), resp.Body, http.StatusOK)
	default:
		return nil, fmt.Errorf("unsupported activity download kind %q", kind)
	}
}

func (c *Client) EventsList(ctx context.Context, opts EventListOptions) (any, error) {
	params := &gen.ListEventsParams{
		Oldest: opts.Oldest,
		Newest: opts.Newest,
		Limit:  opts.Limit,
	}
	if len(opts.Category) > 0 {
		params.Category = &opts.Category
	}
	resp, err := c.api.ListEventsWithResponse(ctx, athleteID, "", params)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) EventGet(ctx context.Context, id int32) (any, error) {
	resp, err := c.api.ShowEventWithResponse(ctx, athleteID, id)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) EventsCreate(ctx context.Context, body []byte) (any, error) {
	params := &gen.CreateEventParams{UpsertOnUid: false}
	resp, err := c.api.CreateEventWithBodyWithResponse(ctx, athleteID, params, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) EventsUpsert(ctx context.Context, body []byte) (any, error) {
	params := &gen.CreateEventParams{UpsertOnUid: true}
	resp, err := c.api.CreateEventWithBodyWithResponse(ctx, athleteID, params, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) EventDelete(ctx context.Context, id int32) (any, error) {
	resp, err := c.api.DeleteEventWithResponse(ctx, athleteID, id, nil)
	if err != nil {
		return nil, err
	}
	if _, err := decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "event_id": id}, nil
}

func (c *Client) WorkoutsList(ctx context.Context) (any, error) {
	resp, err := c.api.ListWorkoutsWithResponse(ctx, athleteID)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) WorkoutGet(ctx context.Context, id int32) (any, error) {
	resp, err := c.api.ShowWorkoutWithResponse(ctx, athleteID, id)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) WorkoutsCreate(ctx context.Context, body []byte) (any, error) {
	resp, err := c.api.CreateWorkoutWithBodyWithResponse(ctx, athleteID, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) WorkoutDownload(ctx context.Context, id int32, format string) ([]byte, error) {
	workoutResp, err := c.api.ShowWorkoutWithResponse(ctx, athleteID, id)
	if err != nil {
		return nil, err
	}
	if _, err := decodeJSON(workoutResp.StatusCode(), workoutResp.Body, http.StatusOK); err != nil {
		return nil, err
	}
	resp, err := c.api.DownloadWorkoutForAthleteWithBodyWithResponse(ctx, athleteID, "."+format, "application/json", bytes.NewReader(workoutResp.Body))
	if err != nil {
		return nil, err
	}
	return decodeBinary(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) WellnessList(ctx context.Context, opts WellnessListOptions) (any, error) {
	params := &gen.ListWellnessRecordsParams{
		Oldest: opts.Oldest,
		Newest: opts.Newest,
	}
	if len(opts.Fields) > 0 {
		params.Fields = &opts.Fields
	}
	resp, err := c.api.ListWellnessRecordsWithResponse(ctx, athleteID, "", params)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) WellnessGet(ctx context.Context, date string) (any, error) {
	resp, err := c.api.GetRecordWithResponse(ctx, athleteID, date)
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) WellnessPut(ctx context.Context, date string, body []byte) (any, error) {
	resp, err := c.api.UpdateWellnessWithBodyWithResponse(ctx, athleteID, date, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK)
}

func (c *Client) WellnessBulkPut(ctx context.Context, body []byte) (any, error) {
	resp, err := c.api.UpdateWellnessBulkWithBodyWithResponse(ctx, athleteID, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if _, err := decodeJSON(resp.StatusCode(), resp.Body, http.StatusOK); err != nil {
		return nil, err
	}
	var items []any
	_ = json.Unmarshal(body, &items)
	return map[string]any{"ok": true, "count": len(items)}, nil
}

func resolveAuth(userAgent string) (gen.RequestEditorFn, string, error) {
	if token := strings.TrimSpace(os.Getenv("INTERVALS_ACCESS_TOKEN")); token != "" {
		return func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("User-Agent", userAgent)
			return nil
		}, "bearer", nil
	}
	if apiKey := strings.TrimSpace(os.Getenv("INTERVALS_API_KEY")); apiKey != "" {
		return func(_ context.Context, req *http.Request) error {
			req.SetBasicAuth("API_KEY", apiKey)
			req.Header.Set("User-Agent", userAgent)
			return nil
		}, "api_key", nil
	}
	return nil, "", errors.New("missing auth: set INTERVALS_ACCESS_TOKEN or INTERVALS_API_KEY")
}

func decodeJSON(status int, body []byte, okStatuses ...int) (any, error) {
	if !containsStatus(status, okStatuses) {
		return nil, &APIError{StatusCode: status, Body: body}
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return map[string]any{"ok": true}, nil
	}
	var out any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode response json: %w", err)
	}
	return out, nil
}

func decodeBinary(status int, body []byte, okStatuses ...int) ([]byte, error) {
	if !containsStatus(status, okStatuses) {
		return nil, &APIError{StatusCode: status, Body: body}
	}
	return body, nil
}

func containsStatus(status int, okStatuses []int) bool {
	for _, allowed := range okStatuses {
		if status == allowed {
			return true
		}
	}
	return false
}

func buildMultipartFile(fieldName, path string) ([]byte, string, error) {
	fileData, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	part, err := writer.CreateFormFile(fieldName, filepath.Base(path))
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(fileData); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}

func RandomExternalID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func ParseInt32(raw string) (int32, error) {
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(n), nil
}
