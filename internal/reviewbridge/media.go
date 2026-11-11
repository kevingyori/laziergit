package reviewbridge

import (
	"encoding/base64"
	"fmt"
	"path"
	"strings"
)

// Media is a picture in an explanation: inline SVG an agent wrote, an image
// file of the repository as of the snapshot, or a base64 data URL.
type Media struct {
	SVG     string `json:"svg,omitempty"`
	Path    string `json:"path,omitempty"`
	DataURL string `json:"data_url,omitempty"`
	Caption string `json:"caption,omitempty"`
}

const (
	mediaPerBlock = 3
	svgBytes      = 200 << 10
	imageBytes    = 4 << 20
)

var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true}

// ValidateMedia checks pictures for one section or the overview.
func ValidateMedia(s Snapshot, media []Media) error {
	if len(media) > mediaPerBlock {
		return fmt.Errorf("at most %d media items per section or overview", mediaPerBlock)
	}
	for i, m := range media {
		set := 0
		for _, v := range []string{m.SVG, m.Path, m.DataURL} {
			if v != "" {
				set++
			}
		}
		if set != 1 || len(m.Caption) > 200 {
			return fmt.Errorf("media %d: give exactly one of svg, path or data_url, and a caption up to 200 characters", i+1)
		}
		switch {
		case m.SVG != "":
			if len(m.SVG) > svgBytes || !strings.HasPrefix(strings.TrimSpace(m.SVG), "<svg") {
				return fmt.Errorf("media %d: svg must be one <svg> element of at most 200 KB", i+1)
			}
		case m.DataURL != "":
			head, data, ok := strings.Cut(m.DataURL, ",")
			if !ok || !strings.HasPrefix(head, "data:image/") || !strings.HasSuffix(head, ";base64") || base64.StdEncoding.DecodedLen(len(data)) > imageBytes {
				return fmt.Errorf("media %d: data_url must be a base64 image of at most 4 MB", i+1)
			}
			if _, err := base64.StdEncoding.DecodeString(data); err != nil {
				return fmt.Errorf("media %d: data_url is not valid base64", i+1)
			}
		default:
			if !imageExts[strings.ToLower(path.Ext(m.Path))] {
				return fmt.Errorf("media %d: path must be a png, jpeg, gif or svg file", i+1)
			}
			if _, err := SourceText(s, m.Path, "new"); err != nil {
				return fmt.Errorf("media %d: %v", i+1, err)
			}
		}
	}
	return nil
}

func mediaSchema() map[string]any {
	str := map[string]any{"type": "string"}
	return map[string]any{"type": "array", "maxItems": mediaPerBlock, "description": "Optional pictures shown with the text: an image file in the repository (screenshots, assets), a base64 data URL, or an <svg> you write. SVG <text> is not drawn: put words in the caption, and use the section diagram for labelled boxes and arrows.", "items": map[string]any{"type": "object", "properties": map[string]any{
		"svg": str, "path": str, "data_url": str, "caption": str,
	}}}
}
