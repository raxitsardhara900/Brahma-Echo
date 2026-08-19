package main

import (
	"strings"
	"testing"
)

// `cookies --help` is where a caller looks for a way to read or set a cookie, and it
// used to answer with delete-everything only. Short is what that listing shows, so the
// verbs and the breadth of clear have to be readable there.
func TestCookiesNamespaceOffersReadAndWriteVerbs(t *testing.T) {
	verbs := map[string]*string{}
	for _, sub := range cookiesCmd.Commands() {
		verbs[sub.Name()] = &sub.Short
	}

	for _, want := range []string{"get", "set", "clear"} {
		if _, ok := verbs[want]; !ok {
			t.Errorf("pinchtab cookies has no %q verb; its subcommands are %v", want, cookiesCmd.Commands())
		}
	}

	clear, ok := verbs["clear"]
	if !ok {
		t.Fatal("no clear verb to check")
	}
	if !strings.Contains(strings.ToLower(*clear), "all origins") {
		t.Errorf("cookies clear Short = %q; --help shows Short only, so the browser-wide effect must be stated there", *clear)
	}
}

func TestCookiesSetTakesTheAttributeFlagsAndATab(t *testing.T) {
	set, _, err := cookiesCmd.Find([]string{"set"})
	if err != nil || set.Name() != "set" {
		t.Fatalf("pinchtab cookies has no set verb: %v", err)
	}
	if set.Args == nil {
		t.Error("cookies set declares no argument count, so a missing value is not rejected")
	}
	for _, flag := range []string{"tab", "url", "domain", "path", "same-site", "secure", "http-only", "json"} {
		if set.Flags().Lookup(flag) == nil {
			t.Errorf("cookies set has no --%s flag", flag)
		}
	}
}
