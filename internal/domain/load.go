package domain

import (
	"sort"
	"time"
)

// Load represents a consumer load on the microgrid bus. Priority 1 loads
// (hospitals, water supply) must be preserved; lower-priority loads are shed in
// sequence when generation is insufficient.
type Load struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Priority  int       `json:"priority"` // 1 = critical, 2 = secondary, 3 = tertiary
	Energized bool      `json:"energized"`
	Shed      bool      `json:"shed"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ShedLoadsByPriority sheds loads in descending priority order (tertiary first)
// while keeping every load whose priority is at or below keepPriority
// energized. The function returns a new slice and the number of loads that were
// newly shed.
func ShedLoadsByPriority(loads []Load, keepPriority int, now time.Time) ([]Load, int) {
	sorted := make([]Load, len(loads))
	copy(sorted, loads)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})

	shedCount := 0
	for i := range sorted {
		if sorted[i].Priority <= keepPriority {
			sorted[i].Shed = false
			sorted[i].Energized = true
		} else if sorted[i].Energized && !sorted[i].Shed {
			sorted[i].Shed = true
			sorted[i].Energized = false
			shedCount++
		}
		sorted[i].UpdatedAt = now
	}
	return sorted, shedCount
}
