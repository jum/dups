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
	"runtime"
	"sort"
	"sync/atomic"
	"syscall"
)

var (
	root   *string = flag.String("root", "test", "root dir for dup check")
	delete *bool   = flag.Bool("delete", false, "do delete the longest dups")
	emptydir *bool = flag.Bool("emptydir", false, "do delete empty directories as well")
	ncpu   *int    = flag.Int("ncpu", runtime.NumCPU(), "number of cpu's to use")
)

const DEBUG = true

func debug(format string, a ...interface{}) {
	if DEBUG {
		fmt.Printf(format, a...)
	}
}

type StringLenSorter []string

func (p StringLenSorter) Len() int           { return len(p) }
func (p StringLenSorter) Less(i, j int) bool { return len(p[i]) >= len(p[j]) }
func (p StringLenSorter) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type HashResult struct {
	FileName string
	Err error
	Hash []byte
}

func main() {
	flag.Parse()
	debug("ncpu %v\n", *ncpu)
	runtime.GOMAXPROCS(*ncpu)
	tree := make(map[int64][]string)
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
		sz := info.Size()
		tree[sz] = append(tree[sz], path)
		return nil
	})
	if err != nil {
		panic(err.Error())
	}
	debug("tree %#v\n", tree)
	for sz, flist := range tree {
		if len(flist) < 2 {
			continue
		}
		debug("File with size %v:\n", sz)
		// we have a list of files with the same size, possibly candiates with equal content.
		hashtree := make(map[[sha1.Size]byte][]string)
		var numOutstanding int32
		done := make(chan *HashResult, *ncpu)
		req := make(chan *HashResult, *ncpu)
		for i := 0; i < runtime.NumCPU(); i++ {
			go func() {
				for r := <-req; r != nil; {
					debug("req path %v\n", r.FileName)
					f, err := os.Open(r.FileName)
					if err != nil {
						//panic(err.Error())
						// continue if file can not be opened
						r.Err = err
						done <- r
						return
					}
					var reader *bufio.Reader
					//reader, err = bufio.NewReaderSize(f, 4*1024*1024)
					reader = bufio.NewReader(f)
					hash := sha1.New()
					_, err = io.Copy(hash, reader)
					if err != nil {
						panic(err.Error())
					}
					f.Close()
					r.Hash = hash.Sum(nil)
					debug("done %#v\n", r)
					done <- r
				}
			} ()
		}
		var killResultFetcher = make(chan int)
		go func() {
			for {
				select {
				case res := <-done:
					if res.Err != nil {
						fmt.Fprintf(os.Stderr, "%v: %v\n", res.FileName, res.Err)
					} else {
						var sum [sha1.Size]byte
						copy(sum[:], res.Hash)
						debug("path %v, hash %v\n", res.FileName, hex.EncodeToString(sum[:]))
						hashtree[sum] = append(hashtree[sum], res.FileName)
					}
					_ = atomic.AddInt32(&numOutstanding, -1)
				case <-killResultFetcher:
					return
				}
			}
		}()
		for _, path := range flist {
			_ = atomic.AddInt32(&numOutstanding, 1)
			req <- &HashResult{FileName: path}
		}
		close(req)
		killResultFetcher <- 1
		for atomic.AddInt32(&numOutstanding, -1) >= 0 {
			res := <-done
			if res.Err != nil {
				fmt.Fprintf(os.Stderr, "%v: %v\n", res.FileName, res.Err)
			} else {
				var sum [sha1.Size]byte
				copy(sum[:], res.Hash)
				debug("path %v, hash %v\n", res.FileName, hex.EncodeToString(sum[:]))
				hashtree[sum] = append(hashtree[sum], res.FileName)
			}
		}
		for sum, flist := range hashtree {
			if len(flist) < 2 {
				continue
			}
			sort.Sort(StringLenSorter(flist))
			fmt.Printf("files with hash %v:\n%v\n", hex.EncodeToString(sum[:]), flist)
			for _, file := range flist[:len(flist)-1] {
				fmt.Printf("Deleting dup %v\n", file)
				if *delete {
					debug("really del %v\n", file)
					err := os.Remove(file)
					if err != nil {
						panic(err.Error())
					} else {
						if *emptydir {
							parent := filepath.Dir(file)
							debug("attempt del dir %v\n", parent)
							err := os.Remove(parent)
							if err != nil {
								if e, ok := err.(*os.PathError); ok {
									if e.Err == syscall.ENOTEMPTY {
										debug("%v is not empty\n", parent)
										continue
									}
								}
								panic(err.Error())
							}
						}
					}
				}
			}
		}
	}
}
