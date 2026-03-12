package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jonaswide/intervals-cli/internal/app"
	"github.com/jonaswide/intervals-cli/internal/out"
	"github.com/jonaswide/intervals-cli/internal/version"
)

type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ce *cliError
	if errors.As(err, &ce) {
		return ce.code
	}
	var apiErr *app.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case httpStatusUnauthorized, httpStatusForbidden:
			return 4
		case httpStatusNotFound:
			return 5
		case httpStatusTooManyRequests:
			return 6
		default:
			return 7
		}
	}
	return 7
}

const (
	httpStatusUnauthorized    = 401
	httpStatusForbidden       = 403
	httpStatusNotFound        = 404
	httpStatusTooManyRequests = 429
)

type service interface {
	AuthStatus(context.Context) (any, error)
	WhoAmI(context.Context) (any, error)
	AthleteGet(context.Context) (any, error)
	AthleteProfile(context.Context) (any, error)
	AthleteTrainingPlan(context.Context) (any, error)
	ActivitiesList(context.Context, app.ActivityListOptions) (any, error)
	ActivitiesSearch(context.Context, app.ActivitySearchOptions) (any, error)
	ActivitiesUpload(context.Context, app.ActivityUploadOptions) (any, error)
	ActivityGet(context.Context, string) (any, error)
	ActivityStreams(context.Context, string, app.ActivityStreamsOptions) (any, error)
	ActivityIntervals(context.Context, string) (any, error)
	ActivityBestEfforts(context.Context, string, app.ActivityBestEffortsOptions) (any, error)
	ActivityDownload(context.Context, string, string) ([]byte, error)
	EventsList(context.Context, app.EventListOptions) (any, error)
	EventGet(context.Context, int32) (any, error)
	EventsCreate(context.Context, []byte) (any, error)
	EventsUpsert(context.Context, []byte) (any, error)
	EventDelete(context.Context, int32) (any, error)
	WorkoutsList(context.Context) (any, error)
	WorkoutGet(context.Context, int32) (any, error)
	WorkoutsCreate(context.Context, []byte) (any, error)
	WorkoutDownload(context.Context, int32, string) ([]byte, error)
	WellnessList(context.Context, app.WellnessListOptions) (any, error)
	WellnessGet(context.Context, string) (any, error)
	WellnessPut(context.Context, string, []byte) (any, error)
	WellnessBulkPut(context.Context, []byte) (any, error)
}

type runtime struct {
	ctx       context.Context
	stdout    io.Writer
	stderr    io.Writer
	stdin     io.Reader
	format    out.Format
	baseURL   string
	timeout   time.Duration
	verbose   bool
	svc       service
	newClient func(app.Config) (service, error)
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	rt := &runtime{
		ctx:    ctx,
		stdout: stdout,
		stderr: stderr,
		stdin:  os.Stdin,
	}
	err := rt.run(args)
	if err != nil {
		_ = out.WriteError(stderr, rt.format, err)
	}
	return err
}

func (r *runtime) run(args []string) error {
	global := flag.NewFlagSet("intervals", flag.ContinueOnError)
	global.SetOutput(io.Discard)
	rawFormat := global.String("format", "", "")
	baseURL := global.String("base-url", "https://intervals.icu", "")
	timeout := global.Duration("timeout", 30*time.Second, "")
	verbose := global.Bool("verbose", false, "")
	versionFlag := global.Bool("version", false, "")
	if err := global.Parse(args); err != nil {
		r.format = out.FormatTable
		return usageErr(err)
	}
	var err error
	r.format, err = out.ResolveFormat(*rawFormat, r.stdout)
	if err != nil {
		return usageErr(err)
	}
	r.baseURL = *baseURL
	r.timeout = *timeout
	r.verbose = *verbose
	if *versionFlag {
		_, _ = fmt.Fprintln(r.stdout, version.Version)
		return nil
	}
	rest := global.Args()
	if len(rest) == 0 || rest[0] == "help" {
		r.printUsage()
		return nil
	}
	switch rest[0] {
	case "auth":
		return r.runAuth(rest[1:])
	case "whoami":
		return r.call(func(s service) (any, error) { return s.WhoAmI(r.ctx) })
	case "athlete":
		return r.runAthlete(rest[1:])
	case "activities":
		return r.runActivities(rest[1:])
	case "activity":
		return r.runActivity(rest[1:])
	case "events":
		return r.runEvents(rest[1:])
	case "event":
		return r.runEvent(rest[1:])
	case "workouts":
		return r.runWorkouts(rest[1:])
	case "workout":
		return r.runWorkout(rest[1:])
	case "wellness":
		return r.runWellness(rest[1:])
	default:
		return usageErr(fmt.Errorf("unknown command %q", rest[0]))
	}
}

