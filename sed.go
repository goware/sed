package sed

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type File struct {
	FilePath string // path to file
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

func (f *File) Replace(fromString, toString string) (err error) {
	// read file
	tmpfile := fmt.Sprintf("tmp-sed-%d", time.Now().UnixNano())
	input, err := os.OpenFile(f.FilePath, os.O_RDWR, 0777)
	if err != nil {
		return err
	}
	output, err := os.OpenFile(tmpfile, os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			input.Close()
			output.Close()
			os.Remove(tmpfile)
			return
		}
		switch f.dryRun {
		case true:
			input.Close()
			output.Close()
			os.Remove(tmpfile)
		case false:
			input.Close()
			os.Remove(f.FilePath)
			output.Close()
			os.Rename(tmpfile, f.FilePath)
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
