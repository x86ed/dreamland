#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m'

fail() { echo -e "${RED}FAIL:${NC} $*" >&2; exit 1; }
warn() { echo -e "${YELLOW}WARN:${NC} $*" >&2; }
pass() { echo -e "${GREEN}PASS:${NC} $*"; }

# ---------------------------------------------------------------------------
# Branch guard (task 7.1)
# Run only when MERGE_CHECK=1 or the current branch tracks main upstream.
# ---------------------------------------------------------------------------
branch_guard() {
  if [[ "${MERGE_CHECK:-0}" == "1" ]]; then
    return 0
  fi

  local upstream
  upstream=$(git rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null || true)
  if [[ "$upstream" != "origin/main" && "$upstream" != "main" ]]; then
    echo "pre-merge-check: not targeting main (upstream='${upstream:-none}'), skipping. Set MERGE_CHECK=1 to force."
    exit 0
  fi
}

# ---------------------------------------------------------------------------
# Task 2.1 / 2.2 — Unit tests
# ---------------------------------------------------------------------------
run_tests() {
  echo "==> Running unit tests..."
  if ! go test ./... 2>&1; then
    fail "Unit tests failed. Fix the failures above before merging."
  fi
  pass "All tests pass."
}

# ---------------------------------------------------------------------------
# Tasks 3.1 – 3.4 — Coverage enforcement
# ---------------------------------------------------------------------------
check_coverage() {
  echo "==> Checking test coverage..."

  local coverfile
  coverfile=$(mktemp /tmp/coverage.XXXXXX.out)
  trap 'rm -f "$coverfile"' RETURN

  # Run with coverage; suppress test output already shown by run_tests.
  go test -coverprofile="$coverfile" ./... > /dev/null 2>&1 || true

  # --- Aggregate coverage ---
  local total_line
  total_line=$(go tool cover -func="$coverfile" | grep '^total:' || true)
  if [[ -z "$total_line" ]]; then
    fail "Could not determine total coverage — no coverage data produced."
  fi
  local total
  total=$(echo "$total_line" | awk '{print $3}' | tr -d '%')

  # Use awk for float comparison (bash can't do floats natively)
  local below90 below95
  below90=$(awk "BEGIN { print ($total < 90) ? 1 : 0 }")
  below95=$(awk "BEGIN { print ($total < 95) ? 1 : 0 }")

  if [[ "$below90" == "1" ]]; then
    echo "  Aggregate coverage: ${total}%"
    # List under-covered packages before auto-remediation step
    echo "  Under-covered packages:"
    go tool cover -func="$coverfile" \
      | grep -v '^total:' \
      | awk -F'\t' '{
          split($3, a, "%"); cov=a[1]+0;
          split($1, f, ":");
          if (cov < 90) printf "    %s (%.1f%%)\n", f[1], cov
        }' | sort -u || true
    fail "Aggregate coverage is ${total}% — must be ≥ 90% before merging."
  fi

  if [[ "$below95" == "1" ]]; then
    warn "Coverage is ${total}% — consider raising it above 95%."
  else
    pass "Coverage: ${total}%"
  fi

  # --- Per-package floor (80%) ---
  local pkg_failures=0
  while IFS= read -r line; do
    [[ "$line" == total:* ]] && continue
    local pct
    pct=$(echo "$line" | awk '{print $3}' | tr -d '%')
    local pkg
    pkg=$(echo "$line" | awk '{print $1}')
    local below80
    below80=$(awk "BEGIN { print ($pct < 80) ? 1 : 0 }")
    if [[ "$below80" == "1" ]]; then
      echo -e "  ${RED}LOW:${NC} $pkg — ${pct}% (minimum 80%)" >&2
      pkg_failures=1
    fi
  done < <(go tool cover -func="$coverfile" | grep -v '^total:')

  if [[ "$pkg_failures" == "1" ]]; then
    fail "One or more packages are below the 80% per-package floor."
  fi
}

