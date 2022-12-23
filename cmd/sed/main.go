package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/goware/pp"
	"github.com/goware/sed"
)

const (
	sedVersion = "0.0.1"
)

var (
	flags   = flag.NewFlagSet("sed", flag.ExitOnError)
	dryRun  = flags.Bool("dryrun", false, "dry run, only diffs are shown and no files are modified")
	help    = flags.Bool("h", false, "print help")
	version = flags.Bool("version", false, "print version")
)

func main() {
	flags.Usage = usage
	flags.Parse(os.Args[1:])

	if *version {
		fmt.Println(sedVersion)
		os.Exit(0)
	}

	args := flags.Args()
	if len(args) == 0 || *help {
		flags.Usage()
		os.Exit(0)
	}

	if len(args) < 3 {
		fmt.Println("invalid number of arguments")
		flags.Usage()
		os.Exit(0)
	}

	fromString, toString, filepathstring := args[0], args[1], args[2]
	files, err := findAllFiles(filepathstring)
	if err != nil {
		log.Fatal(err)
	}

	wg := sync.WaitGroup{}
	errChan := make(chan error)
	fileChan := make(chan *sed.File)

	for _, file := range files {
		if file == nil {
			continue
		}

		wg.Add(1)
		go func(fromString, toString string, file *sed.File) {
			defer wg.Done()
			err := file.Replace(fromString, toString)
			if err != nil {
				errChan <- err
				return
			}
			fileChan <- file
		}(fromString, toString, file)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		wg.Wait()
		cancel()
	}()

	for {
		select {
		case err := <-errChan:
			log.Fatal(err)
		case file := <-fileChan:
			printDiffs(file)
		case <-ctx.Done():
			os.Exit(0)
		}
	}
}
func printDiffs(file *sed.File) {
	for l, r := range file.Replacements {
		fmt.Printf("@%s L%s \n", file.FilePath, l)
		pp.Green("+++ ").Green(r.New).Println()
		pp.Red("---").Red(r.Original).Println()
	}
}

func findAllFiles(filepathstring string) ([]*sed.File, error) {
	// for given regex find all files in directory
	matches, err := filepath.Glob(filepathstring)
	if err != nil {
		return nil, err
	}
	files := make([]*sed.File, len(matches))
	for i, match := range matches {
		fs, _ := os.Stat(match)
		if fs.IsDir() {
			continue
		}
		files[i] = sed.NewFile(match, *dryRun)
	}
	return files, nil
}

func usage() {
	fmt.Println(usagePrefix)
	flags.PrintDefaults()
}

var (
	usagePrefix = `Usage: sed [OPTIONS] FROMSTRING TOSTRING PATHSTRING
Examples:
	sed 'XXX' 'YYY' './foo.txt'
	sed 'XXX' 'YYY' './*.txt'
	sed 'XXX' 'YYY' './foo/*'
	sed -dryrun 'XXX' 'YYY' './foo/*'
Options:
`
)
