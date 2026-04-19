# nostrlib-nip05-namecoin

A proposed `nip05/namecoin` subpackage for [`fiatjaf.com/nostr`](https://viewsource.win/fiatjaf.com/nostrlib) (a.k.a. nostrlib).

It resolves Namecoin `.bit` NIP-05 identifiers (`alice@example.bit`, `example.bit`, `d/example`, `id/alice`) to a `*nostr.ProfilePointer` by querying public Namecoin ElectrumX servers.

## Why a GitHub repo and not a pull request?

nostrlib is hosted on fiatjaf's own git server at `basspistol.org`, which is publish-only — there is no PR surface. This repo is the drop-in shape that a `nip05/namecoin` subpackage in nostrlib would have, staged on GitHub so it can be reviewed, forked, or merged by any hands.

See also: the Nostr-native contribution path — a NIP-34 (kind:1617) git patch event against nostrlib's master tip, addressed to fiatjaf's npub. Linked from [nak PR #123](https://github.com/fiatjaf/nak/pull/123).

## Use it today, as-is

```go
import "github.com/mstrofnone/nostrlib-nip05-namecoin/namecoin"

if namecoin.IsValidIdentifier(input) {
    pp, err := namecoin.QueryIdentifier(ctx, input)
    // ...
}
```

Same signature as `nip05.QueryIdentifier`, so chaining is trivial:

```go
if namecoin.IsValidIdentifier(input) {
    return namecoin.QueryIdentifier(ctx, input)
}
return nip05.QueryIdentifier(ctx, input)
```

## Drop-in path for nostrlib

If/when it lands upstream, users would switch one import:

```go
import "fiatjaf.com/nostr/nip05/namecoin"
```

Nothing else changes — package name, public API, and types are identical.

## Public API

| Function                                                    | Purpose                                                        |
| ----------------------------------------------------------- | -------------------------------------------------------------- |
| `IsValidIdentifier(s string) bool`                          | Cheap check — does this look like a `.bit` / `d/` / `id/` id?  |
| `IsDotBit(s string) bool`                                   | Alias of `IsValidIdentifier` (kept for descriptive callers).   |
| `QueryIdentifier(ctx, id) (*nostr.ProfilePointer, error)`   | Resolve an identifier to a pubkey + relays.                    |
| `ResolveToJSONValue(ctx, id) ([]byte, error)`               | Return the raw Namecoin name value (for `bitcoin`, `http`, …). |

Plus advanced types exposed for custom usage:

- `ElectrumClient`, `ElectrumxServer`, `NameShowResult`, `Transport`
- `DefaultElectrumXServers` (TCP+TLS, ordered public server list)
- `DefaultElectrumXServersWSS` (WSS endpoints for browser/wasm / restricted networks)
- Sentinels: `ErrNameNotFound`, `ErrNameExpired`, `ErrServersUnreachable`

## Transports

Two transports are supported: **TCP+TLS** (the default, what every
traditional Electrum / ElectrumX client uses) and **WSS** (WebSocket
over TLS). Both speak the same JSON-RPC; only the framing differs.

- **TCP+TLS** is the default and what `QueryIdentifier` / `ResolveToJSONValue`
  use out of the box via `DefaultElectrumXServers`. It connects on
  ports `50002` / `57002` on the public servers.
- **WSS** is useful in three situations:
  - **Browser / wasm** targets, where raw TCP is not available. Build
    `github.com/coder/websocket` with `GOOS=js GOARCH=wasm` and the
    same code resolves `.bit` identifiers from inside a browser.
  - **Restricted networks** that block the classic ElectrumX ports
    but allow outbound HTTPS/WSS traffic.
  - **Transport alignment** with the rest of the Nostr stack, which
    is WSS-everywhere.

Pick WSS explicitly by using the WSS server list or by constructing
your own `ElectrumxServer` with `Transport: TransportWSS`:

```go
import "github.com/mstrofnone/nostrlib-nip05-namecoin/namecoin"

client := namecoin.NewElectrumClient()
result, err := client.NameShowWithFallback(ctx, "d/testls", namecoin.DefaultElectrumXServersWSS)
```

The pinned-cert trust store is shared between both transports; the
public operators serve the same certificate on the adjacent ports.

## Dependencies

- Go 1.25+
- `fiatjaf.com/nostr` (for `nostr.PubKey`, `nostr.ProfilePointer`, `nostr.PubKeyFromHex`)
- `github.com/coder/websocket` for the WSS transport — zero transitive
  dependencies, and already a transitive dep of `fiatjaf.com/nostr`.
- Go standard library otherwise.

## Testing

```bash
go test ./...            # unit tests (offline, fast)
go vet ./...
go test -tags=integration ./namecoin -run Integration -v   # hits live ElectrumX
```

Integration tests are gated behind the `integration` build tag so CI stays hermetic.

## Context

- nak PR #123: [fiatjaf/nak#123](https://github.com/fiatjaf/nak/pull/123) — original Namecoin `.bit` addition to `nak`, which prompted fiatjaf's suggestion that this belongs in nostrlib.
- Standalone CLI wrapper for end users: [mstrofnone/nak-nmc](https://github.com/mstrofnone/nak-nmc).
- Reference ports: [Amethyst (Kotlin)](https://github.com/vitorpamplona/amethyst), [Nostur (Swift)](https://github.com/nostur-com/nostur-ios-public).

## License

Public domain (Unlicense), matching nostrlib.
