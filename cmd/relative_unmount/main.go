package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

func main() {
	usage := func() {
		fmt.Println(`Usage: unmount_relative <directory file descriptor> <mount name>

		Calls umount(2) on 'mount_name' in 'directory file descriptor'.
		The file descriptor must be sent over 'fork' + 'exec'.
		This is meant to be called by other programs,
		but it will modify the working directory,
		and serves as an isolation layer.
		`)
	}

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	if os.Args[1] == "-h" || os.Args[1] == "--help" {
		usage()
		os.Exit(0)
	}

	dfd, err := strconv.ParseInt(os.Args[1], 10, 32)
	if err != nil {
		panic(err)
	}
	name := os.Args[2]

	err = unix.Fchdir(int(dfd))
	if err != nil {
		log.Fatal(err)
	}
	err = unix.Unmount(name, 0)
	if err != nil {
		log.Fatal(err)
	}
}
