#!/bin/sh
# Installs the app from a GitHub release; nsgo build wrote this script.
#
#   curl -fsSL https://github.com/@@GITHUB@@/releases/latest/download/install.sh | sh
#
# On macOS it copies the app from the release's disk image into
# /Applications, else ~/Applications; on Linux it installs the release's
# Arch package with pacman, or its Debian one with apt-get or dpkg.
#
# @@ENV@@_VERSION       another release's version, such as 1.2.0 (default: this script's)
# @@ENV@@_DOWNLOAD_URL  where the release's files are instead, such as a mirror
set -eu

repository=@@REPOSITORY@@
name=@@NAME@@
slug=@@SLUG@@
app=@@APP@@
version="${@@ENV@@_VERSION:-}"
[ -n "$version" ] || version=@@VERSION@@
version="${version#v}"
tag=@@TAG_PREFIX@@"$version"
base="${@@ENV@@_DOWNLOAD_URL:-https://github.com/$repository/releases/download/$tag}"

fail() {
  echo "$name: $*" >&2
  exit 1
}

command -v curl > /dev/null 2>&1 || fail "curl is required"

work="$(mktemp -d)"
cleanup() {
  [ ! -d "$work/mnt" ] || hdiutil detach -quiet "$work/mnt" > /dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT
trap 'exit 1' INT TERM

# download fetches a release's file and checks it against its .sha256.
download() {
  echo "Downloading $base/$1"
  curl -fL --progress-bar "$base/$1" -o "$work/$1" || fail "could not download $1 from release $tag"
  curl -fsSL "$base/$1.sha256" -o "$work/$1.sha256" || fail "could not download $1.sha256"
  (
    cd "$work"
    if command -v sha256sum > /dev/null 2>&1; then
      sha256sum -c "$1.sha256" > /dev/null 2>&1
    else
      shasum -a 256 -c "$1.sha256" > /dev/null 2>&1
    fi
  ) || fail "the checksum of $1 does not match"
}

# as_root runs a command as root, through sudo unless the script is root.
as_root() {
  if [ "$(id -u)" = 0 ]; then
    "$@"
  elif command -v sudo > /dev/null 2>&1; then
    sudo "$@"
  else
    fail "installing needs root: run the script as root or install sudo"
  fi
}

case "$(uname -s)" in
  Darwin)
    # Apple silicon even from a shell running under Rosetta.
    arch=x64
    [ "$(sysctl -in hw.optional.arm64 2> /dev/null)" != 1 ] || arch=arm64
    asset="$slug-$version-$arch.dmg"
    download "$asset"
    mkdir "$work/mnt"
    hdiutil attach -quiet -nobrowse -readonly -noautoopen -mountpoint "$work/mnt" "$work/$asset" ||
      fail "could not open $asset"
    [ -d "$work/mnt/$app" ] || fail "$asset holds no $app"
    dest=/Applications
    if [ ! -w "$dest" ] || [ -d "$HOME/Applications/$app" ]; then
      dest="$HOME/Applications"
      mkdir -p "$dest"
    fi
    if pgrep -f "$dest/$app/Contents/MacOS/" > /dev/null 2>&1; then
      echo "Quitting $name"
      pkill -f "$dest/$app/Contents/MacOS/" || true
      sleep 1
    fi
    rm -rf "$dest/$app"
    ditto "$work/mnt/$app" "$dest/$app"
    echo "Installed $name $version in $dest"
    ;;
  Linux)
    case "$(uname -m)" in
      x86_64 | amd64) deb=amd64 pkg=x86_64 ;;
      aarch64 | arm64) deb=arm64 pkg=aarch64 ;;
      *) fail "no package for $(uname -m)" ;;
    esac
    if command -v pacman > /dev/null 2>&1; then
      asset="$slug-$(echo "$version" | sed 's/[-+]/_/g')-1-$pkg.pkg.tar.gz"
      download "$asset"
      as_root pacman -U --noconfirm "$work/$asset"
    elif command -v dpkg > /dev/null 2>&1; then
      # Debian orders a pre-release, 1.2.0-beta.1, before its release as 1.2.0~beta.1.
      asset="${slug}_$(echo "$version" | sed -e 's/-/~/' -e 's/-/./g' -e 's/+/~/g')_$deb.deb"
      download "$asset"
      # apt's sandboxed user reads the package.
      chmod 755 "$work"
      if command -v apt-get > /dev/null 2>&1; then
        as_root apt-get install -y "$work/$asset"
      else
        as_root dpkg -i "$work/$asset"
      fi
    else
      fail "this script installs Arch and Debian packages; see https://github.com/$repository/releases for the app"
    fi
    echo "Installed $name $version"
    ;;
  *)
    fail "no package for $(uname -s); on Windows run: irm https://github.com/$repository/releases/latest/download/install.ps1 | iex"
    ;;
esac
