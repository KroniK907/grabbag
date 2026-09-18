package quips

import (
	"net/http"
)

type pickerErr struct {
	Msg       string
	LibraryID string
	PackID    string
}

func hxRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func (g *Game) writePicker(w http.ResponseWriter, r *http.Request, rowErr pickerErr) {
	name := "picker.html"
	if hxRequest(r) {
		name = "picker-body"
	}
	g.render(w, name, g.pickerView(rowErr), http.StatusOK)
}

func (g *Game) writePickerOK(w http.ResponseWriter, r *http.Request) {
	if hxRequest(r) {
		g.writePicker(w, r, pickerErr{})
		return
	}
	http.Redirect(w, r, "/play/picker", http.StatusSeeOther)
}

func (g *Game) getPicker(w http.ResponseWriter, r *http.Request) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if !h.HasAdmin(r) {
		http.Error(w, "Admin session required.", http.StatusUnauthorized)
		return
	}
	g.render(w, "picker.html", g.pickerView(pickerErr{}), http.StatusOK)
}

func (g *Game) postPack(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	g.togglePack(w, r, r.FormValue("library"), r.FormValue("pack"), formEnabled(r))
}

func (g *Game) postSelectAll(w http.ResponseWriter, r *http.Request) {
	g.selectAllPacks(w, r, true)
}

func (g *Game) postSelectNone(w http.ResponseWriter, r *http.Request) {
	g.selectAllPacks(w, r, false)
}

func (g *Game) togglePack(w http.ResponseWriter, r *http.Request, libraryID, packID string, on bool) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if g.matchFrozen() {
		g.writePicker(w, r, pickerErr{
			Msg: "Pack changes wait until the game ends.", LibraryID: libraryID, PackID: packID,
		})
		return
	}
	cat := scanDataDir(h.DataDir())
	if !packExists(cat, libraryID, packID) {
		g.writePicker(w, r, pickerErr{
			Msg: "That pack is not in the library.", LibraryID: libraryID, PackID: packID,
		})
		return
	}
	settings, ok := g.loadSettings(h)
	settings = reconcileSettings(settings, ok, cat)
	settings.setPack(libraryID, packID, on)
	if err := g.saveSettings(h, settings); err != nil {
		g.writePicker(w, r, pickerErr{
			Msg: "Could not save pack enablement.", LibraryID: libraryID, PackID: packID,
		})
		return
	}
	g.writePickerOK(w, r)
}

func (g *Game) selectAllPacks(w http.ResponseWriter, r *http.Request, on bool) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if g.matchFrozen() {
		g.writePicker(w, r, pickerErr{Msg: "Pack changes wait until the game ends."})
		return
	}
	cat := scanDataDir(h.DataDir())
	settings, ok := g.loadSettings(h)
	settings = reconcileSettings(settings, ok, cat)
	settings.setAllEnabled(cat, on)
	if err := g.saveSettings(h, settings); err != nil {
		g.writePicker(w, r, pickerErr{Msg: "Could not save pack enablement."})
		return
	}
	g.writePickerOK(w, r)
}
