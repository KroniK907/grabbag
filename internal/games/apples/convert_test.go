package apples

import "testing"

func TestConvertOfficialSkipsDuplicateNormalizedText(t *testing.T) {
	t.Parallel()
	raw := []byte(`[
	  {
	    "name": "CAH Base Set",
	    "official": true,
	    "white": [
	      {"text": "White privilege."},
	      {"text": "White privilege."},
	      {"text": "The Three-Fifths compromise."},
	      {"text": "The Three-Fifths Compromise."},
	      {"text": "A rubber chicken"}
	    ],
	    "black": [
	      {"text": "A romantic candlelit dinner would be incomplete without _.", "pick": 1},
	      {"text": "A romantic candlelit dinner would be incomplete without _.", "pick": 1},
	      {"text": "Next from J.K. Rowling: Harry Potter and the Chamber of _.", "pick": 1},
	      {"text": "Next from J.K. Rowling: Harry Potter and the chamber of _.", "pick": 1}
	    ]
	  }
	]`)
	lib, err := convertOfficial(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Packs) != 1 {
		t.Fatalf("packs = %d", len(lib.Packs))
	}
	if got := len(lib.Packs[0].Prompts); got != 2 {
		t.Fatalf("prompts = %d", got)
	}
	if got := len(lib.Packs[0].Answers); got != 3 {
		t.Fatalf("answers = %d", got)
	}
}
