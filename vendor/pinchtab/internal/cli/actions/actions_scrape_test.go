package actions

import (
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/scrape"
)

func previewReport(totalURLsInSitemap int) scrape.Report {
	return scrape.Report{
		Site: scrape.SiteInfo{
			BaseURL:            "https://example.com",
			TotalURLsInSitemap: totalURLsInSitemap,
			SampledPages:       2,
		},
		Pages: []scrape.Page{
			{URL: "https://example.com/", CharCount: 4229},
			{URL: "https://example.com/docs", CharCount: 2332},
		},
	}
}

func TestPrintPreviewSummaryWithoutSitemapOmitsTheTotal(t *testing.T) {
	report := previewReport(0)

	out := captureStdout(t, func() { printPreviewSummary(report) })

	header := strings.SplitN(out, "\n", 2)[0]
	if strings.Contains(header, " of ") {
		t.Fatalf("preview claims a total it does not have: %q", header)
	}
	if !strings.Contains(header, "2 page(s) sampled") {
		t.Fatalf("preview does not report the pages it lists: %q", header)
	}
	for _, page := range report.Pages {
		if !strings.Contains(out, page.URL) {
			t.Fatalf("preview omits %s:\n%s", page.URL, out)
		}
	}
}

func TestPrintPreviewSummaryWithSitemapNamesTheSitemapTotal(t *testing.T) {
	report := previewReport(42)

	out := captureStdout(t, func() { printPreviewSummary(report) })

	header := strings.SplitN(out, "\n", 2)[0]
	if !strings.Contains(header, "2 page(s) sampled of 42 in sitemap") {
		t.Fatalf("preview header = %q", header)
	}
}

// The preview line and the markdown report describe the same two numbers, so
// they must be built from the same summary rather than two hand-written forms.
func TestPreviewHeaderMatchesTheMarkdownReportLine(t *testing.T) {
	for _, total := range []int{0, 42} {
		report := previewReport(total)

		out := captureStdout(t, func() { printPreviewSummary(report) })
		header := strings.SplitN(out, "\n", 2)[0]
		summary := scrape.SampledPagesSummary(len(report.Pages), total)

		if !strings.Contains(header, summary) {
			t.Fatalf("total %d: preview header %q does not carry %q", total, header, summary)
		}
		if md := string(scrape.RenderMarkdown(report)); !strings.Contains(md, summary) {
			t.Fatalf("total %d: markdown report does not carry %q", total, summary)
		}
	}
}
