package pdfpages

import (
	"strings"

	"github.com/xmhrnx/giode/android"
)

const supported = true

// class is the app's Java renderer, android/java/com/xmhrnx/doclens/PdfPages.java,
// compiled in by the giode CLI (android.java in giode.toml).
const class = "com/xmhrnx/doclens/PdfPages"

type platformDoc struct{ id int }

func open(path string) (platformDoc, int, error) {
	id, err := android.CallInt(class, "open", "(Ljava/lang/String;)I", path)
	if err != nil {
		if strings.Contains(err.Error(), "password") {
			return platformDoc{}, 0, ErrPassword
		}
		return platformDoc{}, 0, err
	}
	n, err := android.CallInt(class, "pageCount", "(I)I", id)
	if err != nil {
		android.CallVoid(class, "close", "(I)V", id)
		return platformDoc{}, 0, err
	}
	return platformDoc{id: id}, n, nil
}

func (d platformDoc) pageSize(i int) (float64, float64, error) {
	w, err := android.CallInt(class, "pageWidth", "(II)I", d.id, i)
	if err != nil {
		return 0, 0, err
	}
	h, err := android.CallInt(class, "pageHeight", "(II)I", d.id, i)
	if err != nil {
		return 0, 0, err
	}
	return float64(w), float64(h), nil
}

func (d platformDoc) render(i, w, h int) ([]byte, error) {
	return android.CallBytes(class, "render", "(IIII)[B", d.id, i, w, h)
}

func (d platformDoc) close() error {
	return android.CallVoid(class, "close", "(I)V", d.id)
}
