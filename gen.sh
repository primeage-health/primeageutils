#!/usr/bin/env bash
#
# Regenerate every stub under genproto/ from the contracts under proto/.
#
#   ./gen.sh
#
# Needs protoc, protoc-gen-go and protoc-gen-go-grpc on PATH — see README.md.
#
# genproto/ is generated in full each run. Nothing in it should ever be edited by
# hand; a change belongs in the .proto next to it.

set -euo pipefail

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

for binary in protoc protoc-gen-go protoc-gen-go-grpc; do
    command -v "$binary" >/dev/null 2>&1 || {
        echo "$binary is not on PATH — see README.md" >&2
        exit 1
    }
done

# go_package inside each .proto is the full module path, so paths=source_relative
# would write to proto/<name>/ rather than genproto/<name>/. Generating with the
# default module-relative mode and stripping the module prefix puts the files
# where the import path says they are.
#
# The proto/primeage/ nesting and the primeage.* proto packages are load-bearing,
# not decoration. Generated code registers itself into a process-wide protobuf
# registry at init, keyed by file path and message name, and a duplicate of
# either panics before main runs. A bare auth.proto declaring package "auth" is
# exactly the name some other library in the same binary would also pick — so
# the namespace is what keeps this library linkable alongside anything else.
rm -rf genproto
mkdir -p genproto

protoc \
    --go_out=. \
    --go_opt=module=github.com/primeage-health/primeageutils \
    --go-grpc_out=. \
    --go-grpc_opt=module=github.com/primeage-health/primeageutils \
    proto/primeage/auth/auth.proto \
    proto/primeage/identity/identity.proto \
    proto/primeage/notification/notification.proto

echo "generated:"
find genproto -name '*.go' | sort
