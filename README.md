# LlvmConfigure

**LlvmConfigure** is a lightweight helper utility that generates a ready-to-use build directory and `Makefile` for compiling LLVM IR (`.ll`) source files.

The tool is designed primarily for academic use in **Compiler Construction** courses where student projects **emit LLVM IR in text format** as a backend target. It automates the boilerplate setup required to assemble, link, and produce executables from `.ll` files, allowing students to focus on compiler implementation rather than build configuration.

LlvmConfigure simplifies the workflow by:

* Creating a standardized build folder structure
* Generating a `Makefile` configured for the LLVM toolchain
* Supporting straightforward compilation of textual LLVM IR into native binaries

The tool is designed to be used mainly on **Linux** systems, although it may work on other platforms depending on the available LLVM toolchain and environment configuration.

This makes it particularly suitable for educational environments and rapid prototyping scenarios involving LLVM-based code generation.

## Project layout

- `src/` — all Go source code and module (`go.mod`)
- `scripts/build_release.sh` — release build script
- `Makefile` — root shortcuts for build/run/release/clean
- `LLVMSources.txt` — source list used by the tool

## Quick start

Build from repository root:

```bash
make build
```

Run directly with Go (from root):

```bash
make run -- -B build -S .
```

Build release binary from root:

```bash
make release
```

## Direct Go commands

If you want to build manually:

```bash
cd src
go build -o ../llvm-configure ./cmd/llvm-configure
```
