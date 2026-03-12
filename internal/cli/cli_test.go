package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jonaswide/intervals-cli/internal/app"
)

type fakeService struct {
	whoami           func(context.Context) (any, error)
	activitiesSearch func(context.Context, app.ActivitySearchOptions) (any, error)
}

func (f fakeService) AuthStatus(context.Context) (any, error) {
	return map[string]any{"auth_mode": "bearer"}, nil
}
func (f fakeService) WhoAmI(ctx context.Context) (any, error) { return f.whoami(ctx) }
func (f fakeService) AthleteGet(context.Context) (any, error) {
	return map[string]any{"id": "0", "name": "Jonas"}, nil
}
func (f fakeService) AthleteProfile(context.Context) (any, error) {
	return map[string]any{"athlete": map[string]any{"id": "0", "name": "Jonas"}}, nil
}
func (f fakeService) AthleteTrainingPlan(context.Context) (any, error) {
	return map[string]any{"training_plan_id": 123}, nil
}
func (f fakeService) ActivitiesList(context.Context, app.ActivityListOptions) (any, error) {
	return []any{}, nil
}
func (f fakeService) ActivitiesSearch(ctx context.Context, opts app.ActivitySearchOptions) (any, error) {
	if f.activitiesSearch != nil {
		return f.activitiesSearch(ctx, opts)
	}
	return []any{}, nil
}
func (f fakeService) ActivitiesUpload(context.Context, app.ActivityUploadOptions) (any, error) {
	return map[string]any{"ok": true}, nil
}
func (f fakeService) ActivityGet(context.Context, string) (any, error) {
	return map[string]any{"id": "a1"}, nil
}
func (f fakeService) ActivityStreams(context.Context, string, app.ActivityStreamsOptions) (any, error) {
	return []any{}, nil
}
func (f fakeService) ActivityIntervals(context.Context, string) (any, error) {
	return map[string]any{"icu_intervals": []any{}}, nil
}
func (f fakeService) ActivityBestEfforts(context.Context, string, app.ActivityBestEffortsOptions) (any, error) {
	return map[string]any{"efforts": []any{}}, nil
}
func (f fakeService) ActivityDownload(context.Context, string, string) ([]byte, error) {
	return []byte("bytes"), nil
}
func (f fakeService) EventsList(context.Context, app.EventListOptions) (any, error) {
	return []any{}, nil
}
func (f fakeService) EventGet(context.Context, int32) (any, error) {
	return map[string]any{"id": 1}, nil
}
func (f fakeService) EventsCreate(context.Context, []byte) (any, error) {
	return map[string]any{"id": 1}, nil
}
func (f fakeService) EventsUpsert(context.Context, []byte) (any, error) {
	return map[string]any{"id": 1}, nil
}
func (f fakeService) EventDelete(context.Context, int32) (any, error) {
	return map[string]any{"ok": true}, nil
}
func (f fakeService) WorkoutsList(context.Context) (any, error) { return []any{}, nil }
func (f fakeService) WorkoutGet(context.Context, int32) (any, error) {
	return map[string]any{"id": 1}, nil
}
func (f fakeService) WorkoutsCreate(context.Context, []byte) (any, error) {
	return map[string]any{"id": 1}, nil
}
func (f fakeService) WorkoutDownload(context.Context, int32, string) ([]byte, error) {
	return []byte("bytes"), nil
}
func (f fakeService) WellnessList(context.Context, app.WellnessListOptions) (any, error) {
	return []any{}, nil
}
func (f fakeService) WellnessGet(context.Context, string) (any, error) {
	return map[string]any{"id": "2026-03-12"}, nil
}
func (f fakeService) WellnessPut(context.Context, string, []byte) (any, error) {
	return map[string]any{"ok": true}, nil
}
func (f fakeService) WellnessBulkPut(context.Context, []byte) (any, error) {
	return map[string]any{"ok": true}, nil
}

