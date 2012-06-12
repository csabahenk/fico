// Copyright 2012 Csaba Henk. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.


package main

import (
	"os"
	"io"
	"log"
	"fmt"
	"path"
	"flag"
	"runtime"
	"strings"
	"time"
	"encoding/json"
	"path/filepath"
	"syscall"

	"dir"
)

const (
	BLACK = iota
	RED
	GREEN
	YELLOW
	BLUE
	PURPLE
	CYAN
	WHITE
)

var hint *int
var debug *bool
var workers *int
var filter *string
var filterfiles *bool
var stat *bool
var fuzzy *bool
var dump *bool
var dump_fullpath *bool
var dump_sep byte


func colorize(c int, txt string) string {
	return fmt.Sprintf("\x1b[01;3%dm%s\x1b[00m", c, txt)
}


// filepath.Join replacement: don't waste time on cleaning
func pjoin(elem ...string) string {
	return strings.Join(elem, "/")
}


// churn out the errno from err and match against list
// of tolerated errnos
func relaxerrno(err error, errnos ...syscall.Errno) bool {
	var xerrno syscall.Errno
	switch xerr := err.(type) {
	case *os.PathError:
		xerrno = xerr.Err.(syscall.Errno)
	case *os.SyscallError:
		xerrno = xerr.Err.(syscall.Errno)
	case syscall.Errno:
		xerrno = xerr
	default:
		return false
	}
	for _, errno := range errnos {
		if xerrno == errno { return true }
	}
	return false
}

// job specifier and methods

type jspec struct {
	runid int
	prefix int   // path up to this index not considered when matching
	path string
}

func (js *jspec) join(de *dir.Dirent) *jspec {
	jsn := new(jspec)
	*jsn = *js
	jsn.path = pjoin(js.path, de.Name)
	return jsn
}

func (js *jspec) match(pat string) bool {
	matching, _ := filepath.Match(pat, js.path[js.prefix:])
	if *debug { fmt.Println("match", pat, js.path[js.prefix:], matching) }
	return matching
}


// concurrent file counting

func countdir(js *jspec, c chan *jspec) int {
	// get entries from js.path
	f, err := os.Open(js.path)
	if err != nil {
		if *fuzzy && relaxerrno(err, syscall.ENOENT) {
			fmt.Fprintln(os.Stderr, "[W]", js.path, "got error with", err)
			return 0
		} else {
			log.Fatal(js.path, ": ", err)
		}
	}
	des, err := dir.Readdir(f, *hint)
	f.Close()
	if err != nil {
		if *fuzzy && relaxerrno(err, syscall.ENOENT, syscall.ENOTDIR) {
			fmt.Fprintln(os.Stderr, "[W]", js.path, "got error with", err)
			return 0
		} else {
			log.Fatal(js.path, ": ", err)
		}
	}

	// process the entries
	fc := 0
	for _, de := range des {
		if de.Type == dir.DT_UNKNOWN && *stat {
			de.Type, err = dir.Modestat(js.join(de).path)
			if err != nil {
				if *fuzzy && relaxerrno(err, syscall.ENOENT) {
					fmt.Fprintln(os.Stderr, "[W]", js.join(de).path,
						     "got error with", err)
					continue
				} else {
					log.Fatal(js.join(de).path, ": ", err)
				}
			}
		}
		if *dump {
			var idx int
			if *dump_fullpath {
				idx = 0
			} else {
				idx = js.prefix
			}
			fmt.Printf("%d %d %s %s%c", js.runid, de.Ino, dir.Types[de.Type],
				   js.join(de).path[idx:], dump_sep)
		}
		switch de.Type {
		case dir.DT_UNKNOWN:
			if ! *dump {
				log.Fatal("got no filetype info: ", js.join(de).path)
			}
		case dir.DT_REG:
			if *filter != "" && *filterfiles && js.join(de).match(*filter) {
				continue
			}
			// count
			fc += 1
		case dir.DT_DIR:
			jsn := js.join(de)
			if *filter != "" && jsn.match(*filter) { continue }
			// job request sent back to scheduler
			c <- jsn
		}
	}

	if *debug { fmt.Println(js.path, fc) }

	return fc
}

// worker
func counter(cwi, cwo chan *jspec, cc chan int) {
	count := 0
	for {
		js := <-cwi
		if js == nil {
			// nil indicates end of run, send back result and bye
			cc <- count
			return
		}
		count += countdir(js, cwo)
		cwo <- nil
	}
}

// scheduler
func countdirs(dpaths []string, t int) int {
	// init and fire up workers
	cwi := make(chan *jspec, 3 * *hint / *workers)
	cwo := make(chan *jspec, 3 * *hint / *workers)
	cc := make(chan int)
	for i := 0; i < *workers; i++ {
		go counter(cwi, cwo, cc)
	}

	// send intial tasks
	for _, dp := range dpaths {
		cwi <- &jspec{t, len(dp) + 1, dp}
	}

	// make dynamic "overflow" buffer to handle channel saturation
	q := make([]*jspec, 0, 3 * *hint)
	// n: number of outstanding jobs
	for n := len(dpaths); n != 0; {
		// fetch new job req or termination report from workers
		js := <-cwo
		if *debug { fmt.Println("jobstat", js, n, q, len(cwi), cap(cwi)) }
		if js == nil {
			// termination report
			n -= 1
			// load reduced: if there are jobs stashed to overflow buf,
			// take one and give it a chance to get running
			if len(q) > 0 {
				js = q[0]
				q = q[1:]
			}
		}
		if js != nil {
			// trying to load job to a worker
			select {
			case cwi <- js:
				// succeeded, one more job in progress
				n += 1
			default:
				// failed, save job in overflow buf
				q = append(q, js)
			}
		}
	}

	if *dump {
		fmt.Printf("%d%c", t, dump_sep)
	}

	// harvest result
	count := 0
	for i := 0; i < *workers; i++ {
		cwi <- nil
		count += <-cc
	}

	return count
}


