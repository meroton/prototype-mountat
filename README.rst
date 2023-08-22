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

The `c-prototypes/` directory contains two files
that have the simplest implementation of using the syscalls.
Before writing good error-checking go code I wrote these to prototype.
To understand errors I recommend using `strace`
to see how the syscalls are called and what they return.

The first file `mountat.c` shows how to implement the barest `mountat`.
This mounts `/proc` and `/sys` into the current working directory,
no arguments are handled.

::

    $ gcc mountat.c
    $ mktemp -d
    /tmp/tmp.MlpdhEhm8g
    $ prog=$PWD/a.out
    $ cd /tmp/tmp.MlpdhEhm8g


    $ mkdir sys
    $ mkdir proc
    $ tree
    .
    ├── proc
    └── sys
    $ sudo "$prog"
    $ tree -L 2 | tail -1
    736 directories, 47 files

The second file does the same but with a directory file descriptor,
a small improvement in functionality.

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
