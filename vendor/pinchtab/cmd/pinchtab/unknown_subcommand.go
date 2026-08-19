package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// unknownSubcommandExitCode is 1 because that is what cobra's own error path already returns
// for a top-level unknown command; daemon's hand-rolled 2 is what changed to match.
const unknownSubcommandExitCode = 1

// installUnknownSubcommandGuard runs at Execute rather than in an init because registration
// is spread across many init functions and their order is not a contract. A group declaring
// its own Args opts out, which is how a parent whose first argument is data rather than a verb
// says so.
func installUnknownSubcommandGuard(cmd *cobra.Command) {
	for _, sub := range cmd.Commands() {
		installUnknownSubcommandGuard(sub)
	}
	if !cmd.HasParent() {
		return
	}
	if !cmd.HasSubCommands() {
		installStrayArgumentGuard(cmd)
		return
	}
	if cmd.Args != nil {
		return
	}
	cmd.Args = rejectUnknownSubcommand
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if !cmd.Runnable() {
		// A group with no action of its own never reaches its argument check: cobra returns
		// "print the help" for an unrunnable command BEFORE validating arguments, which is why
		// setting Args alone left `pinchtab cache clera` at exit 0. Printing the help is what
		// the command already did for no arguments, so this keeps that and nothing more.
		cmd.RunE = printGroupHelp
	}
}

func printGroupHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

func rejectUnknownSubcommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	message := fmt.Sprintf("unknown command %q for %q\nValid subcommands: %s",
		args[0], cmd.CommandPath(), strings.Join(subcommandNames(cmd), ", "))
	return newCommandExitError(unknownSubcommandExitCode, fmt.Errorf("%s", message))
}

func subcommandNames(cmd *cobra.Command) []string {
	var names []string
	for _, sub := range cmd.Commands() {
		if sub.IsAvailableCommand() {
			names = append(names, sub.Name())
		}
	}
	sort.Strings(names)
	return names
}

// installStrayArgumentGuard is the leaf half of the same rule. Cobra defaults a command
// with no subcommands and no Args to ArbitraryArgs, which accepts any positional and
// hands it to Run, where it is dropped — so `pinchtab screenshot shot.png` exited 0
// having written an auto-named file somewhere else, and an agent had no way to learn it
// had asked for the wrong thing.
//
// Three states, and only the first two are this guard's business:
//
//	Args == nil            cobra's accept-everything default: refuse a stray argument
//	permits no positional  already refuses, but without the silence flags its refusal
//	                       arrives twice wrapped in a usage block
//	permits a positional   the real opt-out — the leaf takes an operand, so leave it alone
//
// The opt-out is the validator's own answer, not the Use string: parsing "[id]" out of
// help text to decide behaviour makes the prose load-bearing and drifts.
func installStrayArgumentGuard(cmd *cobra.Command) {
	switch {
	case cmd.Args == nil:
		cmd.Args = rejectStrayArgument
	case permitsNoPositionals(cmd):
	default:
		return
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
}

// argumentProbe is the token the classification asks the validator about. It is never
// rendered: a validator's answer to "would you take one argument?" is what separates a
// leaf that takes an operand from one that only looks like it might.
const argumentProbe = "zzz-argument-probe"

// permitsNoPositionals reports a validator that accepts none and refuses one — cobra.NoArgs
// and anything equivalent. Both halves are needed: a command wanting exactly two arguments
// also refuses one, and it is an operand-taker rather than a leaf to silence.
func permitsNoPositionals(cmd *cobra.Command) bool {
	if cmd.Args == nil {
		return false
	}
	return cmd.Args(cmd, nil) == nil && cmd.Args(cmd, []string{argumentProbe}) != nil
}

func rejectStrayArgument(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	message := fmt.Sprintf("unexpected argument %q for %q\n%s",
		args[0], cmd.CommandPath(), strayArgumentRemedy(cmd, args[0]))
	return newCommandExitError(unknownSubcommandExitCode, fmt.Errorf("%s", message))
}

// strayArgumentRemedy names the flag the caller most likely meant. The commands this
// guard was filed for take a path through -o, and `pinchtab screenshot shot.png` is one
// keystroke from correct; every other leaf is pointed at its own help rather than
// guessed at. Derived from the command's own flag set, so a leaf that gains or loses an
// output flag says the right thing without being listed anywhere.
func strayArgumentRemedy(cmd *cobra.Command, stray string) string {
	if cmd.Flags().Lookup("output") != nil {
		return fmt.Sprintf("This command takes no positional arguments; did you mean -o %s?", stray)
	}
	return fmt.Sprintf("This command takes no positional arguments; run %q for its flags.", cmd.CommandPath()+" --help")
}

// rejectExtraArguments is rejectStrayArgument for a leaf whose FIRST argument is data: the
// second one is the same mistake one position along. It lives here so every refusal in the
// CLI is worded and exit-coded in one place, and a leaf declares it rather than the guard
// inferring an arity from anything.
func rejectExtraArguments(cmd *cobra.Command, args []string) error {
	if len(args) <= 1 {
		return nil
	}
	message := fmt.Sprintf("unexpected argument %q for %q\nThis command takes one argument; run %q for its usage.",
		args[1], cmd.CommandPath(), cmd.CommandPath()+" --help")
	return newCommandExitError(unknownSubcommandExitCode, fmt.Errorf("%s", message))
}
