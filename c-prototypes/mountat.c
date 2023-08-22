#include <sys/mount.h>

#include <stddef.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <linux/fcntl.h>
#include <linux/mount.h>
#include <sys/syscall.h>

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
mountat(const char *fstype, const char *source, const char *dirname)
{
    int fd = fsopen(fstype, FSOPEN_CLOEXEC);
    fsconfig(fd, FSCONFIG_SET_STRING, "source", source, 0);
    fsconfig(fd, FSCONFIG_CMD_CREATE, NULL, NULL, 0);
    int mfd = fsmount(fd, FSMOUNT_CLOEXEC, MS_NOEXEC);
    move_mount(mfd, "", AT_FDCWD, dirname, MOVE_MOUNT_F_EMPTY_PATH);
}

int
main()
{
    mountat("proc", "/proc", "proc");
    mountat("sysfs", "/sys", "sys");
}

// vim: foldmethod=marker
