package httpx_test

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/srccensus"
)

// jsonCall matches a JSON write and captures the status it answers with. The status is
// always on the call line even when the payload spans several, which is what makes a text
// scan honest here.
var jsonCall = regexp.MustCompile(`\bhttpx\.JSON\(\s*[^,]+,\s*([A-Za-z0-9_.]+)`)

// A failure written as a hand-shaped JSON body carries its reason nowhere: the middlewares
// see a status and nothing else, which is the defect this closed for the ~550 responses
// that already went through ErrorCode. httpx.JSONError is the way to keep a bespoke body
// AND record the reason, so a non-2xx httpx.JSON call anywhere in the module is a sink
// going dark again.
//
// Module-wide rather than per-package: the four files this started with were found by
// grep, and a fifth added in a package nobody thought to list is exactly what a
// hand-listed scope misses.
func TestNoHandWrittenJSONErrorEscapesTheReasonRecorder(t *testing.T) {
	root := filepath.Join("..", "..")
	files := srccensus.Tree(t, root, 200)

	total := 0
	used := map[string]bool{}
	var offenders []string
	for _, file := range files {
		for i, line := range strings.Split(file.Text, "\n") {
			match := jsonCall.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			total++
			site := file.Name + " | " + strings.TrimSpace(line)
			if _, exempt := variableStatusExemptions[site]; exempt {
				used[site] = true
				continue
			}
			status, known := statusOf(match[1])
			if !known {
				offenders = append(offenders, file.Name+":"+strconv.Itoa(i+1)+" answers a status this census cannot read ("+match[1]+"); name it so the rule keeps applying:\n\t"+strings.TrimSpace(line))
				continue
			}
			if status < 400 {
				continue
			}
			offenders = append(offenders, file.Name+":"+strconv.Itoa(i+1)+" writes a "+match[1]+" through httpx.JSON, so its reason reaches no sink; use httpx.JSONError(w, status, code, message, payload):\n\t"+strings.TrimSpace(line))
		}
	}

	if total < 20 {
		t.Fatalf("found %d httpx.JSON call sites in the module, want many more; the pattern has stopped matching and this census guards nothing", total)
	}
	if len(offenders) > 0 {
		t.Errorf("%d failure response(s) bypass the reason recorder:\n  %s", len(offenders), strings.Join(offenders, "\n  "))
	}
	for site := range variableStatusExemptions {
		if !used[site] {
			t.Errorf("the exemption for %q matches nothing any more; delete it rather than leaving a licence for a site that moved", site)
		}
	}
}

// variableStatusExemptions names the call sites whose status is a variable this text scan
// cannot resolve, with the reason each is not a failure response. Checked in both
// directions: an entry matching nothing fails too, so a moved site loses its licence
// instead of keeping it silently.
var variableStatusExemptions = map[string]string{
	"internal/orchestrator/handlers_instances.go | httpx.JSON(w, status, inst)": "auditAndRespondAttach is a success responder — both call sites pass 200 or 201, and an attach that fails is written by httpx.Error long before it is reached",
}

// statusOf resolves the literal or net/http constant a call answers with. An unrecognised
// spelling is reported rather than skipped: a silent skip is how a census stops covering
// the site it was written for.
func statusOf(token string) (int, bool) {
	if n, err := strconv.Atoi(token); err == nil {
		return n, true
	}
	name, found := strings.CutPrefix(token, "http.Status")
	if !found {
		return 0, false
	}
	status, known := map[string]int{
		"OK": 200, "Created": 201, "Accepted": 202, "NoContent": 204,
		"BadRequest": 400, "Unauthorized": 401, "Forbidden": 403, "NotFound": 404,
		"MethodNotAllowed": 405, "Conflict": 409, "Gone": 410, "RequestEntityTooLarge": 413,
		"UnprocessableEntity": 422, "Locked": 423, "TooManyRequests": 429,
		"InternalServerError": 500, "NotImplemented": 501, "BadGateway": 502,
		"ServiceUnavailable": 503, "GatewayTimeout": 504,
	}[name]
	return status, known
}
