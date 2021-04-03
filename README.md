## Fico the File Counter

[![Fico cute](assests/robert_fico_dvojrocny.jpg)](http://en.wikipedia.org/wiki/Robert_Fico)

This program counts files under a list of given target directories.
User gets control over what system calls to use. Basic mode of operation
uses only [readdir(3)](http://linux.die.net/man/3/readdir)
([getdents(2)](http://linux.die.net/man/2/getdents)), so that filesystem gets less load
(than using [lstat(2)](http://linux.die.net/man/2/lstat)s).

With `-dump`, it can be used as a low-level alternative of
[find(1)](http://pubs.opengroup.org/onlinepubs/9699919799/utilities/find.html),
that guarantees to spare the filesystem from stat'ing. (Nb. on Linux `find`
does stats if `-type` option is used, so it does not export readdir-based
type information.)

The other focus of the utility is monitoring, ie. it repeatedly counts files
/ dumps filenames and can also produce a json feed of the result with timing
information.

Tested/expected to work on Linux x64.

## Build

You need [Go](http://golang.org) for the build.

### Building with the module system (recommended)

Tested with Go 1.16.

    cd fico
    go build

### Building without the module system

This might be the workable procedure for other Go versions if the "with module" instructions don't work.

    mkdir _
    ln -s .. _/src
    GO111MODULE=off GOPATH=$PWD/_ go install fico

(binary will be in `_/bin`).

## Usage

    $ fico -h
    FAST CONCURRENT TUNABLE fiLE coUNTER

    fico [options] [targets...] [-- message]
    options:
      -debug=false: debug mode
      -dump=false: dump entries instead of counting them
      -dump0=false: on dumping, separate entries by zero byte
      -dumpfullpath=false: on dumping, do not strip off path to target
      -filter="": glob pattern to exclude (matching done relatively from targets, matching dirs are not entered)
      -filterfiles=false: apply 'filter' to file counting, too
      -flimit=0: run 'till this number of files is reached (≤0 means no limit)
      -fuzzy=false: tolerate fs fuzzines (errors due to ongoing changes)
      -hili=20: interval to show highlighted scan result
      -hint=256: directory branchiness hint (hint on avg. no. of entires in dirs)
      -logappend=false: append to previously existing logfile
      -logcont=false: continue logging with relative time of earlier logs (implies logappend)
      -logf="": log file
      -scan=10: interval to scan by
      -stat=false: salvage missing dirent type by falling back to lstat
      -turns=0: number of iterations (≤0 means infinite)
      -workers=5: number of workers
