package messages

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"howett.net/plist"
)

type revisionState struct {
	HasEdits       bool
	HasUnsentParts bool
	FullyUnsent    bool
	RevisionAt     int64
	Identity       string
}

func parseMessageSummaryInfo(data []byte) revisionState {
	root, ok := messageSummaryRoot(data)
	if !ok {
		return revisionState{}
	}
	return parseMessageSummaryRoot(root)
}

func messageSummaryRoot(data []byte) (map[string]any, bool) {
	if len(data) == 0 {
		return nil, false
	}
	var root map[string]any
	if _, err := plist.Unmarshal(data, &root); err != nil {
		return nil, false
	}
	return root, true
}

func parseMessageSummaryRoot(root map[string]any) revisionState {
	parts, ok := stringMap(root["otr"])
	if !ok || len(parts) == 0 {
		return revisionState{}
	}
	state := revisionState{}
	if edits, ok := stringMap(root["ec"]); ok {
		for _, value := range edits {
			events, ok := value.([]any)
			if !ok || len(events) == 0 {
				continue
			}
			state.HasEdits = true
			for _, event := range events {
				fields, ok := stringMap(event)
				if !ok {
					continue
				}
				nanoseconds, ok := timestampNanoseconds(fields["d"])
				if ok {
					state.RevisionAt = max(state.RevisionAt, nanoseconds)
				}
			}
		}
	}
	unsent, _ := root["rp"].([]any)
	indexes := map[int64]bool{}
	for _, value := range unsent {
		index, ok := integer(value)
		if ok && index >= 0 && index < int64(len(parts)) {
			indexes[index] = true
		}
	}
	state.HasUnsentParts = len(indexes) > 0
	state.FullyUnsent = len(indexes) == len(parts)
	if state.HasEdits || state.HasUnsentParts {
		orderedIndexes := make([]int64, 0, len(indexes))
		for index := range indexes {
			orderedIndexes = append(orderedIndexes, index)
		}
		sort.Slice(orderedIndexes, func(i, j int) bool { return orderedIndexes[i] < orderedIndexes[j] })
		identity, err := json.Marshal(struct {
			PartCount int     `json:"part_count"`
			Edits     any     `json:"edits,omitempty"`
			Unsent    []int64 `json:"unsent,omitempty"`
		}{PartCount: len(parts), Edits: root["ec"], Unsent: orderedIndexes})
		if err == nil {
			state.Identity = string(identity)
		}
	}
	return state
}

func reconstructCurrentText(root map[string]any, original string) (string, bool) {
	parts, ok := stringMap(root["otr"])
	if !ok || len(parts) == 0 {
		return "", false
	}
	edits, _ := stringMap(root["ec"])
	unsentValues, _ := root["rp"].([]any)
	unsent := make(map[int64]bool, len(unsentValues))
	for _, value := range unsentValues {
		if index, ok := integer(value); ok {
			unsent[index] = true
		}
	}
	originalUnits := utf16.Encode([]rune(original))
	var current strings.Builder
	for index := 0; index < len(parts); index++ {
		key := strconv.Itoa(index)
		part, ok := stringMap(parts[key])
		if !ok {
			return "", false
		}
		offset, offsetOK := integer(part["lo"])
		length, lengthOK := integer(part["le"])
		if !offsetOK || !lengthOK || offset < 0 || length < 0 {
			return "", false
		}
		if editEvents, edited := edits[key]; edited {
			text, ok := latestEditedPartText(editEvents)
			if !ok {
				return "", false
			}
			if !unsent[int64(index)] {
				current.WriteString(text)
			}
			continue
		}
		if unsent[int64(index)] {
			continue
		}
		if offset < 0 || offset > int64(len(originalUnits)) || length > int64(len(originalUnits))-offset {
			return "", false
		}
		current.WriteString(string(utf16.Decode(originalUnits[offset : offset+length])))
	}
	return current.String(), true
}

func latestEditedPartText(value any) (string, bool) {
	events, ok := value.([]any)
	if !ok || len(events) == 0 {
		return "", false
	}
	fields, ok := stringMap(events[len(events)-1])
	if !ok {
		return "", false
	}
	data, ok := fields["t"].([]byte)
	if !ok {
		return "", false
	}
	return decodeAttributedBodyValue(data)
}

func stringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			text, ok := key.(string)
			if !ok {
				return nil, false
			}
			out[text] = item
		}
		return out, true
	default:
		return nil, false
	}
}

func integer(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case uint64:
		if typed <= math.MaxInt64 {
			return int64(typed), true
		}
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		return parsed, err == nil
	}
	return 0, false
}

func timestampNanoseconds(value any) (int64, bool) {
	if seconds, ok := integer(value); ok {
		if seconds > 0 && seconds <= math.MaxInt64/1_000_000_000 {
			return seconds * 1_000_000_000, true
		}
		return 0, false
	}
	seconds, ok := value.(float64)
	if !ok || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 {
		return 0, false
	}
	whole, fraction := math.Modf(seconds)
	if whole > float64(math.MaxInt64/1_000_000_000) {
		return 0, false
	}
	wholeNanoseconds := int64(whole) * 1_000_000_000
	fractionNanoseconds := int64(math.Round(fraction * 1_000_000_000))
	if fractionNanoseconds > math.MaxInt64-wholeNanoseconds {
		return 0, false
	}
	return wholeNanoseconds + fractionNanoseconds, true
}
