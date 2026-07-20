## linkedin agent guard

Generate agent-safety config that blocks destructive linkedin operations

### Synopsis

Classify every API command (read / write / irreversible) from the live command tree
and emit host safety config. linkedin is a READ-ONLY client — every command (including the
GET-only "linkedin api" escape hatch) is a read, so today the guard allows all API commands
and hard-blocks nothing except "linkedin alias set" (minting a shorthand). The guard derives
from the LIVE tree, so if a future write/destructive command is ever added it is hard-blocked
automatically, including its cobra alias paths.

For claude-code the output also includes a PreToolUse hook script
(.claude/hooks/linkedin-guard.sh): it strips quote/backslash obfuscation and matches blocked
subcommand paths at the command position even for path-invoked binaries (./bin/linkedin,
/usr/local/bin/linkedin). "linkedin alias set" is denied so an agent cannot mint a new
shorthand for a future blocked command.

MCP-only operation is the hard guarantee; the Bash rails are best-effort — the hook
defeats quoting tricks and path prefixes, but not variable indirection or shell aliases.
Conservative false positives are accepted: a line that merely QUOTES a blocked command is
denied.

```
linkedin agent guard --host <claude-code|codex|opencode> [flags]
```

### Examples

```
  linkedin agent guard --host claude-code
  linkedin agent guard --host claude-code --write          # write the files into .claude/
  linkedin agent guard --host codex --out ~/.codex/config.toml
  linkedin agent guard --host opencode --all-writes
```

### Options

```
      --all-writes    also hard-block ordinary writes, not just irreversible ops
  -h, --help          help for guard
      --host string   target agent host: claude-code|codex|opencode (required)
      --out string    write to this file instead of stdout
      --write         claude-code only: write hook + settings fragment under .claude/ (never overwrites)
```

### Options inherited from parent commands

```
      --all                   page through all results (search commands)
      --base-url string       override the Voyager API host (default https://www.linkedin.com/voyager/api)
      --columns strings       comma-separated columns to show
      --count int             results per page (search commands) (default 25)
      --daily-cap int         override the ban-safety daily job-detail fetch cap (0 keeps the default of 30)
      --dry-run               print the equivalent curl and make no request
      --jq string             gojq expression applied to the response before rendering
      --limit int             max items to return across pages (search commands)
      --no-color              disable colored output
  -o, --output string         output format: table|json|yaml|csv|id
      --profile string        named profile to use
      --quiet                 suppress non-essential chatter
      --show-token            reveal the session cookies in dry-run output
  -v, --verbose               verbose request logging (stderr)
      --web-base-url string   override the web host used for geo typeahead (default https://www.linkedin.com)
```

### SEE ALSO

* [linkedin agent](linkedin_agent.md)	 - AI-agent integration helpers

