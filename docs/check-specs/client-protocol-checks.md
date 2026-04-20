# client-protocol-checks (Phase 3 / AI-C-5 γ)

Combined spec for the three client-identity checks. These run once
per session at `Login` time and produce **single-shot** kicks; there
is no buffer because the signal is a one-time login fingerprint.

## EditionFaker/A — Bedrock edition spoofing

Flags clients whose `Login` packet declares a `DeviceOS` or
`GameVersion` that does not match the protocol version they actually
implement. Common cases:

- DeviceOS=10 (Win10) but DeviceModel string belongs to a known
  Android emulator profile.
- GameVersion claims 1.21.x but the protocol version field is the
  1.20 value — indicates a modified client lying about its identity
  to bypass version-gated permissions.

- Source: `protocol.LoginClientData` parsed from `packet.Login`.
- FailBuffer 1, MaxBuffer 1, default Kick.

## ClientSpoof/A — known cheat-client signatures

Flags `Login` payloads that match published cheat-client fingerprints:

- Specific `ThirdPartyName`, `LanguageCode`, or `SkinResourcePatch`
  patterns documented in the upstream `client-fingerprints.json`
  list (maintained alongside this spec).
- `DeviceModel` empty + `PlatformOnlineID` non-empty (a documented
  Horion / Toolbox marker).

- The fingerprint table lives in `anticheat/checks/client/fingerprints.go`
  and is loaded once at `NewClientSpoofCheck`.
- FailBuffer 1, default Kick.

## Protocol/A — protocol-version range gate

Flags `Login` packets whose declared `ProtocolVersion` is below the
configured `MinProtocol` (default 712 = 1.21.50) or above
`MaxProtocol` (default 0 = "any future version allowed"). Stops the
common "downgrade to 1.16 to evade modern checks" attack.

- Configured per-server; supports a list of explicitly-blocked versions
  for incident response (`BlockedProtocols []int32`).
- FailBuffer 1, default Kick.

## Acceptance gate

Each check requires:
1. Spec section above finalized.
2. Implementation under `anticheat/checks/client/<name>.go`.
3. Unit tests with synthetic `protocol.LoginClientData` fixtures.
4. Manager registration as a `LoginCheck` (a new lifecycle hook
   registered alongside `OnInput` / `OnAttack` / `OnBlockPlace`).
5. Fingerprint table source documented in repo root README.

## Open questions

- **Touch-only servers:** EditionFaker may need to allow specific
  PocketEdition mismatches if a server explicitly supports older
  pocket builds.
- **Cross-play certification:** Microsoft's certified XBL clients
  occasionally ship with prerelease protocol numbers; need a process
  for whitelisting beta channels.
