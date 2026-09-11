#!/bin/bash
# Darwin integration smoke test for llvm-configure.
#
# Usage: scripts/darwin_smoke_test.sh <path-to-binary> [arm64|x86_64]
#
# Verifies, on a native macOS host:
#   1. Xcode Command Line Tools / SDK availability
#   2. Homebrew LLVM discovery without llvm-as/llc in PATH
#   3. --scan-llvm persists llvm_as, llc, clang and target.triple
#   4. --check-llvm passes
#   5. A generated build directory compiles and runs
#   6. Output executables are Mach-O of the expected native architecture
#   7. A libc-using fixture links through libSystem
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BIN="${1:?usage: darwin_smoke_test.sh <binary> [arch]}"
EXPECTED_ARCH="${2:-$(uname -m)}"
BREW="${BREW:-$(command -v brew || true)}"

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }

[ "$(uname -s)" = "Darwin" ] || fail "must run on macOS"
[ -x "$BIN" ] || fail "binary $BIN is not executable"

# 1. Command Line Tools / SDK
xcrun --find clang >/dev/null || {
    echo "Install Xcode Command Line Tools with: xcode-select --install" >&2
    exit 1
}
xcrun --sdk macosx --show-sdk-path >/dev/null || fail "no macOS SDK found"
pass "macOS SDK available"

# 2. Homebrew LLVM
# brew --prefix llvm exits 0 and prints the expected opt path even when
# the formula is not installed, so decide from the llvm-as binary, not
# from the prefix command's exit status.
if [ -z "$BREW" ]; then
    fail "Homebrew (brew) not found; install it from https://brew.sh"
fi
LLVM_PREFIX="$("$BREW" --prefix llvm 2>/dev/null || true)"
if [ -z "$LLVM_PREFIX" ] || [ ! -x "$LLVM_PREFIX/bin/llvm-as" ]; then
    echo "Installing llvm via Homebrew (this may take a while)..."
    "$BREW" install llvm
fi
LLVM_PREFIX="$("$BREW" --prefix llvm)"
[ -x "$LLVM_PREFIX/bin/llvm-as" ] || fail "llvm-as missing from Homebrew LLVM after install"
[ -x "$LLVM_PREFIX/bin/llc" ] || fail "llc missing from Homebrew LLVM after install"
[ -x "$LLVM_PREFIX/bin/clang" ] || fail "clang missing from Homebrew LLVM after install"
pass "Homebrew LLVM at $LLVM_PREFIX"

# Ensure Homebrew LLVM is NOT on PATH so discovery must use the fallbacks.
# LLVM is keg-only, so its binaries never appear in /usr/local/bin (Intel)
# or /opt/homebrew/bin (Apple Silicon); including those directories keeps
# brew and xcrun reachable while excluding llvm-as/llc.
SAVED_PATH="$PATH"
PATH="/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/opt/homebrew/bin"
export PATH

# 3. Scan (isolated HOME so the test never mutates the real config)
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# The tool reads ~/.llvm-configure/config.json; redirect HOME for isolation.
OLD_HOME="$HOME"
HOME="$WORK/home"
mkdir -p "$HOME"
export HOME

"$BIN" --scan-llvm || fail "--scan-llvm failed"
CONFIG="$HOME/.llvm-configure/config.json"
[ -f "$CONFIG" ] || fail "--scan-llvm did not write the configuration file"
for key in llvm_as llc clang triple; do
    grep -q "\"$key\"" "$CONFIG" || fail "config is missing \"$key\""
done
grep -q '"lld": ""' "$CONFIG" || true # lld optional on Darwin
pass "--scan-llvm persisted tools and target"

"$BIN" --check-llvm || fail "--check-llvm failed"
pass "--check-llvm passed"

# 4. Libc/musl flags are not applicable on macOS
"$BIN" --check-libc || fail "--check-libc must exit 0 on macOS"
"$BIN" --check-musl || fail "--check-musl must exit 0 on macOS"
"$BIN" --scan-libc || fail "--scan-libc must exit 0 without modifying config"
if "$BIN" --with-musl -B "$WORK/build" -S "$WORK/sources" >/dev/null 2>&1; then
    fail "--with-musl must be rejected on macOS"
fi
pass "libc/musl flag semantics"

# 5. Build a minimal zero-exit program without libc references
mkdir -p "$WORK/sources"
cat > "$WORK/sources/zero.ll" <<'EOF'
define i32 @main() {
  ret i32 0
}
EOF

"$BIN" -B "$WORK/build" -S "$WORK/sources" "$WORK/sources/zero.ll" || fail "build configuration failed"
make -C "$WORK/build" || fail "make failed"
"$WORK/build/zero" || fail "generated executable returned nonzero"
[ "$(file -b "$WORK/build/zero" | grep -c Mach-O)" -ge 1 ] || fail "output is not Mach-O"
case "$(file -b "$WORK/build/zero")" in
    *arm64*|*aarch64*) OBSERVED=arm64 ;;
    *x86_64*) OBSERVED=x86_64 ;;
    *) fail "cannot determine architecture of output" ;;
esac
[ "$OBSERVED" = "$EXPECTED_ARCH" ] || fail "expected $EXPECTED_ARCH Mach-O, got $OBSERVED"
pass "Mach-O $OBSERVED executable runs"

# 6. libc-using fixture links through libSystem
cat > "$WORK/sources/hello.ll" <<'EOF'
@msg = private constant [7 x i8] c"hello\0A\00"

declare i32 @puts(ptr)

define i32 @main() {
  %m = getelementptr [7 x i8], ptr @msg, i32 0, i32 0
  %r = call i32 @puts(ptr %m)
  ret i32 0
}
EOF

"$BIN" -B "$WORK/build2" -S "$WORK/sources" "$WORK/sources/hello.ll" || fail "libc fixture configuration failed"
make -C "$WORK/build2" || fail "libc fixture build failed"
OUT="$("$WORK/build2/hello")"
[ "$OUT" = "hello" ] || fail "unexpected program output: $OUT"
otool -L "$WORK/build2/hello" | grep -q "libSystem" || fail "libc fixture should link against libSystem"
pass "libSystem linkage via clang"

# Restore environment.
PATH="$SAVED_PATH"
export PATH
HOME="$OLD_HOME"
export HOME
echo "Darwin smoke test OK"