package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"golang.org/x/sys/unix"
)

const (
	FSCONFIG_SET_FLAG        = 0
	FSCONFIG_SET_STRING      = 1
	FSCONFIG_SET_BINARY      = 2
	FSCONFIG_SET_PATH        = 3
	FSCONFIG_SET_PATH_EMPTY  = 4
	FSCONFIG_SET_FD          = 5
	FSCONFIG_CMD_CREATE      = 6
	FSCONFIG_CMD_RECONFIGURE = 7
)

// Rudimentary function wrapper for the `fsconfig` syscall.
//
// This is implemented as a stop gap solution until real support is merged into the `unix` library.
// See the following patchset: https://go-review.googlesource.com/c/sys/+/399995/
// This only implements the two commands needed for buildbarn's `mountat` functionality.
// And will just exit if any other command is called.
//
// TODO(nils): construct proper `syscall.E*` errors.
//
//	like `unix.errnoErr`, but the function is not exported.
//
// TODO(nils): assert that `ret` is always the same as the error code,
//
//	so the user does not need to worry about a potential file descriptor.
func fsconfig(fsfd int, cmd int, key string, value string, flags int) (err error) {
	switch cmd {
	case FSCONFIG_SET_STRING:
		if len(key) == 0 || len(value) == 0 {
			err = errors.New("`key` and `value` must be provided")
			return
		}
		break
	case FSCONFIG_CMD_CREATE:
		if len(key) != 0 || len(value) != 0 {
			err = errors.New("`key` and `value` must be empty")
			return
		}
		break
	default:
		err = errors.New("not implemented: " + string(cmd))
		return
	}

	var _p0 *byte
	var _p1 *byte

	_p0, err = unix.BytePtrFromString(key)
	if err != nil {
		return
	}
	if key == "" {
		_p0 = nil
	}

	_p1, err = unix.BytePtrFromString(value)
	if err != nil {
		return
	}
	if value == "" {
		_p1 = nil
	}

	r0, _, e1 := unix.Syscall6(
		unix.SYS_FSCONFIG,
		uintptr(fsfd),
		uintptr(cmd),
		uintptr(unsafe.Pointer(_p0)),
		uintptr(unsafe.Pointer(_p1)),
		uintptr(flags),
		0,
	)
	ret := int(r0)
	if e1 != 0 {
		err = e1
		return
	}
	if ret < 0 {
		err = errors.New("negative return code, not converted to an error in `Syscall`: " + string(ret))
		return
	}

	return
}

type entry = struct {
	fd   int
	path string
}

func openFileDescriptor(path string) (int, error) {
	d, err := os.Open(path)
	if err != nil {
		return -1, err
	}
	open_dfd := int(d.Fd())

	// Duplicate the file descriptor without `CLOEXEC` to send it to children.
	dfd, err := unix.Dup(open_dfd)
	if err != nil {
		return -1, err
	}
	return dfd, nil
}

func main() {
	usage := func() {
		fmt.Println(`Usage: bb_mounter_at <pathname> [delay]

		Mounts '/proc' and '/sys' into a directory,
		using mount points called "proc" and "sys" respectively.

		Waits to unmount for 'delay' seconds, default 5.
		`)
	}

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			usage()
			os.Exit(0)
		}
	}

	rootdir := os.Args[1]

	delay := int64(5)
	if len(os.Args) > 2 {
		d, err := strconv.ParseInt(os.Args[2], 10, 32)
		if err != nil {
			log.Fatal(err)
		}
		delay = d
	}

	err := do(rootdir, delay)
	if err != nil {
		log.Fatal(err)
	}
}

