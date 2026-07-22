## linkedin mcp

MCP server management

### Synopsis

Manage MCP servers for AI assistants and code editors

### Options

```
  -h, --help   help for mcp
```

### Options inherited from parent commands

```
      --all                   page through all results (search commands)
      --base-url string       override the Voyager API host (default https://www.linkedin.com/voyager/api)
      --columns strings       comma-separated columns to show
      --count int             results per page (search commands) (default 25)
      --daily-cap int         override the ban-safety daily job-detail fetch cap (0 keeps the default of 30)
      --daily-send-cap int    override the ban-safety daily message-send cap (0 keeps the default of 20)
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

* [linkedin](linkedin.md)	 - An agent-friendly CLI for LinkedIn job search and messaging (unofficial Voyager API)
* [linkedin mcp claude](linkedin_mcp_claude.md)	 - Manage Claude Desktop MCP servers
* [linkedin mcp cursor](linkedin_mcp_cursor.md)	 - Manage Cursor MCP servers
* [linkedin mcp start](linkedin_mcp_start.md)	 - Start the MCP server
* [linkedin mcp stream](linkedin_mcp_stream.md)	 - Stream the MCP server over HTTP
* [linkedin mcp tools](linkedin_mcp_tools.md)	 - Export tools as JSON
* [linkedin mcp vscode](linkedin_mcp_vscode.md)	 - Manage VSCode MCP servers

