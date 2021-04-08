## Fico the File Counter

[![Fico cute](assests/robert_fico_dvojrocny.jpg)](http://en.wikipedia.org/wiki/Robert_Fico)

This program counts files under a list of given target directories.
User gets control over what system calls to use. Basic mode of operation
uses only [readdir(3)](http://linux.die.net/man/3/readdir)
([getdents(2)](http://linux.die.net/man/2/getdents)), so that filesystem gets less load
(than using [lstat(2)](http://linux.die.net/man/2/lstat)s).

With `-mode={dump_dentry|dump_stat}`, it can be used as a low-level alternative of
[find(1)](http://pubs.opengroup.org/onlinepubs/9699919799/utilities/find.html):
- `dump_dentry` shows the information carried by dentries (file name, type, inode
  number), relying on `getdents(2)`, to spare the filesystem from stat'ing. (Nb. on Linux
  `find` does stats if `-type` option is used, so it does not export readdir-based
  type information.)
- `dump_stat` dumps full stat info as JSON.

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
      -debug
        	debug mode
      -dump0
        	on dump_dentry, separate entries by zero byte
      -dumpfullpath
        	on dumping, do not strip off path to target
      -filter string
        	glob pattern to exclude (matching done relatively from targets, matching dirs are not entered)
      -filterfiles
        	apply 'filter' to file counting, too
      -flimit int
        	run 'till this number of files is reached (≤0 means no limit)
      -fuzzy
        	tolerate fs fuzzines (errors due to ongoing changes)
      -hili int
        	interval to show highlighted scan result (default 20)
      -hint int
        	directory branchiness hint (hint on avg. no. of entires in dirs) (default 256)
      -logappend
        	append to previously existing logfile
      -logcont
        	continue logging with relative time of earlier logs (implies logappend)
      -logf string
        	log file
      -mode string
        	counting / dump_dentry / dump_stat (default "counting")
      -scan int
        	interval to scan by (default 10)
      -turns int
        	number of iterations (≤0 means infinite) (default 1)
      -workers int
        	number of workers (default 5)
