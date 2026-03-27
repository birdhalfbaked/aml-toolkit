package store

import (
	"fmt"
	"os"
	"path/filepath"
)

type Layout struct {
	Root string
}

func NewLayout(root string) (*Layout, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &Layout{Root: abs}, nil
}

func (l *Layout) ProjectDir(projectID int64) string {
	return filepath.Join(l.Root, "projects", fmt.Sprintf("%d", projectID))
}

func (l *Layout) CollectionDir(projectID, collectionID int64) string {
	return filepath.Join(l.ProjectDir(projectID), "collections", fmt.Sprintf("%d", collectionID))
}

func (l *Layout) DatasetDir(projectID, datasetID int64) string {
	return filepath.Join(l.ProjectDir(projectID), "datasets", fmt.Sprintf("%d", datasetID))
}

func (l *Layout) EnsureCollectionDir(projectID, collectionID int64) error {
	p := l.CollectionDir(projectID, collectionID)
	return os.MkdirAll(p, 0o755)
}

func (l *Layout) EnsureDatasetDir(projectID, datasetID int64) error {
	p := l.DatasetDir(projectID, datasetID)
	return os.MkdirAll(p, 0o755)
}

func (l *Layout) EnsureProjectDir(projectID int64) error {
	p := l.ProjectDir(projectID)
	return os.MkdirAll(p, 0o755)
}