func do(rootdir string, delay int64) error {
	directory := struct {
		fd   int
		name string // for diagnostic prints, do not use.
	}{}

	mode := fs.FileMode(0755)

	var err error
	err = os.MkdirAll(rootdir, mode)
	if err != nil {
		return err
	}
	dfd, err := openFileDescriptor(rootdir)
	if err != nil {
		return err
	}

	directory.fd = dfd
	directory.name = rootdir
	to_unmount := []entry{}

	for _, mount := range []struct {
		name   string
		fstype string
		source string
	}{
		{"proc", "proc", "/proc"},
		{"sys", "sysfs", "/sys"},
	} {
		err = unix.Mkdirat(directory.fd, mount.name, 0700)
		if err != nil {
			if !os.IsExist(err) {
				return err
			}
		}

		fmt.Printf("Mounting %s into %d (file descriptor for) %s.\n", mount.source, directory.fd, directory.name)
		mfd, err := mountat(directory.fd, mount.fstype, mount.source, mount.name)
		if err != nil {
			return err
		}

		to_unmount = append(to_unmount, entry{mfd, mount.name})
	}

	unit := "seconds"
	if delay == 1 {
		unit = "second"
	}
	fmt.Printf("Sleeping %d %s.\n", delay, unit)
	time.Sleep(time.Duration(delay) * time.Second)
	for _, mount := range to_unmount {
		err := syscall.Close(mount.fd)
		if err != nil {
			log.Fatal(err)
		}
		err = unmountat_relative(directory.fd, mount.path)
		if err != nil {
			log.Fatal(err)
		}
	}

	return nil
}

func mountat(dfd int, fstype, source, mountname string) (int, error) {
	// Mounts the `source` filesystem on `mountname` inside a directory
	// given as `dfd` file descriptor.
	// Using the `fstype` filesystem type.
	//
	// Returns:
	//    `mfd`: the file descriptor to the mount object.
	//    `err`: error
	fd, err := unix.Fsopen(fstype, unix.FSOPEN_CLOEXEC)
	if err != nil {
		return -1, err
	}

	err = fsconfig(fd, FSCONFIG_SET_STRING, "source", source, 0)
	if err != nil {
		return -1, err
	}

	err = fsconfig(fd, FSCONFIG_CMD_CREATE, "", "", 0)
	if err != nil {
		return -1, err
	}

	mfd, err := unix.Fsmount(fd, unix.FSMOUNT_CLOEXEC, unix.MS_NOEXEC)
	if err != nil {
		return -1, err
	}
	err = unix.MoveMount(mfd, "", dfd, mountname, unix.MOVE_MOUNT_F_EMPTY_PATH)
	if err != nil {
		return -1, err
	}

	return mfd, nil
}

