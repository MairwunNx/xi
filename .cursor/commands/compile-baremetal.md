# compile-baremetal

Build the project directly with Go (expects dev container environment from `Dockerfile.dev`).

Compiles the binary to `/tmp` to verify the build without polluting the workspace.

```bash
go build -trimpath -o /tmp/ximanager ./program.go
```
