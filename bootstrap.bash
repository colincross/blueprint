#!/bin/bash

# This script serves two purposes.  First, it can bootstrap the standalone
# Blueprint to generate the minibp binary.  To do this simply run the script
# with no arguments from the desired build directory.
#
# It can also be invoked from another script to bootstrap a custom Blueprint-
# based build system.  To do this, the invoking script must first set some or
# all of the following environment variables, which are documented below where
# their default values are set:
#
#   BOOTSTRAP
#   SRCDIR
#   BOOTSTRAP_MANIFEST
#   GOROOT
#   GOOS
#   GOARCH
#   GOCHAR
#
# The invoking script should then run this script, passing along all of its
# command line arguments.

set -e

EXTRA_ARGS=""

# BOOTSTRAP should be set to the path of the bootstrap script.  It can be
# either an absolute path or one relative to the build directory (which of
# these is used should probably match what's used for SRCDIR).
[ -z "$BOOTSTRAP" ] && BOOTSTRAP="${BASH_SOURCE[0]}"

# SRCDIR should be set to the path of the root source directory.  It can be
# either an absolute path or a path relative to the build directory.  Whether
# its an absolute or relative path determines whether the build directory can
# be moved relative to or along with the source directory without re-running
# the bootstrap script.
[ -z "$SRCDIR" ] && SRCDIR=`dirname "${BOOTSTRAP}"`

# TOPNAME should be set to the name of the top-level Blueprints file
[ -z "$TOPNAME" ] && TOPNAME="Blueprints"

# BOOTSTRAP_MANIFEST is the path to the bootstrap Ninja file that is part of
# the source tree.  It is used to bootstrap a build output directory from when
# the script is run manually by a user.
[ -z "$BOOTSTRAP_MANIFEST" ] && BOOTSTRAP_MANIFEST="${SRCDIR}/build.ninja.in"

# These variables should be set by auto-detecting or knowing a priori the host
# Go toolchain properties.
[ -z "$GOROOT" ] && GOROOT=`go env GOROOT`
[ -z "$GOOS" ]   && GOOS=`go env GOHOSTOS`
[ -z "$GOARCH" ] && GOARCH=`go env GOHOSTARCH`
[ -z "$GOCHAR" ] && GOCHAR=`go env GOCHAR`

# If RUN_TESTS is set, behave like -t was passed in as an option.
[ ! -z "$RUN_TESTS" ] && EXTRA_ARGS="$EXTRA_ARGS -t"

usage() {
    echo "Usage of ${BOOTSTRAP}:"
    echo "  -h: print a help message and exit"
    echo "  -r: regenerate ${BOOTSTRAP_MANIFEST}"
    echo "  -t: include tests when regenerating manifest"
}

# Parse the command line flags.
IN="$BOOTSTRAP_MANIFEST"
REGEN_BOOTSTRAP_MANIFEST=false
while getopts ":hi:rt" opt; do
    case $opt in
        h)
            usage
            exit 1
            ;;
        i) IN="$OPTARG";;
        r) REGEN_BOOTSTRAP_MANIFEST=true;;
        t) EXTRA_ARGS="$EXTRA_ARGS -t";;
        \?)
            echo "Invalid option: -$OPTARG" >&2
            usage
            exit 1
            ;;
        :)
            echo "Option -$OPTARG requires an argument." >&2
            exit 1
            ;;
    esac
done

if [ $REGEN_BOOTSTRAP_MANIFEST = true ]; then
    # This assumes that the script is being run from a build output directory
    # that has been built in the past.
    if [ -x .bootstrap/bin/minibp ]; then
        echo "Regenerating $BOOTSTRAP_MANIFEST"
        ./.bootstrap/bin/minibp $EXTRA_ARGS -o $BOOTSTRAP_MANIFEST $SRCDIR/$TOPNAME
    else
        echo "Executable minibp not found at .bootstrap/bin/minibp" >&2
        exit 1
    fi
fi

sed -e "s|@@SrcDir@@|$SRCDIR|g"                        \
    -e "s|@@GoRoot@@|$GOROOT|g"                        \
    -e "s|@@GoOS@@|$GOOS|g"                            \
    -e "s|@@GoArch@@|$GOARCH|g"                        \
    -e "s|@@GoChar@@|$GOCHAR|g"                        \
    -e "s|@@Bootstrap@@|$BOOTSTRAP|g"                  \
    -e "s|@@BootstrapManifest@@|$BOOTSTRAP_MANIFEST|g" \
    $IN > build.ninja
