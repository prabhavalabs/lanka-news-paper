package reels

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Library is the folder of licensed soundtracks on the server. tracks.json, when present, lists
// each file with a display title and the second where the reel should start (on a big hit):
//
//	[{"file": "epic.mp3", "title": "Epic Cinematic", "start": 28.2}]
//
// Without tracks.json every audio file in the folder is used from its start.
type Library struct{ Dir string }

type track struct {
	File  string  `json:"file"`
	Title string  `json:"title"`
	Start float64 `json:"start"`
}

func (library Library) tracks() []track {
	if library.Dir == "" {
		return nil
	}
	var listed []track
	if data, err := os.ReadFile(filepath.Join(library.Dir, "tracks.json")); err == nil && json.Unmarshal(data, &listed) == nil {
		available := listed[:0]
		for _, t := range listed {
			if t.File != "" && !strings.Contains(t.File, "/") && fileExists(filepath.Join(library.Dir, t.File)) {
				available = append(available, t)
			}
		}
		return available
	}
	entries, err := os.ReadDir(library.Dir)
	if err != nil {
		return nil
	}
	var found []track
	for _, entry := range entries {
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".mp3", ".m4a", ".wav", ".ogg", ".flac":
			found = append(found, track{File: entry.Name(), Title: strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].File < found[j].File })
	return found
}

// choose rotates through the library, preferring the first track not used by recent reels.
func (library Library) choose(recent []string) (track, string, bool) {
	tracks := library.tracks()
	if len(tracks) == 0 {
		return track{}, "", false
	}
	window := min(len(recent), len(tracks)-1)
	for _, t := range tracks {
		if !slices.Contains(recent[:window], t.File) {
			return t, filepath.Join(library.Dir, t.File), true
		}
	}
	return tracks[0], filepath.Join(library.Dir, tracks[0].File), true
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
