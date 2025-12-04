# compile-docker

Build the project using Docker. Only builds the builder stage to verify Go code compilation.

On compile error, the output will show error details.

```bash
docker build --target builder --progress=plain -t xi:compile-check -f Dockerfile .
```
