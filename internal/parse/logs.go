package parse

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// LogEntry is one syslog line from a device buffer.
type LogEntry struct {
	Time     time.Time `json:"time,omitzero"` // device local clock, stored with time.UTC location
	HasTime  bool      `json:"has_time"`      // false for uptime-stamped or unstamped messages
	Facility string    `json:"facility"`
	Severity int       `json:"severity"` // 0 emergency .. 7 debug, -1 unknown
	Mnemonic string    `json:"mnemonic"`
	Message  string    `json:"message"`
	Raw      string    `json:"raw"`
}

// SeverityName maps syslog levels to Cisco names.
var SeverityName = []string{"emergency", "alert", "critical", "error", "warning", "notice", "info", "debug"}

var months = map[string]time.Month{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var (
	// IOS/IOS-XE: [seq:] [*|.]Mon dd [yyyy] hh:mm:ss[.mmm] [TZ]: %MSG
	iosLog = regexp.MustCompile(`^(?:\d+:\s*)?[*.]?([A-Z][a-z]{2})\s+(\d{1,2})\s+(?:(\d{4})\s+)?(\d{1,2}):(\d{2}):(\d{2})(?:\.\d+)?(?:\s+[A-Za-z]{2,5})?:\s*(%.*)$`)
	// IOS uptime-stamped: [seq:] [*]1w2d: %MSG or 00:01:23: %MSG
	iosUptimeLog = regexp.MustCompile(`^(?:\d+:\s*)?[*.]?(?:\d+[ywdh]\d+[wdhm]|\d{2}:\d{2}:\d{2}(?:\.\d+)?):\s*(%.*)$`)
	// NX-OS: yyyy Mon dd hh:mm:ss[.mmm] [TZ] host %MSG
	nxLog = regexp.MustCompile(`^(\d{4})\s+([A-Z][a-z]{2})\s+(\d{1,2})\s+(\d{1,2}):(\d{2}):(\d{2})(?:\.\d+)?(?:\s+[A-Z]{2,5})?\s+\S+\s+(%.*)$`)
	// ASA: Mon dd yyyy hh:mm:ss [host]: %ASA-...
	asaLog = regexp.MustCompile(`^([A-Z][a-z]{2})\s+(\d{1,2})\s+(\d{4})\s+(\d{1,2}):(\d{2}):(\d{2})[^%]*?(%(?:ASA|FTD)-.*)$`)
	// Bare message without timestamp.
	bareLog = regexp.MustCompile(`^(%[A-Z][A-Z0-9_]*-.*)$`)
	// %FACILITY[-SUB]-SEV-MNEMONIC: text   /  %ASA-6-302013: text
	mnemonic = regexp.MustCompile(`^%([A-Z0-9_]+(?:-[A-Z0-9_]+)*?)-([0-7])-([A-Z0-9_]+):?\s*(.*)$`)
)

// ParseLogs parses a log buffer from any supported platform. deviceNow (the
// device clock) is used to infer the year for timestamps that omit it.
func ParseLogs(out string, deviceNow time.Time) []LogEntry {
	var res []LogEntry
	for _, raw := range strings.Split(strings.ReplaceAll(out, "\r", ""), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		e := LogEntry{Raw: line, Severity: -1}
		var msg string
		switch {
		case asaLog.MatchString(line):
			m := asaLog.FindStringSubmatch(line)
			e.Time, e.HasTime = mkTime(m[3], m[1], m[2], m[4], m[5], m[6], deviceNow)
			msg = m[7]
		case nxLog.MatchString(line):
			m := nxLog.FindStringSubmatch(line)
			e.Time, e.HasTime = mkTime(m[1], m[2], m[3], m[4], m[5], m[6], deviceNow)
			msg = m[7]
		case iosLog.MatchString(line):
			m := iosLog.FindStringSubmatch(line)
			e.Time, e.HasTime = mkTime(m[3], m[1], m[2], m[4], m[5], m[6], deviceNow)
			msg = m[7]
		case iosUptimeLog.MatchString(line):
			msg = iosUptimeLog.FindStringSubmatch(line)[1]
		case bareLog.MatchString(line):
			msg = line
		default:
			continue // header lines, wrapped continuations, etc.
		}
		if m := mnemonic.FindStringSubmatch(msg); m != nil {
			e.Facility, e.Mnemonic, e.Message = m[1], m[3], m[4]
			e.Severity, _ = strconv.Atoi(m[2])
		} else {
			e.Message = msg
		}
		res = append(res, e)
	}
	return res
}

func mkTime(year, mon, day, hh, mm, ss string, now time.Time) (time.Time, bool) {
	m, ok := months[strings.ToLower(mon)]
	if !ok {
		return time.Time{}, false
	}
	d, h, mi, s := atoi(day), atoi(hh), atoi(mm), atoi(ss)
	if year != "" {
		return time.Date(atoi(year), m, d, h, mi, s, 0, time.UTC), true
	}
	if now.IsZero() {
		now = time.Now()
	}
	t := time.Date(now.Year(), m, d, h, mi, s, 0, time.UTC)
	// A date in the future relative to the device clock belongs to last year.
	if t.After(now.Add(24 * time.Hour)) {
		t = t.AddDate(-1, 0, 0)
	}
	return t, true
}

// LogFilter selects log entries.
type LogFilter struct {
	From, To    time.Time // zero = unbounded; compared in device local time
	Grep        string    // case-insensitive substring
	MaxSeverity int       // include severities <= MaxSeverity (7 = all)
}

// FilterLogs applies f. Entries without a usable timestamp are dropped when a
// time range is set; the number dropped is returned so the UI can say so.
func FilterLogs(entries []LogEntry, f LogFilter) (kept []LogEntry, undated int) {
	ranged := !f.From.IsZero() || !f.To.IsZero()
	g := strings.ToLower(f.Grep)
	for _, e := range entries {
		if ranged {
			if !e.HasTime {
				undated++
				continue
			}
			if !f.From.IsZero() && e.Time.Before(f.From) {
				continue
			}
			if !f.To.IsZero() && e.Time.After(f.To) {
				continue
			}
		}
		if g != "" && !strings.Contains(strings.ToLower(e.Raw), g) {
			continue
		}
		if e.Severity > f.MaxSeverity && f.MaxSeverity < 7 && e.Severity >= 0 {
			continue
		}
		kept = append(kept, e)
	}
	return kept, undated
}

var clockRe = regexp.MustCompile(`[*.]?(\d{1,2}):(\d{2}):(\d{2})(?:\.\d+)?\s+(?:[A-Za-z]{2,5}\s+)?[A-Z][a-z]{2}\s+([A-Z][a-z]{2})\s+(\d{1,2})\s+(\d{4})`)

// ParseClock reads `show clock` on IOS, NX-OS and ASA. The zone is ignored:
// lazynet compares everything in the device's own local time.
func ParseClock(out string) (time.Time, bool) {
	m := clockRe.FindStringSubmatch(out)
	if m == nil {
		return time.Time{}, false
	}
	return mkTime(m[6], m[4], m[5], m[1], m[2], m[3], time.Time{})
}

// ParseUserTime accepts the time formats users type on the CLI / TUI.
func ParseUserTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	var err error
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02"} {
		var t time.Time
		if t, err = time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t, nil
		}
	}
	return time.Time{}, err
}
