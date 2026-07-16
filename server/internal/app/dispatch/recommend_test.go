package dispatch

import "testing"

func TestRankTechniciansBySkillAreaShiftThenLoad(t *testing.T) {
	candidates := []TechnicianCandidate{
		{ID: "load-low", Skills: []string{"cleaning"}, Areas: []string{"north"}, ShiftAvailable: true, Load: 1},
		{ID: "wrong-skill", Skills: []string{"repair"}, Areas: []string{"north"}, ShiftAvailable: true, Load: 0},
		{ID: "perfect", Skills: []string{"cleaning"}, Areas: []string{"north"}, ShiftAvailable: true, Load: 0},
		{ID: "out-of-area", Skills: []string{"cleaning"}, Areas: []string{"south"}, ShiftAvailable: true, Load: 0},
	}
	got := RankTechnicians(candidates, "cleaning", "north")
	if len(got) != 4 || got[0].ID != "perfect" || got[1].ID != "load-low" {
		t.Fatalf("unexpected ranking: %#v", got)
	}
}
