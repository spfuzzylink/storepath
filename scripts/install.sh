#!/bin/sh
# Install a pinned public release without root privileges or executing downloads.
set -eu

storepath_script_dir=$(CDPATH= cd -P "$(dirname "$0")" && pwd)
storepath_repo_dir=$(CDPATH= cd -P "$storepath_script_dir/.." && pwd)
storepath_install_dir="$storepath_repo_dir/bin"
storepath_version=0.1.0
if [ -r "$storepath_repo_dir/VERSION" ]; then
  storepath_version=$(cat "$storepath_repo_dir/VERSION")
fi

storepath_die() {
  printf 'Storepath installer: %s\n' "$*" >&2
  exit 1
}

storepath_usage() {
  cat <<'USAGE'
Usage: sh scripts/install.sh [--version vX.Y.Z] [--dir DIRECTORY]

Install the pinned release from VERSION (default 0.1.0) into this repo's bin/.
Supported platforms: macOS and Linux, amd64 and arm64. No sudo is used.
An optional destination directory may contain spaces; quote it in your shell.

Downloads use HTTPS from github.com/spfuzzylink/storepath. The installer checks
the release's SHA256 manifest before extracting the binary. Checksums verify
download integrity; they are not a signed statement of provenance.
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      [ "$#" -ge 2 ] || storepath_die '--version requires vX.Y.Z'
      storepath_version=$2
      shift 2
      ;;
    --dir)
      [ "$#" -ge 2 ] && [ -n "$2" ] || storepath_die '--dir requires a directory'
      storepath_install_dir=$2
      shift 2
      ;;
    -h|--help)
      storepath_usage
      exit 0
      ;;
    *) storepath_die "unknown argument: $1 (use --help)" ;;
  esac
done

storepath_version=${storepath_version#v}
case "$storepath_version" in
  ''|*[!0-9.]*) storepath_die 'version must have the form vX.Y.Z (for example v0.1.0)' ;;
esac
if ! printf '%s\n' "$storepath_version" | LC_ALL=C grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'; then
  storepath_die 'version must have the form vX.Y.Z (for example v0.1.0)'
fi

case "$(uname -s)" in
  Darwin) storepath_os=darwin ;;
  Linux) storepath_os=linux ;;
  *) storepath_die 'unsupported operating system; release binaries support macOS and Linux' ;;
esac
case "$(uname -m)" in
  x86_64|amd64) storepath_arch=amd64 ;;
  arm64|aarch64) storepath_arch=arm64 ;;
  *) storepath_die 'unsupported architecture; release binaries support amd64 and arm64' ;;
esac

command -v curl >/dev/null 2>&1 || storepath_die 'curl is required'
command -v tar >/dev/null 2>&1 || storepath_die 'tar is required'
if command -v sha256sum >/dev/null 2>&1; then
  storepath_hash_tool=sha256sum
elif command -v shasum >/dev/null 2>&1; then
  storepath_hash_tool=shasum
else
  storepath_die 'SHA256 verification requires sha256sum or shasum'
fi

# Make relative destinations absolute so utility arguments cannot become options.
case "$storepath_install_dir" in
  /*) ;;
  *) storepath_install_dir="$(pwd)/$storepath_install_dir" ;;
esac
# A trailing slash or dot must not hide a final symlink from test -L.
while [ "$storepath_install_dir" != / ]; do
  case "$storepath_install_dir" in
    */) storepath_install_dir=${storepath_install_dir%/} ;;
    */.) storepath_install_dir=${storepath_install_dir%/.} ;;
    *) break ;;
  esac
done
storepath_destination="$storepath_install_dir/storepath"
[ ! -L "$storepath_install_dir" ] || storepath_die 'refusing a symlink destination directory'
[ ! -L "$storepath_destination" ] || storepath_die 'refusing to replace a symlink binary'
if [ -e "$storepath_destination" ] && [ ! -f "$storepath_destination" ]; then
  storepath_die 'the destination binary exists and is not a regular file'
fi

