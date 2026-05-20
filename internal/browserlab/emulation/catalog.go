package emulation

import (
	"fmt"
	"sort"
	"strings"
)

type DeviceMetrics struct {
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	PixelRatio float64 `json:"pixelRatio"`
}

type Preset struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	DeviceMetrics DeviceMetrics `json:"deviceMetrics"`
	UserAgent     string        `json:"userAgent"`
}

type Selection struct {
	PresetID      string        `json:"presetId,omitempty"`
	Name          string        `json:"name,omitempty"`
	DeviceMetrics DeviceMetrics `json:"deviceMetrics,omitempty"`
	UserAgent     string        `json:"userAgent,omitempty"`
}

func (selection Selection) IsZero() bool {
	return selection.PresetID == "" && selection.Name == "" && selection.UserAgent == "" && selection.DeviceMetrics == (DeviceMetrics{})
}

func Catalog() []Preset {
	presets := []Preset{
		{
			ID:   "iphone-14",
			Name: "iPhone 14",
			DeviceMetrics: DeviceMetrics{
				Width:      390,
				Height:     844,
				PixelRatio: 3,
			},
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
		},
		{
			ID:   "pixel-7",
			Name: "Pixel 7",
			DeviceMetrics: DeviceMetrics{
				Width:      412,
				Height:     915,
				PixelRatio: 2.625,
			},
			UserAgent: "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36",
		},
		{
			ID:   "ipad-mini",
			Name: "iPad Mini",
			DeviceMetrics: DeviceMetrics{
				Width:      768,
				Height:     1024,
				PixelRatio: 2,
			},
			UserAgent: "Mozilla/5.0 (iPad; CPU OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
		},
	}
	sort.Slice(presets, func(i, j int) bool {
		return presets[i].ID < presets[j].ID
	})
	return presets
}

func Lookup(id string) (Preset, bool) {
	id = NormalizeID(id)
	if id == "" {
		return Preset{}, false
	}
	for _, preset := range Catalog() {
		if preset.ID == id {
			return preset, true
		}
	}
	return Preset{}, false
}

func Select(id string) (Selection, error) {
	id = NormalizeID(id)
	if id == "" {
		return Selection{}, nil
	}
	preset, ok := Lookup(id)
	if !ok {
		return Selection{}, fmt.Errorf("unknown Chrome mobile emulation preset %q", id)
	}
	return Selection{
		PresetID:      preset.ID,
		Name:          preset.Name,
		DeviceMetrics: preset.DeviceMetrics,
		UserAgent:     preset.UserAgent,
	}, nil
}

func NormalizeID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}
