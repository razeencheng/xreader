# xReader Landing — Cloudflare Workers

Static landing page deployed via Cloudflare Workers Static Assets (no Worker
script — Cloudflare serves `public/` directly from the edge).

## Layout

```
landing/
  public/            # what gets deployed
    index.html       # the landing page (edit this directly)
    assets/          # images, favicons, og-image
  wrangler.jsonc     # deploy config
  package.json
```

## Deploy

```bash
cd landing
npm install              # installs wrangler
npx wrangler login       # one-time, opens browser to authorize
npm run deploy           # publishes to xreader-landing.<account>.workers.dev
```

Local preview:

```bash
npm run dev              # serves on http://localhost:8787
```

## Custom domain (e.g. xreader.cc)

After the first deploy, attach a route in the Cloudflare dashboard
(Workers & Pages → xreader-landing → Settings → Domains & Routes), or add to
`wrangler.jsonc`:

```jsonc
"routes": [{ "pattern": "xreader.cc", "custom_domain": true }]
```

## Updating content

`public/index.html` is the source of truth — edit it directly, then redeploy
with `npm run deploy`.
