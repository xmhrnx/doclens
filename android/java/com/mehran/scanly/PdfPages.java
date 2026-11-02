package com.mehran.scanly;

import android.graphics.Bitmap;
import android.graphics.Color;
import android.graphics.pdf.PdfRenderer;
import android.os.ParcelFileDescriptor;

import java.io.File;
import java.nio.ByteBuffer;
import java.util.HashMap;
import java.util.Map;

// PdfPages renders PDF pages for Scanly's PDF import (internal/pdfpages).
// PdfRenderer keeps one page open at a time, so each document is used
// under its own lock.
public final class PdfPages {
	private static final Map<Integer, PdfRenderer> docs = new HashMap<Integer, PdfRenderer>();
	private static int next;

	private PdfPages() {}

	// open opens the PDF at path and returns its id.
	public static synchronized int open(String path) throws Exception {
		ParcelFileDescriptor fd = ParcelFileDescriptor.open(new File(path), ParcelFileDescriptor.MODE_READ_ONLY);
		PdfRenderer r;
		try {
			r = new PdfRenderer(fd);
		} catch (SecurityException e) {
			fd.close();
			throw new Exception("password protected");
		} catch (Exception e) {
			fd.close();
			throw new Exception("not a readable PDF: " + e.getMessage());
		}
		next++;
		docs.put(next, r);
		return next;
	}

	public static int pageCount(int id) throws Exception {
		return doc(id).getPageCount();
	}

	// pageWidth and pageHeight are in points (1/72 inch).
	public static int pageWidth(int id, int page) throws Exception {
		PdfRenderer r = doc(id);
		synchronized (r) {
			PdfRenderer.Page p = r.openPage(page);
			try {
				return p.getWidth();
			} finally {
				p.close();
			}
		}
	}

	public static int pageHeight(int id, int page) throws Exception {
		PdfRenderer r = doc(id);
		synchronized (r) {
			PdfRenderer.Page p = r.openPage(page);
			try {
				return p.getHeight();
			} finally {
				p.close();
			}
		}
	}

	// render draws the page scaled to w by h pixels on white and returns
	// it as RGBA bytes.
	public static byte[] render(int id, int page, int w, int h) throws Exception {
		PdfRenderer r = doc(id);
		synchronized (r) {
			PdfRenderer.Page p = r.openPage(page);
			Bitmap bmp = Bitmap.createBitmap(w, h, Bitmap.Config.ARGB_8888);
			try {
				bmp.eraseColor(Color.WHITE);
				p.render(bmp, null, null, PdfRenderer.Page.RENDER_MODE_FOR_DISPLAY);
				// ARGB_8888 is stored as R, G, B, A bytes.
				ByteBuffer buf = ByteBuffer.allocate(w * h * 4);
				bmp.copyPixelsToBuffer(buf);
				return buf.array();
			} finally {
				bmp.recycle();
				p.close();
			}
		}
	}

	public static synchronized void close(int id) {
		PdfRenderer r = docs.remove(id);
		if (r != null) {
			r.close();
		}
	}

	private static synchronized PdfRenderer doc(int id) throws Exception {
		PdfRenderer r = docs.get(id);
		if (r == null) {
			throw new Exception("the document is closed");
		}
		return r;
	}
}
