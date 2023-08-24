Prototype mountat
~~~~~~~~~~~~~~~~~

Implement "mountat" with the new mount API.

README is coming soon,
I'm writing a blog post about the background
and will tie the docs together after that.

Goals
=====

✅Mountat
❌Umountat

Go programs
===========

.. TODO

C prototypes
============

The `c-prototypes/` directory contains our prototype,
that have simple implementations of the functions we want using the new syscalls.
Before writing good error-checking go code I wrote these to prototype.
To understand errors I recommend using `strace`
to see how the syscalls are called and what they return.

Relative mount
--------------

`relative_mount.c` shows that `mount` can take relative paths,
but they must start with "./".
Combined with a `fchdir` to the file descriptor this can be used
to emulate "mountat".
This takes a directory name and creates "proc" inside it.

    https://github.com/torvalds/linux/blob/93f5de5f648d2b1ce3540a4ac71756d4a852dc23/tools/testing/selftests/openat2/resolve_test.c#L75

Mountat
-------

The `mountat_dfd.c` program shows how to create create and place mounts
into a directory file descriptor,
which can be created from any path, relative or absolute.

::

    $ gcc mountat_dfd.c
    $ prog=$PWD/a.out
    $ mktemp -d
    /tmp/tmp.jz4HILGKEA
    $ cd /tmp/tmp.jz4HILGKEA

    $ mkdir -p mnt/{sys,proc}
    $ tree
    .
    └── mnt
        ├── proc
        └── sys

    3 directories, 0 files
    $ sudo "$prog" mnt
    $ tree -L 3 | tail -1
    740 directories, 48 files

Relative unmount
----------------

Just like `mount`_ we can use relative paths in `unmount`
by first changing to the directory in which we operate.
This is avaialble in `relative_unmount.c`.

.. _mount: `relative mount`_

Unmountat
---------

Has not been possible,
see `move mount`_ for the progress.

Move mount
----------

The next exploratory step in trying to unmount the mounts we created.
This attempts an "Indiana-Jones swap" by moving the mount to a better place,
that we can address later.
It should also be a step towards a full unmount,
which can _allegedly_ be unmounted with `move_mount`, `fspick` and so on.

This [tracee document] is also light but indicates that it should work
based on the directory file descriptors and names therein.
But that does not work for me.

::

    $ gcc move_mount.c
    $ prog=$PWD/a.out
    $ mktemp -d
    /tmp/tmp.fcGMUvdIMq
    $ cd /tmp/tmp.fcGMUvdIMq

    $ mkdir -p {mnt,destination}/proc
    $ tree
    .
    ├── destination
    │   └── proc
    └── mnt
        └── proc

    # Create an initial mount,
    # as it can be interesting to run the script multiple times,
    # and it would happily stack mounts,
    # so it is harder to see when a move or unmount succeeded.
    $ mount -t proc /proc mnt/proc

    mount -v | grep $PWD
    /proc on /tmp/tmp.fcGMUvdIMq/mnt/proc type proc (rw,relatime)
    $ sudo strace -s1000 --failed-only "$prog"
    mount -v | grep $PWD
    /proc on /tmp/tmp.fcGMUvdIMq/mnt/proc type proc (rw,relatime)
    /proc on /tmp/tmp.fcGMUvdIMq/destination/proc type proc (rw,relatime)

This is where I fall short, we are closing in on the solution
but a full clone is not sufficient,
we want the original to be unmounted.

The [source file] contains commented out sections that I tried
combined with their failures.
Mostly `EINVAL` errors.

They can probably be investigated further by reading warnings and errors
from the file descriptors,
or by digging into the Linux source code
and potentially debugging them.
But that is a bigger undertaking.

[tracee document]: https://aquasecurity.github.io/tracee/dev/docs/events/builtin/syscalls/move_mount/

Tips and tricks
---------------

List mounts under the current directory:

    $ mount -v | grep $PWD

Unmount everything below the current directory:

    $ mount -v | cut -d' ' -f3 | xargs -n1 sudo umount
    $ mount -v | choose 2      | xargs -n1 sudo umount

This unmounts once, so if you have stacked mounts it must be called repeatedly.
Shout-out to [choose] for many simple `cut` and `awk` use-cases.

looks relative, but it construct an absolute path internally

If we instead create the mount with `mountat` internally
the mounts will have the `noexec` flag:
But we still end up with the original and the moved clone.

    /proc on /tmp/tmp.jz4HILGKEA/destination/proc type proc (rw,noexec,relatime)

[choose]: https://github.com/theryangeary/choose
