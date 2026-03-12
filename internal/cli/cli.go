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
	args = reorderGlobalFlags(args)
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
	if len(rest) == 0 {
		r.printUsage(nil)
		return nil
	}
	if rest[0] == "help" {
		r.printUsage(rest[1:])
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

func reorderGlobalFlags(args []string) []string {
	if len(args) == 0 {
		return args
	}
	var globals []string
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		switch {
		case arg == "--verbose" || arg == "--version":
			globals = append(globals, arg)
		case arg == "--format" || arg == "--base-url" || arg == "--timeout":
			globals = append(globals, arg)
			if i+1 < len(args) {
				globals = append(globals, args[i+1])
				i++
			}
		case strings.HasPrefix(arg, "--format="),
			strings.HasPrefix(arg, "--base-url="),
			strings.HasPrefix(arg, "--timeout="):
			globals = append(globals, arg)
		default:
			rest = append(rest, arg)
		}
	}
	return append(globals, rest...)
}

func (r *runtime) runAuth(args []string) error {
	if len(args) > 0 && isHelpArg(args[0]) {
		r.printUsage([]string{"auth"})
		return nil
	}
	if len(args) == 0 || args[0] != "status" {
		return usageErr(fmt.Errorf("usage: intervals auth status"))
	}
	if len(args) > 1 {
		return usageErr(fmt.Errorf("auth status takes no arguments"))
	}
	return r.call(func(s service) (any, error) { return s.AuthStatus(r.ctx) })
}

func (r *runtime) runAthlete(args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		r.printUsage([]string{"athlete"})
		return nil
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
	if len(args) == 0 || isHelpArg(args[0]) {
		r.printUsage([]string{"activities"})
		return nil
	}
	switch args[0] {
	case "list":
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"activities", "list"})
			return nil
		}
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
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"activities", "search"})
			return nil
		}
		fs := newFlagSet("activities search")
		query := fs.String("query", "", "")
		oldest := fs.String("oldest", "", "")
		newest := fs.String("newest", "", "")
		limit := fs.Int("limit", 0, "")
		if err := fs.Parse(args[1:]); err != nil {
			return usageErr(err)
		}
		if fs.NArg() != 0 || *query == "" {
			return usageErr(fmt.Errorf("usage: intervals activities search --query <q> [--oldest <ISO date|datetime>] [--newest <ISO date|datetime>] [--limit N]"))
		}
		if *newest != "" && *oldest == "" {
			return usageErr(fmt.Errorf("--oldest is required when --newest is set"))
		}
		opts := app.ActivitySearchOptions{Query: *query}
		if *oldest != "" {
			if err := validateDateTime(*oldest); err != nil {
				return usageErr(fmt.Errorf("--oldest: %w", err))
			}
			opts.Oldest = oldest
		}
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
		return r.call(func(s service) (any, error) { return s.ActivitiesSearch(r.ctx, opts) })
	case "upload":
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"activities", "upload"})
			return nil
		}
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
	if len(args) == 0 || isHelpArg(args[0]) {
		r.printUsage([]string{"activity"})
		return nil
	}
	switch args[0] {
	case "get":
		if len(args) != 2 {
			return usageErr(fmt.Errorf("usage: intervals activity get <activity-id>"))
		}
		return r.call(func(s service) (any, error) { return s.ActivityGet(r.ctx, args[1]) })
	case "streams":
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"activity", "streams"})
			return nil
		}
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
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"activity", "best-efforts"})
			return nil
		}
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
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"activity", "download"})
			return nil
		}
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
	if len(args) == 0 || isHelpArg(args[0]) {
		r.printUsage([]string{"events"})
		return nil
	}
	switch args[0] {
	case "list":
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"events", "list"})
			return nil
		}
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
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"events", "create"})
			return nil
		}
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
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"events", "upsert"})
			return nil
		}
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
	if len(args) == 0 || isHelpArg(args[0]) {
		r.printUsage([]string{"event"})
		return nil
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
	if len(args) == 0 || isHelpArg(args[0]) {
		r.printUsage([]string{"workouts"})
		return nil
	}
	switch args[0] {
	case "list":
		if len(args) > 1 {
			return usageErr(fmt.Errorf("workouts list takes no arguments"))
		}
		return r.call(func(s service) (any, error) { return s.WorkoutsList(r.ctx) })
	case "create":
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"workouts", "create"})
			return nil
		}
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
	if len(args) == 0 || isHelpArg(args[0]) {
		r.printUsage([]string{"workout"})
		return nil
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
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"workout", "download"})
			return nil
		}
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
	if len(args) == 0 || isHelpArg(args[0]) {
		r.printUsage([]string{"wellness"})
		return nil
	}
	switch args[0] {
	case "list":
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"wellness", "list"})
			return nil
		}
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
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"wellness", "get"})
			return nil
		}
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
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"wellness", "put"})
			return nil
		}
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
		if containsHelpFlag(args[1:]) {
			r.printUsage([]string{"wellness", "bulk-put"})
			return nil
		}
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

