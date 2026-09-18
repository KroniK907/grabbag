package quips

const (
	startRefuseMsg  = "Not enough prompts in your library to start the game."
	startUnburnMsg  = "You may need to unburn some prompts or get more libraries to continue."
)

func promptDemand(settings matchSettings, seatedHumans int) int {
	if seatedHumans < 1 {
		return 0
	}
	r := settings.RoundCount
	if settings.LastQuipEnabled {
		return (r-1)*seatedHumans + 1
	}
	return r * seatedHumans
}

func enabledPromptCount(cat catalog, settings matchSettings) int {
	n := 0
	for _, lib := range cat.Libraries {
		for _, pack := range lib.Packs {
			if settings.packOn(lib.ID, pack.ID) {
				n += len(pack.Prompts)
			}
		}
	}
	return n
}

func startShortagePiles(piles promptPiles, settings matchSettings, seatedHumans int) string {
	if seatedHumans < 1 {
		return ""
	}
	need := promptDemand(settings, seatedHumans)
	if len(piles.Prompts) < need {
		return startRefuseMsg
	}
	return ""
}

func startShortage(cat catalog, settings matchSettings, seatedHumans int) string {
	piles := buildPromptPiles(cat, settings)
	return startShortagePiles(piles, settings, seatedHumans)
}

func burnedWouldHelp(raw, burned promptPiles, settings matchSettings, seatedHumans int) bool {
	return startShortagePiles(burned, settings, seatedHumans) != "" && startShortagePiles(raw, settings, seatedHumans) == ""
}

func shortageLines(piles promptPiles, settings matchSettings, seatedHumans int, burnedWouldHelp bool) []string {
	if startShortagePiles(piles, settings, seatedHumans) == "" {
		return nil
	}
	lines := []string{startRefuseMsg}
	if burnedWouldHelp {
		lines = append(lines, startUnburnMsg)
	}
	return lines
}
