package actions

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/pinchtab/pinchtab/internal/cli/apiclient"
	"github.com/pinchtab/pinchtab/internal/cli/output"
	"github.com/spf13/cobra"
)

// Enabled checks whether an element identified by ref is enabled (not disabled).
func Enabled(client *http.Client, base, token string, cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		output.Error("enabled", "ref argument is required", 1)
		return
	}
	ref := args[0]

	params := url.Values{}
	params.Set("ref", ref)
	if v, _ := cmd.Flags().GetString("tab"); v != "" {
		params.Set("tabId", v)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")
	if jsonOutput {
		apiclient.DoGet(client, base, token, "/enabled", params)
		return
	}

	body := apiclient.DoGetRaw(client, base, token, "/enabled", params)
	var result struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		output.Value(string(body))
		return
	}
	if result.Enabled {
		output.Value("true")
	} else {
		output.Value("false")
	}
}
