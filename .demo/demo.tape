# linkedin demo — rendered with VHS (https://github.com/charmbracelet/vhs).
# No account, signup, OS keyring or external network: only invented local API data.
# Regenerate from the repository root:
#   go build -o bin/linkedin ./cmd/linkedin && go build -o bin/demoserver ./tools/demoserver && vhs .demo/demo.tape
# The shell exits on a failed command rather than recording an error as a result.
# The checked-out CLI is read-only; an API write cannot be demonstrated.

Output assets/demo.gif

Set Shell bash
Set FontSize 15
Set Width 1200
Set Height 760
Set Padding 18
Set Theme "Dracula"
Set TypingSpeed 28ms

Hide
Type "set -e -o pipefail"
Enter
Type "export PATH=$PWD/bin:$PATH XDG_CONFIG_HOME=$(mktemp -d)"
Enter
Type `export LI_AT=demo-li-at JSESSIONID='"ajax:demo-session"'`
Enter
Type `demoserver 127.0.0.1:8654 >"$XDG_CONFIG_HOME/demoserver.log" 2>&1 & demo_server_pid=$!`
Enter
Type `trap 'kill "$demo_server_pid" 2>/dev/null || true; rm -rf "$XDG_CONFIG_HOME"' EXIT`
Enter
Type `sleep 1; kill -0 "$demo_server_pid"`
Enter
Type `alias linkedin='env -i PATH="$PATH" XDG_CONFIG_HOME="$XDG_CONFIG_HOME" LI_AT="$LI_AT" JSESSIONID="$JSESSIONID" linkedin --base-url http://127.0.0.1:8654/voyager/api --web-base-url http://127.0.0.1:8654'`
Enter
Type "clear"
Enter
Show

Sleep 500ms

Type "linkedin jobs search --keywords golang --remote"
Sleep 250ms
Enter
Sleep 2400ms

Type "linkedin jobs search --keywords golang --remote -o json --jq '.[].title'"
Sleep 250ms
Enter
Sleep 2400ms

Type "clear"
Enter
Sleep 300ms

Type "linkedin jobs get 900101 -o json"
Sleep 250ms
Enter
Sleep 2400ms

Type "clear"
Enter
Sleep 300ms

Type "linkedin company get fictional-demo-workshop -o json"
Sleep 250ms
Enter
Sleep 2400ms
