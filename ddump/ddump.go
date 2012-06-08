// Copyright 2012 Csaba Henk. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.


package main

import (
	"os"
	"log"
	"fmt"

	"dir"
)

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil { log.Fatal(err) }

	des, err := dir.Readdir(f, 0)
	if err != nil { log.Fatal(err) }

	for _, de := range des {
		fmt.Println(dir.Types[de.Type], de.Ino, de.Name)
	}
}
