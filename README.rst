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

The `mountat_dfd.c` program shows how to create create and place mounts
into a directory file descriptor,
which can be created from any path, relative or absolute.

::

    $ gcc mountat_dfd.c
    $ mktemp -d
    /tmp/tmp.jz4HILGKEA
    $ prog=$PWD/a.out
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
