#include <sys/mount.h>

#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/stat.h>
#include <unistd.h>

/* NB: `mount.h` must be included early:
 * https://www.mail-archive.com/debian-glibc@lists.debian.org/msg57333.html
 */

int
main(int argc, char* argv[])
{
    if (argc < 3) {
        printf("Usage: %s <directory> <mountpoint>\n", argv[0]);
        printf("Mounts `/proc` to `mountpoint` inside `directory`.\n");
        printf("and cannot create a directory hierarchy.\n");
        exit(1);
    }

    char* initial = argv[1];
    char* mountpoint = argv[1];
    int dfd = openat(AT_FDCWD, initial, 0);
    mkdirat(dfd, mountpoint, 0755);
    fchdir(dfd);
    int mfd = mount("/proc", mountpoint, "proc", MS_NOSUID | MS_NODEV, "");
}
