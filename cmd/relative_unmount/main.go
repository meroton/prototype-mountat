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
		The file descriptor must be sent over 'fork' + 'exec',
		so not open directories with '*_CLOEXEC'.

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
		fmt.Fprintf(os.Stderr, "Failed to parse file descriptor: '%s'\n", os.Args[1])
		panic(err)
	}
	name := os.Args[2]

	err = unix.Fchdir(int(dfd))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to change directory to file descriptor: '%d'\n", dfd)
		log.Fatal(err)
	}
	err = unix.Unmount(name, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to unmount '%s' in  directory file descriptor: '%d'\n", name, dfd)
		log.Fatal(err)
	}
}
