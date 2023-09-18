#include <sys/mount.h>

#include <fcntl.h>
#include <linux/mount.h>
#include <stddef.h>
#include <stdio.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/types.h>
#include <unistd.h>
#include <stdlib.h>

/* NB: `mount.h` must be included early:
 * https://www.mail-archive.com/debian-glibc@lists.debian.org/msg57333.html
 */

// {{{ Wrapper functions.

/* Function wrappers for the syscalls,
 * These are not implemented in the c library I use,
 * These functions are added in `glibc` 2.36:
 * https://www.phoronix.com/news/GNU-C-Library-Glibc-2.36
 */

int
fsopen(const char *fs_name, unsigned int flags)
{
    return syscall(SYS_fsopen, fs_name, flags);
}

int
fsconfig(int fd, unsigned int cmd, const char *key, const char *value, int aux)
{
    return syscall(SYS_fsconfig, fd, cmd, key, value, aux);
}

int
fsmount(int fs_fd, unsigned int flags, unsigned int attr_flags)
{
    return syscall(SYS_fsmount, fs_fd, flags, attr_flags);
}

int
move_mount(int from_dfd, const char *from_pathname, int to_dfd, const char *to_pathname, unsigned int flags)
{
    return syscall(SYS_move_mount, from_dfd, from_pathname, to_dfd, to_pathname, flags);
}

// }}} Wrapper functions.

void
mountat(int dfd, const char *fstype, const char *source, const char *name)
{
    int fd = fsopen(fstype, FSOPEN_CLOEXEC);
    fsconfig(fd, FSCONFIG_SET_STRING, "source", source, 0);
    fsconfig(fd, FSCONFIG_CMD_CREATE, NULL, NULL, 0);
    int mfd = fsmount(fd, FSMOUNT_CLOEXEC, MS_NOEXEC);
    move_mount(mfd, "", dfd, name, MOVE_MOUNT_F_EMPTY_PATH);
    close(mfd);
    close(fd);
}

int
main(int argc, char* argv[])
{
    if (argc < 2) {
        printf("Usage: %s <directory>\n", argv[0]);
        exit(1);
    }

    char* dir = argv[1];
    int dfd = openat(AT_FDCWD, dir, 0);
    mkdirat(dfd, "proc", 0777);
    mkdirat(dfd, "sys", 0777);

    mountat(dfd, "proc", "/proc", "proc");
    mountat(dfd, "sysfs", "/sys", "sys");

    sleep(10);

    fchdir(dfd);
    umount("proc");
    umount("sys");
}

// vim: foldmethod=marker
