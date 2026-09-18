package quips

// scoringMultiplier is the point multiplier for scoring round roundNum (GM-011-012).
func scoringMultiplier(roundNum, increaseBy int) int {
	if roundNum <= 1 || increaseBy <= 0 {
		return 1
	}
	m := 1
	for r := 2; r <= roundNum; r++ {
		m += increaseBy
		if m > 3 {
			m = 3
		}
	}
	return m
}

func applyMultiplier(basePts, mult int) int {
	if basePts <= 0 || mult <= 0 {
		return 0
	}
	return basePts * mult
}

type votePick struct {
	Voter  string
	Target string
	Seated bool
}

func tallySegmentPoints(picks map[string]votePick, seatedPts, audiencePts, mult int) map[string]int {
	awards := map[string]int{}
	for _, pick := range picks {
		if pick.Target == "" {
			continue
		}
		weight := audiencePts
		if pick.Seated {
			weight = seatedPts
		}
		awards[pick.Target] += applyMultiplier(weight, mult)
	}
	return awards
}

func coChampions(scores map[string]int, order []string) []string {
	if len(scores) == 0 {
		return nil
	}
	max := 0
	for _, id := range order {
		if s := scores[id]; s > max {
			max = s
		}
	}
	if max == 0 {
		return nil
	}
	var champs []string
	for _, id := range order {
		if scores[id] == max {
			champs = append(champs, id)
		}
	}
	return champs
}
