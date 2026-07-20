# Auth and config

`linkedin` borrows your **browser session** — it does not use a token or password.

## Cookies it needs
- `li_at` — the LinkedIn session cookie.
- `JSESSIONID` — value looks like `"ajax:1234567890"` **with the quotes**. The `csrf-token` header
  is derived by trimming those quotes; the `Cookie` header keeps them.

## Capture
```sh
linkedin auth --cookie-from-browser chrome   # or brave|chromium|edge|firefox
```
Extracted via [kooky](https://github.com/browserutils/kooky) for `.linkedin.com` and stored in the
OS keyring (service `linkedin-cli`, key `<profile>`), with an AES-256-GCM encrypted-file fallback
(`credentials.enc`, keyed by `LINKEDIN_KEYRING_PASSWORD`) on headless hosts.

## Headless / CI
```sh
export LI_AT='AQED...'
export JSESSIONID='"ajax:1234567890"'
```
Both are required together (JSESSIONID is needed to derive csrf). Env overrides the keyring.

## Profiles
`--profile <name>` scopes a stored session and host overrides. `config use <name>` sets the default;
`config set voyager_base_url|web_base_url <url> --profile <name>` overrides a host.
