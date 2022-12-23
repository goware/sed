package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

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

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for _, file := range files {
		if file == nil {
			continue
		}

		wg.Add(1)
		go func(fromString, toString string, file *sed.File) {
			defer wg.Done()
			// lets only create temp files
			// when we are sure that we have a happy ending for all files,
			// we will replace the original file
			err := file.ReplaceOnlyCreateTmpFile(fromString, toString)
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

	filesForReplacements := make([]*sed.File, 0)

	panicCleanup := func() {
		// lets delete tmp files for all files in progress
		for _, file := range files {
			if file == nil {
				continue
			}
			if file.TmpFilePath != "" {
				// ensure that we close all files in case of panic
				file.Input.Close()
				file.Output.Close()
				os.Remove(file.TmpFilePath)
			}
		}
	}

	// happy ending
	defer func() {
		for _, file := range filesForReplacements {
			if file == nil {
				continue
			}
			if *dryRun {
				os.Remove(file.TmpFilePath)
				continue
			}
			os.Remove(file.FilePath)
			os.Rename(file.TmpFilePath, file.FilePath)
		}
	}()

	for {
		select {
		case err := <-errChan:
			panicCleanup()
			log.Fatal(err)
		case file := <-fileChan:
			filesForReplacements = append(filesForReplacements, file)
			printDiffs(file)
		case <-ctx.Done():
			return
		case <-sigs:
			panicCleanup()
			os.Exit(1)
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
