package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var files []string
var fileChan = make(chan string)
var output = make(chan string)

func main() {

	// flags
	var path string
	flag.StringVar(&path, "f", "", "specify the file or directory")

	// concurrency flag
	var concurrency int
	flag.IntVar(&concurrency, "c", 20, "set the concurrency level")

	// process the flags
	flag.Parse()

	// if file was not provided in the parsed flags, then first parsed arg is assumed as the file
	if path == "" {
		path = flag.Arg(0)
	}

	// TODO: add domain / web root detection via apache config file parsing or some other way

	// map files
	mapFiles(path)

	// backup files
	// Create output file
	out, err := os.Create("backup.tar.gz")
	if err != nil {
		log.Fatalln("Error writing archive:", err)
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {

		}
	}(out)
	// make archive
	err = createArchive(files, out)
	if err != nil {
		return
	}

	// work group for changing perms
	var processWG sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		processWG.Add(1)
		// go routine
		go func() {
			for r := range fileChan {
				// detect if file or dir
				maybeDir, err := detectDir(r)
				if err != nil {
					log.Fatalln(err)
				}
				// if dir do dir method else file method
				if maybeDir {
					changeDir(r)
				} else {
					changeFile(r)
				}
			}
			processWG.Done()
		}()
	}
	var outputWG sync.WaitGroup
	outputWG.Add(1)
	go func() {
		for o := range output {
			fmt.Println(o + " Permissions corrected")
		}
		outputWG.Done()
	}()
	// close up channels when done processing
	func() {
		processWG.Wait()
		close(fileChan)
		outputWG.Wait()
		close(output)
	}()

	//sleep just a bit
	time.Sleep(1 * time.Second)
}

func detectDir(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.IsDir(), err
}

// function for mapping files, adding the files to working channel and []string for backup
func mapFiles(path string) {
	err := filepath.Walk(path, func(pathBoi string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}
		fileChan <- pathBoi
		files = append(files, pathBoi)
		return nil
	})
	if err != nil {
		fmt.Println(err)
	}
}

func changeFile(path string) {
	// TODO: add ownership change
	err := os.Chmod(path, 0644)
	if err != nil {
		log.Fatal(err)
	}
	output <- path
}

func changeDir(path string) {
	// TODO: add ownership change
	err := os.Chmod(path, 0755)
	if err != nil {
		log.Fatal(err)
	}
	output <- path
}

func createArchive(files []string, buf io.Writer) error {
	// Create new Writers for gzip and tar, and chain them
	gw := gzip.NewWriter(buf)
	defer func(gw *gzip.Writer) {
		err := gw.Close()
		if err != nil {

		}
	}(gw)
	tw := tar.NewWriter(gw)
	defer func(tw *tar.Writer) {
		err := tw.Close()
		if err != nil {

		}
	}(tw)

	// Iterate over files and add them to the tar archive
	for _, file := range files {
		err := addToArchive(tw, file)
		if err != nil {
			return err
		}
	}

	return nil
}

func addToArchive(tw *tar.Writer, filename string) error {
	// Open the file which will be written into the archive
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	// Get FileInfo about our file providing file size, mode, etc.
	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Create a tar Header from the FileInfo data
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	header.Name = filename

	// Write file header to the tar archive
	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}

	// Copy file content to tar archive
	_, err = io.Copy(tw, file)
	if err != nil {
		return err
	}

	return nil
}
