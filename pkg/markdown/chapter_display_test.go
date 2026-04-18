package markdown

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChapterDisplayTitle(t *testing.T) {
	assert.Equal(t, "Hello · World", ChapterDisplayTitle("d1", "d1/Hello/World", "T"))
	// Path has no docID+"/" prefix: relative segment is the whole path (legacy behavior).
	assert.Equal(t, "d1", ChapterDisplayTitle("d1", "d1", "T"))
	assert.Equal(t, "Document", ChapterDisplayTitle("d1", "d1/", "  "))
	assert.Equal(t, "Only fallback", ChapterDisplayTitle("d1", "d1/", "Only fallback"))
}
