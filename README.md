## Fico the File Counter

[![Fico cute](http://blog.sme.sk/blog/1090/65024/robert_fico_dvojrocny.jpg)](http://en.wikipedia.org/wiki/Robert_Fico)

This program counts files under a list of given target directories.
User can log results of repeated scans, and gets control over what
system calls to use. Basic mode of operation uses only readdir(3),
so that filesystem gets less load (than using stats).

## Build

- get [Go ≥ 1.0](http://golang.org)
- optionally, get the [go-gb](http://code.google.com/p/go-gb) build tool

If using go-gb, just run `gb` at the top of source directory (binary will be in `_bin`).
Otherwise, follow these steps:

    mkdir _
    ln -s .. _/src
    GOPATH=$PWD/_ go install fico

(binary will be in `_/bin`).

## Usage

    $ fico -h
    FAST CONCURRENT TUNABLE fiLE coUNTER

    fico [options] [targets...]
    options:
      -debug=false: debug mode
      -filter="": glob pattern to exclude (matching done relatively from targets, matching dirs are not entered)
      -filterfiles=false: apply 'filter' to file counting, too
      -fuzzy=false: tolerate fs fuzzines (errors due to ongoing changes)
      -hili=20: interval to show highlighted scan result
      -hint=256: directory branchiness hint (hint on avg. no. of entires in dirs)
      -logf="": log file
      -scan=10: interval to scan by
      -stat=false: salvage missing dirent type by falling back to lstat
      -turns=0: number of iterations (≤0 means infinite)
      -workers=5: number of workers
