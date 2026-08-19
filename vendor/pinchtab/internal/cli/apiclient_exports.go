package cli

import (
	"net/http"
	"net/url"

	"github.com/pinchtab/pinchtab/internal/cli/apiclient"
)

func DoGet(client *http.Client, base, token, path string, params url.Values) map[string]any {
	return apiclient.DoGet(client, base, token, path, params)
}

func DoPost(client *http.Client, base, token, path string, body map[string]any) map[string]any {
	return apiclient.DoPost(client, base, token, path, body)
}

func ResolveInstanceBase(orchBase, token, instanceID, bind string) string {
	return apiclient.ResolveInstanceBase(orchBase, token, instanceID, bind)
}

func CheckServerAndGuide(client *http.Client, base, token string) bool {
	return apiclient.CheckServerAndGuide(client, base, token)
}