func (r *runtime) printUsage(path []string) {
	_, _ = fmt.Fprint(r.stdout, usageText(path))
}

func usageText(path []string) string {
	switch strings.Join(path, " ") {
	case "":
		return `intervals is an agent-first CLI for Intervals.icu.

Usage:
  intervals [global flags] <command> [args]
  intervals help <command> [subcommand]

Global flags:
  --format json|table|plain   default is table on TTY, json otherwise
  --base-url URL              default https://intervals.icu
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

Agent notes:
  - prefer --format json
  - use absolute dates, not relative values like "tomorrow"
  - use events create/upsert for scheduled calendar items
  - use workouts create for reusable library items
  - use --file - or a temp file for complex writes
`
	case "auth":
		return `Usage:
  intervals auth status

Checks which auth source is active and validates it with a lightweight authenticated request.
`
	case "athlete":
		return `Usage:
  intervals athlete get
  intervals athlete profile
  intervals athlete training-plan
`
	case "activities":
		return `Usage:
  intervals activities list ...
  intervals activities search ...
  intervals activities upload ...

Choose:
  - list: fetch a date window and reason over structured activity fields
  - search: text/tag lookup, optionally bounded by a date window
  - upload: upload a new activity file
`
	case "activities list":
		return `Usage:
  intervals activities list --oldest <ISO date|datetime> [--newest <ISO date|datetime>] [--limit N] [--route-id ID] [--fields a,b,c]

Use this when the user asks for semantic filtering such as:
  - weekend long runs
  - hard sessions in the current block
  - rides on a specific route
`
	case "activities search":
		return `Usage:
  intervals activities search --query <q> [--oldest <ISO date|datetime>] [--newest <ISO date|datetime>] [--limit N]

Search behavior:
  - plain text matches activity names case-insensitively
  - queries starting with # match tags exactly
  - if --oldest/--newest are set, the CLI fetches the date window and filters locally

Examples:
  intervals activities search --query "#threshold" --oldest 2026-03-01 --newest 2026-03-12 --format json
  intervals activities search --query "tempo" --oldest 2026-03-01 --format json
`
	case "activities upload":
		return `Usage:
  intervals activities upload --file <path> [--name ...] [--description ...] [--device-name ...] [--external-id ...] [--paired-event-id ...]
`
	case "activity":
		return `Usage:
  intervals activity get ...
  intervals activity streams ...
  intervals activity intervals ...
  intervals activity best-efforts ...
  intervals activity download ...
`
	case "activity streams":
		return `Usage:
  intervals activity streams <activity-id> [--types watts,hr,pace] [--include-defaults]
`
	case "activity best-efforts":
		return `Usage:
  intervals activity best-efforts <activity-id> --stream <name> (--duration-sec N | --distance-m M) [--count N] [--min-value X] [--exclude-intervals] [--start-index N] [--end-index N]
`
	case "activity download":
		return `Usage:
  intervals activity download <activity-id> --kind original|fit|gpx --output <path|->

Notes:
  - --output is required
  - use - for stdout only if the caller expects binary/text data directly
`
	case "events":
		return `Usage:
  intervals events list ...
  intervals events create ...
  intervals events upsert ...

Events are scheduled calendar items on a specific date.
Use events create/upsert when the user asks for something "for Monday", "tomorrow", or "next week".
`
	case "events list":
		return `Usage:
  intervals events list [--oldest <ISO date>] [--newest <ISO date>] [--category a,b,c] [--limit N]
`
	case "events create":
		return `Usage:
  intervals events create --file <path|->

Creates a calendar event from raw Intervals-compatible EventEx JSON.

Recommended pattern:
  1. resolve the date to an absolute local datetime
  2. write JSON to a temp file or pass it via --file -
  3. call intervals events create --format json

Examples:
  printf '%s\n' '{"category":"WORKOUT","start_date_local":"2026-03-16T00:00:00","type":"Run","name":"10km @ 5:00/km","moving_time":3000,"description":"- 10km 5:00/km Pace"}' | intervals events create --file - --format json
  tmp="$(mktemp)" && cp examples/events/create-10km-run.json "$tmp" && intervals events create --file "$tmp" --format json && rm -f "$tmp"

Use this for:
  - workouts scheduled on a specific date
  - planned runs/rides/sessions in the calendar
`
	case "events upsert":
		return `Usage:
  intervals events upsert --file <path|->

Upserts a calendar event from raw Intervals-compatible EventEx JSON.
The payload must include uid.

Prefer upsert when duplicate creation would be harmful or the agent may retry.

Examples:
  printf '%s\n' '{"uid":"example-10km-run-2026-03-16","category":"WORKOUT","start_date_local":"2026-03-16T00:00:00","type":"Run","name":"10km @ 5:00/km","moving_time":3000,"description":"- 10km 5:00/km Pace"}' | intervals events upsert --file - --format json
  tmp="$(mktemp)" && cp examples/events/upsert-10km-run.json "$tmp" && intervals events upsert --file "$tmp" --format json && rm -f "$tmp"
`
	case "event":
		return `Usage:
  intervals event get <event-id>
  intervals event delete <event-id>
`
	case "workouts":
		return `Usage:
  intervals workouts list
  intervals workouts create --file <path|->

Workouts are reusable library objects, not calendar items tied to a date.
`
	case "workouts create":
		return `Usage:
  intervals workouts create --file <path|->

Creates a workout library object from raw Intervals-compatible WorkoutEx JSON.

Use this for:
  - reusable workout templates
  - workout library items

Do not use this to schedule a workout on a date. Use events create/upsert for that.

Examples:
  printf '%s\n' '{"name":"10km @ 5:00/km","description":"- 10km 5:00/km Pace","type":"Run","moving_time":3000}' | intervals workouts create --file - --format json
  tmp="$(mktemp)" && cp examples/workouts/create-simple-run.json "$tmp" && intervals workouts create --file "$tmp" --format json && rm -f "$tmp"
`
	case "workout":
		return `Usage:
  intervals workout get <workout-id>
  intervals workout download <workout-id> --format zwo|mrc|erg|fit --output <path|->
`
	case "workout download":
		return `Usage:
  intervals workout download <workout-id> --format zwo|mrc|erg|fit --output <path|->

Notes:
  - fetches the workout first, then converts it
  - --output is required
`
	case "wellness":
		return `Usage:
  intervals wellness list ...
  intervals wellness get ...
  intervals wellness put ...
  intervals wellness bulk-put ...
`
	case "wellness list":
		return `Usage:
  intervals wellness list [--oldest <ISO date>] [--newest <ISO date>] [--fields a,b,c]
`
	case "wellness get":
		return `Usage:
  intervals wellness get --date <YYYY-MM-DD>
`
	case "wellness put":
		return `Usage:
  intervals wellness put --date <YYYY-MM-DD> --file <path|->

Writes a single wellness record from raw Intervals-compatible Wellness JSON.
The date comes from --date. The payload must be a JSON object.

Examples:
  printf '%s\n' '{"restingHR":48,"weight":78.2,"sleepSecs":27000}' | intervals wellness put --date 2026-03-12 --file - --format json
  tmp="$(mktemp)" && cp examples/wellness/put-day.json "$tmp" && intervals wellness put --date 2026-03-12 --file "$tmp" --format json && rm -f "$tmp"
`
	case "wellness bulk-put":
		return `Usage:
  intervals wellness bulk-put --file <path|->

Writes multiple wellness records from a JSON array.

Example:
  printf '%s\n' '[{"id":"2026-03-11","restingHR":49,"weight":78.4},{"id":"2026-03-12","restingHR":48,"weight":78.2,"sleepSecs":27000}]' | intervals wellness bulk-put --file - --format json
`
	default:
		return "unknown help topic\n"
	}
}

func isHelpArg(arg string) bool {
	return arg == "--help" || arg == "-h" || arg == "help"
}

func containsHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
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
