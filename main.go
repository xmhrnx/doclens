// Command doclens is a document scanner: it finds pages in photos,
// flattens and cleans them up, and keeps them as PDF-ready documents.
package main

import (
	"bytes"
	"embed"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"

	gioapp "gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/opentype"
	"github.com/xmhrnx/giode"

	"github.com/xmhrnx/doclens/internal/library"
	"github.com/xmhrnx/doclens/ui"
)

//go:embed assets/icon.png
var appIcon []byte

// logo is the icon with rounded corners, shown inside the app.
//
//go:embed assets/logo.png
var logoPNG []byte

//go:embed assets/fonts/*.ttf
var fontFS embed.FS

//go:embed assets/icons
var iconFS embed.FS

//go:embed assets/licenses.json
var licensesJSON []byte

func main() {
	if err := loadFonts(); err != nil {
		log.Fatal(err)
	}
	if err := giode.RegisterIconsFS(iconFS, "assets/icons"); err != nil {
		log.Fatal(err)
	}
	giode.SecureStore.Service = "com.xmhrnx.doclens"

	dir, err := dataDir()
	if err != nil {
		log.Fatal(err)
	}
	store, err := library.Open(dir)
	if err != nil {
		log.Fatal(err)
	}
	bold, err := fontFS.ReadFile("assets/fonts/Inter-Bold.ttf")
	if err != nil {
		log.Fatal(err)
	}

	var u *ui.App
	app := giode.New(giode.Config{
		Title:               "DocLens",
		Size:                image.Pt(420, 860),
		MinWidth:            360,
		MinHeight:           600,
		Resizable:           true,
		Background:          "theme.background",
		Icon:                appIcon,
		Orientation:         giode.OrientationPortrait,
		OnBackground:        func() { u.OnBackground() },
		OnSystemThemeChange: func(dark bool) { u.OnSystemTheme(dark) },
		OnShareReceived:     func(s giode.SharedContent) { u.OnShare(s) },
	})
	logo, err := png.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		log.Fatal(err)
	}
	u = ui.New(app, store, ui.Assets{Font: bold, Licenses: licensesJSON, Logo: logo})
	if err := app.Run(u.View); err != nil {
		log.Fatal(err)
	}
}

// loadFonts registers Inter in every weight the UI uses.
func loadFonts() error {
	entries, err := fontFS.ReadDir("assets/fonts")
	if err != nil {
		return err
	}
	var faces []font.FontFace
	for _, e := range entries {
		data, err := fontFS.ReadFile("assets/fonts/" + e.Name())
		if err != nil {
			return err
		}
		fs, err := opentype.ParseCollection(data)
		if err != nil {
			return err
		}
		faces = append(faces, fs...)
	}
	giode.RegisterFonts(faces)
	return nil
}

// dataDir is where the library lives: the app's private storage.
func dataDir() (string, error) {
	base, err := gioapp.DataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "DocLens")
	return dir, os.MkdirAll(dir, 0o755)
}