umask 077
mkdir -p "$storepath_install_dir" || storepath_die 'cannot create the destination directory'
storepath_temp=$(mktemp -d "$storepath_install_dir/.storepath-install.XXXXXXXX") || storepath_die 'cannot create a staging directory'
trap 'rm -rf "$storepath_temp"' 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

storepath_asset="storepath_${storepath_version}_${storepath_os}_${storepath_arch}.tar.gz"
storepath_base_url="https://github.com/spfuzzylink/storepath/releases/download/v${storepath_version}"
storepath_archive="$storepath_temp/$storepath_asset"
storepath_manifest="$storepath_temp/checksums.txt"

storepath_download() {
  # --disable must be first: ignore ~/.curlrc and its authentication/settings.
  curl --disable --fail --silent --show-error --location \
    --proto '=https' --proto-redir '=https' --connect-timeout 15 --max-time 120 \
    --output "$2" "$1"
}

printf 'Downloading Storepath v%s for %s/%s...\n' "$storepath_version" "$storepath_os" "$storepath_arch"
storepath_download "$storepath_base_url/checksums.txt" "$storepath_manifest" || storepath_die 'checksum manifest download failed; the existing binary was preserved'
storepath_download "$storepath_base_url/$storepath_asset" "$storepath_archive" || storepath_die 'release download failed; the existing binary was preserved'

# Accept exactly one entry for this exact asset name, never a substring match.
storepath_expected=$(LC_ALL=C awk -v asset="$storepath_asset" '
  $2 == asset || $2 == "*" asset {
    count++
    if (NF != 2 || length($1) != 64 || $1 !~ /^[0-9a-fA-F]+$/) bad=1
    digest=tolower($1)
  }
  END { if (count != 1 || bad) exit 1; print digest }
' "$storepath_manifest") || storepath_die 'the manifest must contain one valid SHA256 entry for the exact release asset'

if [ "$storepath_hash_tool" = sha256sum ]; then
  storepath_hash_output=$(sha256sum "$storepath_archive") || storepath_die 'SHA256 calculation failed'
else
  storepath_hash_output=$(shasum -a 256 "$storepath_archive") || storepath_die 'SHA256 calculation failed'
fi
storepath_actual=${storepath_hash_output%% *}
[ "$storepath_actual" = "$storepath_expected" ] || storepath_die 'SHA256 mismatch; the existing binary was preserved'

# Inspect only the expected member. Reject duplicate names, directories, and
# links. Extract to stdout so no archive paths can be written to the filesystem.
# GNU tar accepts environment options, including command execution hooks.
unset TAR_OPTIONS GZIP
storepath_members=$(tar -tzf "$storepath_archive" storepath) || storepath_die 'release archive does not contain the expected storepath binary'
[ "$storepath_members" = storepath ] || storepath_die 'release archive must contain exactly one top-level storepath binary'
storepath_member_details=$(tar -tvzf "$storepath_archive" storepath) || storepath_die 'cannot inspect the release binary'
case "$storepath_member_details" in
  -*) ;;
  *) storepath_die 'release binary must be a regular file, not a directory or link' ;;
esac
storepath_staged_binary="$storepath_temp/storepath"
tar -xOzf "$storepath_archive" storepath > "$storepath_staged_binary" || storepath_die 'release extraction failed; the existing binary was preserved'
[ -s "$storepath_staged_binary" ] || storepath_die 'release binary is empty'
chmod 0755 "$storepath_staged_binary" || storepath_die 'cannot make the staged binary executable'

# Staging is on the destination filesystem, so the final rename is atomic.
[ ! -L "$storepath_install_dir" ] && [ ! -L "$storepath_destination" ] || storepath_die 'destination changed into a symlink during installation'
if [ -e "$storepath_destination" ] && [ ! -f "$storepath_destination" ]; then
  storepath_die 'destination changed into a non-regular file during installation'
fi
mv -f "$storepath_staged_binary" "$storepath_destination" || storepath_die 'cannot replace the destination binary'
printf 'Installed Storepath v%s at %s\n' "$storepath_version" "$storepath_destination"
