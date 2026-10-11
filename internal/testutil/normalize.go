package testutil

import (
	"regexp"
	"strings"
)

var (
	timestampRE = regexp.MustCompile(`\b\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)?\b|\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z\b|\b\d{2}:\d{2}:\d{2}\b`)
	durationRE  = regexp.MustCompile(`\b\d+h?\d*m?\d*(?:\.\d+)?s\b|\b\d{2}:\d{2}:\d{2}\b`)
	uuidRE      = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F-]{27}\b`)
	shortIDRE   = regexp.MustCompile(`\b[0-9a-fA-F]{8}\b`)
	hashRE      = regexp.MustCompile(`\b[0-9a-fA-F]{40,64}\b`)
	slurmIDRE   = regexp.MustCompile(`(?i)(\bslurm(?: job| ID)?[=: ]+)\d+\b`)
	tempPathRE  = regexp.MustCompile(`/tmp/[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)+`)
)

// Normalize makes CLI output stable while preserving command text and
// meaningful wording. It deliberately does not replace arbitrary numbers.
func Normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = timestampRE.ReplaceAllString(s, "<TIMESTAMP>")
	s = durationRE.ReplaceAllString(s, "<DURATION>")
	s = uuidRE.ReplaceAllString(s, "<LOCAL_ID>")
	s = hashRE.ReplaceAllString(s, "<CASE_HASH>")
	s = slurmIDRE.ReplaceAllString(s, `${1}<SLURM_ID>`)
	s = tempPathRE.ReplaceAllString(s, "<TEMP_PATH>")
	s = shortIDRE.ReplaceAllString(s, "<ID>")
	s = strings.ReplaceAll(s, "<TEMP_PATH><CASE_HASH>", "<TEMP_PATH>/<CASE_HASH>")
	s = strings.ReplaceAll(s, "<TEMP_PATH><LOCAL_ID>", "<TEMP_PATH>/<LOCAL_ID>")
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	s = strings.Join(lines, "\n")
	return s
}
