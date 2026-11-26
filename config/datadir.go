package config

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/SREsAreHumanToo/ARTEX-CC/internal/fsperm"
)

const permsMarkerFile = ".perms-tightened"

// ownedPaths lists what ARTEX writes directly in the data directory. Only these
// are tightened, since --data defaults to the working directory and may be a
// project or home directory. traffic/ tightens its own tree. Each name is
// also checked with a .tmp suffix, which an interrupted FileStore write leaves.
var ownedPaths = []string{
	engagementFile,
	IdentityKeyFile,
	ownerLabelKeyFile,
	"auth.json",
	"mcp_servers.json",
	"mcp-config.json",
	"approvals.json",
	"intercept_rules.json",
	"chat-mcp",
	"conversations",
	"settings",
	"agents",
	"skills",
	"workspace",
	"evidence",
}

// PrepareDataDir creates dataDir 0711 when it is missing, so other users can
// reach the public CA certificate under traffic/_ca and nothing else. On the
// first start after an upgrade it restricts what earlier versions wrote at
// ownedPaths to the owner. An existing directory keeps its own mode.
func PrepareDataDir(dataDir string) error {
	if _, err := os.Stat(dataDir); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dataDir, 0o711); err != nil {
			return err
		}
		if err := os.Chmod(dataDir, 0o711); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	marker := filepath.Join(dataDir, permsMarkerFile)
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	ok := true
	for _, name := range ownedPaths {
		for _, p := range []string{name, name + ".tmp"} {
			path := filepath.Join(dataDir, p)
			if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
				continue
			} else if err != nil {
				ok = false
				continue
			}
			if !fsperm.Tighten(path) {
				ok = false
			}
		}
	}
	if ok {
		if err := os.WriteFile(marker, nil, 0o600); err != nil {
			slog.Warn("failed to record permission tightening", "path", marker, "err", err)
		}
	}
	return nil
}
