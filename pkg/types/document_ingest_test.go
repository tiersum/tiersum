package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateDocumentRequest_EffectiveIngestMode(t *testing.T) {
	assert.Equal(t, DocumentIngestModeHot, (CreateDocumentRequest{}).EffectiveIngestMode())
	assert.Equal(t, DocumentIngestModeHot, (CreateDocumentRequest{ForceHot: true}).EffectiveIngestMode())
	assert.Equal(t, DocumentIngestModeCold, (CreateDocumentRequest{IngestMode: "cold", ForceHot: true}).EffectiveIngestMode())
	assert.Equal(t, DocumentIngestModeHot, (CreateDocumentRequest{IngestMode: "HOT"}).EffectiveIngestMode())
	assert.Equal(t, DocumentIngestModeHot, (CreateDocumentRequest{IngestMode: "unknown"}).EffectiveIngestMode())
}
