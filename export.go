package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ExportFileData struct {
	Filename string     `json:"filename"`
	FileType string     `json:"file_type"`
	Info     FileInfo   `json:"info"`
	Pages    []PageDetail `json:"pages"`
}

func runExportJSON(filenames []string) error {
	result := make([]ExportFileData, 0, len(filenames))

	for _, arg := range filenames {
		// Support "name=path" format for custom display names
		fn := arg
		displayName := ""
		if idx := strings.Index(arg, "="); idx > 0 {
			displayName = arg[:idx]
			fn = arg[idx+1:]
		}

		fi, err := os.Stat(fn)
		if err != nil {
			return fmt.Errorf("cannot stat %s: %w", fn, err)
		}
		totalPages := int(fi.Size() / PageSize)

		fileType := "unknown"
		pages := make([]PageSummary, 0, totalPages)
		details := make([]PageDetail, 0, totalPages)

		for i := 0; i < totalPages; i++ {
			pg, err := ReadPage(fn, i)
			if err != nil {
				pages = append(pages, PageSummary{PageNum: i, Type: "error"})
				details = append(details, PageDetail{PageNum: i, Type: "error"})
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
			details = append(details, buildPageDetail(pg))
		}

		name := displayName
		if name == "" {
			name = filepath.Base(fn)
		}

		info := FileInfo{
			Filename:   name,
			TotalPages: totalPages,
			FileType:   fileType,
			Pages:      pages,
		}

		result = append(result, ExportFileData{
			Filename: name,
			FileType: fileType,
			Info:     info,
			Pages:    details,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(result)
}
