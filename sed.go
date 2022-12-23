package sed

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type File struct {
	FilePath    string // path to file
	TmpFilePath string // path to tmp file
	// reader
	// writer
	Replacements map[string]struct {
		Original string
		New      string
	} // map of replacements
	dryRun bool // dry run
}

func NewFile(filePath string, dryRun bool) *File {
	return &File{
		FilePath: filePath,
		dryRun:   dryRun,
		Replacements: make(map[string]struct {
			Original string
			New      string
		}),
	}
}

func (f *File) Replace(fromString, toString string) error {
	return f.replace(fromString, toString, false)
}

func (f *File) ReplaceOnlyCreateTmpFile(fromString, toString string) error {
	return f.replace(fromString, toString, true)
}

func (f *File) replace(fromString, toString string, onlyCreateTmpFile bool) (err error) {
	// read file

	f.TmpFilePath = fmt.Sprintf("%s/tmp-sed-%d", filepath.Dir(f.FilePath), time.Now().UnixNano())
	input, err := os.OpenFile(f.FilePath, os.O_RDWR, 0777)
	if err != nil {
		return err
	}
	output, err := os.OpenFile(f.TmpFilePath, os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			input.Close()
			output.Close()
			os.Remove(f.TmpFilePath)
			return
		}
		if onlyCreateTmpFile {
			input.Close()
			output.Close()
			return
		}

		switch f.dryRun {
		case true:
			input.Close()
			output.Close()
			os.Remove(f.TmpFilePath)
		case false:
			input.Close()
			os.Remove(f.FilePath)
			output.Close()
			os.Rename(f.TmpFilePath, f.FilePath)
		}
	}()
	reader := bufio.NewReader(input)

	lineNum := 1
	var line string
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				return err
			}
		}
		if strings.Contains(line, fromString) {
			// add to replacements
			newString := strings.Replace(line, fromString, toString, -1)
			f.Replacements[fmt.Sprintf("%d", lineNum)] = struct {
				Original string
				New      string
			}{Original: line,
				New: newString}
			// write to output
			output.WriteString(newString)
		} else {
			output.WriteString(line)
		}

		if err != nil {
			if err == io.EOF {
				break
			}
		}
		lineNum++
	}
	return nil
}
