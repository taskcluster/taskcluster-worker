package runtime

import (
	"fmt"
	"sort"
)

// A Section represents a section of markdown documentation.
type Section struct {
	Title   string // Title of section rendered as headline level 2
	Content string // Section contents, maybe contains headline level 3 and higher
}

// RenderDocument creates a markdown document with given title from a list of
// sections. Ordering sections alphabetically.
func RenderDocument(title string, sections []Section) string {
	sections = append([]Section{}, sections...)
	sort.Slice(sections, func(i int, j int) bool {
		return sections[i].Title < sections[j].Title
	})

	docs := fmt.Sprintf("# %s\n", title)
	for _, section := range sections {
		docs += fmt.Sprintf("\n\n## %s\n%s\n", section.Title, section.Content)
	}
	return docs
}
