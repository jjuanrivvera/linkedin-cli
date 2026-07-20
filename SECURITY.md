# Security Policy

## Supported versions

The latest minor release receives security fixes. Older versions: upgrade.

| Version | Supported |
|---|---|
| latest release | yes |
| anything older | no |

## Unofficial API — account-risk note

`linkedin` uses LinkedIn's **internal, unofficial Voyager API** with the user's **own browser
session cookies**. This is not sanctioned by LinkedIn and may violate the User Agreement; automated
access can lead to the account being restricted. Ban-safety mitigations are **on by default**
(human-paced delays, a per-day job-detail cap, and no retry on HTTP 999 / 429 / challenge). This is
a usage/account risk, not a software vulnerability — do not report it as one.

## Credential (session cookie) handling

- The borrowed session is the pair `li_at` + `JSESSIONID` (the latter's value keeps its quotes).
  There is no password or API token.
- It is stored as a JSON blob in the OS keyring (macOS Keychain, Linux Secret Service, Windows
  Credential Manager) under service `linkedin-cli`, keyed `<profile>`.
- On hosts without a keyring it falls back to an AES-256-GCM encrypted file (`credentials.enc`, mode
  0600) under the config dir. The key derives from `LINKEDIN_KEYRING_PASSWORD` via scrypt when set;
  otherwise from a host-bound seed, which is obfuscation, not a security boundary — set the password
  on shared hosts.
- Cookies never appear in `config.yaml`, command output, or `--dry-run` curls (redacted unless you
  pass `--show-token`). The `LI_AT` / `JSESSIONID` env overrides are read but never persisted.
- Cleartext `http://` base URLs are rejected for non-loopback hosts.
- `linkedin auth logout` removes exactly the active profile's stored session.

## Reporting a vulnerability

Please use GitHub's private vulnerability reporting on this repository
(Security → Report a vulnerability). Do not open a public issue for anything
sensitive. Reports get a response within a week.
