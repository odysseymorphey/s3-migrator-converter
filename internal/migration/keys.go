package migration

import (
	"path"
	"strings"
)

func isPNGKey(key string) bool {
	return strings.EqualFold(path.Ext(key), ".png")
}

func destinationKey(sourceKey, sourcePrefix, destPrefix string) string {
	relative := strings.TrimPrefix(sourceKey, sourcePrefix)
	relative = strings.TrimPrefix(relative, "/")

	webpName := strings.TrimSuffix(relative, path.Ext(relative)) + ".webp"
	if destPrefix == "" {
		return webpName
	}

	return destPrefix + webpName
}
