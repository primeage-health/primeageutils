# primeageutils

The shared library every PrimeAge service depends on: the gRPC contract between
them, the transport that carries it, the error envelope that crosses a layer
boundary, and the log.

```
proto/          .proto sources, the contract
genproto/       generated stubs (do not hand-edit; run ./gen.sh)
grpcclient/     dial helper + client-side TLS
grpcserver/     listener helper + server-side TLS + the service-token interceptor
certs/          where the TLS material is read from
errs/           *RestError, the failure envelope
logger/         structured logging
```

## Two decisions worth knowing

**The default logger writes.** `logger.FromCtx` on a context nobody attached a
logger to returns a real logger on stderr, not a silent one. Logging that
silently discards everything unless the service remembered an initialisation call
is worse than no logging at all — it looks like it works. Silence has to be asked
for by name, with `logger.Nop()`.

**The contracts are namespaced.** The `.proto` files live under `proto/primeage/`
with `primeage.*` proto packages, rather than a bare `auth` or `notification`.
Generated protobuf code registers itself into a process-wide registry at init,
keyed by file path and message name, and duplicate registration panics before
`main` runs. Any other library linked into the same binary that also declares an
`auth` package would take the process down at startup. The namespace costs
nothing and removes the whole class of problem.

## Consuming it

The module path is `github.com/primeage-health/primeageutils`, and the repo
exists. Until a version is tagged and pushed, consumers carry:

```
replace github.com/primeage-health/primeageutils => ../primeageutils
```

**Docker image builds do not work while that replace is in place.** A build
context is one repo, so the relative target is not in it. Local `go build` and
`go test` are unaffected. Tagging this module and dropping four `replace` lines
is what closes the gap.

## Regenerating the stubs

Needs `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc` on `PATH`:

```sh
brew install protobuf
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

./gen.sh
```

## Transport

TLS on every hop, server-side only — the client verifies the server against the
CA, the server does not verify the client. So the caller still has to prove who
it is: each RPC carries the callee's service token in the `x-service-token`
metadata key, and the server's interceptor compares it in constant time. The
token is the authentication; TLS only provides the confidentiality that makes
sending it safe.

Certificates are read from `CERT_DIR`, default `/app/cert`, where the deployment
mounts them as a Secret:

| file | read by |
|---|---|
| `ca-cert.pem` | clients, to verify the server |
| `server-cert.pem`, `server-key.pem` | servers |

They are issued by `deployer.primeage.com/cert-gen/generate-certs.sh` and
installed with that repo's `scripts/place-grpc-certs.sh`. The SAN on each
certificate is the Kubernetes Service name callers dial, which is what makes the
handshake succeed.

## A note on struct tags

The workspace tag rules put `json` tags only in `dto/`. `errs.RestError` carries
one because it is the response body itself, and `genproto/` carries them because
protoc writes them. A shared library is not a service's `dto/` package, and both
are exempt.
