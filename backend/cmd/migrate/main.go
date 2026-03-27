package main

import (
	"flag"
	"log"
	"os"

	"com.birdhalfbaked.aml-toolkit/internal/db"
)

func main() {
	log.SetFlags(0)
	dbPath := flag.String("db", "", "SQLite database file (default: same as server, from AUDIO_TAGGER_DB or <AUDIO_TAGGER_DATA>/app.db)")
	flag.Parse()

	path := *dbPath
	if path == "" {
		path = db.DefaultDBPath()
	}

	dataDir := db.DefaultDataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatal(err)
	}

	sqldb, err := db.Open(path)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer sqldb.Close()

	if err := db.RunMigrations(sqldb); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Printf("migrations applied: %s", path)
}
