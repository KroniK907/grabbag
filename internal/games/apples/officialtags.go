package apples

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
)

const (
	officialTagsFormatVersion = 1
	kidFriendlyTag            = "kid friendly"
)

var officialTagNames = map[string]bool{
	"sex":          true,
	"violence":     true,
	"politics":     true,
	kidFriendlyTag: true,
}

var officialTagKeyPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

// officialTagsRaw is the reviewed tag overlay for JSON Against Humanity.
// It holds only text hashes and tags, never card text. A key with an empty
// list was reviewed and left untagged. A missing key was never reviewed.
//
//go:embed cahtags/official.json
var officialTagsRaw []byte

type officialTagFile struct {
	FormatVersion int                 `json:"formatVersion"`
	Source        string              `json:"source,omitempty"`
	Cards         map[string][]string `json:"cards"`
}

var loadOfficialTags = sync.OnceValues(func() (map[string][]string, error) {
	return parseOfficialTags(officialTagsRaw)
})

func parseOfficialTags(raw []byte) (map[string][]string, error) {
	var file officialTagFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("official tags: unreadable JSON")
	}
	if file.FormatVersion != officialTagsFormatVersion {
		return nil, fmt.Errorf("official tags: formatVersion must be %d", officialTagsFormatVersion)
	}
	if file.Cards == nil {
		file.Cards = map[string][]string{}
	}
	for key, tags := range file.Cards {
		if !officialTagKeyPattern.MatchString(key) {
			return nil, fmt.Errorf("official tags: bad key %q", key)
		}
		if err := checkOfficialTags(tags); err != nil {
			return nil, fmt.Errorf("official tags: %s: %w", key, err)
		}
	}
	return file.Cards, nil
}

// officialTagKey is the overlay key for card text: the first 8 bytes of the
// SHA-256 of the normalized text, hex encoded. Card ids end in the same hash.
func officialTagKey(text string) string {
	sum := sha256.Sum256([]byte(normalizeCardText(text)))
	return hex.EncodeToString(sum[:8])
}

func officialTagsFor(tags map[string][]string, text string) []string {
	found := tags[officialTagKey(text)]
	if len(found) == 0 {
		return nil
	}
	return append([]string(nil), found...)
}

// checkOfficialTags enforces the four known tags, no repeats, and that
// kid friendly never sits beside sex, violence, or politics.
func checkOfficialTags(tags []string) error {
	seen := map[string]bool{}
	for _, tag := range tags {
		if !officialTagNames[tag] {
			return fmt.Errorf("unknown tag %q", tag)
		}
		if seen[tag] {
			return fmt.Errorf("duplicate tag %q", tag)
		}
		seen[tag] = true
	}
	if seen[kidFriendlyTag] && len(seen) > 1 {
		return fmt.Errorf("kid friendly combined with a mature tag")
	}
	return nil
}
