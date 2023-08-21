package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"

	"golang.org/x/sys/unix"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: bb_mounter <path>")
		os.Exit(1)
	}

	rootdir := os.Args[1]
	mode := fs.FileMode(0755)

	var err error
	err = os.MkdirAll(rootdir, mode)
	if err != nil {
		panic(err)
	}

	/// Change directory in case there are relative paths,
	// the program should only operate on absolute paths
	// this is only a precaution.
	err = os.Chdir(rootdir)
	if err != nil {
		panic(err)
	}

	donotcare := ""
	noflags := 0
	nothing := uintptr(0)

	for _, mount := range []struct {
		point   string
		fstype  string
		options uintptr
	}{
		{"/proc", "proc", nothing},
		{"/sys", "sysfs", nothing},
		{"/run", "DO NOT CARE", unix.MS_BIND},
	} {
		fmt.Printf("mounting '%v'.\n", mount.point)
		absolute := path.Join(rootdir, mount.point)
		err = os.Mkdir(absolute, mode)
		if err != nil {
			fmt.Println("Mkdir error")
			panic(err)
		}
		defer os.Remove(absolute)

		err = unix.Mount(mount.point, absolute, mount.fstype, mount.options, donotcare)
		if err != nil {
			fmt.Println("Mount error")
			panic(err)
		}
		defer unix.Unmount(absolute, noflags)
	}

	dev_dir := path.Join(rootdir, "dev")
	err = os.Mkdir(dev_dir, mode)
	if err != nil {
		panic(err)
	}
	defer os.Remove(dev_dir)

	devices := []string{"full", "null", "random", "tty", "urandom", "zero"}
	// NB(nils): these are created as files rather than directories.
	//           otherwise `mount(8)` and `mount(2)` fail with
	//           `ENOTDIR` and
	//           `mount: /tmp/nils-test-chroot-sh/dev/full3: mount(2) system call failed: Not a directory.`
	//           respectively, which is confusing to say the least.
	for _, device := range devices {
		bindfs := "" // This is ignored as per the manpage.
		bindflags := uintptr(unix.MS_BIND)

		dev := path.Join(dev_dir, device)
		fmt.Printf("mounting '%v'.\n", dev)
		_, err := os.Create(dev)
		if err != nil {
			panic(err)
		}
		defer os.Remove(dev)

		err = unix.Mount("/dev/"+device, dev, bindfs, bindflags, donotcare)
		if err != nil {
			panic(err)
		}
		defer unix.Unmount(dev, noflags)
	}

}
