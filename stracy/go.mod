module github.com/iimos/play/stracy

go 1.19

replace github.com/hugelgupf/go-strace => /proj/src/github.com/hugelgupf/go-strace

require (
	github.com/docker/go-units v0.5.0
	github.com/go-chi/chi v1.5.4
	github.com/hugelgupf/go-strace v0.0.0-20210320044838-ac8c2b116f12
	github.com/seccomp/libseccomp-golang v0.10.0
	golang.org/x/sync v0.1.0
	golang.org/x/sys v0.7.0
)
