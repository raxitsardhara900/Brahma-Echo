package main

import "testing"

func TestSaveCommandsDeclareJSONOnlyWhereTheOutputShapeNeedsIt(t *testing.T) {
	want := map[string]bool{
		"capture":    true,
		"download":   false,
		"screenshot": false,
		"pdf":        false,
	}

	for name, wantJSON := range want {
		cmd, _, err := rootCmd.Find([]string{name})
		if err != nil || cmd == nil || cmd.Name() != name {
			t.Fatalf("%q is not a command on this CLI, so this guard would pass over nothing", name)
		}
		if cmd.Flags().Lookup("output") == nil && cmd.Flags().ShorthandLookup("o") == nil {
			t.Fatalf("%q declares no -o flag, so it is no longer one of the save commands this guard covers", name)
		}

		gotJSON := cmd.Flags().Lookup("json") != nil
		if gotJSON != wantJSON {
			t.Errorf("%q declares --json=%v, want %v; if that is intended, decide and pin what --json does when combined with -o", name, gotJSON, wantJSON)
		}
	}
}
