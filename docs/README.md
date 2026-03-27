# GitHub Pages documentation

This directory is the static site for project documentation.

## Publishing

In the repository **Settings → Pages**:

- **Source**: Deploy from a branch
- **Branch**: `main` (or your default branch)
- **Folder**: `/docs`

When published from this repo, the site is **[https://birdhalfbaked.github.io/aml-toolkit/](https://birdhalfbaked.github.io/aml-toolkit/)** (path matches the repository name).

## Local preview

Open `index.html` in a browser, or run a static server from the repo root:

```bash
npx --yes serve docs -p 8080
```

The main narrative is **`index.html`**, starting with **Get started** (prerequisites, migrate, run backend + Vite, open the app). Screenshots live under **`images/`**.

Right after the **table of contents**, **Standalone builds** sets expectations: the current dev-style setup is rough; easier packaging may follow if there is interest (no timeline promised).
