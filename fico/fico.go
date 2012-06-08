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

func colorize(c int, txt string) string {
	return fmt.Sprintf("\x1b[01;3%dm%s\x1b[00m", c, txt)
}

// don't waste time on cleaning
func pjoin(elem ...string) string {
	return strings.Join(elem, "/")
}

func countdir(dpath string, c chan string) int {
	f, err := os.Open(dpath)
	if err != nil { log.Fatal(err) }
	des, err := dir.Readdir(f, *hint)
	f.Close()
	if err != nil { log.Fatal(err) }

	fc := 0
	for _, de := range des {
		switch de.Type {
		case dir.DT_UNKNOWN:
			log.Fatalf("got no filetype info: %s", pjoin(dpath, de.Name))
		case dir.DT_REG:
			fc += 1
		case dir.DT_DIR:
			c <- pjoin(dpath, de.Name)
		}
	}

	if *debug { fmt.Println(dpath, fc) }

	return fc
}

func counter(cwi, cwo chan string, cc chan int) {
	count := 0
	for {
		dp := <-cwi
		if dp == "" {
			cc <- count
			return
		}
		count += countdir(dp, cwo)
		cwo <- ""
	}
}

func countdirs(dpaths []string) int {
	cwi := make(chan string, 3 * *hint / *workers)
	cwo := make(chan string, 3 * *hint / *workers)
	cc := make(chan int)
	for i := 0; i < *workers; i++ {
		go counter(cwi, cwo, cc)
	}

	for _, dp := range dpaths {
		cwi <- dp
	}

	q := make([]string, 0, 3 * *hint)
	for n := len(dpaths); n != 0; {
		dp := <-cwo
		if *debug { fmt.Println("jobstat", dp, n, q, len(cwi), cap(cwi)) }
		if dp == "" {
			n -= 1
			if len(q) > 0 {
				dp = q[0]
				q = q[1:]
			}
		}
		if dp != "" {
			select {
			case cwi <- dp:
				n += 1
			default:
				q = append(q, dp)
			}
		}
	}

	count := 0
	for i := 0; i < *workers; i++ {
		cwi <- ""
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
	scan := flag.Int("scanby", 10, "interval to scan by")
	rec  := flag.Int("recby", 20, "interval to record by")
	turns := flag.Int("turns", 0, "number of iterations")
	logp := flag.String("logf", "", "log file")
	flag.Parse()

	runtime.GOMAXPROCS(*workers)

	var logj *json.Encoder
	if *logp != "" {
		logf, err := os.OpenFile(*logp, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil { log.Fatal(err) }
		logj = json.NewEncoder(logf)
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
			if logj != nil {
				err := logj.Encode(scanrec{ tn0, tn1, t, count})
				if err != nil { log.Fatal(err) }
			}
			if tr == 0 { os.Exit(0) }
		}(t, *turns)
		to := t
		t = next(*scan, *rec, t)
		time.Sleep(time.Duration(t - to) * time.Second)
	}
}
