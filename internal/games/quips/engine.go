package quips

// matchEngine runs an in-progress Quick Quips match. A later task wires this
// from Start; until then Game.engine stays nil.
type matchEngine struct{}
