package library

import (
	"github.com/mehran/scanly/internal/pdf"
	"github.com/mehran/scanly/internal/scan"
)

// Quality is the export resolution and compression.
type Quality string

const (
	QualityLow    Quality = "low"
	QualityMedium Quality = "medium"
	QualityHigh   Quality = "high"
)

// JPEG returns the JPEG quality and the longest page side in pixels.
func (q Quality) JPEG() (quality, maxSide int) {
	switch q {
	case QualityLow:
		return 62, 1600
	case QualityHigh:
		return 92, 4000
	}
	return 80, 2600
}

// Theme is the color scheme.
type Theme string

const (
	ThemeSystem Theme = "system"
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
)

// Settings are the user's preferences.
type Settings struct {
	DefaultFilter scan.Filter  `json:"defaultFilter"`
	AutoCrop      bool         `json:"autoCrop"`
	PageSize      pdf.PageSize `json:"pageSize"`
	Margins       bool         `json:"margins"`
	Quality       Quality      `json:"quality"`
	Theme         Theme        `json:"theme"`
	AppLock       bool         `json:"appLock"`
	Author        string       `json:"author"`
	// AutoCapture takes the picture by itself once the camera holds
	// steady on a page.
	AutoCapture bool `json:"autoCapture"`
	// AutoText recognizes the text of new pages in the background, for
	// searching and searchable PDFs.
	AutoText bool `json:"autoText"`
	// Biometric unlocks the app lock with a fingerprint or face.
	Biometric bool `json:"biometric"`
	// SkipReview saves scans straight away, without stopping at the
	// crop and filter screens, when detection is confident.
	SkipReview bool `json:"skipReview"`
}

// DefaultSettings are used on first launch.
func DefaultSettings() Settings {
	return Settings{
		DefaultFilter: scan.FilterMagic,
		AutoCrop:      true,
		PageSize:      pdf.SizeAuto,
		Margins:       false,
		Quality:       QualityMedium,
		Theme:         ThemeSystem,
		AutoCapture:   true,
		AutoText:      true,
	}
}
