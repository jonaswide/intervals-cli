package out

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"golang.org/x/term"
)

type Format string

const (
	FormatJSON  Format = "json"
	FormatTable Format = "table"
	FormatPlain Format = "plain"
)

func ResolveFormat(raw string, stdout io.Writer) (Format, error) {
	switch raw {
	case "":
		if IsTTY(stdout) {
			return FormatTable, nil
		}
		return FormatJSON, nil
	case string(FormatJSON):
		return FormatJSON, nil
	case string(FormatTable):
		return FormatTable, nil
	case string(FormatPlain):
		return FormatPlain, nil
	default:
		return "", fmt.Errorf("invalid format %q", raw)
	}
}

func IsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func Render(w io.Writer, format Format, value any) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	case FormatTable:
		return renderTable(w, value)
	case FormatPlain:
		return renderPlain(w, value)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func WriteError(w io.Writer, format Format, err error) error {
	if format == FormatJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
	}
	_, e := fmt.Fprintln(w, err.Error())
	return e
}

func renderTable(w io.Writer, value any) error {
	v, err := normalize(value)
	if err != nil {
		return err
	}
	switch x := v.(type) {
	case map[string]any:
		tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
		keys := sortedKeys(x)
		for _, key := range keys {
			_, _ = fmt.Fprintf(tw, "%s\t%s\n", key, compact(x[key]))
		}
		return tw.Flush()
	case []any:
		if len(x) == 0 {
			return nil
		}
		rows := make([]map[string]any, 0, len(x))
		allMap := true
		for _, item := range x {
			row, ok := item.(map[string]any)
			if !ok {
				allMap = false
				break
			}
			rows = append(rows, row)
		}
		if !allMap {
			for _, item := range x {
				if _, err := fmt.Fprintln(w, compact(item)); err != nil {
					return err
				}
			}
			return nil
		}
		cols := unionKeys(rows)
		tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, strings.Join(cols, "\t"))
		for _, row := range rows {
			values := make([]string, 0, len(cols))
			for _, col := range cols {
				values = append(values, compact(row[col]))
			}
			_, _ = fmt.Fprintln(tw, strings.Join(values, "\t"))
		}
		return tw.Flush()
	default:
		_, err := fmt.Fprintln(w, compact(v))
		return err
	}
}

func renderPlain(w io.Writer, value any) error {
	v, err := normalize(value)
	if err != nil {
		return err
	}
	switch x := v.(type) {
	case map[string]any:
		for _, key := range sortedKeys(x) {
			if _, err := fmt.Fprintf(w, "%s=%s\n", key, compact(x[key])); err != nil {
				return err
			}
		}
		return nil
	case []any:
		for _, item := range x {
			if _, err := fmt.Fprintln(w, compact(item)); err != nil {
				return err
			}
		}
		return nil
	default:
		_, err := fmt.Fprintln(w, compact(v))
		return err
	}
}

func normalize(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch x := value.(type) {
	case json.RawMessage:
		if len(x) == 0 {
			return nil, nil
		}
		var out any
		if err := json.Unmarshal(x, &out); err != nil {
			return string(x), nil
		}
		return out, nil
	case []byte:
		var out any
		if err := json.Unmarshal(x, &out); err != nil {
			return string(x), nil
		}
		return out, nil
	default:
		b, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var out any
		if err := json.Unmarshal(b, &out); err != nil {
			return string(b), nil
		}
		return out, nil
	}
}

func compact(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%f", x), "0"), ".")
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprint(x)
		}
		return string(bytes.TrimSpace(b))
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func unionKeys(rows []map[string]any) []string {
	seen := map[string]struct{}{}
	for _, row := range rows {
		for key := range row {
			seen[key] = struct{}{}
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
