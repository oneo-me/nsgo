#!/bin/sh
# Rebuilds classes.dex and classes.jar from the Java runtime in ../java
# after changing it: the dex goes in apps' APKs, the jar in the libraries
# nsgo build -embed makes for host apps.
#
# These are the artifacts NSGo cannot build with Go, so they are committed.
# It needs a JDK 17+ and a recent d8: the SDK's cmdline-tools, or
# build-tools 36+. Set ANDROID_HOME to the SDK that holds them.
set -eu

dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
sdk=${ANDROID_HOME:-$HOME/Android}
jar=$sdk/platforms/android-36/android.jar
d8=$sdk/cmdline-tools/latest/bin/d8
if [ ! -x "$d8" ]; then
	d8=$(ls "$sdk"/build-tools/*/d8 2>/dev/null | sort | tail -1)
fi
if ! command -v javac >/dev/null || [ -z "$d8" ] || [ ! -f "$jar" ]; then
	echo "set PATH to a JDK 17+ and ANDROID_HOME to an SDK with platform-36 and a recent d8" >&2
	exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/classes"

# The runtime uses lambdas, so it compiles against the JDK's own java.*
# classes; d8 then reads the result with android.jar's stubs.
javac -nowarn -Xlint:-options --release 17 -classpath "$jar" \
	-d "$tmp/classes" "$dir"/../java/dev/oneo/nsgo/*.java
"$d8" --lib "$jar" --min-api 28 --output "$tmp" \
	$(find "$tmp/classes" -name '*.class')
cp "$tmp/classes.dex" "$dir/classes.dex"
# A host's build compiles against the jar and dexes it itself; no
# timestamps, so a rebuild of the same code is the same jar.
(cd "$tmp/classes" && find . -name '*.class' | LC_ALL=C sort > ../list && \
	touch -t 200001010000.00 $(cat ../list) && \
	jar --create --file "$dir/classes.jar" --no-manifest --date=2000-01-01T00:00:02Z @../list)
echo "wrote $dir/classes.dex and $dir/classes.jar"
