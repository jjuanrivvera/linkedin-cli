# Output and filtering

One renderer serves every command.

- `-o table` (default) — colored only on a TTY; honors `NO_COLOR`/`--no-color`; deterministic column
  order; wide cells truncated (use `-o json` for full values).
- `-o json` / `-o yaml` — byte-faithful structured output.
- `-o csv` — spreadsheet-safe (formula-injection guarded).
- `-o id` — one id per line, pipeable to `xargs`.
- `--jq '<expr>'` — slice any response with gojq before rendering.
- `--columns a,b,c` — pick table/csv columns.
- `--dry-run` — print the equivalent `curl` (session cookies redacted unless `--show-token`) and send
  nothing.

Example — collect ids then fetch details deliberately (mind the daily cap):
```sh
linkedin jobs search --keywords sre --remote --since 7d -o id | head -5 | \
  while read id; do linkedin jobs get "$id" --jq '{id, title, apply: .companyApplyUrl}'; done
```
