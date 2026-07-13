package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/N1xev/spin/internal/log"
	"github.com/N1xev/spin/internal/registry"
)

var updateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Refresh a pinned template (or all, if name is omitted)",
	Long:  "Re-clone or re-copy the on-disk cache of a pinned template so the next `spin new` sees the latest spin.toml and _base/ tree. If name is omitted, every pinned template is refreshed.",
	Example: `  # Refresh one pinned template
  spin update go-cli

  # Refresh every pinned template
  spin update`,
	Args:          cobra.MaximumNArgs(1),
	RunE:          runUpdate,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

// runUpdate drives `spin update [name]`. Iterates either over the
// single named pin or over all of them, calling refreshOne (which
// does the rollback-aware clone/copy + version bump + Pin write)
// for each.
func runUpdate(cmd *cobra.Command, args []string) error {
	client := registry.New()
	pinned, err := client.ListPinned(cmd.Context())
	if err != nil {
		return err
	}
	if len(pinned) == 0 {
		printInfo("no pinned templates to update")
		printHint("use `spin add <spec>` to pin one (local path or git URL)")
		return nil
	}

	// Filter to the named pin (if given) or to all. An unknown
	// name is an error, not a silent no-op, so the user notices a
	// typo before assuming "it ran".
	var targets []registry.Pinned
	if len(args) == 1 {
		name := args[0]
		for _, p := range pinned {
			if p.Name == name {
				targets = []registry.Pinned{p}
				break
			}
		}
		if len(targets) == 0 {
			return fmt.Errorf("spin update: no pinned template named %q (run `spin list`)", name)
		}
	} else {
		targets = pinned
	}

	var ok, failed int
	for _, p := range targets {
		updated, err := refreshOne(cmd.Context(), client, p)
		if err != nil {
			printWarn("%s: %v", p.Name, err)
			failed++
			continue
		}
		short := updated.Version
		if len(short) > 10 {
			short = short[:10]
		}
		printSuccess("updated %s -> %s", updated.Name, short)
		ok++
	}
	if failed > 0 {
		log.Error("template refresh finished with failures", "updated", ok, "failed", failed)
		return fmt.Errorf("spin update: %d template(s) failed to refresh", failed)
	}
	if ok == 1 {
		log.Stdout.Print("1 template refreshed")
	} else {
		log.Stdout.Info(fmt.Sprintf("%d templates refreshed", ok))
	}
	return nil
}

// refreshOne re-clones or re-copies the cache for a single pin,
// with rollback: the old on-disk cache is moved aside before the
// refresh runs; on failure, the old cache is moved back into
// place. The returned Pinned has the new Version; the caller is
// expected to persist it via client.Pin.
//
// Returns a non-nil error if EITHER the refresh OR the pin write
// failed. The error is the LAST thing that failed; the rollback
// itself is best-effort and reported as a warning, since by then
// we've done everything we can.
func refreshOne(ctx context.Context, client *registry.Client, p registry.Pinned) (registry.Pinned, error) {
	if p.LocalPath == "" {
		return p, fmt.Errorf("no LocalPath on pin; re-run `spin add %s`", p.Source)
	}
	// (1) Snapshot the old cache to a sibling .bak-<ts> dir so we
	// can move it back on failure. We use a rename (atomic on the
	// same filesystem) instead of a copy, because the cache is
	// potentially hundreds of MB and we don't need a second copy.
	backup, haveBackup := backupPath(p.LocalPath)
	var backupErr error
	// Only snapshot if the cache dir actually exists. Refresh's
	// own stat check rejects a missing cache with a "re-run
	// `spin add`" error, so we don't need a snapshot to roll
	// back to.
	if _, err := os.Stat(p.LocalPath); err == nil {
		if err := os.Rename(p.LocalPath, backup); err != nil {
			// Rename can fail across filesystems. If so, copy
			// then remove (slower, but correct). We don't bail
			// out -- the user would still want the update
			// attempted, with the old cache left in place as the
			// safety net.
			backupErr = err
		} else {
			haveBackup = true
		}
	}

	// (2) Run the refresh. Refresh does its own clone/copy onto
	// the original LocalPath; on success we delete the .bak and
	// return the new pin. On failure, we attempt to move .bak
	// back into place.
	updated, err := client.Refresh(ctx, p)
	if err != nil {
		// Roll back if we can. A failed rollback is reported but
		// not returned, since the original error is the one the
		// user needs to act on.
		if haveBackup {
			if rbErr := os.Rename(backup, p.LocalPath); rbErr != nil {
				printWarn("rollback also failed for %s: original=%v rollback=%v (backup at %s)",
					p.Name, err, rbErr, backup)
			}
		} else if backupErr != nil {
			printWarn("could not back up %s before refresh; update left old cache in place: %v",
				p.Name, backupErr)
		}
		return p, err
	}

	// (3) Success: persist the updated pin. We do this LAST so a
	// write failure doesn't strand the user with a refreshed
	// cache but an outdated Version.
	if err := client.Pin(ctx, updated); err != nil {
		// Cache is already refreshed; pin write failed. Roll back
		// the cache so the recorded Version matches what's on
		// disk.
		if haveBackup {
			if rbErr := os.Rename(backup, p.LocalPath); rbErr != nil {
				printWarn("pin write failed for %s AND rollback failed: pin-err=%v rollback-err=%v (backup at %s)",
					p.Name, err, rbErr, backup)
			}
		}
		return p, fmt.Errorf("pin write failed: %v", err)
	}

	// (4) All green. Delete the .bak.
	if haveBackup {
		if rmErr := os.RemoveAll(backup); rmErr != nil {
			log.Debug("failed to remove update backup", "path", backup, "err", rmErr)
		}
	}
	return updated, nil
}

// backupPath returns the path to use for the on-disk rollback
// snapshot: LocalPath + ".bak-<unix-ts>". A timestamp keeps
// repeated updates from clobbering each other's backups (e.g. if
// a user runs `spin update` twice in a second).
func backupPath(localPath string) (string, bool) {
	if localPath == "" {
		return "", false
	}
	ts := time.Now().Unix()
	return fmt.Sprintf("%s.bak-%d", localPath, ts), true
}
