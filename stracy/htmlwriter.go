package main

import (
	"io"
	"os"
	"path/filepath"
)

type HTMLWriter struct {
	w    io.WriteCloser
	path string
}

func NewHTMLWriter(path string) (*HTMLWriter, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}

	w := &HTMLWriter{
		w:    out,
		path: path,
	}
	return w, nil
}

func (w *HTMLWriter) Close() error {
	return w.w.Close()
}

func (w *HTMLWriter) WriteString(s string) error {
	_, err := w.w.Write([]byte(s))
	return err
}
