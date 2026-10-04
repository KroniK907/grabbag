// Bench-only autofill. Fills empty Quips answers and Borrowed Truths facts
// with random test lines so one person can drive a full room.
(() => {
  const pools = {
    text: [
      "A suspiciously damp sock",
      "Three raccoons in a trench coat",
      "Grandma's secret hot sauce",
      "The world's slowest parade",
      "Free samples of regret",
      "A haunted spreadsheet",
      "Interpretive dance, but angry",
      "One extremely confident goose",
      "Soup that fights back",
      "A tiny hat for a large dog",
    ],
    truth: [
      "I once got locked inside a library overnight.",
      "I can't whistle at all.",
      "I cried at a dog food commercial last week.",
      "I have eaten cereal with orange juice on purpose.",
      "I've never seen a single Star Wars movie.",
      "I was the mascot for my school for one game.",
      "I talk to my houseplants by name.",
      "I fell asleep on a roller coaster.",
    ],
    lie: [
      "I once won a pie-eating contest against a firefighter.",
      "I have a fear of escalators.",
      "I was an extra in a shampoo commercial.",
      "I can juggle four eggs without breaking any.",
      "I named my car after a medieval king.",
      "I accidentally joined a marathon in college.",
      "I own forty identical gray hoodies.",
      "I got stuck in a revolving door for an hour.",
    ],
  };
  const used = new Set();
  const pick = (list) => {
    const fresh = list.filter((s) => !used.has(s));
    const from = fresh.length ? fresh : list;
    const s = from[Math.floor(Math.random() * from.length)];
    used.add(s);
    return s;
  };
  const fill = () => {
    for (const el of document.querySelectorAll("textarea[name=text], textarea[name=truth], textarea[name=lie]")) {
      if (el.disabled || el.readOnly || el.value.trim() !== "" || el.dataset.benchFilled) continue;
      if (el.name === "text" && !el.closest(".quips-compose-card")) continue;
      el.dataset.benchFilled = "1";
      let s = pick(pools[el.name]);
      const cap = el.maxLength;
      if (cap > 0 && s.length > cap) s = s.slice(0, cap);
      el.value = s;
      el.dispatchEvent(new Event("input", { bubbles: true }));
      el.dispatchEvent(new Event("change", { bubbles: true }));
    }
  };
  fill();
  new MutationObserver(fill).observe(document.documentElement, { childList: true, subtree: true });
})();
