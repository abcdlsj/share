# nestg

Pack `Go` binary to minimal `Docker Container`

## Quick Start

```bash
go install github.com/abcdlsj/share/go/nestg@latest
```

```bash
nestg -h
```

## Key Features

1. **Automatic Binary Name Detection**: Uses the Go module name or directory name to determine the binary name.

2. **Multi-stage Docker Builds**: Implements a two-stage build process for smaller final images:
   - Stage 1: Builds the Go binary using `golang:alpine`
   - Stage 2: Creates a minimal `scratch` image with only the necessary components

3. **Dynamic Dockerfile Generation**: Generates a Dockerfile on-the-fly based on the project and user inputs.

4. **Flexible Build Options**:
   - Custom `ldflags` support for optimized builds
   - Ability to expose ports
   - Custom image naming
   - Additional execution flags

5. **SSL Certificate Handling**: Copies SSL certificates to ensure HTTPS functionality in the final image.

6. **Dependency Management**: Automatically copies required shared libraries to the final image.

7. **Debug Mode**: Offers a debug option for testing and troubleshooting.

8. **User-friendly Output**: Provides colored console output for better readability.

9. **Temporary File Handling**: Creates and manages a temporary Dockerfile, cleaning up after the build process.

