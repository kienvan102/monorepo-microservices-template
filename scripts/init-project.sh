#!/usr/bin/env bash
# init-project.sh turns a fresh copy of this template into a new project:
# it renames every Go module path to the new project's, resets the template's
# backlog, points git at your own repo, starts a fresh git history, and
# optionally removes the example service and deployment. Run it
# once, from the repo root, right after copying the template. Works with the
# bash 3.2 that ships with macOS.
set -euo pipefail

EXAMPLE_DIRS="services/mongoanalyzer cmd/db-analyzer"
LOCAL_ARTIFACTS="bin output"

usage() {
	cat <<'EOF'
Usage: bash scripts/init-project.sh [<new-module-path>] [--remote <url>]
                                    [--strip-examples] [--yes] [--dry-run]

Renames this template's Go module path to your project's. Every module in
the repo (commonlib/, core/, services/*, cmd/*) is renamed to start with the
new path, e.g. <new-module-path>/core.

The path is usually your repo URL without https://, for example
  github.com/your-account/your-repo
If you leave it out, the script asks for it.

It also makes the clone your own repository: git's origin is set to your repo
(https://<new-module-path>.git unless --remote is given) and the template's
git history is replaced by a single initial commit. That deletes .git and
cannot be undone, so the script asks you to confirm first.

Options:
  --remote <url>    URL to use as git's origin instead of
                    https://<new-module-path>.git (e.g. an SSH URL)
  --yes             don't ask for confirmation (required when not run from
                    a terminal)
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
assume_yes=false
remote_url=""
while [ $# -gt 0 ]; do
	arg=$1
	shift
	case "$arg" in
	--strip-examples) strip_examples=true ;;
	--dry-run) dry_run=true ;;
	--yes | -y) assume_yes=true ;;
	--remote)
		[ $# -gt 0 ] || die "--remote needs a URL"
		remote_url=$1
		shift
		;;
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
# A path equal to the current one means the rename was already done (for
# example by an earlier run); the rename and backlog reset are then skipped
# and the git setup below still runs.
rename_needed=true
[ "$new" != "$old" ] || rename_needed=false

[ -n "$remote_url" ] || remote_url="https://$new.git"
command -v git >/dev/null || die "git is required"
git rev-parse --git-dir >/dev/null 2>&1 || die "this directory isn't a git repository"
[ "$remote_url" != "$(git remote get-url origin 2>/dev/null || true)" ] ||
	die "origin already points at $remote_url; this project looks initialized already"

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
	if $rename_needed; then
		echo "Would replace  $old"
		echo "          with $new"
		echo "in:"
		list_files_with_old | tr '\0' '\n' | sed 's|^\./|  |'
		echo
		[ -f docs/backlog.md ] && echo "Would reset docs/backlog.md to its header"
	else
		echo "Module path is already $new; would skip the rename and the backlog reset"
	fi
	[ -z "$remove_dirs" ] || echo "Would delete:$remove_dirs"
	if $strip_examples; then
		echo "Would drop the examples from go.work and delete go.work.sum"
	fi
	echo "Would set git origin to $remote_url"
	echo "Would delete .git and start a fresh history with one initial commit"
	exit 0
fi

if ! $assume_yes; then
	[ -t 0 ] || die "not running from a terminal; pass --yes to confirm"
	echo "This will rewrite files in place and set git origin to $remote_url"
	echo "It will also DELETE .git and replace the history with one initial commit."
	echo "That cannot be undone."
	read -r -p "Continue? [y/N] " answer
	case "$answer" in
	y | Y | yes | YES) ;;
	*) die "aborted; nothing was changed" ;;
	esac
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

if $rename_needed; then
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
else
	echo "module path is already $new; skipped the rename and the backlog reset"
fi

echo
echo "Checking that every module still builds..."
build_failed=false
embed_missing=false
# Same module set as the root Makefile's MODULES.
for m in commonlib core services/* cmd/*; do
	[ -f "$m/go.mod" ] || continue
	if out=$(cd "$m" && GOWORK=off go build ./... 2>&1); then
		echo "  ok   $m"
	elif echo "$out" | grep -q 'pattern .*: no matching files found'; then
		# //go:embed files (analysis scripts, plans) are gitignored on purpose,
		# so a fresh clone doesn't have them yet. That isn't a rename problem.
		echo "  todo $m (add the files it embeds, then build it)"
		echo "$out" | sed 's/^/         /'
		embed_missing=true
	else
		echo "  FAIL $m"
		echo "$out" | sed 's/^/         /'
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

echo
if $build_failed; then
	echo "Some modules failed to build; see above."
	echo "git was left untouched. Fix the errors, then run:"
	echo "  rm -rf .git && git init -b main && git remote add origin $remote_url"
	exit 1
fi

rm -rf .git
git init -q -b main
git remote add origin "$remote_url"
git add -A
if git commit -q -m "chore: initial commit"; then
	echo "started a fresh git history with one initial commit"
else
	echo "git init done, but the commit failed (is user.name/user.email set?)."
	echo "Set them, then run: git add -A && git commit -m 'chore: initial commit'"
fi
echo "git origin is now $remote_url"
if $embed_missing; then
	echo "Renamed. Some modules need files that are gitignored; see the todo lines above."
fi
echo "Done. Create the empty repo at $remote_url if you haven't, then: git push -u origin main"
