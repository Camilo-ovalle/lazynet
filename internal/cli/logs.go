// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"github.com/Camilo-ovalle/lazynet/internal/render"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Camilo-ovalle/lazynet/internal/parse"
	"github.com/Camilo-ovalle/lazynet/internal/theme"
	"github.com/spf13/cobra"
)

func newLogsCmd(g *globals) *cobra.Command {
	var from, to, grep, sev string
	var last time.Duration
	c := &cobra.Command{
		Use:   "logs <device>",
		Short: "Show the device log buffer, filtered by date/time range",
		Long: `Read the device's log buffer (show logging / show logging logfile) and
filter it locally. Times are in the device's own clock.

Time formats: "2026-11-01 08:00", "2026-11-01 08:00:30", "2026-11-01".`,
		Example: `  lazynet logs core-sw1 --from "2026-11-01 08:00" --to "2026-11-01 18:00"
  lazynet logs core-sw1 --last 2h --severity warning
  lazynet logs fw-edge --grep 302013`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			f := parse.LogFilter{Grep: grep, MaxSeverity: 7}
			var err error
			if from != "" {
				if f.From, err = parse.ParseUserTime(from); err != nil {
					return fmt.Errorf("--from: %w", err)
				}
			}
			if to != "" {
				if f.To, err = parse.ParseUserTime(to); err != nil {
					return fmt.Errorf("--to: %w", err)
				}
				if len(strings.TrimSpace(to)) == 10 { // date only: include the whole day
					f.To = f.To.Add(24*time.Hour - time.Second)
				}
			}
			if sev != "" {
				if f.MaxSeverity, err = parseSeverity(sev); err != nil {
					return err
				}
			}
			s, err := g.connect(g.device(args[0]))
			if err != nil {
				return err
			}
			defer s.Close()
			lr, err := s.Driver.Logs()
			if err != nil {
				return err
			}
			if last > 0 {
				now := lr.DeviceClock
				if now.IsZero() {
					now = time.Now().UTC()
				}
				f.From = now.Add(-last)
			}
			kept, undated := parse.FilterLogs(lr.Entries, f)
			if g.json() {
				return printJSON(kept)
			}
			for _, e := range kept {
				fmt.Println(render.Log(e))
			}
			summary := fmt.Sprintf("%d of %d entries", len(kept), len(lr.Entries))
			if undated > 0 {
				summary += fmt.Sprintf(" · %d without date skipped (device uses uptime timestamps: configure 'service timestamps log datetime')", undated)
			}
			if !lr.DeviceClock.IsZero() {
				summary += " · device clock " + lr.DeviceClock.Format("2006-01-02 15:04:05")
			}
			fmt.Fprintln(os.Stderr, theme.Dim.Render(summary))
			return nil
		},
	}
	fl := c.Flags()
	fl.StringVar(&from, "from", "", "start time (device clock)")
	fl.StringVar(&to, "to", "", "end time (device clock)")
	fl.DurationVar(&last, "last", 0, "only the last duration, e.g. 30m, 2h, 24h")
	fl.StringVar(&grep, "grep", "", "case-insensitive text filter")
	fl.StringVar(&sev, "severity", "", "maximum severity: 0-7 or emergency..debug (e.g. warning)")
	return c
}

func parseSeverity(s string) (int, error) {
	if n, err := strconv.Atoi(s); err == nil && n >= 0 && n <= 7 {
		return n, nil
	}
	for i, n := range parse.SeverityName {
		if strings.HasPrefix(n, strings.ToLower(s)) && len(s) >= 3 {
			return i, nil
		}
	}
	return 0, fmt.Errorf("unknown severity %q (0-7 or %s)", s, strings.Join(parse.SeverityName, ", "))
}
