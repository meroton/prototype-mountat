Prototype mountat
~~~~~~~~~~~~~~~~~

Implement "mountat" with the new mount API.

This is reference code for our work
to improve `chroot` in `Buildbarn`_.
A technical write up is `available here`_

.. _Buildbarn: https://github.com/buildbarn/bb-remote-execution/
.. _available here: https://meroton.com/docs/improved-chroot-in-Buildbarn/implementing-mountat/

Goals
=====

✅Implement `mountat`

❌Implement `umountat`

Go programs
===========

A go implementation of `mountat` is available in `bb_mounter_at`_.
This currently has the `unmount-through-fstab`_ hack,
but will be rewritten to use `relative-unmount`_.
There is also a stub implementation using regular `mount` in `bb_mounter`,
which exists mostly for completeness sake, it is not valuable here.

.. _bb_mounter_at: https://github.com/meroton/prototype-mountat/blob/main/cmd/bb_mounter_at/main.go
.. _bb_mounter: https://github.com/meroton/prototype-mountat/blob/main/cmd/bb_mounter/main.go

.. _unmount-through-fstab: http://white:3000/docs/improved-chroot-in-buildbarn/integrating-mountat/#second-best-effort-use-new-mountat-but-hack-unmounting-through-absolute-paths
.. _relative-unmount: http://white:3000/docs/improved-chroot-in-buildbarn/implementing-unmountat/#relative-unmount

Development log: EBUSY
----------------------

tl;dr: you must close the mount file descriptor before calling `unmount` on the mount point.

The go programs got caught up in the unmount path,
that the mount points are busy.
Even with the `MNT_FORCE` flag.

::

    755587 umount2("/tmp/tmp.jz4HILGKEA/bazel-run/sys", MNT_FORCE <unfinished ...>
    755587 <... umount2 resumed>)           = -1 EBUSY (Device or resource busy)

Note that this is `umount2`,

With the unmount script from the `toolbox`_ we use the `unmount` program.
Which always succeeds, though it does a lot more bookkeeping that the single `umount2` call.
Is this another misunderstanding of what to do?

::

    756273 umount2("/tmp/tmp.jz4HILGKEA/bazel-run/proc", 0) = 0

For the reference the Kubernetes `mount-utils`_ package
uses the `unmount` `program rather than the function`_ from the `unix package`_

.. _mount-utils: https://github.com/kubernetes/mount-utils/
.. _program rather than the function: https://github.com/kubernetes/mount-utils/blob/master/mount_linux.go#L808
.. _unix package: https://pkg.go.dev/golang.org/x/sys@v0.11.0/unix#Unmount

We can fork to exec `umount` internally,
But it seems to fail too.
From the console output::

    Unmounting 'proc' at '/tmp/tmp.jz4HILGKEA/bazel-run/proc'.
    2023/08/28 13:47:59 exit status 32

Whereas strace indicates success::

    778943 execve("/usr/bin/umount", ["umount", "/tmp/tmp.jz4HILGKEA/bazel-run/proc"], 0xc0001a4680 /* 24 vars */ <unfinished ...>
    778943 <... execve resumed>)            = 0

And the mount remains.

File descriptor
---------------

Is this because we have an open file descriptor to the mount?
We can try this by sleeping for much longer and try to unmount from outside,
which has always worked after the process completes

::

    $ sudo ./bb_mounter_at /tmp/tmp.jz4HILGKEA/bazel-run 100
    mounting /proc into 3 (file descriptor for) /tmp/tmp.jz4HILGKEA/bazel-run.
    mounting /sys into 3 (file descriptor for) /tmp/tmp.jz4HILGKEA/bazel-run.
    sleeping 100 seconds.

    /tmp/tmp.jz4HILGKEA $ ./list                                                   tmux: 1/2
    /proc on /tmp/tmp.jz4HILGKEA/bazel-run/proc type proc (rw,noexec,relatime)
    /sys on /tmp/tmp.jz4HILGKEA/bazel-run/sys type sysfs (rw,noexec,relatime)
    /tmp/tmp.jz4HILGKEA $ ./unmount                                                tmux: 1/2
    umount: /tmp/tmp.jz4HILGKEA/bazel-run/proc: target is busy.
    umount: /tmp/tmp.jz4HILGKEA/bazel-run/sys: target is busy.
    /tmp/tmp.jz4HILGKEA $ ./list                                                   tmux: 1/2
    /proc on /tmp/tmp.jz4HILGKEA/bazel-run/proc type proc (rw,noexec,relatime)
    /sys on /tmp/tmp.jz4HILGKEA/bazel-run/sys type sysfs (rw,noexec,relatime)

