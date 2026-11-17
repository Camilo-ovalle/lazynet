// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/Camilo-ovalle/lazynet/internal/config"
	"github.com/Camilo-ovalle/lazynet/internal/render"
	"github.com/Camilo-ovalle/lazynet/internal/theme"
	"github.com/Camilo-ovalle/lazynet/internal/tui"
	"github.com/Camilo-ovalle/lazynet/internal/update"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newUpdateCmd(g *globals) *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update lazynet to the latest release",
		Long: `Update lazynet to the latest release on GitHub, the same way it was installed:

  install script, tarball, go install  replaces the binary where it is, like
                                       running the install script again
  .deb / .rpm package                  downloads the new package and installs
                                       it with dpkg / rpm

Every download is checked against the release's SHA-256 checksums before it
is used. sudo asks for your password when the install location needs root
(the install script's default, /usr/local/bin). Afterwards lazynet prints
what changed.

Use --check to only report whether a newer release exists.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if g.demo {
				return errors.New("update is not available in demo mode")
			}
			if g.output != "table" && g.output != "json" {
				return fmt.Errorf("--output must be 'table' or 'json'")
			}
			current, _, _ := buildInfo()
			client := update.NewClient(current)
			if check {
				return checkUpdate(cmd.Context(), client, current, g.json())
			}
			if g.json() {
				return errors.New("-o json is only supported with --check")
			}
			_, err := runUpdate(cmd.Context(), client, current, os.Stdout, true)
			return err
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "only report whether a newer release exists")
	return cmd
}

func checkUpdate(ctx context.Context, client update.Client, current string, asJSON bool) error {
	rel, err := client.Latest(ctx)
	if err != nil {
		return err
	}
	newer := update.IsRelease(current) && update.Compare(rel.Version, current) > 0
	if asJSON {
		return printJSON(map[string]any{
			"current": current, "latest": rel.Version, "update_available": newer, "url": rel.URL,
		})
	}
	switch {
	case !update.IsRelease(current):
		fmt.Printf("lazynet %s is a development build; the latest release is %s\n", current, rel.Version)
	case newer:
		fmt.Println(theme.Caution.Render("lazynet "+rel.Version+" is available") + " (you have " + current + ")")
		fmt.Println("Run 'lazynet update' to install it. Release notes: " + rel.URL)
	default:
		fmt.Println(theme.Good.Render("✓ lazynet " + current + " is up to date"))
	}
	return nil
}

// runUpdate installs the latest release. With showNotes, the new binary
// prints what changed and the notes are recorded as seen; otherwise the
// terminal UI shows them when it opens again. It returns the path of the
// updated binary, or "" when there was nothing to do.
func runUpdate(ctx context.Context, client update.Client, current string, out io.Writer, showNotes bool) (string, error) {
	if !update.IsRelease(current) {
		return "", fmt.Errorf("lazynet %s is a development build: update it with 'git pull && make build', or install a release (%s)", current, update.ReleasesPage)
	}
	fmt.Fprintln(out, theme.Dim.Render("checking for a new release…"))
	rel, err := client.Latest(ctx)
	if err != nil {
		return "", err
	}
	if update.Compare(rel.Version, current) <= 0 {
		fmt.Fprintln(out, theme.Good.Render("✓ lazynet "+current+" is up to date"))
		return "", nil
	}
	fmt.Fprintln(out, theme.Bold.Render("Updating lazynet "+current+" → "+rel.Version))
	in, err := update.NewInstaller(client, out)
	if err != nil {
		return "", err
	}
	if err := in.Install(ctx, rel); err != nil {
		return "", fmt.Errorf("update failed: %w", err)
	}
	fmt.Fprintln(out, theme.Good.Render("✓ lazynet "+rel.Version+" installed"))
	if showNotes {
		fmt.Fprintln(out)
		// The new binary knows its own release notes.
		cmd := exec.Command(in.Exe, "changelog", "--since", current)
		cmd.Stdout, cmd.Stderr = out, io.Discard
		if cmd.Run() != nil {
			fmt.Fprintln(out, "What's new: "+rel.URL)
		}
		if p, err := config.DefaultPaths(); err == nil {
			_ = update.MarkSeen(p.UpdateState, rel.Version)
		}
	}
	return in.Exe, nil
}

// updateFromTUI runs after the user chose "update" in the terminal UI. The UI
// has closed; on success the new binary replaces this process and opens the
// UI again, which shows what changed.
func updateFromTUI(ctx context.Context, current string) error {
	exe, err := runUpdate(ctx, update.NewClient(current), current, os.Stdout, false)
	if err != nil || exe == "" {
		return err
	}
	fmt.Println(theme.Dim.Render("opening lazynet again…"))
	if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
		return fmt.Errorf("lazynet was updated, but could not be started again (%w): run lazynet", err)
	}
	return nil
}

// updates returns what the terminal UI needs to announce releases, or nil
// when it must stay quiet: demo mode and development builds.
func (g *globals) updates() tui.Updates {
	current, _, _ := buildInfo()
	if g.demo || !update.IsRelease(current) {
		return nil
	}
	return &update.Checker{
		Version:   current,
		StatePath: g.app.Paths.UpdateState,
		Client:    update.NewClient(current),
		Online:    g.app.Cfg.CheckUpdates,
	}
}

func newChangelogCmd(g *globals) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:   "changelog",
		Short: "Show what changed in each lazynet release",
		Long: `Show the release notes embedded in this lazynet, newest first: every
release up to the running version, or only those newer than --since.`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if g.output != "table" && g.output != "json" {
				return fmt.Errorf("--output must be 'table' or 'json'")
			}
			if since != "" && !update.IsRelease(since) {
				return fmt.Errorf("--since: %q is not a release version (for example 0.1.0)", since)
			}
			current, _, _ := buildInfo()
			notes := update.Notes()
			if update.IsRelease(current) {
				var upTo []update.Note
				for _, n := range notes {
					if update.Compare(n.Version, current) <= 0 {
						upTo = append(upTo, n)
					}
				}
				notes = upTo
			}
			if since != "" {
				var after []update.Note
				for _, n := range notes {
					if update.Compare(n.Version, since) > 0 {
						after = append(after, n)
					}
				}
				notes = after
			}
			if g.json() {
				if notes == nil {
					notes = []update.Note{}
				}
				return printJSON(notes)
			}
			if len(notes) == 0 {
				fmt.Println("No release notes to show.")
				return nil
			}
			fmt.Print(changelogText(notes, termWidth()))
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "only releases newer than this version")
	return cmd
}

func changelogText(notes []update.Note, width int) string {
	var b strings.Builder
	for i, n := range notes {
		if i > 0 {
			b.WriteString("\n")
		}
		head := theme.Title.Render("lazynet " + n.Version)
		if n.Date != "" {
			head += theme.Dim.Render("  " + n.Date)
		}
		b.WriteString(head + "\n\n")
		for _, l := range render.ReleaseNotes(n.Body, width) {
			b.WriteString(l + "\n")
		}
	}
	return b.String()
}

func termWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return min(w, 100)
	}
	return 80
}
