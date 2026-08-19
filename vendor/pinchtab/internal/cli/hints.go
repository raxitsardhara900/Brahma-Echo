package cli

import (
	"fmt"
	"io"
)

// CommandHint is one "<command>  <comment>" row in a CLI hint group.
type CommandHint struct {
	Command string
	Comment string
}

// WriteCommandHints renders a heading followed by aligned command/comment rows.
// When styled, the heading/command/comment use the cli styles (ANSI); otherwise
// they are emitted plain. width is the command-column pad (the %-44s/%-64s the
// call sites used inline); padding counts ANSI bytes when styled, matching the
// previous inline behavior exactly.
func WriteCommandHints(out io.Writer, heading string, hints []CommandHint, width int, styled bool) {
	if styled {
		_, _ = fmt.Fprintln(out, StyleStdout(HeadingStyle, heading))
	} else {
		_, _ = fmt.Fprintln(out, heading)
	}
	for _, h := range hints {
		cmd, comment := h.Command, h.Comment
		if styled {
			cmd = StyleStdout(CommandStyle, cmd)
			comment = StyleStdout(MutedStyle, comment)
		}
		_, _ = fmt.Fprintf(out, "  %-*s %s\n", width, cmd, comment)
	}
}

// SessionCreateCommand is the one spelling of the create-a-session command, so
// every place that recommends it stays in lockstep.
const SessionCreateCommand = "export PINCHTAB_SESSION=$(pinchtab session create --agent-id <id>)"

// NoSessionHint is the single wording for "this caller has no agent session".
// The CLI cannot tell whether the server has sessions enabled, so this must be
// true in BOTH states: the applicable half leads, the prerequisite survives as a
// conditional fallback, and the command ends the line so lifting it to
// end-of-line yields exactly the command — it is the only place that command is
// published, and its readers are agents.
//
// The THIRD state is a bridge, where no config value mounts the family at all, so the
// fallback must not name a config edit as the universal enabler: it is one of two
// enablers, and the other is running a different process. The clause therefore attributes
// each remedy to the mode it works in rather than prescribing one for both. The command
// itself is unconditional — creating a session is what the caller wanted in every state,
// and the server now answers each with its own code and remedy if it cannot.
const NoSessionHint = "this tab is shared — no agent session is set. If agent sessions are enabled on this server, create one now; " +
	"otherwise a full server needs sessions.agent.enabled = true in config.json and a restart, and a bridge has no agent sessions at all. " +
	"Either way the command is: " +
	SessionCreateCommand

// NextStepsRunningHints is the "Next steps" group shown when the server is up;
// shared by the root banner and `pinchtab health` so the two stay in lockstep.
var NextStepsRunningHints = []CommandHint{
	{SessionCreateCommand, "# start a dedicated session"},
	{"pinchtab nav <url>", "# navigate the current tab (headless by default)"},
	{"pinchtab snap", "# inspect interactive elements"},
}
