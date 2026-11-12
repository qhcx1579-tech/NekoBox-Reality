#!/bin/bash
set -e

source "buildScript/init/env.sh"
ENV_NB4A=1
source "buildScript/lib/core/get_source_env.sh"
pushd ..

####

if [ ! -d "sing-box" ]; then
  git clone --no-checkout https://github.com/MatsuriDayo/sing-box.git
fi
pushd sing-box
git checkout "$COMMIT_SING_BOX"
# Apply the narrowly scoped REALITY compatibility fix to the pinned core.
PATCH="$SRC_ROOT/buildScript/lib/core/patches/reality-mlkem.patch"
if git apply --check "$PATCH"; then
  git apply "$PATCH"
elif ! git apply --reverse --check "$PATCH"; then
  echo "REALITY patch does not match the pinned sing-box source." >&2
  exit 1
fi
cp "$SRC_ROOT"/buildScript/lib/core/tests/*.go common/tls/
popd

####

if [ ! -d "libneko" ]; then
  git clone --no-checkout https://github.com/MatsuriDayo/libneko.git
fi
pushd libneko
git checkout "$COMMIT_LIBNEKO"
popd

####

popd
