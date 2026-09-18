package quips

const startRefuseMsg = "Not enough prompts in your library to start the game."

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

func startShortage(cat catalog, settings matchSettings, seatedHumans int) string {
	if seatedHumans < 1 {
		return ""
	}
	need := promptDemand(settings, seatedHumans)
	if enabledPromptCount(cat, settings) < need {
		return startRefuseMsg
	}
	return ""
}

func shortageLines(cat catalog, settings matchSettings, seatedHumans int) []string {
	if startShortage(cat, settings, seatedHumans) == "" {
		return nil
	}
	return []string{startRefuseMsg}
}
