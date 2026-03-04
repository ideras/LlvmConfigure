# LlvmConfigure

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![Platform](https://img.shields.io/badge/Platform-Linux-blue)
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
* ✅ Using the system LLVM toolchain (`llvm-as`, `llc`, `lld`)
* ✅ Auto-detects toolchain binaries and libc (glibc or musl), stores the resolved paths in a JSON configuration file, and references them when generating the Makefile to guarantee reproducible builds.
* ✅ Supporting clean rebuild workflows

Perfect for:

* Compiler courses
* Academic environments
* Rapid prototyping of LLVM-based backends
* Automated evaluation pipelines

---

## 📦 Features

* Lightweight CLI tool written in Go
* Generates reproducible build environments
* Works with textual LLVM IR (`.ll`)
* Minimal dependencies
* Built and tested on Linux. Other operating systems may work but are unverified.
* MIT Licensed

---

## 🏗 Project Structure

```
.
├── src/                  # Go source code
│   ├── go.mod
│   └── cmd/llvm-configure
├── scripts/build_release.sh
├── Makefile              # Root build shortcuts
└── README.md
```

---

## ⚙️ Requirements

* Linux environment
* LLVM toolchain installed (`clang`, `llc`, `llvm-as`)
* Go 1.21+ (for building from source)
* GNU Make

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
make run -- -B build -S .
```

Build release binary:

```bash
make release
```

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
