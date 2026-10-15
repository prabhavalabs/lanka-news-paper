package storyline

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestValidateGroups(t *testing.T) {
	events := []InputEvent{{ID: "a", Sources: 3}, {ID: "b", Sources: 3}, {ID: "c", Sources: 1}, {ID: "d", Sources: 1}}
	existing := []InputStoryline{{ID: "known"}}
	raw := `{"groups":[
 {"storyline_id":"unknown","title_si":"පුවත්","summary_si":"","event_ids":["a"]},
 {"storyline_id":"","title_si":"පුවත්","summary_si":"","event_ids":["a","b","unknown","a"]},
 {"storyline_id":"known","title_si":"පුවත්","summary_si":"සාරාංශය","event_ids":["a","c","already-assigned"]},
 {"storyline_id":"","title_si":"පුවත්","summary_si":"","event_ids":["d"]}]}`
	groups, err := validateGroups(raw, events, existing)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	require.Equal(t, []string{"a", "b"}, groups[0].EventIDs)
	require.Equal(t, []string{"c"}, groups[1].EventIDs)
	_, err = validateGroups(raw+`{}`, events, existing)
	require.Error(t, err)
	_, err = validateGroups(`{"extra":true,"groups":[]}`, events, existing)
	require.Error(t, err)
}
