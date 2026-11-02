package ui

import (
	"image"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/styles/properties"

	"github.com/mehran/scanly/internal/library"
)

// docThumb shows a document's first page.
func docThumb(a *App, d *library.Document, w, h int) giode.Element {
	if len(d.Pages) == 0 {
		return thumbFrame(w, h, nil)
	}
	return pageThumb(a, d.ID, d.Pages[0], w, h)
}

// pageThumb shows a page's thumbnail, or a placeholder while it loads.
func pageThumb(a *App, docID string, p *library.Page, w, h int) giode.Element {
	return thumbFrame(w, h, a.thumbs.get(docID, p))
}

func thumbFrame(w, h int, img image.Image) giode.Element {
	st := giode.Styles{
		Width: properties.Width(w), Height: properties.Height(h), BorderRadius: 10, Background: cFill, Overflow: giode.OverflowHidden,
		BorderWidth: 1, BorderColor: cBorder, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
	}
	if img == nil {
		return giode.Box(st, icon("file-text", max(16, w/3), cMuted))
	}
	return giode.Box(st, giode.Image(img, giode.Styles{Width: properties.Width(w), Height: properties.Height(h), Fit: giode.FitCover}))
}