# ---------------------------------------------------------------------------
# Tasks 4.1 – 4.3 — Auto-remediation of missing tests
# ---------------------------------------------------------------------------
find_untested_packages() {
  echo "==> Checking for packages without test files..."

  local missing=0
  while IFS= read -r dir; do
    # Skip vendor, testdata, hidden dirs
    [[ "$dir" == *"/vendor/"* || "$dir" == *"/testdata/"* ]] && continue

    # Determine package name from source files (-h suppresses filename prefix)
    local pkgname
    pkgname=$(grep -h '^package ' "${dir}"/*.go 2>/dev/null | head -1 | awk '{print $2}' || true)

    # Skip executable (main) packages — they have no exported symbols to cover
    [[ "$pkgname" == "main" ]] && continue

    local has_test
    has_test=$(find "$dir" -maxdepth 1 -name '*_test.go' | head -1)
    if [[ -z "$has_test" ]]; then
      # Use pkgname for the filename so "." dirs don't produce "._test.go"
      local stub="${dir}/${pkgname}_test.go"
      echo "  Scaffolding $stub"

      {
        echo "package ${pkgname}_test"
        echo ""
        echo "// TODO: implement tests for package ${pkgname}."
        echo "// Exported symbols to cover:"
        grep -h -E '^func [A-Z]|^type [A-Z]|^var [A-Z]' "${dir}"/*.go 2>/dev/null \
          | sed 's/^/\/\/   /' || true
      } > "$stub"

      missing=1
    fi
  done < <(find . -type f -name '*.go' ! -name '*_test.go' \
              ! -path '*/vendor/*' ! -path '*/testdata/*' \
              | xargs -I{} dirname {} | sort -u)

  if [[ "$missing" == "1" ]]; then
    echo ""
    echo "  Test stubs have been generated for the packages above."
    echo "  Please implement the TODOs, then re-run MERGE_CHECK=1 bash scripts/pre-merge-check.sh"
    fail "Coverage check aborted: missing test files were scaffolded — implement the stubs."
  fi

  pass "All packages have test files."
}

# ---------------------------------------------------------------------------
# Tasks 5.1 / 5.2 — Godoc on exported symbols
# ---------------------------------------------------------------------------
check_godoc() {
  echo "==> Checking godoc on exported symbols..."

  local failures=0
  while IFS= read -r gofile; do
    # Read file into array for look-behind check
    mapfile -t lines < "$gofile"
    local n=${#lines[@]}
    for (( i=0; i<n; i++ )); do
      local line="${lines[$i]}"
      # Match exported func/type/var at the start of a line
      if [[ "$line" =~ ^(func|type|var)\ [A-Z] ]]; then
        # Check preceding 3 lines for a // comment
        local found_comment=0
        for (( j=i-1; j>=0 && j>=i-3; j-- )); do
          if [[ "${lines[$j]}" =~ ^[[:space:]]*//(.*) ]]; then
            found_comment=1
            break
          fi
          # Blank lines between comment and declaration are ok; non-blank non-comment stops search
          if [[ -n "${lines[$j]}" && ! "${lines[$j]}" =~ ^[[:space:]]*$ ]]; then
            break
          fi
        done
        if [[ "$found_comment" == "0" ]]; then
          echo -e "  ${RED}MISSING godoc:${NC} ${gofile}:$((i+1)): ${line}" >&2
          failures=1
        fi
      fi
    done
  done < <(find . -name '*.go' ! -name '*_test.go' \
              ! -path '*/vendor/*' ! -path '*/testdata/*')

  if [[ "$failures" == "1" ]]; then
    fail "Exported symbols above are missing godoc comments."
  fi
  pass "All exported symbols have godoc."
}

# ---------------------------------------------------------------------------
# Tasks 6.1 – 6.3 — Version bump via Git tags
# ---------------------------------------------------------------------------
parse_semver() {
  # Strips leading 'v', outputs "major minor patch" space-separated
  local tag="${1#v}"
  IFS='.' read -r major minor patch <<< "$tag"
  echo "${major:-0} ${minor:-0} ${patch:-0}"
}

check_version_bump() {
  echo "==> Checking version bump..."

  # Resolve current branch tag
  local current_tag
  current_tag=$(git describe --tags --abbrev=0 2>/dev/null || true)
  if [[ -z "$current_tag" ]]; then
    fail "No semver tag found on this branch. Create a tag (e.g., git tag v0.2.0) before merging."
  fi

  # Resolve main's latest tag; default to v0.0.0 if main has none
  local main_tag
  main_tag=$(git describe --tags --abbrev=0 main 2>/dev/null || echo "v0.0.0")

  echo "  Current tag: $current_tag"
  echo "  Main tag:    $main_tag"

  read -r cur_major cur_minor cur_patch <<< "$(parse_semver "$current_tag")"
  read -r main_major main_minor main_patch <<< "$(parse_semver "$main_tag")"

  if [[ "$current_tag" == "$main_tag" ]]; then
    fail "Version is unchanged ($current_tag). Bump the minor or major version before merging."
  fi

  if (( cur_major > main_major )); then
    # Major bump — also verify go.mod module path
    local expected_path
    expected_path="v${cur_major}"
    if ! grep -q "module .*/${expected_path}$\|module .*/${expected_path} " go.mod 2>/dev/null; then
      fail "Major version bumped to ${cur_major} but go.mod module path does not include /${expected_path}. Update the module directive in go.mod (e.g., 'module dreamland/${expected_path}')."
    fi
    pass "Major version bump: ${main_tag} → ${current_tag} (go.mod updated)."
    return
  fi

  if (( cur_major == main_major && cur_minor > main_minor )); then
    pass "Minor version bump: ${main_tag} → ${current_tag}."
    return
  fi

  # Only patch changed (or version regressed)
  fail "Version ${current_tag} is only a patch bump from ${main_tag}. A minor or major version increment is required before merging."
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
branch_guard

echo ""
echo "┌─────────────────────────────────────────┐"
echo "│        pre-merge quality gate           │"
echo "└─────────────────────────────────────────┘"
echo ""

find_untested_packages
run_tests
check_coverage
check_godoc
check_version_bump

echo ""
echo -e "${GREEN}All checks passed. Ready to merge.${NC}"
