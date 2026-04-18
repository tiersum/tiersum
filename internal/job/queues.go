package job

import "github.com/tiersum/tiersum/pkg/types"

// PromoteQueue receives cold document IDs eligible for promotion to hot (see query path).
var PromoteQueue = make(chan string, 100)

// HotIngestQueue carries hot documents that need deferred analysis and chapter materialization after the document row exists.
var HotIngestQueue = make(chan types.HotIngestWork, 100)
