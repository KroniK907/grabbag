# grabbag.gg marketing site

Static landing page for GitHub Pages. Source lives in this folder; the workflow at `.github/workflows/pages.yml` publishes `site/` plus `docs/index.html` and `docs/game-contract.html` under `/docs/` on pushes to `main`.

## Custom domain

`CNAME` is set to `grabbag.gg`. In the repo **Settings → Pages**, choose **GitHub Actions** as the source (the workflow deploys the artifact). Under **Custom domain**, enter `grabbag.gg` and enable HTTPS once DNS validates.

Typical DNS at your registrar:

- Apex `grabbag.gg`: `A` records to GitHub Pages IPs (`185.199.108.153`, `185.199.109.153`, `185.199.110.153`, `185.199.111.153`), or an `ALIAS`/`ANAME` if your provider supports it.
- `www`: `CNAME` to `<user>.github.io` if you use a `www` host as well.

## Local preview

From the repo root, mirror the Pages layout:

```text
mkdir -p _site/docs && cp -r site/. _site/ && cp docs/index.html docs/game-contract.html _site/docs/
cd _site && python3 -m http.server 8080
```

Then open `http://127.0.0.1:8080`. Doc pages are at `/docs/index.html` and `/docs/game-contract.html`.
