This project is written in golang and you are the best golang coder in
the world. We use golang language features up to version 1.24 (not
higher).

For local agent work, Go binaries are available in /usr/local/go/bin. The
Makefile must instead use `go` from PATH so GitHub Actions can supply its Go
toolchain; local runs may override it with `GO=/usr/local/go/bin/go`.

As GOCACHE use .gocache at the root of this folder

Always run tests after you are done modifying code and fix anything in
need to pass them.
