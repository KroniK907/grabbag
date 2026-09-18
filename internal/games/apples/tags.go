package apples

import (
	"hash/fnv"
	"sort"
)

var tagPalette = []string{
	"#ff5ec8",
	"#7cff6b",
	"#ffe66d",
	"#6ecbff",
	"#ff8a4c",
	"#c38fff",
	"#4dffd2",
	"#ff6b6b",
}

func collectCatalogTags(cat catalog) []string {
	seen := map[string]struct{}{}
	for _, lib := range cat.Libraries {
		for _, pack := range lib.Packs {
			for _, tag := range packTags(pack) {
				seen[tag] = struct{}{}
			}
		}
	}
	return sortedKeys(seen)
}

func packTags(pack packFile) []string {
	seen := map[string]struct{}{}
	for _, card := range pack.Prompts {
		for _, tag := range card.Tags {
			if tag != "" {
				seen[tag] = struct{}{}
			}
		}
	}
	for _, card := range pack.Answers {
		for _, tag := range card.Tags {
			if tag != "" {
				seen[tag] = struct{}{}
			}
		}
	}
	return sortedKeys(seen)
}

func sortedKeys(seen map[string]struct{}) []string {
	out := make([]string, 0, len(seen))
	for tag := range seen {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

func catalogHasTag(cat catalog, tag string) bool {
	if tag == "" {
		return false
	}
	for _, have := range collectCatalogTags(cat) {
		if have == tag {
			return true
		}
	}
	return false
}

func tagColor(id string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return tagPalette[int(h.Sum32())%len(tagPalette)]
}

func (s *matchSettings) tagOn(tag string) bool {
	if s == nil || s.Tags == nil {
		return true
	}
	on, known := s.Tags[tag]
	if !known {
		return true
	}
	return on
}

func (s *matchSettings) setTag(tag string, on bool) {
	if s.Tags == nil {
		s.Tags = map[string]bool{}
	}
	s.Tags[tag] = on
}

func (s matchSettings) cardTagsAllowed(tags []string) bool {
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		if !s.tagOn(tag) {
			return false
		}
	}
	return true
}

func tagViews(ids []string, settings matchSettings) []tagView {
	out := make([]tagView, 0, len(ids))
	for _, id := range ids {
		out = append(out, tagView{ID: id, Color: tagColor(id), Enabled: settings.tagOn(id)})
	}
	return out
}
