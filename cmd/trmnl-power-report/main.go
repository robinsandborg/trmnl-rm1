// trmnl-power-report reads cycle JSONL from stdin and emits a redacted JSON summary.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/robinsandborg/rm1-trmnl/internal/diagnostics"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, input io.Reader, output, errors io.Writer) error {
	flags := flag.NewFlagSet("trmnl-power-report", flag.ContinueOnError)
	flags.SetOutput(errors)
	since := flags.String("since", "", "inclusive cycle start (RFC3339)")
	until := flags.String("until", "", "exclusive cycle start (RFC3339)")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("expected JSONL on stdin; no positional arguments")
	}
	var filter diagnostics.Filter
	for text, destination := range map[*string]*time.Time{since: &filter.Since, until: &filter.Until} {
		if *text == "" {
			continue
		}
		value, err := time.Parse(time.RFC3339, *text)
		if err != nil {
			return fmt.Errorf("since and until must use RFC3339 timestamps")
		}
		*destination = value
	}
	report, err := diagnostics.Read(input, filter)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
