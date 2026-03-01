package main

import (
	"context"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails-bound struct. Its exported methods are callable from the frontend.
type App struct {
	ctx   context.Context
	files []AppFile
}

type AppFile struct {
	Filename   string
	TotalPages int
}

func NewApp(filenames []string) (*App, error) {
	files := make([]AppFile, 0, len(filenames))
	for _, fn := range filenames {
		fi, err := os.Stat(fn)
		if err != nil {
			return nil, fmt.Errorf("cannot stat %s: %w", fn, err)
		}
		totalPages := int(fi.Size() / PageSize)
		if fi.Size()%PageSize != 0 {
			fmt.Fprintf(os.Stderr, "Warning: %s size %d is not a multiple of %d\n", fn, fi.Size(), PageSize)
		}
		files = append(files, AppFile{Filename: fn, TotalPages: totalPages})
	}
	return &App{files: files}, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetFiles returns the list of loaded files.
func (a *App) GetFiles() []FileEntry {
	entries := make([]FileEntry, len(a.files))
	for i, f := range a.files {
		entries[i] = FileEntry{Index: i, Filename: f.Filename, TotalPages: f.TotalPages}
	}
	return entries
}

// GetFileInfo returns page summaries for a specific file.
func (a *App) GetFileInfo(fileIdx int) (*FileInfo, error) {
	if fileIdx < 0 || fileIdx >= len(a.files) {
		return nil, fmt.Errorf("invalid file index: %d", fileIdx)
	}
	f := a.files[fileIdx]

	fileType := "unknown"
	pages := make([]PageSummary, 0, f.TotalPages)

	for i := 0; i < f.TotalPages; i++ {
		pg, err := ReadPage(f.Filename, i)
		if err != nil {
			pages = append(pages, PageSummary{PageNum: i, Type: "error"})
			continue
		}
		if i == 0 {
			fileType = pg.Detected.String()
		}
		h := &pg.Header
		numItems := 0
		if h.Lower > PageHeaderSize {
			numItems = int(h.Lower-PageHeaderSize) / ItemIdSize
		}
		freeSpace := 0
		if h.Upper > h.Lower {
			freeSpace = int(h.Upper - h.Lower)
		}
		pages = append(pages, PageSummary{
			PageNum:     i,
			Type:        pg.Detected.String(),
			NumItems:    numItems,
			FreeSpace:   freeSpace,
			SpecialSize: pg.SpecialSize(),
		})
	}

	return &FileInfo{
		Filename:   f.Filename,
		TotalPages: f.TotalPages,
		FileType:   fileType,
		Pages:      pages,
	}, nil
}

// OpenFile opens a native file dialog and adds the selected file to the list.
// Returns the updated file list, or an error if the file is invalid.
func (a *App) OpenFile() ([]FileEntry, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open PostgreSQL Data File",
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		// User cancelled
		return a.GetFiles(), nil
	}

	// Check if already open
	for _, f := range a.files {
		if f.Filename == path {
			return a.GetFiles(), nil
		}
	}

	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot stat %s: %w", path, err)
	}
	totalPages := int(fi.Size() / PageSize)
	if fi.Size()%PageSize != 0 {
		fmt.Fprintf(os.Stderr, "Warning: %s size %d is not a multiple of %d\n", path, fi.Size(), PageSize)
	}

	a.files = append(a.files, AppFile{Filename: path, TotalPages: totalPages})
	return a.GetFiles(), nil
}

// CloseFile removes a file from the list by index.
// Returns the updated file list.
func (a *App) CloseFile(fileIdx int) ([]FileEntry, error) {
	if fileIdx < 0 || fileIdx >= len(a.files) {
		return nil, fmt.Errorf("invalid file index: %d", fileIdx)
	}
	a.files = append(a.files[:fileIdx], a.files[fileIdx+1:]...)
	return a.GetFiles(), nil
}

// GetPageDetail returns full page detail for a specific file and page number.
func (a *App) GetPageDetail(fileIdx int, pageNum int) (*PageDetail, error) {
	if fileIdx < 0 || fileIdx >= len(a.files) {
		return nil, fmt.Errorf("invalid file index: %d", fileIdx)
	}
	f := a.files[fileIdx]
	if pageNum < 0 || pageNum >= f.TotalPages {
		return nil, fmt.Errorf("invalid page number: %d", pageNum)
	}

	page, err := ReadPage(f.Filename, pageNum)
	if err != nil {
		return nil, err
	}

	detail := buildPageDetail(page)
	return &detail, nil
}
