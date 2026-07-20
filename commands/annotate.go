package commands

import (
	"github.com/spf13/cobra"
)

// cmdKind classifies a command for MCP/agent-guard annotations.
type cmdKind int

const (
	kindRead        cmdKind = iota // read-only: MCP readOnlyHint (+idempotentHint)
	kindWrite                      // creates/changes remote state: MCP openWorldHint
	kindDestructive                // irreversible: MCP destructiveHint
)

// MCP tool annotation keys (the singular MCP hint keys; ophis reads these from
// cmd.Annotations). Stamped as each command is built — never retrofitted later.
const (
	annReadOnly    = "readOnlyHint"
	annDestructive = "destructiveHint"
	annOpenWorld   = "openWorldHint"
	annIdempotent  = "idempotentHint"
)

// annotate stamps the MCP classification for kind onto cmd.
func annotate(cmd *cobra.Command, kind cmdKind) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	switch kind {
	case kindRead:
		cmd.Annotations[annReadOnly] = "true"
		cmd.Annotations[annIdempotent] = "true"
	case kindWrite:
		cmd.Annotations[annOpenWorld] = "true"
	case kindDestructive:
		cmd.Annotations[annOpenWorld] = "true"
		cmd.Annotations[annDestructive] = "true"
	}
	return cmd
}
