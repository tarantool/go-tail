# Unreleased

# Version v1.4.14
* PR #4: re-check the file right after arming the inotify watch. The
  watch is only added once the file has been read to EOF, so a write, a
  truncation or a rotation landing in between produced no event and
  never would: the tailer kept waiting with unread data in the file
  until some later, unrelated write woke it up.

# Version v1.4.13
* PR #1: possible line loss and duplication when file is renamed.
* PR #2: drain the open file to EOF before handling a rename or deletion notification.

# Version v1.4.11
* Bump fsnotify to v1.6.0. Should fix some issues.

# Version v1.4.9
* Bump fsnotify to v1.5.1 fixes issue #28, hpcloud/tail#90.
* PR #27: "Add timeout to tests"by @kokes++. Also timeout on FreeBSD.
* PR #29: "Use temp directory for tests, instead of relative" by @ches++.

# Version v1.4.7-v1.4.8
* Documentation updates.
* Small linter cleanups.
* Added example in test.

# Version v1.4.6

* Document the usage of Cleanup when re-reading a file (thanks to @lesovsky) for issue #18.
* Add example directories with example and tests for issues.

# Version v1.4.4-v1.4.5

* Fix of checksum problem because of forced tag. No changes to the code.

# Version v1.4.1

* Incorporated PR 162 by by Mohammed902: "Simplify non-Windows build tag".

# Version v1.4.0

* Incorporated PR 9 by mschneider82: "Added seekinfo to Tail".

# Version v1.3.1

* Incorporated PR 7: "Fix deadlock when stopping on non-empty file/buffer",
fixes upstream issue 93.


# Version v1.3.0

* Incorporated changes of unmerged upstream PR 149 by mezzi: "added line num
to Line struct".

# Version v1.2.1

* Incorporated changes of unmerged upstream PR 128 by jadekler: "Compile-able
code in readme".
* Incorporated changes of unmerged upstream PR 130 by fgeller: "small change
to comment wording".
* Incorporated changes of unmerged upstream PR 133 by sm3142: "removed
spurious newlines from log messages".

# Version v1.2.0

* Incorporated changes of unmerged upstream PR 126 by Code-Hex: "Solved the
 problem for never return the last line if it's not followed by a newline".
* Incorporated changes of unmerged upstream PR 131 by StoicPerlman: "Remove
deprecated os.SEEK consts". The changes bumped the minimal supported Go
release to 1.9.

# Version v1.1.0

* migration to go modules.
* release of master branch of the dormant upstream, because it contains
fixes and improvement no present in the tagged release.

