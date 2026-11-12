# Running harnsy-hr-mcp

The server is one static binary. It reads the catalog's signed release from disk, verifies it with the catalog public
key and serves it over MCP on loopback. It holds no secret, writes nothing and opens no outgoing connection. A web
server in front ends TLS and limits the request rate.

## Build

Go 1.25 or newer (the MCP SDK needs it). The binary has no C parts and runs on a machine without Go:

```
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=<version>" -o harnsy-hr-mcp ./cmd/harnsy-hr-mcp
```

Build it where the toolchain is new enough and copy the file; `go.mod` names the toolchain it was tested with.

The binary contains the libraries' code, and their licences ask that the notices travel with every copy: wherever
the binary goes, `LICENSE`, `NOTICE` and `THIRD_PARTY_LICENSES` go with it, in the same folder.

## Install

1. A system user of its own, with no shell and no home. It must be able to read the catalog's root and must not be
   able to read the catalog's signing key.
2. The binary, owned by root, not writable by that user.
3. `harnsy-hr-mcp.service.in` → the unit, with your values in place of `@BIN@`, `@USER@`, `@ROOT@`, `@PUB@`, `@KID@`,
   `@HOST@`, `@PORT@`. `@PUB@` is the catalog public key (base64 of the raw 32-byte Ed25519 key): it is public, and it
   is the only thing that decides what the server trusts.
4. `nginx-mcp-limits.conf` → the http context; `nginx-mcp-location.conf.in` → inside the catalog host's TLS server
   block. Read the two notes at the top of the location file before `nginx -t`:
   - never pass the public `Host` to the server (no `proxy_set_header Host $host;`), or every call is answered 403;
   - a host that refuses every method but GET and HEAD must let `POST /mcp` through.

## What it does while it runs

- Every 30 s (`--every`) it looks at `<root>/current`. A new release is verified whole (the feed's signature, each
  bundle's size, hashes and catalog signature) before it replaces the one in memory. A release that fails is not
  loaded: the last good one stays and the reason is logged.
- Past the feed's `expires` it serves nothing and answers `503` with `Retry-After`, as it does before the first good
  release. So the job that re-signs the feed is a dependency of this service.
- Log (stderr, JSON lines): the release it loaded (`seq`, how many employees, `expires`), each tool call (the tool, the
  employee id, ok or refused), never the words of a search and never an address.

## Check after a start or a change

With `H=https://<host>/mcp`:

```
# 200 and four tools
curl -s -X POST $H -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
  --data-binary '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
# 200 and cards
curl -s -X POST $H -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
  --data-binary '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_employees","arguments":{"limit":3}}}'
curl -s -o /dev/null -w '%{http_code}\n' $H                                  # 405: POST only
curl -s -o /dev/null -w '%{http_code}\n' -X POST $H -H 'Origin: https://example.org' \
  -H 'Content-Type: application/json' --data-binary '{}'                     # 403: a browser page of another site
head -c 70000 /dev/zero | curl -s -o /dev/null -w '%{http_code}\n' -X POST $H \
  -H 'Content-Type: application/json' --data-binary @-                       # 413: over 64 KB
```

`403` on the first call means the proxy passes the public `Host`. `405` on a POST means the host's method rule stands
in front of the location. `503` means no release verifies: read the service's log for the reason (a wrong `@PUB@` or
`@KID@`, an expired feed, a bundle on another host than `@HOST@`).

## The registry entry

`server.json` at the repository root is the entry for the MCP Registry: the name `dev.harnsy/hr` and the remote
endpoint, nothing to install. The file alone publishes nothing. What is published there is dedicated to the public
domain and a published version cannot be removed, so before a publish set `version` to the release being announced and
read the text once more. `go test ./internal/registry` keeps the file within the registry's schema.

## Take it away

Remove the location and the limits file, reload the web server, stop and disable the unit. Nothing else changes: the
server keeps no state and the catalog does not know it exists.
