The doloscmd.proto is actually a copy of

src/platform/dolos/tools/doloscmd/proto/doloscmd.proto

and this local copy needs to be kept in sync with that
file.

Generating the pb.go file is

go generate ./...

in this directory.