// utility func to get smallest number over t
// that is a multiple of some of vals
func next(t int, vals ...int) int {
	n := t + vals[0]
	for _, v := range vals {
		v = t - (t % v) + v
		if (v < n) { n = v }
	}
	return n
}


const usage = `FAST CONCURRENT TUNABLE fiLE coUNTER

%s [options] [targets...] [-- message]
options:
`

type scanrec struct {
	Tstart, Tend time.Time
	Trel, Files int
}

type scanhead struct {
	Tstart time.Time
	Args []string
}

type scanmessage struct {
	Tstart time.Time
	Message string
}

func writelog(logf io.Writer, j interface{}) {
	jlog, err := json.Marshal(j)
	if err != nil { log.Fatal(err) }
	jlog = append(jlog, '\n')
	_, err = logf.Write(jlog)
	if err != nil { log.Fatal("error writing to logfile: ", err) }
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	debug = flag.Bool("debug", false, "debug mode")
	hint = flag.Int("hint", 256, "directory branchiness hint (hint on avg. no. of entires in dirs)")
	workers = flag.Int("workers", runtime.NumCPU() + 1, "number of workers")
	filter = flag.String("filter", "", "glob pattern to exclude " +
			     "(matching done relatively from targets, matching dirs are not entered)")
	filterfiles = flag.Bool("filterfiles", false, "apply 'filter' to file counting, too")
	stat = flag.Bool("stat", false, "salvage missing dirent type by falling back to lstat")
	fuzzy = flag.Bool("fuzzy", false, "tolerate fs fuzzines (errors due to ongoing changes)")
	dump = flag.Bool("dump", false, "dump entries instead of counting them")
	dump_fullpath = flag.Bool("dumpfullpath", false, "on dumping, do not strip off path to target")
	dump_zero := flag.Bool("dump0", false, "on dumping, separate entries by zero byte")
	scan := flag.Int("scan", 10, "interval to scan by")
	hili  := flag.Int("hili", 20, "interval to show highlighted scan result")
	turns := flag.Int("turns", 0, "number of iterations (≤0 means infinite)")
	flimit := flag.Int("flimit", 0, "run 'till this number of files is reached (≤0 means no limit)")
	logp := flag.String("logf", "", "log file")
	logappend := flag.Bool("logappend", false, "append to previously existing logfile")
	logcont := flag.Bool("logcont", false, "continue logging with relative time of earlier logs" +
			     " (implies logappend)")
	flag.Parse()
	oargs := os.Args
	nargs := flag.Args()

	runtime.GOMAXPROCS(*workers)

	var logf *os.File
	var err error
	toff := 0
	if *logp != "" {
		oflags := os.O_RDWR|os.O_CREATE
		if !(*logappend || *logcont) { oflags |= os.O_TRUNC }
		logf, err = os.OpenFile(*logp, oflags, 0600)
		if err != nil { log.Fatal("cannot open logfile ", *logp, ": ", err) }
		if *logcont {
			jdec := json.NewDecoder(logf)
			var je map[string]interface{}
			for {
				var jex map[string]interface{}
				err = jdec.Decode(&jex)
				if err == io.EOF { break }
				if err != nil { log.Fatal(err) }
				je = jex
			}
			// ducktyping on scanrec
			if _, ok := je["Trel"]; ok {
				// re-decode the json to scanrec
				var jsr scanrec
				jblob, err := json.Marshal(je)
				if err != nil { log.Fatal(err) }
				err = json.Unmarshal(jblob, &jsr)
				if err != nil { log.Fatal(err) }
				toff = jsr.Trel + int(time.Since(jsr.Tstart).Seconds())
			}
		} else if *logappend {
			logf.Seek(0, os.SEEK_END)
		}
		msg := ""
		for i,a := range(nargs) {
			if a == "--" {
				msg = strings.Join(nargs[i+1:], " ")
				oargs = oargs[:len(oargs) - len(nargs) + i]
				nargs = nargs[:i]
				break
			}
		}
		writelog(logf, scanhead{ time.Now(), oargs })
		if msg != "" { writelog(logf, scanmessage{ time.Now(), msg }) }
	}

	if *filter != "" {
		// XXX have to match against non-empty string to test pattern
		_, err := filepath.Match(*filter, "x")
		if err != nil { log.Fatal(err) }
	}

	if *dump_zero {
		dump_sep = '\x00'
	} else {
		dump_sep = '\n'
	}

	targets := nargs
	if len(targets) == 0 {
		targets = []string { "." }
	}
	for i, _ := range targets {
		targets[i] = path.Clean(targets[i])
	}

	for t := toff;; {
		*turns -= 1
		go func(t, tr int) {
			tn0 := time.Now()
			count := countdirs(targets, t)
			tn1 := time.Now()
			if !*dump {
				m := fmt.Sprintf("%3d %6d", t, count)
				if t % *hili == 0 {
					m = colorize(RED, m)
				}
				fmt.Println(m)
			}
			if logf != nil {
				writelog(logf, scanrec{ tn0, tn1, t, count})
			}
			if tr == 0 || (*flimit > 0 && count >= *flimit) { os.Exit(0) }
		}(t, *turns)
		to := t
		t = next(t, *scan, *hili)
		time.Sleep(time.Duration(t - to) * time.Second)
	}
}
