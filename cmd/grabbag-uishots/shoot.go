package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	uaIOS     = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Mobile/15E148 Safari/604.1"
	uaIPad    = "Mozilla/5.0 (iPad; CPU OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Mobile/15E148 Safari/604.1"
	uaAndroid = "Mozilla/5.0 (Linux; Android 15; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Mobile Safari/537.36"
	uaTablet  = "Mozilla/5.0 (Linux; Android 15; SM-X710) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36"
)

// probe is what the page reports about its own layout.
type probe struct {
	ScrollWidth  int       `json:"scrollWidth"`
	ClientWidth  int       `json:"clientWidth"`
	HScroll      bool      `json:"hscroll"`
	Clipped      []string  `json:"clipped,omitempty"`
	Outside      []string  `json:"outside,omitempty"`
	SmallTargets []string  `json:"smallTargets,omitempty"`
	Fonts        []float64 `json:"-"`
}

// probeJS lists horizontal scroll, clipped or sideways-scrolling boxes, boxes past the right edge,
// tap targets under 44 CSS px, and the font size of every text-bearing node.
const probeJS = `(() => {
  const de = document.documentElement, vw = de.clientWidth;
  const out = {scrollWidth: de.scrollWidth, clientWidth: vw, hscroll: de.scrollWidth > vw + 1, clipped: [], outside: [], smallTargets: [], fonts: []};
  const label = el => {
    let s = el.tagName.toLowerCase();
    if (el.id) s += "#" + el.id;
    if (el.classList.length) s += "." + Array.from(el.classList).slice(0, 2).join(".");
    const t = (el.innerText || el.value || "").trim().replace(/\s+/g, " ").slice(0, 30);
    return t ? s + " \"" + t + "\"" : s;
  };
  const push = (list, v) => { if (list.length < 25 && list.indexOf(v) < 0) list.push(v); };
  for (const el of document.body.querySelectorAll("*")) {
    const cs = getComputedStyle(el);
    if (cs.display === "none" || cs.visibility === "hidden") continue;
    const r = el.getBoundingClientRect();
    if (r.width <= 2 || r.height <= 2) continue;
    if (el.closest("[hidden], [aria-hidden=true]:not(.is-open)")) continue;
    const clipX = cs.overflowX === "hidden" || cs.overflowX === "clip" || cs.textOverflow === "ellipsis";
    const clipY = cs.overflowY === "hidden" || cs.overflowY === "clip";
    const scrollX = cs.overflowX === "auto" || cs.overflowX === "scroll";
    if (clipX && el.scrollWidth > el.clientWidth + 1) push(out.clipped, label(el));
    else if (clipY && el.scrollHeight > el.clientHeight + 1) push(out.clipped, label(el));
    else if (scrollX && el !== document.body && el.scrollWidth > el.clientWidth + 1) push(out.clipped, label(el) + " (scrolls sideways)");
    if (cs.position !== "fixed" && r.left < vw && r.right > vw + 1 && r.width <= vw) push(out.outside, label(el));
    if (el.matches("a[href], button, select, textarea, input:not([type=hidden])") && (r.width < 44 || r.height < 44)) {
      push(out.smallTargets, label(el) + " " + Math.round(r.width) + "x" + Math.round(r.height));
    }
    for (const n of el.childNodes) {
      if (n.nodeType === 3 && n.textContent.trim()) { out.fonts.push(parseFloat(cs.fontSize)); break; }
    }
  }
  return out;
})()`

// scaleFontsJS multiplies every element's computed font size, the way an
// Android OS font scale enlarges px text in Chrome. Sizes are read first so
// nested em sizes are not scaled twice.
const scaleFontsJS = `(() => {
  const els = [document.documentElement, ...document.querySelectorAll("body, body *")];
  const sizes = els.map(el => parseFloat(getComputedStyle(el).fontSize));
  els.forEach((el, i) => { el.style.fontSize = (sizes[i] * %g) + "px"; });
  return true;
})()`

type probeRaw struct {
	probe
	RawFonts []float64 `json:"fonts"`
}