func (r *runtime) runAuth(args []string) error {
	if len(args) == 0 || args[0] != "status" {
		return usageErr(fmt.Errorf("usage: intervals auth status"))
	}
	if len(args) > 1 {
		return usageErr(fmt.Errorf("auth status takes no arguments"))
	}
	return r.call(func(s service) (any, error) { return s.AuthStatus(r.ctx) })
}

func (r *runtime) runAthlete(args []string) error {
	if len(args) == 0 {
		return usageErr(fmt.Errorf("usage: intervals athlete get|profile|training-plan"))
	}
	switch args[0] {
	case "get":
		if len(args) > 1 {
			return usageErr(fmt.Errorf("athlete get takes no arguments"))
		}
		return r.call(func(s service) (any, error) { return s.AthleteGet(r.ctx) })
	case "profile":
		if len(args) > 1 {
			return usageErr(fmt.Errorf("athlete profile takes no arguments"))
		}
		return r.call(func(s service) (any, error) { return s.AthleteProfile(r.ctx) })
	case "training-plan":
		if len(args) > 1 {
			return usageErr(fmt.Errorf("athlete training-plan takes no arguments"))
		}
		return r.call(func(s service) (any, error) { return s.AthleteTrainingPlan(r.ctx) })
	default:
		return usageErr(fmt.Errorf("unknown athlete subcommand %q", args[0]))
	}
}

