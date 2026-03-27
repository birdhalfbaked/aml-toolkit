# GitHub Pages documentation

This directory is the static site for project documentation.

## Publishing

In the repository **Settings → Pages**:

- **Source**: Deploy from a branch
- **Branch**: `main` (or your default branch)
- **Folder**: `/docs`

The site will be available at `https://<user>.github.io/<repo>/`.

## Local preview

Open `index.html` in a browser, or run a static server from the repo root:

```bash
npx --yes serve docs -p 8080
```

The main narrative is **`index.html`**. Screenshots live under **`images/`** and are referenced from there.

The **Standalone builds** section in `index.html` notes that a no–dev-tools release may follow if the project proves worth shipping that way.
