package dir

import (
	"os"
	"syscall"
	"unsafe"
)

const (
	DT_UNKNOWN = 0
	DT_FIFO    = 1
	DT_CHR     = 2
	DT_DIR     = 4
	DT_BLK     = 6
	DT_REG     = 8
	DT_LNK     = 10
	DT_SOCK    = 12
	DT_WHT     = 14
)

var Types = [...]string{
	DT_UNKNOWN : "UNKNOWN",
	DT_FIFO    : "FIFO",
	DT_CHR     : "CHR",
	DT_DIR     : "DIR",
	DT_BLK     : "BLK",
	DT_REG     : "REG",
	DT_LNK     : "LNK",
	DT_SOCK    : "SOCK",
	DT_WHT     : "WHT",
}

type Dirent struct {
	Ino  uint64
	Type uint8
	Name string
}

func clen(n []byte) int {
	for i := 0; i < len(n); i++ {
		if n[i] == 0 {
			return i
		}
	}
	return len(n)
}

func ParseDirent(buf []byte, max int, dirents []*Dirent) (consumed int, count int, newdirents []*Dirent) {
	origlen := len(buf)
	count = 0
	for max != 0 && len(buf) > 0 {
		dirent := (*syscall.Dirent)(unsafe.Pointer(&buf[0]))
		buf = buf[dirent.Reclen:]
		if dirent.Ino == 0 { // File absent in directory.
			continue
		}
		bytes := (*[10000]byte)(unsafe.Pointer(&dirent.Name[0]))
		var name = string(bytes[0:clen(bytes[:])])
		if name == "." || name == ".." { // Useless names
			continue
		}
		de := new(Dirent)
		de.Ino = dirent.Ino
		de.Type = dirent.Type
		de.Name = name
		max--
		count++
		dirents = append(dirents, de)
	}
	return origlen - len(buf), count, dirents
}

type dirInfo struct {
	buf  []byte // buffer for directory I/O
	nbuf int    // length of buf; return value from Getdirentries
	bufp int    // location of next record in buf.
}

func Readdir(f *os.File, n int) (dirents []*Dirent, err error) {
	d := new(dirInfo)
	d.buf = make([]byte, 4096)

	if n <= 0 {
		n = 100
	}

	fd := int(f.Fd())
	dirents = make([]*Dirent, 0, n) // Empty with room to grow.
	for {
		// Refill the buffer if necessary
		if d.bufp >= d.nbuf {
			d.bufp = 0
			var errno error
			d.nbuf, errno = syscall.ReadDirent(fd, d.buf)
			if errno != nil {
				return dirents, os.NewSyscallError("readdirent", errno)
			}
			if d.nbuf <= 0 {
				break // EOF
			}
		}

		// Drain the buffer
		var nb int
		nb, _, dirents = ParseDirent(d.buf[d.bufp:d.nbuf], -1, dirents)
		d.bufp += nb
	}
	return dirents, nil
}