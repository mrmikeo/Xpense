package integration

import (
	"path"

	"github.com/mrmikeo/Xpense/utils"
)

func isInterrupted(chaindataDir string) bool {
	return utils.FileExists(path.Join(chaindataDir, "unfinished"))
}
