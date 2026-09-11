# LlvmConfigure

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)
![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS-blue)
![LLVM](https://img.shields.io/badge/LLVM-IR-orange)
![Build](https://img.shields.io/badge/Build-Makefile-informational)

**LlvmConfigure** is a lightweight CLI utility that automatically generates a ready-to-use build directory and `Makefile` for compiling **LLVM IR (`.ll`) source files** into native executables.

It is designed primarily for **Compiler Construction courses**, where student projects emit textual LLVM IR as a backend target.

Instead of spending time configuring toolchains and writing repetitive build scripts, students and instructors can focus on compiler implementation.

---

## 🚀 Why LlvmConfigure?

When working with LLVM IR in academic settings:

* Students generate `.ll` files
* They need to assemble, link, and produce executables
* Boilerplate build configuration becomes repetitive and error-prone

`LlvmConfigure` automates this process by:

* ✅ Creating a standardized build folder structure
* ✅ Generating a properly configured `Makefile`
* ✅ Using the system LLVM toolchain (`llvm-as`, `llc`, plus a platform linker driver)
* ✅ Auto-detecting toolchain binaries, storing the resolved paths and the native **target triple** in a JSON configuration file, and referencing them when generating the `Makefile` to guarantee reproducible builds
* ✅ Supporting clean rebuild workflows

Perfect for:

* Compiler courses
* Academic environments
* Rapid prototyping of LLVM-based backends
* Automated evaluation pipelines

---

## 🖥 Supported Platforms

| Host | Target scope | Linker driver | Object format |
|------|-------------|---------------|---------------|
| Linux (amd64/arm64) | Native only | `lld` (GNU flavor) | ELF |
| macOS (Intel & Apple Silicon) | Native only | `clang` | Mach-O |

* Builds are **native only**: a macOS arm64 host produces an arm64 Mach-O executable, a macOS x86_64 host produces an x86_64 Mach-O executable, and a Linux host produces a Linux executable. Universal binaries, `lipo`, and cross-compilation are not supported.
* On macOS the linker driver is `clang`, which selects SDK startup behavior and `libSystem`. Raw `lld` is **not** needed on macOS.

---

## 📦 Features

* Lightweight CLI tool written in Go
* Generates reproducible build environments
* Works with textual LLVM IR (`.ll`)
* Minimal dependencies
* Built and tested on Linux and macOS
* MIT Licensed

---

## 🏗 Project Structure

```
.
├── src/                  # Go source code
│   ├── go.mod
│   └── cmd/llvm-configure
├── scripts/              # Release and smoke-test scripts
├── .github/workflows/    # CI (Linux + macOS arm64/x86_64)
├── Makefile              # Root build shortcuts
└── README.md
```

---

## ⚙️ Requirements

* **Linux:** GNU libc (or musl for static builds), LLVM toolchain (`llvm-as`, `llc`, `lld`)
* **macOS:**
  * Xcode Command Line Tools:

    ```bash
    xcode-select --install
    ```

  * Homebrew LLVM:

    ```bash
    brew install llvm
    ```

  * No separate `brew install lld` is required: Homebrew LLVM does not ship `lld` by default, and `lld` is unnecessary for the supported Darwin path.
* Go 1.25+ (for building from source; matches `src/go.mod`)
* GNU Make

### Optional: manual `PATH` configuration

Manual `PATH` configuration is **optional** — the application can query Homebrew and `xcrun` directly. If you prefer the tools on your `PATH` anyway:

```bash
export PATH="$(brew --prefix llvm)/bin:$PATH"
```

---

## 🔧 Installation

### Option 1 — Build from Source

From repository root:

```bash
make build
```

### Option 2 — Manual Go Build

```bash
cd src
go build -o ../llvm-configure ./cmd/llvm-configure
```

---

## ▶️ Usage

Run via Make:

```bash
make run ARGS="-B build -S ."
```

Check whether the required LLVM tools are installed (on macOS this also validates clang, the macOS SDK, and the target triple):

```bash
make run ARGS="--check-llvm"
```

Check whether GNU libc is installed and contains the required startup object files (Linux only):

```bash
make run ARGS="--check-libc"
```

Check whether musl libc is installed (Linux only):

```bash
make run ARGS="--check-musl"
```

Refresh the LLVM paths (and, on macOS, the target triple) in `~/.llvm-configure/config.json` from the current system:

```bash
make run ARGS="--scan-llvm"
```

Scan for GNU libc and its dynamic linker, then update their paths in `~/.llvm-configure/config.json` (Linux only):

```bash
make run ARGS="--scan-libc"
```

### macOS behavior of libc/musl options

| Option | macOS behavior | Exit status |
|--------|----------------|-------------|
| `--check-llvm` | Validates `llvm-as`, `llc`, clang, the macOS SDK, and the stored/detected target triple | `0` when valid, `1` otherwise |
| `--scan-llvm` | Discovers tools and the native target triple, then persists both | `0` when saved, `1` otherwise |
| `--check-libc` | Prints `not applicable on macOS` | `0` |
| `--scan-libc` | Prints `not applicable on macOS`; does **not** modify the config | `0` |
| `--check-musl` | Prints `not applicable on macOS` | `0` |
| `--with-musl` | **Rejected**: unsupported on macOS | nonzero |

`--check-llvm` prints the status of each configured path and any fallback discovered in `PATH`. Dependency checks can be combined and exit with status `0` only when every requested dependency is available. Scan options can also be combined; combined checks do not fail solely because a Linux-only check is not applicable on Darwin.

`--check-llvm` prints the status of each configured path and any fallback discovered in `PATH`, including Homebrew, `xcrun --find clang`, and SDK availability.

### Configuration file

`~/.llvm-configure/config.json` stores the resolved toolchain. On macOS it looks like:

```json
{
  "llvm": {
    "llvm_as": "/opt/homebrew/opt/llvm/bin/llvm-as",
    "llc": "/opt/homebrew/opt/llvm/bin/llc",
    "lld": "",
    "clang": "/usr/bin/clang"
  },
  "target": {
    "triple": "arm64-apple-macosx14.0.0",
    "deployment_target": "",
    "detected_by": "/usr/bin/clang"
  },
  "libc": {
    "use_musl": false,
    "path": "",
    "dyn_linker_path": ""
  }
}
```

* `target.triple` is detected from the selected clang (`clang -print-target-triple`), the authority for the effective native target, and is **validated before every macOS build**.
* `target.deployment_target` is an optional macOS compatibility policy. When empty, the toolchain's default macOS target is used. When set (e.g. `13.0`), it is validated, normalized into the code-generation triple, and passed to clang as `-mmacosx-version-min`.
* If the stored target becomes stale or no longer matches the current native clang toolchain, a build fails with an actionable message; run `--scan-llvm` to refresh it.
* A normal build uses a valid stored target. If none is stored, the build detects the target in memory but does not silently rewrite the configuration — run `--scan-llvm` to persist it.
* Configuration files written before the `clang`/`target` sections existed continue to load unchanged.
* Stored Linux `libc` paths are ignored on macOS.
* Tool paths can be overridden manually by editing the configuration; configured paths always take precedence over discovery.

Build release binary:

```bash
make release
```

### Release binaries

CI publishes architecture-named binaries for every push to `main` and pull request as workflow artifacts:

* `llvm-configure-linux-amd64`
* `llvm-configure-darwin-arm64`
* `llvm-configure-darwin-amd64`

Pushing a tag `v*` (e.g. `v1.0.0`) additionally publishes them as assets of an automatically generated GitHub release. The macOS binaries must be run on a matching-architecture Mac; the tool binary is cross-buildable, but the LLVM projects it generates still build natively on their target host.

The tool will:

1. Generate a build directory
2. Create a configured `Makefile`
3. Prepare compilation targets for LLVM IR files

---

## 🧪 Example Workflow

Assume your compiler generates:

```
program.ll
```

Run LlvmConfigure to generate the build system, then:

```bash
make
```

This will assemble and link the IR into a native executable using your system LLVM toolchain.

---

## 🎓 Designed for Education

LlvmConfigure is especially useful in:

* Compiler Construction
* Programming Languages
* Systems Programming courses

---

## 📄 License

This project is licensed under the **MIT License**.

You are free to use, modify, and distribute it under the terms of the MIT license.

---

## 🔎 Keywords

LLVM, LLVM IR, Compiler Construction, Code Generation, Backend, Makefile Generator, Build Automation, Systems Programming, Programming Languages, Education Tooling