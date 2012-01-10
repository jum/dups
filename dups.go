// dups.go - a simple command line tool to find duplicate files in a
// directory tree.
//
// jum@anubis.han.de

package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

var (
	root   *string = flag.String("root", "test", "root dir for dup check")
	delete *bool   = flag.Bool("delete", false, "do delete the longest dups")
)

const DEBUG = false

func debug(format string, a ...interface{}) {
	if DEBUG {
		fmt.Printf(format, a...)
	}
}

type StringLenSorter []string

func (p StringLenSorter) Len() int           { return len(p) }
func (p StringLenSorter) Less(i, j int) bool { return len(p[i]) >= len(p[j]) }
func (p StringLenSorter) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func main() {
	flag.Parse()
	tree := make(map[string][]string)
	err := filepath.Walk(*root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			debug("walkFn path %v, err %v\n", path, err)
			return err
		}
		// skip over non-regular files
		if info.Mode() & os.ModeType != 0 {
			return nil
		}
		debug("walkFn path %v\n", path)
		f, err := os.Open(path)
		if err != nil {
			//panic(err.Error())
			// continue if file can not be opened
			fmt.Fprintf(os.Stderr, "%v: %v\n", path, err)
			return nil
		}
		defer f.Close()
		var reader *bufio.Reader
		//reader, err = bufio.NewReaderSize(f, 4*1024*1024)
		reader = bufio.NewReader(f)
		sha1 := sha1.New()
		_, err = io.Copy(sha1, reader)
		if err != nil {
			panic(err.Error())
		}
		hash := hex.EncodeToString(sha1.Sum(nil))
		debug("path %v, hash %v\n", path, hash)
		tree[hash] = append(tree[hash], path)
		return nil
	})
	if err != nil {
		panic(err.Error())
	}
	debug("tree %#v\n", tree)
	for hash, flist := range tree {
		if len(flist) < 2 {
			continue
		}
		sort.Sort(StringLenSorter(flist))
		fmt.Printf("files with hash %v:\n%v\n", hash, flist)
		for _, file := range flist[:len(flist)-1] {
			fmt.Printf("Deleting dup %v\n", file)
			if *delete {
				debug("really del %v\n", file)
				err := os.Remove(file)
				if err != nil {
					panic(err.Error())
				}
			}
		}
	}
}
