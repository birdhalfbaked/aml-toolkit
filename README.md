This project is to help me tag audio.

it has several key features that I need on a project bases (you create projects):

1. Place to just dump audio files 
  - concept of a "collection" that we create which is just a folder
  - drag and drop of individual mp3/wav files or zip file which then decompresses and stores all found waves
    - if zip file we ignore folder heirarchy and prepend the folder path to the filename instead 
2. ability to see the wav form and create segments
3. on segments add labels (autocomplete for already seen labels in the project)


Tech is Vue with vuetify for the frontend in the frontend/ folder (use **Yarn** for installs: `yarn` / `yarn dev`, not npm)


Backend is golang with julienschmidt/httprouter

SQL backend is sqlite with regular sql wrapped in DTOs that expose the relevant models.

**Database migrations** are not run by the API server on startup. After pulling changes or deploying, apply migrations once:

```bash
cd backend && go run ./cmd/migrate
```

Uses the same `AUDIO_TAGGER_DATA` / `AUDIO_TAGGER_DB` env vars as the server. Override the file with `go run ./cmd/migrate -db /path/to/app.db`.