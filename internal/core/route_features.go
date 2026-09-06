package core

import "regexp"

// Text feature detectors shared by the request-summary builder (gateway) and
// complexity-aware route selectors. They scan the *current* user text only:
// complexity is about the task being asked, not the whole transcript.

var fencedCodeRE = regexp.MustCompile("(?s)```[\\w+-]*\\n.+?```")

// numberedItemRE matches lines starting with "1."-style list markers.
var numberedItemRE = regexp.MustCompile(`(?m)^\s*\d{1,2}[.)]\s+\S`)

// bulletItemRE matches lines starting with common bullet markers.
var bulletItemRE = regexp.MustCompile(`(?m)^\s*[-*•]\s+\S`)

// DetectCodeFence reports whether text contains a fenced code block.
func DetectCodeFence(text string) bool {
	return fencedCodeRE.MatchString(text)
}

// DetectMultiStepList reports whether text carries a multi-step plan:
// three or more numbered items, or four or more bullets. The asymmetry
// mirrors observed user behaviour (numbered plans are deliberate).
func DetectMultiStepList(text string) bool {
	if len(numberedItemRE.FindAllStringIndex(text, 3)) >= 3 {
		return true
	}
	return len(bulletItemRE.FindAllStringIndex(text, 4)) >= 4
}
