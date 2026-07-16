package dispatch

import "sort"

type TechnicianCandidate struct {
	ID             string
	Skills         []string
	Areas          []string
	ShiftAvailable bool
	Load           int
}

func RankTechnicians(candidates []TechnicianCandidate, requiredSkill, area string) []TechnicianCandidate {
	result := append([]TechnicianCandidate(nil), candidates...)
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		aSkill, bSkill := contains(a.Skills, requiredSkill), contains(b.Skills, requiredSkill)
		if aSkill != bSkill {
			return aSkill
		}
		aArea, bArea := contains(a.Areas, area), contains(b.Areas, area)
		if aArea != bArea {
			return aArea
		}
		if a.ShiftAvailable != b.ShiftAvailable {
			return a.ShiftAvailable
		}
		return a.Load < b.Load
	})
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
