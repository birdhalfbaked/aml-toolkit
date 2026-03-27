# Audio ML Toolkit

For now, this is a small UI aimed at creating datasets and speeding up labeling: projects and collections, waveform segments, a per-collection schema for fields, and exports (splits, augmentation, and related options) you can feed into other tools.

The direction is to make the project more useful for training and evaluation workflows, especially classification and automatic speech recognition (ASR), and over time to offer more than a browser UI (for example CLIs, integrations, or helpers that sit next to training pipelines). None of that is promised on a schedule; what exists today is the labeling and dataset-export stack below.

Full documentation (setup, UI tour, env vars): [https://birdhalfbaked.github.io/aml-toolkit/](https://birdhalfbaked.github.io/aml-toolkit/)

## Features

- Projects and collections - Top-level projects contain collections (folders of audio). Open a collection to label every file inside it.
- Uploads - Drag-and-drop MP3/WAV or ZIP. ZIP contents are flattened: nested paths are turned into filename prefixes (no directory tree in storage).
- Configurable schema - Per collection, define segment-scoped and file-scoped fields (taxonomy, text, etc.). Required fields drive a labeling queue so you can see what still needs work.
- Waveform labeling - Interactive waveform with regions. New regions stay draft in the browser until you save the segment; switching files or drawing again discards an unsaved draft.
- Label focus mode - Optional full-width layout that hides the file list; Next file and the N shortcut advance through the queue.
- Datasets - Save as dataset from labeling to build an export with options for splits, augmentation, and more. Browse builds under Datasets and open a detail view for status and paths.
- Persistence - SQLite database and on-disk audio/layout under configurable paths (see the docs for `AUDIO_TAGGER_DATA`, `AUDIO_TAGGER_DB`, and related env vars).

## Tech stack

| Layer    | Details |
| -------- | ------- |
| Frontend | Vue 3, Vuetify, Vite; Yarn for installs (`yarn` / `yarn dev`) |
| Backend  | Go, [julienschmidt/httprouter](https://github.com/julienschmidt/httprouter), REST API under `/api` |
| Data     | SQLite (modernc.org driver); migrations via `go run ./cmd/migrate` |

## Screenshots

<p align="center">
  <img src="docs/images/04-labeling-layout.png" alt="Labeling workspace" width="88%" />
  <br /><sub>Labeling: waveform, segments, and field editor in one layout.</sub>
</p>

<p align="center">
  <img src="docs/images/06-waveform-segments.png" alt="Waveform with regions" width="88%" />
  <br /><sub>Waveform: draw and resize regions; drafts stay local until you save.</sub>
</p>

<p align="center">
  <img src="docs/images/07-segment-fields.png" alt="Segment fields" width="88%" />
  <br /><sub>Segment fields: schema-driven labels and text, save, play, trim.</sub>
</p>

<p align="center">
  <img src="docs/images/09-dataset-creation.png" alt="Save as dataset" width="88%" />
  <br /><sub>Dataset export: splits, augmentation, and other generation options.</sub>
</p>

<p align="center">
  <img src="docs/images/10-dataset-details.png" alt="Dataset detail" width="88%" />
  <br /><sub>Dataset detail: status, paths, and metadata for a finished build.</sub>
</p>

## Quick start

Detailed steps (prerequisites, ports, troubleshooting) are on the doc site: [Get started](https://birdhalfbaked.github.io/aml-toolkit/#get-started).

```bash
cd backend && go run ./cmd/migrate   # once per clone / after schema changes
cd backend && go run .               # API on :8080 by default
cd frontend && yarn && yarn dev       # UI on :5173, proxies /api to the API
```

Then open [http://localhost:5173/](http://localhost:5173/), create a project, add a collection, upload audio, and open the collection for labeling.

## Database migrations

The API does not apply migrations on startup. After pulling changes that touch the schema, run:

```bash
cd backend && go run ./cmd/migrate
```

Uses the same `AUDIO_TAGGER_DATA` and `AUDIO_TAGGER_DB` environment variables as the server, or pass an explicit file:

```bash
go run ./cmd/migrate -db /path/to/app.db
```

## Looking ahead

- ML workflows - Stronger support for classification and ASR training/eval, and non-UI surfaces where they help, as the design becomes clear.
- Distribution - Today the app expects a dev-style workflow (clone, Go + Yarn, two processes). If the project is useful to others, I would like to offer simpler ways to run it (installers, prebuilt binaries, containers, or similar). No timeline; see the Standalone builds section on the [documentation site](https://birdhalfbaked.github.io/aml-toolkit/#distribution).
- Product polish - Ongoing improvements to labeling UX, exports, and schema tooling as feedback and time allow.
