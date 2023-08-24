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
    char* mountpoint = "./proc";
    if (argc < 2) {
        exit(1);
    }

    char* initial = argv[1];
    int dfd = openat(AT_FDCWD, initial, 0);
    fchdir(dfd);
    umount(mountpoint);
}
