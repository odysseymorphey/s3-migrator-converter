package migration

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime"
	"path"
	"strings"

	"github.com/gen2brain/avif"
)

type imageFormat struct {
	Name        string
	Extensions  []string
	ContentType string
	MagicMatch  func(header []byte) bool
	MagicLen    int
	Decode      func(io.Reader) (image.Image, error)
}

var supportedFormats = map[string]imageFormat{
	"png": {
		Name:        "png",
		Extensions:  []string{".png"},
		ContentType: "image/png",
		MagicLen:    8,
		MagicMatch: func(header []byte) bool {
			return bytes.HasPrefix(header, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
		},
		Decode: func(r io.Reader) (image.Image, error) {
			return png.Decode(r)
		},
	},
	"jpeg": {
		Name:        "jpeg",
		Extensions:  []string{".jpg", ".jpeg"},
		ContentType: "image/jpeg",
		MagicLen:    3,
		MagicMatch: func(header []byte) bool {
			return bytes.HasPrefix(header, []byte{0xFF, 0xD8, 0xFF})
		},
		Decode: func(r io.Reader) (image.Image, error) {
			return jpeg.Decode(r)
		},
	},
	"avif": {
		Name:        "avif",
		Extensions:  []string{".avif"},
		ContentType: "image/avif",
		MagicLen:    12,
		MagicMatch: func(header []byte) bool {
			if len(header) < 12 {
				return false
			}
			if !bytes.Equal(header[4:8], []byte("ftyp")) {
				return false
			}
			brand := string(header[8:12])
			return brand == "avif" || brand == "avis"
		},
		Decode: func(r io.Reader) (image.Image, error) {
			return avif.Decode(r)
		},
	},
}

func detectFormat(key string, contentType string, header []byte, enabled []string) *imageFormat {
	ext := strings.ToLower(path.Ext(key))
	for _, name := range enabled {
		f, ok := supportedFormats[name]
		if !ok {
			continue
		}
		for _, e := range f.Extensions {
			if ext == e {
				return &f
			}
		}
	}

	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			mediaType = strings.TrimSpace(strings.Split(contentType, ";")[0])
		}
		for _, name := range enabled {
			f, ok := supportedFormats[name]
			if !ok {
				continue
			}
			if strings.EqualFold(mediaType, f.ContentType) {
				return &f
			}
		}
	}

	if len(header) > 0 {
		for _, name := range enabled {
			f, ok := supportedFormats[name]
			if !ok {
				continue
			}
			if f.MagicMatch(header) {
				return &f
			}
		}
	}

	return nil
}

func maxMagicLen(enabled []string) int {
	n := 0
	for _, name := range enabled {
		if f, ok := supportedFormats[name]; ok && f.MagicLen > n {
			n = f.MagicLen
		}
	}
	return n
}

func ValidateFormats(names []string) error {
	for _, name := range names {
		if _, ok := supportedFormats[name]; !ok {
			known := make([]string, 0, len(supportedFormats))
			for k := range supportedFormats {
				known = append(known, k)
			}
			return fmt.Errorf("unknown source format %q, supported: %s", name, strings.Join(known, ", "))
		}
	}
	return nil
}
