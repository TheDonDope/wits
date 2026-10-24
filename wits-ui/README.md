# wits-ui

The browser interface to wits: Angular 22 and Angular Material, served by
`wits-server` (`cmd/wits-server`, `pkg/api`). See the main README's
"In a browser" for what it shows and how it guards the data.

```sh
npm ci
npx ng serve        # http://localhost:4200, proxying /api to wits-server on :7341
npx ng build        # dist/wits-ui/browser, what wits-server --ui serves
npx ng test         # Vitest
```

From the repository root, `make serve` builds both and serves the repository in
the working directory.
