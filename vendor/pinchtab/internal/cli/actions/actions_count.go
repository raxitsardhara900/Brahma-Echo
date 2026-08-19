package actions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pinchtab/pinchtab/internal/cli/apiclient"
	"github.com/pinchtab/pinchtab/internal/cli/output"
	"github.com/spf13/cobra"
)

// Count returns the number of elements matching a CSS selector.
func Count(client *http.Client, base, token string, cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		output.Error("count", "selector argument is required", 1)
		return
	}
	selector := args[0]

	params := url.Values{}
	params.Set("selector", selector)
	if v, _ := cmd.Flags().GetString("tab"); v != "" {
		params.Set("tabId", v)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")
	if jsonOutput {
		apiclient.DoGet(client, base, token, "/count", params)
		return
	}

	body := apiclient.DoGetRaw(client, base, token, "/count", params)
	var result struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		output.Value(string(body))
		return
	}
	output.Value(fmt.Sprintf("%d", result.Count))
}
