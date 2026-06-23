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

  # Measure coverage only for non-main packages; entry-point main() is not
  # unit-testable by convention and would otherwise drag the aggregate down.
  local coverpkgs
  coverpkgs=$(go list -f '{{if ne .Name "main"}}{{.ImportPath}}{{end}}' ./... \
    2>/dev/null | tr '\n' ',')
  coverpkgs="${coverpkgs%,}"

  if [[ -z "$coverpkgs" ]]; then
    warn "No non-main packages found; skipping coverage check."
    return
  fi

  local coverfile
  coverfile=$(mktemp /tmp/dreamland-cov.XXXXXX)
  trap 'rm -f "$coverfile"' RETURN

  go test -coverprofile="$coverfile" -coverpkg="$coverpkgs" ./... > /dev/null 2>&1 || true

  # --- Aggregate coverage ---
  local total_line
  total_line=$(go tool cover -func="$coverfile" | grep '^total:' || true)
  if [[ -z "$total_line" ]]; then
    fail "Could not determine total coverage — no coverage data produced."
  fi
  local total
  total=$(echo "$total_line" | awk '{print $NF}' | tr -d '%')

  local below90 below95
  below90=$(awk "BEGIN { print ($total < 90) ? 1 : 0 }")
  below95=$(awk "BEGIN { print ($total < 95) ? 1 : 0 }")

  if [[ "$below90" == "1" ]]; then
    echo "  Aggregate coverage: ${total}%"
    echo "  Under-covered functions:"
    go tool cover -func="$coverfile" | grep -v '^total:' | awk '{
      pct = $NF; gsub(/%/, "", pct); pct += 0
      if (pct < 90) printf "    %s %s (%s)\n", $1, $2, $NF
    }' || true
    fail "Aggregate coverage is ${total}% — must be ≥ 90% before merging."
  fi

  if [[ "$below95" == "1" ]]; then
    warn "Coverage is ${total}% — consider raising it above 95%."
  else
    pass "Coverage: ${total}%"
  fi

  # --- Per-package floor (80%) —
  # Compute per-package coverage by grouping function entries by package.
  local pkg_failures=0
  while IFS= read -r pkg_line; do
    local pkg pct
    pkg=$(echo "$pkg_line" | awk '{print $1}')
    pct=$(echo "$pkg_line" | awk '{print $2}')
    local below80
    below80=$(awk "BEGIN { print ($pct < 80) ? 1 : 0 }")
    if [[ "$below80" == "1" ]]; then
      echo -e "  ${RED}LOW:${NC} $pkg — ${pct}% (minimum 80%)" >&2
      pkg_failures=1
    fi
  done < <(go tool cover -func="$coverfile" | grep -v '^total:' | awk '{
      # $1 is "pkg/sub/file.go:line:" — strip ":line:" then get dirname
      ref = $1; sub(/:[0-9]+:$/, "", ref)
      n = split(ref, parts, "/")
      pkg = ""; for (i=1; i<n; i++) pkg = (i==1) ? parts[i] : pkg "/" parts[i]
      if (pkg == "") pkg = ref
      pct = $NF; gsub(/%/, "", pct); pct += 0
      sum[pkg] += pct; count[pkg]++
    }
    END {
      for (p in sum) if (p != "") printf "%s %.1f\n", p, sum[p]/count[p]
    }' | sort)

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

  # Use awk to do look-behind without bash arrays (portable; no mapfile needed).
  local issues
  issues=$(find . -name '*.go' ! -name '*_test.go' \
      ! -path '*/vendor/*' ! -path '*/testdata/*' | sort | while IFS= read -r gofile; do
    awk '
      { lines[FNR] = $0 }
      END {
        for (i = 1; i <= FNR; i++) {
          if (lines[i] ~ /^(func|type|var) [A-Z]/) {
            found = 0
            for (j = i-1; j >= 1 && j >= i-3; j--) {
              if (lines[j] ~ /^[[:space:]]*\/\//) { found = 1; break }
              # Non-blank, non-comment line above stops the look-behind
              if (lines[j] !~ /^[[:space:]]*$/) break
            }
            if (!found) printf "%s:%d: %s\n", FILENAME, i, lines[i]
          }
        }
      }
    ' "$gofile"
  done)

  if [[ -n "$issues" ]]; then
    while IFS= read -r line; do
      echo -e "  ${RED}MISSING godoc:${NC} $line" >&2
    done <<< "$issues"
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
