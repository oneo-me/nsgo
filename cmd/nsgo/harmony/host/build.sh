#!/bin/sh
# Rebuilds this directory's host skeleton from ../templates: the ArkTS
# host, compiled by hvigor. These are the parts NSGo cannot build with Go,
# so they are committed. Needs hvigorw on PATH and OHOS_BASE_SDK_HOME set
# to an OpenHarmony SDK laid out a directory per API, as hvigor reads it.
#
# The bundle name is the first argument, a fixed-length placeholder by
# default. The compiled bytecode names each module <bundle>/<module>/<path>;
# harmony.Package replaces this placeholder byte for byte with an equally
# long name for the app, so one host serves every app. A build machine with
# the SDK does not use this host: harmony.CompileHost builds one for the
# app's own bundle from these templates. Keep this in step with hostBundle
# in ../harmony.go (docs/architecture/harmonyos.md).
set -eu
if [ $# -gt 0 ]; then
	bundle=$1
else
	bundle=com.nsgo.host.placeholder000000000000000000000000000000000000000
fi
dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cp -R "$dir/../templates/." "$work/"
find "$work" -type f \( -name '*.json5' -o -name '*.json' -o -name '*.ts' -o -name '*.ets' \) \
	-exec perl -pi -e "s/\{\{name\}\}/NSGo/g; s/\{\{bundle\}\}/$bundle/g; s/\{\{version\}\}/0.0.0/g; s/\{\{code\}\}/1/g" {} +
mkdir -p "$work/entry/src/main/resources/rawfile/web"
printf '%s\n' '<!doctype html>' > "$work/entry/src/main/resources/rawfile/web/index.html"
( cd "$work" && hvigorw --mode module -p product=default -p module=entry@default -p buildMode=debug assembleHap --no-daemon >/dev/null )
hap="$work/entry/build/default/outputs/default/entry-default-unsigned.hap"
rm -rf "$dir/ets" "$dir/resources" "$dir/resources.index" "$dir/pkgSdkInfo.json" "$dir/rpcid.sc" "$dir/module.json" "$dir/pack.info"
( cd "$dir" && unzip -q "$hap" -d . )
rm -f "$dir/ets/sourceMaps.map"
rm -rf "$dir/resources/rawfile"
echo "wrote $dir for $bundle"