func chromeBinary(flag string) string {
	if flag != "" {
		return flag
	}
	for _, name := range []string{"chromium-browser", "chromium", "google-chrome", "google-chrome-stable", "headless_shell"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// shoot runs jobs across cfg.workers tabs of one headless browser.
func shoot(ctx context.Context, cfg config, base string, jobs []job) ([]result, error) {
	bin := chromeBinary(cfg.chrome)
	if bin == "" {
		return nil, fmt.Errorf("grabbag-uishots: no Chromium found; install chromium or pass -chrome")
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(bin),
		chromedp.Flag("hide-scrollbars", false),
		chromedp.Flag("font-render-hinting", "none"),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()
	browser, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	if err := chromedp.Run(browser); err != nil {
		return nil, fmt.Errorf("grabbag-uishots: start %s: %w", bin, err)
	}
	results := make([]result, len(jobs))
	next := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for w := 0; w < cfg.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				results[i] = shootOne(browser, cfg, base, jobs[i])
				if results[i].Error != "" {
					results[i] = shootOne(browser, cfg, base, jobs[i])
				}
				mu.Lock()
				done++
				if done%25 == 0 || done == len(jobs) {
					log.Printf("%d/%d", done, len(jobs))
				}
				mu.Unlock()
			}
		}()
	}
	for i := range jobs {
		next <- i
	}
	close(next)
	wg.Wait()
	return results, nil
}

func userAgent(j job) string {
	switch {
	case j.UA == "ios" && j.Tablet:
		return uaIPad
	case j.UA == "ios":
		return uaIOS
	case j.UA == "android" && j.Tablet:
		return uaTablet
	case j.UA == "android":
		return uaAndroid
	}
	return ""
}

func shootOne(browser context.Context, cfg config, base string, j job) result {
	res := result{job: j}
	tab, cancel := chromedp.NewContext(browser)
	defer cancel()
	tab, cancelTime := context.WithTimeout(tab, cfg.timeout)
	defer cancelTime()
	width, height, dpr := j.Width, j.Height, j.Scale
	pageZoom := j.TextMode == "device" && j.UA == "ios"
	fontScale := j.TextMode == "device" && j.UA != "ios"
	if pageZoom {
		width = int(float64(j.Width)/j.TextScale + 0.5)
		height = int(float64(j.Height)/j.TextScale + 0.5)
		dpr = j.Scale * j.TextScale
	}
	metrics := emulation.SetDeviceMetricsOverride(int64(width), int64(height), dpr, j.Mobile)
	if j.Mobile {
		o := &emulation.ScreenOrientation{Type: emulation.OrientationTypePortraitPrimary}
		if j.Landscape {
			o = &emulation.ScreenOrientation{Type: emulation.OrientationTypeLandscapePrimary, Angle: 90}
		}
		metrics = metrics.WithScreenOrientation(o)
	}
	var raw probeRaw
	var png []byte
	actions := chromedp.Tasks{
		metrics,
		emulation.SetTouchEmulationEnabled(j.Mobile),
	}
	if !pageZoom {
		actions = append(actions, page.SetFontSizes(&page.FontSizes{Standard: int64(16*j.TextScale + 0.5)}))
	}
	if ua := userAgent(j); ua != "" {
		actions = append(actions, emulation.SetUserAgentOverride(ua))
	}
	actions = append(actions,
		chromedp.Navigate(j.URL(base)),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(`document.fonts.ready.then(() => true)`, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !fontScale {
				return nil
			}
			return chromedp.Evaluate(fmt.Sprintf(scaleFontsJS, j.TextScale), nil).Do(ctx)
		}),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Evaluate(probeJS, &raw),
		chromedp.CaptureScreenshot(&png),
	)
	if err := chromedp.Run(tab, actions); err != nil {
		res.Error = err.Error()
		return res
	}
	res.probe = raw.probe
	res.Fonts = raw.RawFonts
	file := filepath.Join(cfg.out, filepath.FromSlash(j.File))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		res.Error = err.Error()
		return res
	}
	if err := os.WriteFile(file, png, 0o644); err != nil {
		res.Error = err.Error()
	}
	return res
}
