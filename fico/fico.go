package main

import (
	"os"
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

func colorize(c int, txt string) string {
	return fmt.Sprintf("\x1b[01;3%dm%s\x1b[00m", c, txt)
}

// don't waste time on cleaning
func pjoin(elem ...string) string {
	return strings.Join(elem, "/")
}

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

type jspec struct {
	prefix int
	path string
}

func (js *jspec) join(de *dir.Dirent) *jspec {
	return &jspec{js.prefix, pjoin(js.path, de.Name)}
}

func (js *jspec) match(pat string) bool {
	matching, _ := filepath.Match(pat, js.path[js.prefix:])
	if *debug { fmt.Println("match", pat, js.path[js.prefix:], matching) }
	return matching
}

func countdir(js *jspec, c chan *jspec) int {
	f, err := os.Open(js.path)
	if err != nil {
		if *fuzzy && relaxerrno(err, syscall.ENOENT) {
			fmt.Fprintln(os.Stderr, "[W]", js.path, "got error with", err)
			return 0
		} else {
			log.Fatal(err)
		}
	}
	des, err := dir.Readdir(f, *hint)
	f.Close()
	if err != nil {
		if *fuzzy && relaxerrno(err, syscall.ENOENT, syscall.ENOTDIR) {
			fmt.Fprintln(os.Stderr, "[W]", js.path, "got error with", err)
			return 0
		} else {
			log.Fatal(err)
		}
	}

	fc := 0
	for _, de := range des {
		if de.Type == dir.DT_UNKNOWN {
			if *stat {
				de.Type, err = dir.Modestat(js.join(de).path)
				if err != nil {
					if *fuzzy && relaxerrno(err, syscall.ENOENT) {
						fmt.Fprintln(os.Stderr, "[W]", js.join(de).path,
							     "got error with", err)
						continue
					} else {
						log.Fatal(err)
					}
				}
			} else {
				log.Fatalf("got no filetype info: %s", js.join(de).path)
			}
		}
		switch de.Type {
		case dir.DT_REG:
			if *filter != "" && *filterfiles && js.join(de).match(*filter) {
				continue
			}
			fc += 1
		case dir.DT_DIR:
			jsn := js.join(de)
			if *filter != "" && jsn.match(*filter) { continue }
			c <- jsn
		}
	}

	if *debug { fmt.Println(js.path, fc) }

	return fc
}

func counter(cwi, cwo chan *jspec, cc chan int) {
	count := 0
	for {
		js := <-cwi
		if js == nil {
			cc <- count
			return
		}
		count += countdir(js, cwo)
		cwo <- nil
	}
}

func countdirs(dpaths []string) int {
	cwi := make(chan *jspec, 3 * *hint / *workers)
	cwo := make(chan *jspec, 3 * *hint / *workers)
	cc := make(chan int)
	for i := 0; i < *workers; i++ {
		go counter(cwi, cwo, cc)
	}

	for _, dp := range dpaths {
		cwi <- &jspec{len(dp) + 1, dp}
	}

	q := make([]*jspec, 0, 3 * *hint)
	for n := len(dpaths); n != 0; {
		js := <-cwo
		if *debug { fmt.Println("jobstat", js, n, q, len(cwi), cap(cwi)) }
		if js == nil {
			n -= 1
			if len(q) > 0 {
				js = q[0]
				q = q[1:]
			}
		}
		if js != nil {
			select {
			case cwi <- js:
				n += 1
			default:
				q = append(q, js)
			}
		}
	}

	count := 0
	for i := 0; i < *workers; i++ {
		cwi <- nil
		count += <-cc
	}

	return count
}

func next(n, m , t int) int {
	n = t - (t % n) + n
	m = t - (t % m) + m
	if (n < m) {
		return n
	}
	return m
}


const usage = `Intuit mon.sh clone

%s [options] [targets...]
options:
`

type scanrec struct {
	Tstart, Tend time.Time
	Trel, Files int
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		flag.PrintDefaults()
	}
	debug = flag.Bool("debug", false, "debug mode")
	hint = flag.Int("hint", 256, "filenumber hint")
	workers = flag.Int("workers", runtime.NumCPU() + 1, "number of workers")
	filter = flag.String("filter", "", "glob patterns to exclude")
	filterfiles = flag.Bool("filterfiles", false, "apply filter to files, too")
	stat = flag.Bool("stat", false, "salvage missing dirent type by lstat")
	fuzzy = flag.Bool("fuzzy", false, "tolerate fs fuzzines (errors due to ongoing changes)")
	scan := flag.Int("scanby", 10, "interval to scan by")
	rec  := flag.Int("recby", 20, "interval to record by")
	turns := flag.Int("turns", 0, "number of iterations")
	logp := flag.String("logf", "", "log file")
	flag.Parse()

	runtime.GOMAXPROCS(*workers)

	var logf *os.File
	var err error
	if *logp != "" {
		logf, err = os.OpenFile(*logp, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil { log.Fatal(err) }
	}

	if *filter != "" {
		// XXX have to match against non-empty string to test pattern
		_, err := filepath.Match(*filter, "x")
		if err != nil { log.Fatal(err) }
	}

	targets := flag.Args()
	if len(targets) == 0 {
		targets = []string { "." }
	}
	for i, _ := range targets {
		targets[i] = path.Clean(targets[i])
	}

	for t := 0;; {
		*turns -= 1
		go func(t, tr int) {
			tn0 := time.Now()
			count := countdirs(targets)
			tn1 := time.Now()
			m := fmt.Sprintf("%3d %6d", t, count)
			if t % *rec == 0 {
				m = colorize(RED, m)
			}
			fmt.Println(m)
			if logf != nil {
				jlog, err := json.Marshal(scanrec{ tn0, tn1, t, count})
				if err != nil { log.Fatal(err) }
				jlog = append(jlog, '\n')
				_, err = logf.Write(jlog)
				if err != nil { log.Fatal(err) }
			}
			if tr == 0 { os.Exit(0) }
		}(t, *turns)
		to := t
		t = next(*scan, *rec, t)
		time.Sleep(time.Duration(t - to) * time.Second)
	}
}
