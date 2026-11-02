//go:build !android && !darwin

package pdfpages

const supported = false

type platformDoc struct{}

func open(string) (platformDoc, int, error) { return platformDoc{}, 0, ErrNotSupported }

func (platformDoc) pageSize(int) (float64, float64, error) { return 0, 0, ErrNotSupported }
func (platformDoc) render(int, int, int) ([]byte, error)   { return nil, ErrNotSupported }
func (platformDoc) close() error                           { return nil }
