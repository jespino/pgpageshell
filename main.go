package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/chzyer/readline"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: pgpageshell <postgres-data-file>\n")
		fmt.Fprintf(os.Stderr, "  Inspect PostgreSQL heap/index data files page by page.\n")
		os.Exit(1)
	}

	filename := os.Args[1]

	fi, err := os.Stat(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	totalPages := int(fi.Size() / PageSize)
	if fi.Size()%PageSize != 0 {
		fmt.Fprintf(os.Stderr, "Warning: file size %d is not a multiple of %d\n", fi.Size(), PageSize)
	}

	// Detect file type from page 0
	fileType := "unknown"
	if totalPages > 0 {
		pg0, err := ReadPage(filename, 0)
		if err == nil {
			fileType = pg0.Detected.String()
		}
	}

	fmt.Printf("pgpageshell - PostgreSQL Page Inspector\n")
	fmt.Printf("File: %s (%d bytes, %d pages, detected: %s)\n", filename, fi.Size(), totalPages, fileType)
	fmt.Println()
	printHelp()
	fmt.Println()

	currentPage := 0
	var page *Page

	if totalPages > 0 {
		page, err = ReadPage(filename, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading page 0: %v\n", err)
		} else {
			fmt.Printf("[page 0 loaded, type: %s]\n", page.Detected)
		}
	}

	completer := readline.NewPrefixCompleter(
		readline.PcItem("page"),
		readline.PcItem("cat"),
		readline.PcItem("format"),
		readline.PcItem("info"),
		readline.PcItem("data"),
		readline.PcItem("pages"),
		readline.PcItem("help"),
		readline.PcItem("quit"),
		readline.PcItem("exit"),
	)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:            fmt.Sprintf("pgpageshell(page %d)> ", currentPage),
		HistoryFile:       "/tmp/pgpageshell_history",
		AutoComplete:      completer,
		InterruptPrompt:   "^C",
		EOFPrompt:         "quit",
		HistorySearchFold: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing readline: %v\n", err)
		os.Exit(1)
	}
	defer rl.Close()

	for {
		rl.SetPrompt(fmt.Sprintf("pgpageshell(page %d)> ", currentPage))
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			continue
		}
		if err == io.EOF {
			fmt.Println("Bye.")
			return
		}
		if err != nil {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "quit", "exit", "q":
			fmt.Println("Bye.")
			return

		case "help", "h", "?":
			printHelp()

		case "page", "p":
			if len(parts) < 2 {
				fmt.Printf("Current page: %d (of %d, type: %s)\n", currentPage, totalPages, page.Detected)
				continue
			}
			n, err := strconv.Atoi(parts[1])
			if err != nil || n < 0 || n >= totalPages {
				fmt.Printf("Invalid page number. Valid range: 0-%d\n", totalPages-1)
				continue
			}
			page, err = ReadPage(filename, n)
			if err != nil {
				fmt.Printf("Error reading page %d: %v\n", n, err)
				continue
			}
			currentPage = n
			fmt.Printf("[page %d loaded, type: %s]\n", n, page.Detected)

		case "cat", "c":
			if page == nil {
				fmt.Println("No page loaded.")
				continue
			}
			CmdCat(page)

		case "format", "f":
			if page == nil {
				fmt.Println("No page loaded.")
				continue
			}
			CmdFormat(page)

		case "info", "i":
			if page == nil {
				fmt.Println("No page loaded.")
				continue
			}
			CmdInfo(page)

		case "data", "d":
			if page == nil {
				fmt.Println("No page loaded.")
				continue
			}
			CmdData(page)

		case "pages":
			for i := 0; i < totalPages; i++ {
				pg, err := ReadPage(filename, i)
				if err != nil {
					fmt.Printf("  Page %3d: error: %v\n", i, err)
					continue
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
				fmt.Printf("  Page %3d: type=%-7s items=%-4d free=%-5d special=%-4d\n",
					i, pg.Detected, numItems, freeSpace, pg.SpecialSize())
			}

		default:
			fmt.Printf("Unknown command: %s (type 'help' for commands)\n", cmd)
		}
	}
}

func printHelp() {
	fmt.Println("Commands:")
	fmt.Println("  page <n>    - select page number (0-based)")
	fmt.Println("  cat         - hex dump of current page")
	fmt.Println("  format      - ASCII art page layout")
	fmt.Println("  info        - page header and special region details")
	fmt.Println("  data        - line pointers and tuple data")
	fmt.Println("  pages       - list all pages with summary")
	fmt.Println("  help        - show this help")
	fmt.Println("  quit/exit   - exit")
}