Yes! `syscall.Close(mfd)` does the trick.

Relative unmount in go
----------------------

We can now proceed to implement `relative-unmount` in go,
and integrate it into `bb_mounter_at`,
which drives it and feeds the file descriptor.

.. Note::

   We have not yet made sure to keep the directory file descriptor open,
   so the unmounting program may receive a number that is not a valid descriptor.
   We will address that in due time.

Debug
-----

One consequence is that we can no longer use the convenience symlink
to run the command.
As it requires the runfiles tree,
that the runfile library handles for us,
we just need some environment variables.

::

    $ bazel run -c dbg --script_path=run //cmd/bb_mounter_at
    $ sed -i '$s|^|sudo '$(which dlv)' exec |' debug
    $ sudo ./run /tmp/tmp.jz4HILGKEA/bazel-run 1

But this is much worse at finding the source files.
So we need to `remap the debug symbol paths`_,
as is customary for bazel projects.

::

    (dlv) config substitute-path external /home/nils/.cache/bazel/_bazel_nils/0604d25345427c49ad66cdd3255cacf2/execroot/__main__/external
    (dlv) config substitute-path cmd      /home/nils/.cache/bazel/_bazel_nils/0604d25345427c49ad66cdd3255cacf2/execroot/__main__/cmd

This helps us inspect the runfiles::

    *github.com/bazelbuild/rules_go/go/runfiles.Runfiles {
            impl: github.com/bazelbuild/rules_go/go/runfiles.runfiles(github.com/bazelbuild/rules_go/go/runfiles.manifest) [
                    "__main__/cmd/bb_mounter_at/bb_mounter_at_/bb_mounter_at": "/home/nils/.cache/bazel/_bazel_nils/0604d25345427c49ad66cdd3255c...+90 more",
                    "__main__/cmd/relative_unmount/relative_unmount_/relative_unmount": "/home/nils/.cache/bazel/_bazel_nils/0604d25345427c49ad66cdd3255c...+99 more",


fork/exec::

    2023/08/28 16:48:30 fork/exec /home/nils/.cache/bazel/_bazel_nils/0604d25345427c49ad66cdd3255cacf2/execroot/__main__/bazel-out/k8-dbg/bin/cmd/relative_unmount/relative_unmount_/relative_unmount: invalid argument

.. _remap the debug symbol paths: https://github.com/bazelbuild/rules_go/issues/1708#issuecomment-791114337

Though strace indicates some kind of success.

::

    $ bazel run -c dbg --run_under "sudo strace -f -s1000 -e execve" //cmd/bb_mounter_at -- /tmp/tmp.jz4HILGKEA/bazel-run 1
    ...
    [pid 987247] execve("/home/nils/.cache/bazel/_bazel_nils/0604d25345427c49ad66cdd3255cacf2/execroot/__main__/bazel-out/k8-dbg/bin/cmd/relative_unmount/relative_unmount_/relative_unmount", ["/home/nils/.cache/bazel/_bazel_nils/0604d25345427c49ad66cdd3255cacf2/execroot/__main__/bazel-out/k8-dbg/bin/cmd/relative_unmount/relative_unmount_/relative_unmount", "\3", "proc"], 0xc0000c0340 /* 24 vars */) = 0
    ...
    [pid 988512] --- SIGCHLD {si_signo=SIGCHLD, si_code=CLD_EXITED, si_pid=988520, si_uid=0, si_status=2, si_utime=0, si_stime=0} ---

    2023/08/29 09:38:33 exit status 2

This looks like the inner process does spawn,
it just fails with error code 2

Debug wrappee
-------------

This is always a fun experiment.
The first order of business is to add tracing,
the `exec.Command().Run()` code does not plumb the wrappee's output through,
but we can see it with strace: `-e write`::

    [pid 992352] write(2, "Failed to parse file descriptor: '\3'\n", 37) = 37
    [pid 992352] write(2, "panic: ", 7)     = 7

We saw `above`_ that the argument is "\3"::

    execve("...relative_unmount", [..., "\3", "proc"], ... /* 24 vars */) = 0

Which is now a problem.

.. _above: `Debug`_

PT_INTERP
---------

This is a deep dive into everything Linux.
shell::

    $ readelf -l /bin/sh

    Elf file type is DYN (Position-Independent Executable file)
    Entry point 0x4ef0
    There are 13 program headers, starting at offset 64

    Program Headers:
      Type           Offset             VirtAddr           PhysAddr
                     FileSiz            MemSiz              Flags  Align
      PHDR           0x0000000000000040 0x0000000000000040 0x0000000000000040
                     0x00000000000002d8 0x00000000000002d8  R      0x8
      INTERP         0x0000000000000318 0x0000000000000318 0x0000000000000318
                     0x000000000000001c 0x000000000000001c  R      0x1
          [Requesting program interpreter: /lib64/ld-linux-x86-64.so.2]

relative_unmount::

    $ readelf -l /home/nils/.cache/bazel/_bazel_nils/0604d25345427c49ad66cdd3255cacf2/execroot/__main__/bazel-out/k8-dbg/bin/cmd/relative_unmount/relative_unmount_/relative_unmount | head -20

    Elf file type is EXEC (Executable file)
    Entry point 0x45fc80
    There are 7 program headers, starting at offset 64

    Program Headers:
      Type           Offset             VirtAddr           PhysAddr
                     FileSiz            MemSiz              Flags  Align
      PHDR           0x0000000000000040 0x0000000000400040 0x0000000000400040
                     0x0000000000000188 0x0000000000000188  R      0x1000
      NOTE           0x0000000000000fe8 0x0000000000400fe8 0x0000000000400fe8
                     0x0000000000000018 0x0000000000000018  R      0x4
      LOAD           0x0000000000000000 0x0000000000400000 0x0000000000400000
                     0x000000000008ce57 0x000000000008ce57  R E    0x1000
      LOAD           0x000000000008d000 0x000000000048d000 0x000000000048d000
                     0x0000000000099138 0x0000000000099138  R      0x1000
      LOAD           0x0000000000127000 0x0000000000527000 0x0000000000527000
                     0x0000000000018180 0x0000000000049c30  RW     0x1000
      GNU_STACK      0x0000000000000000 0x0000000000000000 0x0000000000000000
                     0x0000000000000000 0x0000000000000000  RW     0x8

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
===============

.. toolbox:

Working with mounts in your scratch area
----------------------------------------

List mounts under the current directory:

    $ mount -v | grep $PWD

Unmount everything below the current directory:

    $ mount -v | cut -d' ' -f3 | xargs -n1 sudo umount
    $ mount -v | choose 2      | xargs -n1 sudo umount

This unmounts once, so if you have stacked mounts it must be called repeatedly.
Shout-out to [choose] for many simple `cut` and `awk` use-cases.
This is available as `./unmount` from the project root.

If we instead create the mount with `mountat` internally
the mounts will have the `noexec` flag:
But we still end up with the original and the moved clone.

    /proc on /tmp/tmp.jz4HILGKEA/destination/proc type proc (rw,noexec,relatime)

[choose]: https://github.com/theryangeary/choose

Debugging the go program
------------------------

::

    $ bazel build -c dbg //cmd/bb_mounter_at
    Target //cmd/bb_mounter_at:bb_mounter_at up-to-date:
      bazel-bin/cmd/bb_mounter_at/bb_mounter_at_/bb_mounter_at
    $ ln -s $PWD/bazel-bin/cmd/bb_mounter_at/bb_mounter_at_/bb_mounter_at bb_mounter_at

Then use the `execroot`-trick to debug with `dlv`.

::

    ./debug-bb_mounter_at /tmp/tmp.jz4HILGKEA

