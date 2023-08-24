#include <sys/mount.h>

#include <time.h>
#include <fcntl.h>
#include <linux/mount.h>
#include <stddef.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/types.h>
#include <unistd.h>

/* NB: `mount.h` must be included early:
 * https://www.mail-archive.com/debian-glibc@lists.debian.org/msg57333.html
 */

/* move_mount examples from the man pages,
 * to somehow unmount with the Indiana-Jones technique.
 */

// {{{ Wrapper functions.

int
fsopen(const char *fs_name, unsigned int flags)
/* fs/fsopen.c */
/* SYSCALL_DEFINE2(fsopen, const char __user *, _fs_name, unsigned int, flags) */
{
    return syscall(SYS_fsopen, fs_name, flags);
}

int
fspick(int dfd, char* path, unsigned int flags)
/* fs/fsopen.c */
/* SYSCALL_DEFINE3(fspick, int, dfd, const char __user *, path, unsigned int, flags) */
{
    return syscall(SYS_fspick, dfd, path, flags);
}

int
fsconfig(int fd, unsigned int cmd, const char *key, const char *value, int aux)
/* fs/fsopen.c */
/* SYSCALL_DEFINE5(fsconfig,
 *   	int, fd,
 *   	unsigned int, cmd,
 *   	const char __user *, _key,
 *   	const void __user *, _value,
 *   	int, aux)
 */
{
    return syscall(SYS_fsconfig, fd, cmd, key, value, aux);
}

int
fsmount(int fs_fd, unsigned int flags, unsigned int attr_flags)
/* fs/namespace.c */
/* SYSCALL_DEFINE3(fsmount, int, fs_fd, unsigned int, flags, unsigned int, attr_flags) */
{
    return syscall(SYS_fsmount, fs_fd, flags, attr_flags);
}

int
move_mount(int from_dfd, const char *from_pathname, int to_dfd, const char *to_pathname, unsigned int flags)
/* fs/namespace.c */
/* SYSCALL_DEFINE5(move_mount,
 *  	int, from_dfd, const char __user *, from_pathname,
 *  	int, to_dfd, const char __user *, to_pathname,
 *  	unsigned int, flags)
 */
{
    return syscall(SYS_move_mount, from_dfd, from_pathname, to_dfd, to_pathname, flags);
}

int
open_tree(int dfd, const char* filename, unsigned int flags)
/* fs/namespace.c */
/* SYSCALL_DEFINE3(open_tree, int, dfd, const char __user *, filename, unsigned, flags) */
{
    return syscall(SYS_open_tree, dfd, filename, flags);
}
// }}} Wrapper functions.

int
mountat(int dfd, const char *fstype, const char *source, const char *name)
{
    int fd = fsopen(fstype, FSOPEN_CLOEXEC);
    fsconfig(fd, FSCONFIG_SET_STRING, "source", source, 0);
    fsconfig(fd, FSCONFIG_CMD_CREATE, NULL, NULL, 0);
    int mfd = fsmount(fd, FSMOUNT_CLOEXEC, MS_NOEXEC);
    move_mount(mfd, "", dfd, name, MOVE_MOUNT_F_EMPTY_PATH);

    return mfd;
}

int
main(int argc, char* argv[])
{
    char* mountpoint = "proc";
    if (argc < 3) {
        printf("Usage: %s <initial directory> <destination directory>\n", argv[0]);
        printf("Moves a mount from one directory to another, using directory file descriptors.\n");
        exit(1);
    }

    char* initial = argv[1];
    char* destination = argv[2];
    int s_dfd = openat(AT_FDCWD, initial, 0);
    int d_dfd = openat(AT_FDCWD, destination, 0);

    // Try to use 'move_mount' directly on the mount point.
    //     code:   move_mount(s_dfd, mountpoint, d_dfd, mountpoint, 0);
    //     strace: move_mount(3, "proc", 6, "proc", 0)     = -1 EINVAL (Invalid argument)

    // Try to use 'move_mount' on the mount file descriptor from `fspick`.
    // Pick up, a so called, "filesystem configuration context".
    //     code:   int cfd = fspick(s_dfd, mountpoint, FSPICK_NO_AUTOMOUNT | FSPICK_CLOEXEC);
    //     code:   move_mount(cfd, "", d_dfd, mountpoint, 0);
    //     strace: move_mount(7, "", 6, "proc", 0)         = -1 ENOENT (No such file or directory)
    //     code:   move_mount(cfd, "", d_dfd, mountpoint, MOVE_MOUNT_F_EMPTY_PATH);
    //     strace: move_mount(7, "", 6, "proc", MOVE_MOUNT_F_EMPTY_PATH) = -1 EINVAL (Invalid argument)
    // Must we create a mount again?
    // So `fspick` is equivalent to `fsopen`.
    // Additionally, the manpage says that we must reconfigure it
    //     code:   fsconfig(cfd, FSCONFIG_CMD_RECONFIGURE, NULL, NULL, 0);
    //     code:   int mfd = fsmount(cfd, FSMOUNT_CLOEXEC, MS_NOEXEC);
    //     strace: fsmount(5, FSMOUNT_CLOEXEC, MOUNT_ATTR_NOEXEC) = -1 EBUSY (Device or resource busy)
    // But `fsmount` fails, so this does not seem to be it.

    // And using `open_tree` gives failures for `fsconfig` directly.
    //     code:   int mmmfd = open_tree(s_dfd, mountpoint, 0);
    //     code:   fsconfig(mmmfd, FSCONFIG_CMD_RECONFIGURE, NULL, NULL, 0);
    //     strace: fsconfig(5, FSCONFIG_CMD_RECONFIGURE, NULL, NULL, 0) = -1 EBADF (Bad file descriptor)

    // Try to use 'move_mount' on the tree file descriptor from 'open_tree'
    //    code:   int mmfd = open_tree(s_dfd, mountpoint, 0);
    //    code:   move_mount(mmfd, "", d_dfd, mountpoint, MOVE_MOUNT_F_EMPTY_PATH);
    //    strace: move_mount(8, "", 6, "proc", MOVE_MOUNT_F_EMPTY_PATH) = -1 EINVAL (Invalid argument)

    // We can successfully move a clone, but not the original mount it seems.
    int mmmfd = open_tree(s_dfd, mountpoint, OPEN_TREE_CLONE);
    move_mount(mmmfd, "", d_dfd, mountpoint, MOVE_MOUNT_F_EMPTY_PATH);
}

// vim: foldmethod=marker
