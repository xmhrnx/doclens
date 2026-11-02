# DocLens

A document scanner for Android and iOS built with [Giode](https://github.com/xmhrnx/giode).

- **Live scanner**: the camera picture with the page outline tracked
  as you aim, automatic capture once the page holds still, torch, tap to
  focus and the scan modes; the full-resolution shot is then analysed again.
- **Scanning**: finds the page in a photo (Hough lines, edge contours and color
  segmentation, scored against the photo's edges and refined to sub-pixel accuracy),
  recovers the page's true aspect ratio from the perspective and flattens it.
  Batch, ID card (front and back on one sheet at real size), book (two-page spread
  split in two), import from the photo library and import of existing PDFs (each page
  rendered to an image: PdfRenderer on Android, Core Graphics on iOS, in
  `internal/pdfpages` and `android/java`).
- **Enhancement**: Magic Color, No Shadow, Grayscale, B&W and Original, with brightness
  and contrast; shadows and uneven light are removed. Every edit is non-destructive.
- **Editing**: crop with draggable corners and sides, a magnifier and snapping; rotate;
  retake; reorder, select, delete with undo, extract pages into a new document, merge.
- **Markup and signing**: pen and highlighter; saved hand-drawn signatures placed and
  resized on pages.
- **Export**: PDF (Auto, A4, Letter, Legal; margins; three quality levels), AES-256
  password protection, watermarks, JPEG pages and one long image; share or save.
- **Text**: recognized on the device in the background; search finds words inside
  pages, a page's text can be copied or shared, and PDFs get an invisible, searchable
  text layer.
- **Library**: folders, favorites, search, PIN app lock with fingerprint or Face ID,
  light and dark themes, pinch-to-zoom pages, printing, Save to Photos and sharing
  every page at once.

## Layout

```
main.go              wiring: fonts, icons, the library, the window
internal/scan        detection, perspective correction, filters, annotations (no UI)
internal/pdf         PDF writer with AES-256 encryption
internal/library     documents, pages, settings, storage, rendering and export
ui/                  screens and components, one per file
assets/              Inter font, Lucide icons, app icon, licenses
```

## Build

```sh
giode run              # on this computer
giode run android      # on a connected phone
giode build android ios
go test ./...
```
