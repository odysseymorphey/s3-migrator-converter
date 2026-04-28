package migration

import (
	"path"
	"strings"
)

func destinationKey(sourceKey, sourcePrefix, destPrefix string) string {
	relative := strings.TrimPrefix(sourceKey, sourcePrefix)
	relative = strings.TrimPrefix(relative, "/")

	webpName := strings.TrimSuffix(relative, path.Ext(relative)) + ".webp"
	if destPrefix == "" {
		return webpName
	}

	return destPrefix + webpName
}
