package actions

import (
	"net/http"
	"net/url"

	"github.com/pinchtab/pinchtab/internal/cli/apiclient"
	"github.com/spf13/cobra"
)

func Snapshot(client *http.Client, base, token string, cmd *cobra.Command, selectorOverride string) {
	params := url.Values{}
	full, _ := cmd.Flags().GetBool("full")
	if !full {
		if v, _ := cmd.Flags().GetBool("interactive"); v {
			params.Set("filter", "interactive")
		}
		if v, _ := cmd.Flags().GetBool("compact"); v {
			params.Set("format", "compact")
		}
	}
	if v, _ := cmd.Flags().GetBool("text"); v {
		params.Set("format", "text")
	}
	if v, _ := cmd.Flags().GetBool("diff"); v {
		params.Set("diff", "true")
	}
	if v, _ := cmd.Flags().GetString("selector"); v != "" {
		params.Set("selector", v)
	} else if selectorOverride != "" {
		params.Set("selector", selectorOverride)
	}
	if v, _ := cmd.Flags().GetString("max-tokens"); v != "" {
		params.Set("maxTokens", v)
	}
	if v, _ := cmd.Flags().GetString("depth"); v != "" {
		params.Set("depth", v)
	}
	if v, _ := cmd.Flags().GetString("tab"); v != "" {
		params.Set("tabId", v)
	}
	result := apiclient.DoGetCapturingVocab(client, base, token, "/snapshot", params, params.Get("tabId"))
	apiclient.SuggestNextAction("snapshot", result)
}
