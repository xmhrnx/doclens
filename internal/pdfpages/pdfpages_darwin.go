package pdfpages

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation

#include <stdlib.h>
#include <CoreGraphics/CoreGraphics.h>

// scanlyPDFOpen opens a PDF. *status is 0 on success, 1 when the file
// can't be read and 2 when it needs a password.
static CGPDFDocumentRef scanlyPDFOpen(const char *path, int *status) {
	*status = 1;
	CFURLRef url = CFURLCreateFromFileSystemRepresentation(NULL, (const UInt8 *)path, strlen(path), false);
	if (url == NULL) {
		return NULL;
	}
	CGPDFDocumentRef doc = CGPDFDocumentCreateWithURL(url);
	CFRelease(url);
	if (doc == NULL) {
		return NULL;
	}
	// Documents with only an owner password open with an empty one.
	if (CGPDFDocumentIsEncrypted(doc) && !CGPDFDocumentIsUnlocked(doc) && !CGPDFDocumentUnlockWithPassword(doc, "")) {
		CGPDFDocumentRelease(doc);
		*status = 2;
		return NULL;
	}
	*status = 0;
	return doc;
}

// scanlyPDFPageSize returns page i's (from 1) size in points as shown,
// rotation included.
static void scanlyPDFPageSize(CGPDFDocumentRef doc, int i, double *w, double *h) {
	*w = *h = 0;
	CGPDFPageRef page = CGPDFDocumentGetPage(doc, i);
	if (page == NULL) {
		return;
	}
	CGRect r = CGPDFPageGetBoxRect(page, kCGPDFCropBox);
	if (CGPDFPageGetRotationAngle(page) % 180 != 0) {
		*w = r.size.height;
		*h = r.size.width;
	} else {
		*w = r.size.width;
		*h = r.size.height;
	}
}

// scanlyPDFRender draws page i (from 1) into pix, w by h RGBA pixels, on
// white.
static int scanlyPDFRender(CGPDFDocumentRef doc, int i, void *pix, int w, int h) {
	CGPDFPageRef page = CGPDFDocumentGetPage(doc, i);
	if (page == NULL) {
		return 0;
	}
	CGColorSpaceRef cs = CGColorSpaceCreateDeviceRGB();
	CGContextRef ctx = CGBitmapContextCreate(pix, w, h, 8, w * 4, cs, kCGImageAlphaPremultipliedLast | kCGBitmapByteOrder32Big);
	CGColorSpaceRelease(cs);
	if (ctx == NULL) {
		return 0;
	}
	CGContextSetRGBFillColor(ctx, 1, 1, 1, 1);
	CGContextFillRect(ctx, CGRectMake(0, 0, w, h));
	CGContextSetInterpolationQuality(ctx, kCGInterpolationHigh);
	double pw, ph;
	scanlyPDFPageSize(doc, i, &pw, &ph);
	// CGPDFPageGetDrawingTransform never scales up, so draw into a
	// page-sized rectangle and scale the context to the bitmap.
	if (pw > 0 && ph > 0) {
		CGContextScaleCTM(ctx, w / pw, h / ph);
	}
	CGContextConcatCTM(ctx, CGPDFPageGetDrawingTransform(page, kCGPDFCropBox, CGRectMake(0, 0, pw, ph), 0, true));
	CGContextDrawPDFPage(ctx, page);
	CGContextRelease(ctx);
	return 1;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

const supported = true

type platformDoc struct{ doc C.CGPDFDocumentRef }

func open(path string) (platformDoc, int, error) {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var status C.int
	doc := C.scanlyPDFOpen(cp, &status)
	switch {
	case status == 2:
		return platformDoc{}, 0, ErrPassword
	case doc == nil:
		return platformDoc{}, 0, errors.New("this file isn't a readable PDF")
	}
	return platformDoc{doc: doc}, int(C.CGPDFDocumentGetNumberOfPages(doc)), nil
}

func (d platformDoc) pageSize(i int) (float64, float64, error) {
	var w, h C.double
	C.scanlyPDFPageSize(d.doc, C.int(i+1), &w, &h)
	return float64(w), float64(h), nil
}

func (d platformDoc) render(i, w, h int) ([]byte, error) {
	pix := make([]byte, w*h*4)
	if C.scanlyPDFRender(d.doc, C.int(i+1), unsafe.Pointer(&pix[0]), C.int(w), C.int(h)) == 0 {
		return nil, errors.New("couldn't draw the PDF page")
	}
	return pix, nil
}

func (d platformDoc) close() error {
	C.CGPDFDocumentRelease(d.doc)
	return nil
}
