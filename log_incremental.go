package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

type codexScanState struct {
	Total *codexTokenUsage
	Model string
}
type incrementalLog struct {
	Offset, Size, Modified int64
	Prefix                 []byte
	Records                []UsageRecord
	State                  codexScanState
}

var incrementalLogs = struct {
	sync.Mutex
	Items map[string]incrementalLog
}{Items: map[string]incrementalLog{}}

func (ls *LogService) cachedIncremental(path, provider, project string, cutoff time.Time) []UsageRecord {
	incrementalLogs.Lock()
	defer incrementalLogs.Unlock()
	info, e := os.Stat(path)
	if e != nil {
		delete(incrementalLogs.Items, path)
		return nil
	}
	old, ok := incrementalLogs.Items[path]
	if ok && old.Size == info.Size() && old.Modified == info.ModTime().UnixNano() {
		return filterRecordsByCutoff(old.Records, cutoff)
	}
	f, e := os.Open(path)
	if e != nil {
		return nil
	}
	defer f.Close()
	prefix := make([]byte, min(info.Size(), 4096))
	_, _ = io.ReadFull(f, prefix)
	if !ok || info.Size() < old.Size || !bytes.HasPrefix(prefix, old.Prefix) || info.Size() == old.Size {
		old = incrementalLog{State: codexScanState{Model: "gpt-5-codex"}}
	}
	_, _ = f.Seek(old.Offset, io.SeekStart)
	// Keep only complete JSONL lines in the checkpoint; a partially written final line is retried.
	tail, e := io.ReadAll(io.LimitReader(f, 128<<20))
	if e != nil {
		return filterRecordsByCutoff(old.Records, cutoff)
	}
	last := bytes.LastIndexByte(tail, '\n')
	if len(tail) > last+1 && json.Valid(bytes.TrimSpace(tail[last+1:])) {
		last = len(tail) - 1
	}
	if last < 0 {
		return filterRecordsByCutoff(old.Records, cutoff)
	}
	tail = tail[:last+1]
	var records []UsageRecord
	if provider == "claude" {
		records, e = ls.parseClaudeReader(bytes.NewReader(tail), path, project, time.Time{})
	} else {
		records, e = ls.parseCodexReader(bytes.NewReader(tail), path, project, time.Time{}, &old.State)
	}
	if e != nil {
		return filterRecordsByCutoff(old.Records, cutoff)
	}
	old.Records = dedupUsageRecords(append(old.Records, records...))
	old.Offset += int64(len(tail))
	old.Size = info.Size()
	if old.Offset < info.Size() && len(tail) >= 127<<20 {
		old.Size = -1
	}
	old.Modified = info.ModTime().UnixNano()
	old.Prefix = prefix
	incrementalLogs.Items[path] = old
	// Bound cached file count and remove logs that have been deleted or rotated.
	if len(incrementalLogs.Items) > 2000 {
		for p := range incrementalLogs.Items {
			if p != path {
				delete(incrementalLogs.Items, p)
				if len(incrementalLogs.Items) <= 1500 {
					break
				}
			}
		}
	}
	return filterRecordsByCutoff(old.Records, cutoff)
}
