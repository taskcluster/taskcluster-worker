package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMardownUtil(t *testing.T) {
	assert.EqualValues(t, strings.Join([]string{
		"Hello `world`",
		"This is pretty neat",
		"\tYes",
		"\tWe can have",
		"\tindentation",
		"",
		"The end.",
	}, "\n"), Markdown(`
		Hello 'world'
		This is pretty neat
			Yes
			We can have
			indentation

		The end.
	`))
}