func TestRunDefaultsToJSONForNonTTY(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := &runtime{
		ctx:    context.Background(),
		stdout: &stdout,
		stderr: &stderr,
		stdin:  strings.NewReader(""),
		newClient: func(cfg app.Config) (service, error) {
			return fakeService{
				whoami: func(context.Context) (any, error) {
					return map[string]any{"athlete": map[string]any{"id": "0", "name": "Jonas"}}, nil
				},
			}, nil
		},
	}
	if err := rt.run([]string{"whoami"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout.String(), "\"name\": \"Jonas\"") {
		t.Fatalf("expected json output, got %q", stdout.String())
	}
}

func TestExplicitTableFormat(t *testing.T) {
	var stdout bytes.Buffer
	rt := &runtime{
		ctx:    context.Background(),
		stdout: &stdout,
		stderr: io.Discard,
		stdin:  strings.NewReader(""),
		newClient: func(cfg app.Config) (service, error) {
			return fakeService{}, nil
		},
	}
	if err := rt.run([]string{"--format", "table", "athlete", "get"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "name") || !strings.Contains(out, "Jonas") {
		t.Fatalf("expected table output, got %q", out)
	}
}

func TestGlobalFormatFlagWorksAfterCommand(t *testing.T) {
	var stdout bytes.Buffer
	rt := &runtime{
		ctx:    context.Background(),
		stdout: &stdout,
		stderr: io.Discard,
		stdin:  strings.NewReader(""),
		newClient: func(cfg app.Config) (service, error) {
			return fakeService{}, nil
		},
	}
	if err := rt.run([]string{"athlete", "get", "--format", "json"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout.String(), "\"name\": \"Jonas\"") {
		t.Fatalf("expected json output, got %q", stdout.String())
	}
}

func TestActivitiesSearchAcceptsDateWindow(t *testing.T) {
	var got app.ActivitySearchOptions
	rt := &runtime{
		ctx:    context.Background(),
		stdout: io.Discard,
		stderr: io.Discard,
		stdin:  strings.NewReader(""),
		newClient: func(cfg app.Config) (service, error) {
			return fakeService{
				activitiesSearch: func(_ context.Context, opts app.ActivitySearchOptions) (any, error) {
					got = opts
					return []any{}, nil
				},
			}, nil
		},
	}
	if err := rt.run([]string{"activities", "search", "--query", "tempo", "--oldest", "2026-03-01", "--newest", "2026-03-12"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got.Oldest == nil || *got.Oldest != "2026-03-01" {
		t.Fatalf("expected oldest to be forwarded, got %+v", got)
	}
	if got.Newest == nil || *got.Newest != "2026-03-12" {
		t.Fatalf("expected newest to be forwarded, got %+v", got)
	}
}

func TestActivitiesSearchRequiresOldestWhenNewestIsSet(t *testing.T) {
	rt := &runtime{
		ctx:    context.Background(),
		stdout: io.Discard,
		stderr: io.Discard,
		stdin:  strings.NewReader(""),
		newClient: func(cfg app.Config) (service, error) {
			t.Fatal("service should not be initialized")
			return nil, nil
		},
	}
	err := rt.run([]string{"activities", "search", "--query", "tempo", "--newest", "2026-03-12"})
	if ExitCode(err) != 2 {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestBestEffortsValidationHappensBeforeServiceInit(t *testing.T) {
	rt := &runtime{
		ctx:    context.Background(),
		stdout: io.Discard,
		stderr: io.Discard,
		stdin:  strings.NewReader(""),
		newClient: func(cfg app.Config) (service, error) {
			t.Fatal("service should not be initialized")
			return nil, nil
		},
	}
	err := rt.run([]string{"activity", "best-efforts", "a1", "--stream", "watts"})
	if ExitCode(err) != 2 {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestEventUpsertRequiresUIDBeforeServiceInit(t *testing.T) {
	rt := &runtime{
		ctx:    context.Background(),
		stdout: io.Discard,
		stderr: io.Discard,
		stdin:  strings.NewReader(`{"name":"test"}`),
		newClient: func(cfg app.Config) (service, error) {
			t.Fatal("service should not be initialized")
			return nil, nil
		},
	}
	err := rt.run([]string{"events", "upsert", "--file", "-"})
	if ExitCode(err) != 2 {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestConfigErrorMapsToExitCode3(t *testing.T) {
	rt := &runtime{
		ctx:    context.Background(),
		stdout: io.Discard,
		stderr: io.Discard,
		stdin:  strings.NewReader(""),
		newClient: func(cfg app.Config) (service, error) {
			return nil, errors.New("missing auth")
		},
	}
	err := rt.run([]string{"whoami"})
	if ExitCode(err) != 3 {
		t.Fatalf("expected exit code 3, got %d (%v)", ExitCode(err), err)
	}
}

func TestWriteOutputFileCreatesParents(t *testing.T) {
	dir := t.TempDir()
	target := dir + "/nested/out.bin"
	if err := writeOutputFile(target, []byte("abc"), io.Discard); err != nil {
		t.Fatalf("writeOutputFile: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != "abc" {
		t.Fatalf("unexpected file contents %q", string(data))
	}
}

func TestValidateDateTime(t *testing.T) {
	for _, raw := range []string{"2026-03-12", "2026-03-12T12:30", time.Now().UTC().Format(time.RFC3339)} {
		if err := validateDateTime(raw); err != nil {
			t.Fatalf("expected %q to be valid: %v", raw, err)
		}
	}
	if err := validateDateTime("tomorrow"); err == nil {
		t.Fatal("expected relative date to be rejected")
	}
}
