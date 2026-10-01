#!/usr/bin/env bash
# init-project.sh turns a fresh copy of this template into a new project:
# it renames every Go module path to the new project's, resets the template's
# backlog, and optionally removes the example service and deployment. Run it
# once, from the repo root, right after copying the template. It deletes
# itself when done. Works with the bash 3.2 that ships with macOS.
set -euo pipefail

EXAMPLE_DIRS="services/mongoanalyzer cmd/db-analyzer"
LOCAL_ARTIFACTS="bin output"

usage() {
	cat <<'EOF'
Usage: bash scripts/init-project.sh [<new-module-path>] [--strip-examples] [--dry-run]

Renames this template's Go module path to your project's. Every module in
the repo (commonlib/, core/, services/*, cmd/*) is renamed to start with the
new path, e.g. <new-module-path>/core.

The path is usually your repo URL without https://, for example
  github.com/your-account/your-repo
If you leave it out, the script asks for it.

Options:
  --strip-examples  also remove the example service (services/mongoanalyzer)
                    and deployment (cmd/db-analyzer), leaving only
                    commonlib/ and core/
  --dry-run         print what would change, without writing anything
  -h, --help        show this help
EOF
}

die() {
	echo "error: $*" >&2
	exit 1
}

# module_path_from_remote prints origin's URL as a Go module path
# (https://host/a/b.git or git@host:a/b.git -> host/a/b), or nothing.
module_path_from_remote() {
	local url
	url=$(git remote get-url origin 2>/dev/null) || return 0
	url=${url%.git}
	case "$url" in
	https://* | http://*) url=${url#*://} ;;
	ssh://*) url=${url#ssh://}; url=${url#*@} ;;
	*@*:*) url=${url#*@}; url=${url/://} ;;
	*) return 0 ;;
	esac
	echo "$url"
}

new=""
strip_examples=false
dry_run=false
for arg in "$@"; do
	case "$arg" in
	--strip-examples) strip_examples=true ;;
	--dry-run) dry_run=true ;;
	-h | --help) usage; exit 0 ;;
	-*) usage >&2; die "unknown option: $arg" ;;
	*)
		[ -z "$new" ] || die "only one module path can be given (got '$new' and '$arg')"
		new=$arg
		;;
	esac
done

[ -f go.work ] && [ -f core/go.mod ] || die "run this from the repo root (the directory containing go.work)"

old=$(awk '$1 == "module" { print $2; exit }' core/go.mod)
case "$old" in
*/core) old=${old%/core} ;;
*) die "core/go.mod's module path '$old' doesn't end in /core; can't tell the current prefix" ;;
esac

if [ -z "$new" ]; then
	[ -t 0 ] || { usage >&2; die "no module path given"; }
	echo "This renames the template's Go module path to your project's."
	echo "Current path: $old"
	echo
	echo "The new path is usually your repo URL without https://,"
	echo "e.g. github.com/your-account/your-repo"
	echo
	suggestion=$(module_path_from_remote)
	# A clone whose origin still points at the template suggests the old path.
	[ "$suggestion" != "$old" ] || suggestion=""
	if [ -n "$suggestion" ]; then
		echo "Detected from git remote: $suggestion"
		read -r -p "New module path [$suggestion]: " new
		new=${new:-$suggestion}
	else
		read -r -p "New module path: " new
	fi
fi

[ -n "$new" ] || die "no module path given"
echo "$new" | grep -Eq '^[A-Za-z0-9._~-]+(/[A-Za-z0-9._~-]+)*$' ||
	die "'$new' isn't a valid module path (letters, digits, . _ ~ - separated by single /, no spaces, no trailing /)"
[ "$new" != "$old" ] || die "the module path is already '$old'; this project looks initialized already"

# Examples are removed before rewriting, so their files aren't rewritten first.
remove_dirs=""
for d in $LOCAL_ARTIFACTS; do
	[ -e "$d" ] && remove_dirs="$remove_dirs $d"
done
if $strip_examples; then
	for d in $EXAMPLE_DIRS; do
		[ -e "$d" ] && remove_dirs="$remove_dirs $d"
	done
fi

# list_files_with_old prints every text file mentioning the old path, NUL-separated.
list_files_with_old() {
	local exclude="--exclude-dir=.git"
	for d in $remove_dirs; do
		exclude="$exclude --exclude-dir=$(basename "$d")"
	done
	# grep exits 1 when nothing matches, which isn't an error here.
	grep -rlIF --null $exclude -- "$old" . || true
}

if $dry_run; then
	echo "Dry run: nothing will be written."
	echo
	echo "Would replace  $old"
	echo "          with $new"
	echo "in:"
	list_files_with_old | tr '\0' '\n' | sed 's|^\./|  |'
	echo
	[ -f docs/backlog.md ] && echo "Would reset docs/backlog.md to its header"
	[ -z "$remove_dirs" ] || echo "Would delete:$remove_dirs"
	if $strip_examples; then
		echo "Would drop the examples from go.work and delete go.work.sum"
	fi
	exit 0
fi

for d in $remove_dirs; do
	rm -rf -- "$d"
	echo "deleted $d/"
done

if $strip_examples; then
	for d in $EXAMPLE_DIRS; do
		go work edit -dropuse="./$d"
	done
	rm -f go.work.sum
	echo "dropped the examples from go.work"
fi

changed_go_files=""
count=0
while IFS= read -r -d '' f; do
	OLD="$old" NEW="$new" perl -pi -e 's/\Q$ENV{OLD}\E/$ENV{NEW}/g' "$f"
	count=$((count + 1))
	case "$f" in *.go) changed_go_files="$changed_go_files $f" ;; esac
done < <(list_files_with_old)
echo "renamed $old -> $new in $count files"

# The new path can sort differently from the old one within an import block.
[ -z "$changed_go_files" ] || gofmt -w $changed_go_files

# Keep everything above the first "## " heading: the title and intro.
if [ -f docs/backlog.md ]; then
	awk '/^## /{ exit } { print }' docs/backlog.md >docs/backlog.md.tmp
	mv docs/backlog.md.tmp docs/backlog.md
	echo "reset docs/backlog.md"
fi

echo
echo "Checking that every module still builds..."
build_failed=false
# Same module set as the root Makefile's MODULES.
for m in commonlib core services/* cmd/*; do
	[ -f "$m/go.mod" ] || continue
	if (cd "$m" && GOWORK=off go build ./...); then
		echo "  ok   $m"
	else
		echo "  FAIL $m"
		build_failed=true
	fi
done

if $strip_examples; then
	leftovers=$(grep -rnIE --exclude-dir=.git 'db-analyzer|mongoanalyzer' . | grep -v '^\./scripts/' || true)
	if [ -n "$leftovers" ]; then
		echo
		echo "To do: these lines still mention the removed examples; update them by hand:"
		echo "$leftovers" | sed 's|^\./|  |'
	fi
fi

# This script is only needed once.
self=${BASH_SOURCE[0]}
rm -f -- "$self"
rmdir "$(dirname "$self")" 2>/dev/null || true

echo
if $build_failed; then
	echo "Done, but some modules failed to build; see above."
	exit 1
fi
echo "Done. Review the changes (git diff --stat) and commit."