func unmountat_relative(dfd int, mountname string) error {
	/// Hacky unmountat
	// Uses a subprocess to isolate the `fchdir` from the main program.

	fmt.Printf("Unmounting %s from %d (directory file descriptor).\n", mountname, dfd)
	unmounter, err := runfiles.Rlocation("__main__/cmd/relative_unmount/relative_unmount_/relative_unmount")
	if err != nil {
		return err
	}
	cmd := exec.Command(unmounter, fmt.Sprintf("%d", dfd), mountname)
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func unmountat_fstab(mountname, directory_path_segments string) error {
	/// Hacky unmountat
	//
	// Will unmount a named mount point `mountname` using `/etc/mtab` to look up previous mounts.
	// This has "at" in it, though there are no file descriptors,
	// nor knowledge of what we have done previously.
	// For shortcomings of the `move_mount` syscall in the "new mount API".
	// Consider it a workaround until a better API-conforming solution is found.
	//
	// We look for any entry of `mountname`
	// that contains the `directory_path_segment`.
	//
	// The path segment logic is a little bit involved,
	// as we cannot canonicalize the paths
	// and still do not know the absolute path.
	//
	// Returns:
	//     error - propagated from calls,
	//           - or if there are multiple matches
	//             to the `directory_path_segment` and `mountname`.
	//           - EBUSY: If there is an open file descriptor of the mount.
	//
	/// Look for any previous mount
	// that matches what information we have about the mount point.
	// We parse `/etc/mtab`, assuming the following grammar
	//     <line>
	//     line = <path> <other>
	// for <path>.
	//
	// Note: this is called "unmount" just like `unix.Unmount`,
	// even though the syscall (we emulate) is called "umount" for historical reasons.
	//
	// Example:
	//     sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
	//     systemd-1 /proc/sys/fs/binfmt_misc autofs rw,relatime,fd=29,pgrp=1,timeout=0,minproto=5,maxproto=5,direct,pipe_ino=18761 0 0
	//     /dev/loop13 /snap/firefox/2211 squashfs ro,nodev,relatime,errors=continue 0 0
	//     /sys /tmp/nils-test-chroot/sys sysfs rw,relatime 0 0
	//     /proc /tmp/test-relative-mount-at/proc proc rw,noexec,relatime 0 0
	//      /tmp/test-relative-mount/proc proc rw,relatime 0 0
	//
	// Yes, the last one has a leading space.
	// These would parse to:
	//     /sys
	//     /proc/sys/fs/binfmt_misc
	//     /dev/loop13
	//     /sys
	//     /proc
	//     /tmp/test-relative-mount/proc
	//
	// Where only the latter two are correctly parse.
	// Instead we may parse from the left as an improvement.
	//
	// TODO(nils): find a good format to document the format, BNF? haskell record?

	last := func(a []string) string {
		return a[len(a)-1]
	}
	mountpath := func(line string) string {
		split := strings.Split(line, " ")
		from_the_end := len(split) - 5
		return split[from_the_end]
	}
	segment := func(a string) []string {
		/// Segment a string into path components,
		// discard any empty components.
		crude := strings.Split(a, "/")
		refined := make([]string, 0, len(crude))
		for _, s := range crude {
			if s != "" {
				refined = append(refined, s)
			}
		}
		return refined
	}

	var err error

	mtab, err := os.Open("/etc/mtab")
	if err != nil {
		return err
	}

	var matches []string
	scanner := bufio.NewScanner(mtab)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		// TODO(nils): maybe refactor this to a two-pass algorithm,
		//             where we can more easily feed it with known string arrays later.
		//             It probably needs a tuple/struct scheme to map
		//             the component array to the original unsplit strings.
		line := scanner.Text()
		line = strings.Trim(line, " ")

		entry := mountpath(line)

		mount_segments := segment(entry)
		if len(mount_segments) == 0 {
			continue
		}
		basename_ok := last(mount_segments) == mountname

		known_segments := segment(directory_path_segments)
		segments_ok := false
		// compare the two path segment sets against each other for a full match.
		possible_positions := len(mount_segments) - len(known_segments)

		for start_of_known := 0; start_of_known <= possible_positions; start_of_known++ {
			// TODO(nils): the inner loop would be `slices.Compare`: https://pkg.go.dev/golang.org/x/exp/slices#Compare
			full_match := true
			for inside_known := 0; inside_known < len(known_segments); inside_known++ {
				i := inside_known
				k := start_of_known + inside_known

				full_match = full_match && (mount_segments[k] == known_segments[i])
			}
			if full_match {
				segments_ok = true
				break
			}
		}

		if basename_ok && segments_ok {
			matches = append(matches, entry)
		}
	}

	error_metadata := fmt.Sprintf("for '%v' and '%v'", mountname, directory_path_segments)
	match_count := len(matches)
	if match_count > 1 {
		return fmt.Errorf("Ambiguous `mtab` lookup %v.\n%v", error_metadata, matches)
	}
	if match_count == 0 {
		return fmt.Errorf("No match found in `mtab` %v.\n", error_metadata)
	}
	match := matches[0]

	fmt.Printf("Unmounting '%s' at '%s'.\n", mountname, match)
	err = unmount(match)
	if err != nil {
		return err
	}

	return nil
}

func unmount(path string) error {
	return unix.Unmount(path, 0)
}
