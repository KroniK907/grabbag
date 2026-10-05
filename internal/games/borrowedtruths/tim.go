package borrowedtruths

import (
	"sort"
	"time"
)

// pickNone is the This Is My vote for "none of the three took it".
const pickNone = "none"

// claimantCount is how many players claim each This Is My photo.
const claimantCount = 3

// photo is one uploaded This Is My photo. ID is the random file name; it
// reaches markup only once Shown is set, when the photo is on the board.
type photo struct {
	ID    string
	Owner string
	Used  bool
	Shown bool
}

// timRound is the This Is My round in play. Owner and NotTheirs stay out of
// the board view until the reveal.
type timRound struct {
	Photo     int
	Claimants []string // speaking order
	Speaker   int
	NotTheirs bool
}

// SetPhoto stores a seated player's photo during facts. It returns the ID of
// the photo it replaced, so the caller can delete that file.
func (e *engine) SetPhoto(id, photoID string) (old string, msg string) {
	if msg := e.photoRefusal(id); msg != "" {
		return "", msg
	}
	for i := range e.Photos {
		if e.Photos[i].Owner == id {
			old = e.Photos[i].ID
			e.Photos[i].ID = photoID
			return old, ""
		}
	}
	e.Photos = append(e.Photos, photo{ID: photoID, Owner: id})
	return "", ""
}

// photoRefusal is why id may not upload a photo now, or "" when they may.
func (e *engine) photoRefusal(id string) string {
	if e.player(id) == nil {
		return "You are not in this match."
	}
	if e.Phase != phaseFacts {
		return "Photos are closed."
	}
	return ""
}

// HasPhoto reports whether a player has uploaded a photo.
func (e *engine) HasPhoto(id string) bool {
	for _, p := range e.Photos {
		if p.Owner == id {
			return true
		}
	}
	return false
}

// photoShown reports whether photoID has been on the board, so it may be served.
func (e *engine) photoShown(photoID string) bool {
	for _, p := range e.Photos {
		if p.ID == photoID {
			return p.Shown
		}
	}
	return false
}

// timCount is the number of This Is My rounds: the setting, capped by the
// photos uploaded, and in Alternate placement by the number of tells minus one.
func (e *engine) timCount(tells int) int {
	n := min(e.Settings.timRounds(len(e.Players)), len(e.Photos))
	if len(e.Players) < claimantCount+1 {
		n = 0
	}
	if e.Settings.TIMPlacement == placeAlternate {
		n = min(n, tells-1)
	}
	return max(n, 0)
}

// matchOrder places k This Is My rounds among the tells. Middle puts them in
// a block after the first half of the tells. Alternate spreads them evenly
// between tells, never two in a row, never first or last.
func matchOrder(tellers []string, k int, placement string) []slot {
	t := len(tellers)
	after := map[int]int{} // tells before a This Is My round -> rounds there
	if placement == placeAlternate {
		k = min(k, t-1)
		for j := 0; j < k; j++ {
			after[(j+1)*t/(k+1)]++
		}
	} else if k > 0 {
		after[t/2] = k
	}
	var out []slot
	for i := 0; i <= t; i++ {
		for c := 0; c < after[i]; c++ {
			out = append(out, slot{TIM: true})
		}
		if i < t {
			out = append(out, slot{Teller: tellers[i]})
		}
	}
	return out
}

