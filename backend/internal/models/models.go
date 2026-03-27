package models

import "time"

type Project struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type Collection struct {
	ID              int64     `json:"id"`
	ProjectID       int64     `json:"projectId"`
	Name            string    `json:"name"`
	CreatedAt       time.Time `json:"createdAt"`
	FieldSchemaJSON string    `json:"fieldSchemaJson"`
}

type Label struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"projectId"`
	Name      string `json:"name"`
}

type AudioFile struct {
	ID             int64             `json:"id"`
	CollectionID   int64             `json:"collectionId"`
	StoredFilename string            `json:"storedFilename"`
	OriginalName   string            `json:"originalName"`
	Format         string            `json:"format"`
	DurationMs     *int64            `json:"durationMs,omitempty"`
	UploadedAt     time.Time         `json:"uploadedAt"`
	FieldValues    map[string]string `json:"fieldValues,omitempty"`
}

type Segment struct {
	ID             int64             `json:"id"`
	AudioFileID    int64             `json:"audioFileId"`
	StartMs        int64             `json:"startMs"`
	EndMs          int64             `json:"endMs"`
	LabelID        *int64            `json:"labelId,omitempty"`
	Transcription  *string           `json:"transcription,omitempty"`
	LabelName      *string           `json:"labelName,omitempty"` // joined for display
	FieldValues    map[string]string `json:"fieldValues,omitempty"`
}

type LabelingQueueItem struct {
	AudioFileID int64  `json:"audioFileId"`
	Reason      string `json:"reason"` // no_segments | incomplete_fields
}

type Dataset struct {
	ID          int64     `json:"id"`
	ProjectID   int64     `json:"projectId"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"createdAt"`
	OptionsJSON string    `json:"optionsJson"`
	StorageRoot string    `json:"storageRoot"`
}

type DatasetSample struct {
	ID                int64   `json:"id"`
	DatasetID         int64   `json:"datasetId"`
	Split             string  `json:"split"`
	Filename          string  `json:"filename"`
	RelPath           string  `json:"relPath"`
	Label             string  `json:"label"`
	Transcription     *string `json:"transcription,omitempty"`
	SourceSegmentID   int64   `json:"sourceSegmentId"`
	AugmentationJSON  *string `json:"augmentationJson,omitempty"`
}

type CreateDatasetRequest struct {
	Name              string   `json:"name"`
	TrainRatio        float64  `json:"trainRatio"`
	ValidationRatio   float64  `json:"validationRatio"`
	EvaluationRatio   float64  `json:"evaluationRatio"`
	Seed              *int64   `json:"seed,omitempty"`
	CollectionIDs     []int64  `json:"collectionIds,omitempty"` // empty = all
	RequireTranscription bool  `json:"requireTranscription"`
	SilenceTrimRMS    *float64 `json:"silenceTrimRms,omitempty"`
	AugmentNoiseDB    *float64 `json:"augmentNoiseDb,omitempty"`
	AugmentMaxShiftMs *int64   `json:"augmentMaxShiftMs,omitempty"`
	AugmentVariants   int      `json:"augmentVariantsPerClip"`
}
