# Proposal: `fiatjaf.com/nostr/nip05/namecoin`

For fiatjaf.

You commented on [nak PR #123](https://github.com/fiatjaf/nak/pull/123):

> This should go on https://viewsource.win/fiatjaf.com/nostrlib instead, maybe as a subpackage of `nip05`. But I'll have to think about it first.

This is that subpackage, staged on GitHub so you can look at it without having to apply anything.

## Proposed directory slot

```
fiatjaf.com/nostr/
├── nip05/
│   ├── nip05.go
│   ├── nip05_easyjson.go
│   ├── nip05_test.go
│   └── namecoin/          ← this package
│       ├── doc.go
│       ├── namecoin.go
│       ├── electrumx.go
│       ├── script.go
│       ├── servers.go
│       ├── tls.go
│       ├── namecoin_test.go
│       └── electrumx_integration_test.go  (//go:build integration)
```

Everything in `namecoin/` is self-contained. No touching `nip05/nip05.go`.

## Proposed API

Mirrors `nip05` on purpose, so call sites stay boring:

| Function                                                      | Mirrors                                    |
| ------------------------------------------------------------- | ------------------------------------------ |
| `IsValidIdentifier(s string) bool`                            | `nip05.IsValidIdentifier`                  |
| `QueryIdentifier(ctx, id) (*nostr.ProfilePointer, error)`     | `nip05.QueryIdentifier`                    |
| `ResolveToJSONValue(ctx, id) ([]byte, error)`                 | new — returns the raw Namecoin name value  |

Plus `IsDotBit` (alias) and advanced types (`ElectrumClient`, `ElectrumxServer`, `NameShowResult`, `DefaultElectrumXServers`, error sentinels).

Caller pattern becomes a two-line chain:

```go
if namecoin.IsValidIdentifier(input) {
    return namecoin.QueryIdentifier(ctx, input)
}
return nip05.QueryIdentifier(ctx, input)
```

## What it does

Parses `.bit` / `d/` / `id/` identifiers, queries a Namecoin ElectrumX server for the current value of the underlying `d/<domain>` or `id/<name>`, extracts the `nostr` field (both the simple-string and `{names, relays}` object forms), and returns a `*nostr.ProfilePointer`.

Ported from the Kotlin reference in Amethyst and the Swift port in Nostur.

## Tradeoffs

- **Net new code**: ~1.3k LOC across eight files, half of which is tests. Nothing global — no init funcs, no side effects at import time.
- **Outbound TLS connections** from nostrlib itself. This is new: today, `nip05` only speaks HTTPS to user-provided domains. Here we dial a short list of public ElectrumX servers. Probably worth a CHANGELOG note.
- **Pinned self-signed certs**. The two long-running public Namecoin ElectrumX endpoints (`electrumx.testls.space`, `nmc2.bitcoins.sk`) both serve self-signed certificates. We ship pinned PEMs in `servers.go` and only trust those for the pinned hostnames. Happy to drop this in favor of a "caller provides servers" API if you'd prefer nostrlib stay cert-policy-neutral.
- **No new module dependencies**. Stdlib + `fiatjaf.com/nostr` types only.
- **No background goroutines, no connection pooling.** One short-lived TCP/TLS socket per lookup. Cheap to call, easy to reason about.

## Happy to reshape

- Fold in as `nip05/namecoin/` (proposed) — ideal, keeps discoverability.
- Or as a sibling standalone `nip05namecoinclient` package — if you want the `nip05` namespace kept narrowly DNS-oriented.
- Or drop the pinned certs and require callers to pass their own `[]ElectrumxServer` — if bundling certs in nostrlib is a non-starter.
- Or rename `ResolveToJSONValue` → `Fetch` to match `nip05.Fetch`.

Any or all of the above — say the word and I'll PR the reshape here first so you can see it land cleanly before committing to anything on your side.

## If you'd rather not land it

Totally fine. This repo stands alone as a usable Go package, and [`mstrofnone/nak-nmc`](https://github.com/mstrofnone/nak-nmc) covers end users who just want `.bit` resolution in `nak`.

## Nostr-native contribution path

There's also a NIP-34 git patch event (kind:1617) against `nostrlib`'s master tip (`f50b7b0f8dcb`), addressed to your npub. Event id and nevent encoding are in the comment on nak PR #123.