// dealTIM picks an unused photo and three claimants, and opens the look. It
// fails when fewer than three photos are left, which ends This Is My early.
func (e *engine) dealTIM(now time.Time) bool {
	e.resetCard()
	e.Card = nil
	e.Round = nil
	var open []int
	for i, p := range e.Photos {
		if !p.Used && e.player(p.Owner) != nil {
			open = append(open, i)
		}
	}
	if len(open) < claimantCount || len(e.Players) < claimantCount+1 {
		return false
	}
	// Prefer photos whose owner has claimed the least, so claims spread out.
	e.rng.Shuffle(len(open), func(i, j int) { open[i], open[j] = open[j], open[i] })
	sort.SliceStable(open, func(i, j int) bool {
		return e.Claims[e.Photos[open[i]].Owner] < e.Claims[e.Photos[open[j]].Owner]
	})
	pi := open[0]
	owner := e.Photos[pi].Owner

	// Not theirs needs a voter left over besides the hidden owner.
	notTheirs := len(e.Players) >= claimantCount+2 && e.rng.Intn(100) < e.Settings.NotTheirsPct
	var others []string
	for _, p := range e.Players {
		if p.ID != owner {
			others = append(others, p.ID)
		}
	}
	e.rng.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })
	sort.SliceStable(others, func(i, j int) bool { return e.Claims[others[i]] < e.Claims[others[j]] })
	claimants := append([]string(nil), others[:claimantCount-1]...)
	if notTheirs {
		claimants = append(claimants, others[claimantCount-1])
	} else {
		claimants = append(claimants, owner)
	}
	e.rng.Shuffle(len(claimants), func(i, j int) { claimants[i], claimants[j] = claimants[j], claimants[i] })
	for _, id := range claimants {
		e.Claims[id]++
	}
	e.Photos[pi].Used = true
	e.Photos[pi].Shown = true
	e.Round = &timRound{Photo: pi, Claimants: claimants, NotTheirs: notTheirs}
	e.enter(phaseLook, now)
	return true
}

func (e *engine) isClaimant(id string) bool {
	if e.Round == nil {
		return false
	}
	for _, c := range e.Round.Claimants {
		if c == id {
			return true
		}
	}
	return false
}

// timAnswer is the right This Is My pick: the owner, or none.
func (e *engine) timAnswer() string {
	if e.Round.NotTheirs {
		return pickNone
	}
	return e.Photos[e.Round.Photo].Owner
}

// knewTIM records an I knew it pick during the look: a claimant or none.
func (e *engine) knewTIM(id, pick string, now time.Time) string {
	if !e.Settings.KnewIt || e.Phase != phaseLook {
		return "I already know this is closed."
	}
	if !e.isVoter(id) {
		return "You cannot call this one."
	}
	if id == e.insider() {
		return "This one is yours."
	}
	if _, done := e.Knew[id]; done {
		return "You already called it."
	}
	if pick != pickNone && !e.isClaimant(pick) {
		return "Pick a claimant or None of them."
	}
	e.Knew[id] = knewPick{Pick: pick}
	e.checkKnewVoid(now)
	return ""
}

func (e *engine) scoreTIMKnew() {
	in, answer := e.insider(), e.timAnswer()
	for _, p := range e.voters() {
		if k, ok := e.Knew[p.ID]; ok && p.ID != in && k.Pick == answer {
			p.Calls++
			e.give(p.ID, 1, "Already knew")
		}
	}
}

// scoreTIM pays the This Is My vote: 2 for the right claimant, 3 for right on
// None of them, 1 to each claimant per vote, and 2 to a hidden owner when
// fewer than half of the voters picked None.
func (e *engine) scoreTIM() {
	e.Awards = nil
	e.scoreTIMKnew()
	answer := e.timAnswer()
	got := map[string]int{}
	voted, nones := 0, 0
	for _, p := range e.countedVoters() {
		pick, ok := e.Votes[p.ID]
		if !ok {
			continue
		}
		voted++
		if pick == pickNone {
			nones++
		} else {
			got[pick]++
		}
		if pick != answer {
			continue
		}
		p.Calls++
		if pick == pickNone {
			e.give(p.ID, 3, "None of them")
		} else {
			e.give(p.ID, 2, "Right claimant")
		}
	}
	for _, id := range e.Round.Claimants {
		e.give(id, got[id], "Convinced the room")
	}
	if in := e.insider(); in != "" && voted > 0 && nones*2 < voted {
		e.player(in).Straight++
		e.give(in, 2, "Hid in plain sight")
	}
}
