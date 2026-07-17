#!/usr/bin/env sh
set -eu

archive_version="${1:-}"
dist_dir="${2:-}"

if [ -z "$archive_version" ]; then
	echo "usage: sh scripts/release-archive.sh VERSION [DIST_DIR]" >&2
	exit 2
fi

case "$archive_version" in
	*[!A-Za-z0-9._-]*)
		echo "qdecimal: refusing unsafe archive version: $archive_version" >&2
		exit 2
		;;
esac

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
module_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)

if [ -z "$dist_dir" ]; then
	dist_dir="${TMPDIR:-/tmp}/qdecimal-release-${archive_version}-$$"
fi

case "$dist_dir" in
	/*) ;;
	*)
		echo "qdecimal: release dist dir must be absolute: $dist_dir" >&2
		exit 2
		;;
esac

dist_base=$(basename -- "$dist_dir")
case "$dist_base" in
	qdecimal-release-*) ;;
	*)
		echo "qdecimal: refusing unsafe release dist dir: $dist_dir" >&2
		exit 2
		;;
esac

case "$dist_dir" in
	"$module_dir"|"$module_dir"/*)
		echo "qdecimal: refusing to stage release inside module source: $dist_dir" >&2
		exit 2
		;;
esac

package_dir="qdecimal-${archive_version}"

rm -rf "$dist_dir"
mkdir -p "$dist_dir/$package_dir"
cp -R "$module_dir"/. "$dist_dir/$package_dir"/

(
	cd "$dist_dir"
	tar -czf "$package_dir.tar.gz" "$package_dir"
	rm -rf "$package_dir"
	sha256sum "$package_dir.tar.gz" > checksums.txt
	sha256sum -c checksums.txt >/dev/null
)

printf '%s\n' "$dist_dir"
