package config

import (
	"os"
	"path/filepath"

	"github.com/SREsAreHumanToo/ARTEX-CC/internal/fsperm"
)

const permsMarkerFile = ".perms-tightened"

// PrepareDataDir creates dataDir and, on the first start after an upgrade,
// restricts what earlier versions wrote in it to the owner. The root itself
// stays traversable (0711) so other users can reach the public CA certificate
// under traffic/_ca; traffic/ tightens its own tree.
func PrepareDataDir(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o711); err != nil {
		return err
	}
	marker := filepath.Join(dataDir, permsMarkerFile)
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	if err := os.Chmod(dataDir, 0o711); err != nil {
		return err
	}
	fsperm.TightenOnce(dataDir, marker, "traffic")
	return nil
}
