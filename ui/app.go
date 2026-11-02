// Package ui is DocLens's user interface.
package ui

import (
	"context"
	"errors"
	"sync"

	"github.com/xmhrnx/giode"

	"github.com/xmhrnx/doclens/internal/library"
)

// screen is one page of the app's navigation stack.
type screen interface {
	view() giode.Element
}

// App owns the library, the navigation stack and the services every
// screen uses.
type App struct {
	g      *giode.App
	store  *library.Store
	stack  []screen
	toasts *giode.ToasterElement
	thumbs *imageCache // small renders for lists and grids
	pages  *imageCache // large renders for the page viewer
	// font draws watermarks into exported pages.
	font []byte
	// licenses is the open-source notices file (giode licenses).
	licenses []byte

	busy     string  // a blocking task's message; "" when idle
	progress float64 // the blocking task's progress, 0 when unknown

	locked bool
	lock   *lockScreen
	home   *homeScreen
	text   *textReader

	saveMu sync.Mutex
}

// New creates the UI for a library. font is a TrueType font for
// watermarks; licenses is the JSON that giode licenses writes.
func New(g *giode.App, store *library.Store, font, licenses []byte) *App {
	setupPalettes()
	a := &App{
		g:        g,
		store:    store,
		font:     font,
		licenses: licenses,
		toasts:   giode.Toaster().Placement(giode.ToastBottomCenter).Max(2),
	}
	a.thumbs = newImageCache(a, 400, func(docID string, p *library.Page) (imageResult, error) {
		img, err := store.Thumbnail(docID, p)
		return imageResult{img: img}, err
	})
	a.pages = newImageCache(a, 6, func(docID string, p *library.Page) (imageResult, error) {
		img, err := store.RenderPage(docID, p)
		if err != nil {
			return imageResult{}, err
		}
		return imageResult{img: downscaleForScreen(img)}, nil
	})
	a.text = newTextReader(a)
	a.home = newHomeScreen(a)
	a.stack = []screen{a.home}
	a.lock = newLockScreen(a)
	a.locked = store.Settings.AppLock
	a.applyThemeSetting()
	for _, d := range store.Docs {
		pruneInBackground(a, d)
	}
	a.text.queueAll()
	return a
}

// View is the root view.
func (a *App) View() giode.Element {
	if a.locked {
		return giode.Box(giode.Styles{Width: -1, Height: -1, Background: cBg}, a.lock.view(), a.toasts)
	}
	if len(a.stack) > 1 {
		giode.HandleBack(func() { a.pop() })
	}
	top := a.stack[len(a.stack)-1]
	return giode.Box(giode.Styles{Width: -1, Height: -1, Background: cBg, FlexDirection: giode.FlexDirectionColumn},
		top.view(),
		a.busyOverlay(),
		a.toasts,
	)
}

// OnBackground locks the app when it leaves the screen, if the user
// asked for a lock, and saves.
func (a *App) OnBackground() {
	a.save()
	if a.store.Settings.AppLock {
		a.lock.reset()
		a.locked = true
	}
}

// OnSystemTheme follows the system's light or dark mode while the
// user's choice is "system".
func (a *App) OnSystemTheme(dark bool) {
	if a.store.Settings.Theme == library.ThemeSystem {
		applyTheme(dark)
	}
}

func (a *App) applyThemeSetting() {
	switch a.store.Settings.Theme {
	case library.ThemeDark:
		applyTheme(true)
	case library.ThemeLight:
		applyTheme(false)
	default:
		// An unknown system preference means light.
		dark, err := giode.SystemDarkMode()
		applyTheme(dark && err == nil)
	}
}

// activeScreen is a screen that holds resources (the camera) only while
// it's on top.
type activeScreen interface {
	// shown runs when the screen comes on top.
	shown()
	// hidden runs when it's covered by another screen or closed.
	hidden()
}

func (a *App) push(s screen) {
	a.changeTop(func() { a.stack = append(a.stack, s) })
}

func (a *App) pop() {
	a.changeTop(func() {
		if len(a.stack) > 1 {
			a.stack = a.stack[:len(a.stack)-1]
		}
	})
}

// popTo pops screens until s is on top.
func (a *App) popTo(s screen) {
	a.changeTop(func() {
		for len(a.stack) > 1 && a.stack[len(a.stack)-1] != s {
			a.stack = a.stack[:len(a.stack)-1]
		}
	})
}

// changeTop edits the stack and tells the screens leaving and taking
// the top.
func (a *App) changeTop(edit func()) {
	before := a.stack[len(a.stack)-1]
	edit()
	after := a.stack[len(a.stack)-1]
	if before == after {
		return
	}
	if s, ok := before.(activeScreen); ok {
		s.hidden()
	}
	if s, ok := after.(activeScreen); ok {
		s.shown()
	}
}

// save writes the library in the background.
func (a *App) save() {
	data, err := a.store.Snapshot()
	if err != nil {
		a.fail(err)
		return
	}
	go func() {
		a.saveMu.Lock()
		defer a.saveMu.Unlock()
		if err := a.store.Persist(data); err != nil {
			a.fail(err)
		}
	}()
}

// fail shows an error, ignoring cancellations. Safe from any goroutine.
func (a *App) fail(err error) {
	if err == nil || errors.Is(err, giode.ErrCanceled) || errors.Is(err, context.Canceled) {
		return
	}
	a.toasts.Show(giode.ToastError, err.Error())
}

// notify shows a short confirmation. Safe from any goroutine.
func (a *App) notify(msg string) { a.toasts.Show(giode.ToastSuccess, msg) }

// work runs fn on a goroutine while a blocking overlay shows msg, then
// runs done on the UI goroutine if fn succeeded.
func (a *App) work(msg string, fn func(progress func(float64)) error, done func()) {
	a.busy, a.progress = msg, 0
	go func() {
		err := fn(func(p float64) { a.g.Post(func() { a.progress = p }) })
		a.g.Post(func() {
			a.busy = ""
			if err != nil {
				a.fail(err)
				return
			}
			if done != nil {
				done()
			}
		})
	}()
}

// busyOverlay blocks the screen while a task runs.
func (a *App) busyOverlay() giode.Element {
	if a.busy == "" {
		return giode.Box(giode.Styles{Display: giode.DisplayNone})
	}
	card := []giode.Element{
		giode.Spinner(giode.Styles{Width: 36, Height: 36, Color: cPrimary}),
		giode.Text(a.busy, with(tLabel, giode.Styles{TextAlign: giode.TextAlignCenter})),
	}
	if a.progress > 0 {
		card = append(card, giode.Box(giode.Styles{Width: 180},
			giode.Progress(float32(a.progress), giode.Styles{Color: cPrimary, Background: cFill, Height: 6})))
	}
	return giode.Box(giode.Styles{
		Position: giode.PositionAbsolute, Left: giode.Flush, Right: giode.Flush, Top: giode.Flush, Bottom: giode.Flush,
		ZIndex: 50, Background: "#0A0C1499", Align: giode.AlignCenter, Justify: giode.JustifyCenter,
	},
		// Swallow taps on the scrim.
		giode.Pressable(func(giode.PressState) giode.Element {
			return giode.Stack(giode.Styles{
				Background: cSurface, BorderRadius: radiusCard, Padding: giode.SymmetricInset(28, 24),
				Align: giode.AlignCenter, Gap: 14, MinWidth: 220, BoxShadow: shadowCard,
			}, card...)
		}),
	)
}
