// This go.mod exists to exclude the toolchain directory from the parent
// module's `go test ./...` and `go mod tidy` commands.
module toolchain