func (r *runtime) runActivities(args []string) error {
	if len(args) == 0 {
		return usageErr(fmt.Errorf("usage: intervals activities list|search|upload"))
	}
	switch args[0] {
	case "list":
		fs := newFlagSet("activities list")
		oldest := fs.String("oldest", "", "")
		newest := fs.String("newest", "", "")
		limit := fs.Int("limit", 0, "")
		routeID := fs.Int64("route-id", 0, "")
		fields := fs.String("fields", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 {
			return usageErr(fmt.Errorf("activities list takes flags only"))
		}
		if err := validateDateTime(*oldest); err != nil {
			return usageErr(fmt.Errorf("--oldest: %w", err))
		}
		opts := app.ActivityListOptions{Oldest: *oldest}
		if *newest != "" {
			if err := validateDateTime(*newest); err != nil {
				return usageErr(fmt.Errorf("--newest: %w", err))
			}
			opts.Newest = newest
		}
		if *limit > 0 {
			v := int32(*limit)
			opts.Limit = &v
		}
		if fs.Lookup("route-id").Value.String() != "0" {
			v := *routeID
			opts.RouteID = &v
		}
		opts.Fields = splitCSV(*fields)
		return r.call(func(s service) (any, error) { return s.ActivitiesList(r.ctx, opts) })
	case "search":
		fs := newFlagSet("activities search")
		query := fs.String("query", "", "")
		limit := fs.Int("limit", 0, "")
		if err := fs.Parse(args[1:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 || *query == "" {
			return usageErr(fmt.Errorf("usage: intervals activities search --query <q> [--limit N]"))
		}
		opts := app.ActivitySearchOptions{Query: *query}
		if *limit > 0 {
			v := int32(*limit)
			opts.Limit = &v
		}
		return r.call(func(s service) (any, error) { return s.ActivitiesSearch(r.ctx, opts) })
	case "upload":
		fs := newFlagSet("activities upload")
		filePath := fs.String("file", "", "")
		name := fs.String("name", "", "")
		description := fs.String("description", "", "")
		deviceName := fs.String("device-name", "", "")
		externalID := fs.String("external-id", "", "")
		pairedEventID := fs.Int("paired-event-id", 0, "")
		if err := fs.Parse(args[1:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 || *filePath == "" {
			return usageErr(fmt.Errorf("usage: intervals activities upload --file <path> [flags]"))
		}
		opts := app.ActivityUploadOptions{FilePath: *filePath}
		assignString(&opts.Name, *name)
		assignString(&opts.Description, *description)
		assignString(&opts.DeviceName, *deviceName)
		assignString(&opts.ExternalID, *externalID)
		if fs.Lookup("paired-event-id").Value.String() != "0" {
			v := int32(*pairedEventID)
			opts.PairedEventID = &v
		}
		return r.call(func(s service) (any, error) { return s.ActivitiesUpload(r.ctx, opts) })
	default:
		return usageErr(fmt.Errorf("unknown activities subcommand %q", args[0]))
	}
}

func (r *runtime) runActivity(args []string) error {
	if len(args) == 0 {
		return usageErr(fmt.Errorf("usage: intervals activity get|streams|intervals|best-efforts|download"))
	}
	switch args[0] {
	case "get":
		if len(args) != 2 {
			return usageErr(fmt.Errorf("usage: intervals activity get <activity-id>"))
		}
		return r.call(func(s service) (any, error) { return s.ActivityGet(r.ctx, args[1]) })
	case "streams":
		if len(args) < 2 {
			return usageErr(fmt.Errorf("usage: intervals activity streams <activity-id> [flags]"))
		}
		fs := newFlagSet("activity streams")
		types := fs.String("types", "", "")
		includeDefaults := fs.Bool("include-defaults", false, "")
		if err := fs.Parse(args[2:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 {
			return usageErr(fmt.Errorf("activity streams takes exactly one id"))
		}
		var include *bool
		if flagPassed(fs, "include-defaults") {
			include = includeDefaults
		}
		return r.call(func(s service) (any, error) {
			return s.ActivityStreams(r.ctx, args[1], app.ActivityStreamsOptions{
				Types:           splitCSV(*types),
				IncludeDefaults: include,
			})
		})
	case "intervals":
		if len(args) != 2 {
			return usageErr(fmt.Errorf("usage: intervals activity intervals <activity-id>"))
		}
		return r.call(func(s service) (any, error) { return s.ActivityIntervals(r.ctx, args[1]) })
	case "best-efforts":
		if len(args) < 2 {
			return usageErr(fmt.Errorf("usage: intervals activity best-efforts <activity-id> --stream <name> (--duration-sec N | --distance-m M)"))
		}
		fs := newFlagSet("activity best-efforts")
		stream := fs.String("stream", "", "")
		durationSec := fs.Int("duration-sec", 0, "")
		distanceM := fs.Float64("distance-m", 0, "")
		count := fs.Int("count", 0, "")
		minValue := fs.Float64("min-value", 0, "")
		excludeIntervals := fs.Bool("exclude-intervals", false, "")
		startIndex := fs.Int("start-index", 0, "")
		endIndex := fs.Int("end-index", 0, "")
		if err := fs.Parse(args[2:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 || *stream == "" {
			return usageErr(fmt.Errorf("usage: intervals activity best-efforts <activity-id> --stream <name> (--duration-sec N | --distance-m M)"))
		}
		if (*durationSec > 0) == (*distanceM > 0) {
			return usageErr(fmt.Errorf("exactly one of --duration-sec or --distance-m is required"))
		}
		opts := app.ActivityBestEffortsOptions{Stream: *stream}
		if *durationSec > 0 {
			v := int32(*durationSec)
			opts.DurationSec = &v
		}
		if *distanceM > 0 {
			v := float32(*distanceM)
			opts.DistanceM = &v
		}
		if *count > 0 {
			v := int32(*count)
			opts.Count = &v
		}
		if flagPassed(fs, "min-value") {
			v := float32(*minValue)
			opts.MinValue = &v
		}
		if flagPassed(fs, "exclude-intervals") {
			opts.ExcludeIntervals = excludeIntervals
		}
		if flagPassed(fs, "start-index") {
			v := int32(*startIndex)
			opts.StartIndex = &v
		}
		if flagPassed(fs, "end-index") {
			v := int32(*endIndex)
			opts.EndIndex = &v
		}
		return r.call(func(s service) (any, error) { return s.ActivityBestEfforts(r.ctx, args[1], opts) })
	case "download":
		if len(args) < 2 {
			return usageErr(fmt.Errorf("usage: intervals activity download <activity-id> --kind original|fit|gpx --output <path|->"))
		}
		fs := newFlagSet("activity download")
		kind := fs.String("kind", "", "")
		output := fs.String("output", "", "")
		if err := fs.Parse(args[2:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 || *kind == "" || *output == "" {
			return usageErr(fmt.Errorf("usage: intervals activity download <activity-id> --kind original|fit|gpx --output <path|->"))
		}
		return r.callBinary(*output, func(s service) ([]byte, error) { return s.ActivityDownload(r.ctx, args[1], *kind) })
	default:
		return usageErr(fmt.Errorf("unknown activity subcommand %q", args[0]))
	}
}

func (r *runtime) runEvents(args []string) error {
	if len(args) == 0 {
		return usageErr(fmt.Errorf("usage: intervals events list|create|upsert"))
	}
	switch args[0] {
	case "list":
		fs := newFlagSet("events list")
		oldest := fs.String("oldest", "", "")
		newest := fs.String("newest", "", "")
		category := fs.String("category", "", "")
		limit := fs.Int("limit", 0, "")
		if err := fs.Parse(args[1:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 {
			return usageErr(fmt.Errorf("events list takes flags only"))
		}
		opts := app.EventListOptions{}
		if *oldest != "" {
			if err := validateDate(*oldest); err != nil {
				return usageErr(fmt.Errorf("--oldest: %w", err))
			}
			opts.Oldest = oldest
		}
		if *newest != "" {
			if err := validateDate(*newest); err != nil {
				return usageErr(fmt.Errorf("--newest: %w", err))
			}
			opts.Newest = newest
		}
		opts.Category = splitCSV(*category)
		if *limit > 0 {
			v := int32(*limit)
			opts.Limit = &v
		}
		return r.call(func(s service) (any, error) { return s.EventsList(r.ctx, opts) })
	case "create":
		fs := newFlagSet("events create")
		filePath := fs.String("file", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 || *filePath == "" {
			return usageErr(fmt.Errorf("usage: intervals events create --file <path|->"))
		}
		body, _, err := readJSONInput(r.stdin, *filePath)
		if err != nil {
			return usageErr(err)
		}
		return r.call(func(s service) (any, error) { return s.EventsCreate(r.ctx, body) })
	case "upsert":
		fs := newFlagSet("events upsert")
		filePath := fs.String("file", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 || *filePath == "" {
			return usageErr(fmt.Errorf("usage: intervals events upsert --file <path|->"))
		}
		body, payload, err := readJSONInput(r.stdin, *filePath)
		if err != nil {
			return usageErr(err)
		}
		obj, ok := payload.(map[string]any)
		if !ok || strings.TrimSpace(asString(obj["uid"])) == "" {
			return usageErr(fmt.Errorf("event upsert requires a non-empty uid in the payload"))
		}
		return r.call(func(s service) (any, error) { return s.EventsUpsert(r.ctx, body) })
	default:
		return usageErr(fmt.Errorf("unknown events subcommand %q", args[0]))
	}
}

func (r *runtime) runEvent(args []string) error {
	if len(args) == 0 {
		return usageErr(fmt.Errorf("usage: intervals event get|delete"))
	}
	switch args[0] {
	case "get":
		if len(args) != 2 {
			return usageErr(fmt.Errorf("usage: intervals event get <event-id>"))
		}
		id, err := app.ParseInt32(args[1])
		if err != nil {
			return usageErr(fmt.Errorf("invalid event id: %w", err))
		}
		return r.call(func(s service) (any, error) { return s.EventGet(r.ctx, id) })
	case "delete":
		if len(args) != 2 {
			return usageErr(fmt.Errorf("usage: intervals event delete <event-id>"))
		}
		id, err := app.ParseInt32(args[1])
		if err != nil {
			return usageErr(fmt.Errorf("invalid event id: %w", err))
		}
		return r.call(func(s service) (any, error) { return s.EventDelete(r.ctx, id) })
	default:
		return usageErr(fmt.Errorf("unknown event subcommand %q", args[0]))
	}
}

func (r *runtime) runWorkouts(args []string) error {
	if len(args) == 0 {
		return usageErr(fmt.Errorf("usage: intervals workouts list|create"))
	}
	switch args[0] {
	case "list":
		if len(args) > 1 {
			return usageErr(fmt.Errorf("workouts list takes no arguments"))
		}
		return r.call(func(s service) (any, error) { return s.WorkoutsList(r.ctx) })
	case "create":
		fs := newFlagSet("workouts create")
		filePath := fs.String("file", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 || *filePath == "" {
			return usageErr(fmt.Errorf("usage: intervals workouts create --file <path|->"))
		}
		body, _, err := readJSONInput(r.stdin, *filePath)
		if err != nil {
			return usageErr(err)
		}
		return r.call(func(s service) (any, error) { return s.WorkoutsCreate(r.ctx, body) })
	default:
		return usageErr(fmt.Errorf("unknown workouts subcommand %q", args[0]))
	}
}

func (r *runtime) runWorkout(args []string) error {
	if len(args) == 0 {
		return usageErr(fmt.Errorf("usage: intervals workout get|download"))
	}
	switch args[0] {
	case "get":
		if len(args) != 2 {
			return usageErr(fmt.Errorf("usage: intervals workout get <workout-id>"))
		}
		id, err := app.ParseInt32(args[1])
		if err != nil {
			return usageErr(fmt.Errorf("invalid workout id: %w", err))
		}
		return r.call(func(s service) (any, error) { return s.WorkoutGet(r.ctx, id) })
	case "download":
		if len(args) < 2 {
			return usageErr(fmt.Errorf("usage: intervals workout download <workout-id> --format zwo|mrc|erg|fit --output <path|->"))
		}
		fs := newFlagSet("workout download")
		format := fs.String("format", "", "")
		output := fs.String("output", "", "")
		if err := fs.Parse(args[2:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 || *format == "" || *output == "" {
			return usageErr(fmt.Errorf("usage: intervals workout download <workout-id> --format zwo|mrc|erg|fit --output <path|->"))
		}
		id, err := app.ParseInt32(args[1])
		if err != nil {
			return usageErr(fmt.Errorf("invalid workout id: %w", err))
		}
		return r.callBinary(*output, func(s service) ([]byte, error) { return s.WorkoutDownload(r.ctx, id, *format) })
	default:
		return usageErr(fmt.Errorf("unknown workout subcommand %q", args[0]))
	}
}

func (r *runtime) runWellness(args []string) error {
	if len(args) == 0 {
		return usageErr(fmt.Errorf("usage: intervals wellness list|get|put|bulk-put"))
	}
	switch args[0] {
	case "list":
		fs := newFlagSet("wellness list")
		oldest := fs.String("oldest", "", "")
		newest := fs.String("newest", "", "")
		fields := fs.String("fields", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 {
			return usageErr(fmt.Errorf("wellness list takes flags only"))
		}
		opts := app.WellnessListOptions{Fields: splitCSV(*fields)}
		if *oldest != "" {
			if err := validateDate(*oldest); err != nil {
				return usageErr(fmt.Errorf("--oldest: %w", err))
			}
			opts.Oldest = oldest
		}
		if *newest != "" {
			if err := validateDate(*newest); err != nil {
				return usageErr(fmt.Errorf("--newest: %w", err))
			}
			opts.Newest = newest
		}
		return r.call(func(s service) (any, error) { return s.WellnessList(r.ctx, opts) })
	case "get":
		fs := newFlagSet("wellness get")
		date := fs.String("date", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 || *date == "" {
			return usageErr(fmt.Errorf("usage: intervals wellness get --date <YYYY-MM-DD>"))
		}
		if err := validateDate(*date); err != nil {
			return usageErr(fmt.Errorf("--date: %w", err))
		}
		return r.call(func(s service) (any, error) { return s.WellnessGet(r.ctx, *date) })
	case "put":
		fs := newFlagSet("wellness put")
		date := fs.String("date", "", "")
		filePath := fs.String("file", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 || *date == "" || *filePath == "" {
			return usageErr(fmt.Errorf("usage: intervals wellness put --date <YYYY-MM-DD> --file <path|->"))
		}
		if err := validateDate(*date); err != nil {
			return usageErr(fmt.Errorf("--date: %w", err))
		}
		_, payload, err := readJSONInput(r.stdin, *filePath)
		if err != nil {
			return usageErr(err)
		}
		obj, ok := payload.(map[string]any)
		if !ok {
			return usageErr(fmt.Errorf("wellness put payload must be a JSON object"))
		}
		obj["id"] = *date
		body, err := json.Marshal(obj)
		if err != nil {
			return usageErr(err)
		}
		return r.call(func(s service) (any, error) { return s.WellnessPut(r.ctx, *date, body) })
	case "bulk-put":
		fs := newFlagSet("wellness bulk-put")
		filePath := fs.String("file", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 || *filePath == "" {
			return usageErr(fmt.Errorf("usage: intervals wellness bulk-put --file <path|->"))
		}
		body, payload, err := readJSONInput(r.stdin, *filePath)
		if err != nil {
			return usageErr(err)
		}
		if _, ok := payload.([]any); !ok {
			return usageErr(fmt.Errorf("wellness bulk-put payload must be a JSON array"))
		}
		return r.call(func(s service) (any, error) { return s.WellnessBulkPut(r.ctx, body) })
	default:
		return usageErr(fmt.Errorf("unknown wellness subcommand %q", args[0]))
	}
}

func (r *runtime) getService() (service, error) {
	if r.svc != nil {
		return r.svc, nil
	}
	factory := r.newClient
	if factory == nil {
		factory = func(cfg app.Config) (service, error) {
			return app.NewClient(cfg)
		}
	}
	svc, err := factory(app.Config{
		BaseURL:   r.baseURL,
		Timeout:   r.timeout,
		Verbose:   r.verbose,
		Stderr:    r.stderr,
		UserAgent: "intervals/" + version.Version,
	})
	if err != nil {
		return nil, configErr(err)
	}
	r.svc = svc
	return r.svc, nil
}

func (r *runtime) call(fn func(service) (any, error)) error {
	svc, err := r.getService()
	if err != nil {
		return err
	}
	return r.output(fn(svc))
}

func (r *runtime) callBinary(target string, fn func(service) ([]byte, error)) error {
	svc, err := r.getService()
	if err != nil {
		return err
	}
	data, err := fn(svc)
	if err != nil {
		return err
	}
	return writeOutputFile(target, data, r.stdout)
}

func (r *runtime) output(value any, err error) error {
	if err != nil {
		var ce *cliError
		if errors.As(err, &ce) {
			return ce
		}
		if strings.Contains(err.Error(), "missing auth:") {
			return configErr(err)
		}
		return err
	}
	return out.Render(r.stdout, r.format, value)
}

func (r *runtime) printUsage() {
	_, _ = fmt.Fprint(r.stdout, `intervals is an agent-first CLI for Intervals.icu.

Usage:
  intervals [global flags] <command> [args]

Global flags:
  --format json|table|plain
  --base-url https://intervals.icu
  --timeout 30s
  --verbose
  --version

Commands:
  auth status
  whoami
  athlete get|profile|training-plan
  activities list|search|upload
  activity get|streams|intervals|best-efforts|download
  events list|create|upsert
  event get|delete
  workouts list|create
  workout get|download
  wellness list|get|put|bulk-put
`)
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func usageErr(err error) error  { return &cliError{code: 2, err: err} }
func configErr(err error) error { return &cliError{code: 3, err: err} }

func validateDateTime(raw string) error {
	if raw == "" {
		return errors.New("value is required")
	}
	layouts := []string{
		"2006-01-02",
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, layout := range layouts {
		if _, err := time.Parse(layout, raw); err == nil {
			return nil
		}
	}
	return errors.New("must be an absolute ISO date or datetime")
}

func validateDate(raw string) error {
	if raw == "" {
		return errors.New("value is required")
	}
	if _, err := time.Parse("2006-01-02", raw); err != nil {
		return errors.New("must be in YYYY-MM-DD format")
	}
	return nil
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func assignString(dst **string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	*dst = &value
}

func flagPassed(fs *flag.FlagSet, name string) bool {
	passed := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			passed = true
		}
	})
	return passed
}

func readJSONInput(stdin io.Reader, path string) ([]byte, any, error) {
	var data []byte
	var err error
	switch path {
	case "-":
		data, err = io.ReadAll(stdin)
	default:
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, nil, err
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, nil, fmt.Errorf("invalid JSON: %w", err)
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	return normalized, payload, nil
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return ""
	}
}

func writeOutputFile(target string, data []byte, stdout io.Writer) error {
	if target == "-" {
		_, err := stdout.Write(data)
		return err
	}
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}
