set -o errexit  # Exit on most errors
set -o errtrace # Make sure any error trap is inherited
set -o nounset  # Disallow expansion of unset variables
set -o pipefail # Use last non-zero exit code in a pipeline
# set -o xtrace

go fmt ./...
go test ./...

# PKG=github.com/mebyus/hit/main
# VERSION=$(git describe --tags --always)
# -ldflags="-X ${PKG}.Version=${VERSION}"

CGO_ENABLED=0 go install .
