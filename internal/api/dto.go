package api

import (
	"time"

	"bookshelf/internal/library"
	"bookshelf/internal/scanner"
)

type ProgressDTO struct {
	CurrentPage      int        `json:"currentPage"`
	TotalPages       int        `json:"totalPages"`
	Percent          float64    `json:"percent"`
	LastReadAt       *time.Time `json:"lastReadAt"`
	CurrentlyReading bool       `json:"currentlyReading"`
	Status           string     `json:"status"`
}

type BookmarkDTO struct {
	ID        int64     `json:"id"`
	Page      int       `json:"page"`
	Label     *string   `json:"label"`
	CreatedAt time.Time `json:"createdAt"`
}

type BookSummaryDTO struct {
	Path          string       `json:"path"`
	Title         string       `json:"title"`
	Category      string       `json:"category"`
	SizeBytes     int64        `json:"sizeBytes"`
	Fingerprint   string       `json:"fingerprint"`
	Progress      *ProgressDTO `json:"progress"`
	BookmarkCount int          `json:"bookmarkCount"`
	CollectionIDs []int64      `json:"collectionIds"`
	Status        string       `json:"status"`
	Hidden        bool         `json:"hidden"`
}

type BookDTO struct {
	Path          string        `json:"path"`
	Title         string        `json:"title"`
	Category      string        `json:"category"`
	SizeBytes     int64         `json:"sizeBytes"`
	Fingerprint   string        `json:"fingerprint"`
	AddedAt       time.Time     `json:"addedAt"`
	Progress      *ProgressDTO  `json:"progress"`
	Bookmarks     []BookmarkDTO `json:"bookmarks"`
	Notes         []NoteDTO     `json:"notes"`
	CollectionIDs []int64       `json:"collectionIds"`
	Hidden        bool          `json:"hidden"`
}

type NoteDTO struct {
	ID        int64     `json:"id"`
	Page      int       `json:"page"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	X         *float64  `json:"x,omitempty"`
	Y         *float64  `json:"y,omitempty"`
}

type PostNoteReq struct {
	Page int      `json:"page"`
	Body string   `json:"body"`
	X    *float64 `json:"x"`
	Y    *float64 `json:"y"`
}

type PatchNoteReq struct {
	Page          *int     `json:"page"`
	Body          *string  `json:"body"`
	X             *float64 `json:"x"`
	Y             *float64 `json:"y"`
	ClearPosition bool     `json:"clearPosition"`
}

type CategoryDTO struct {
	Name  string           `json:"name"`
	Books []BookSummaryDTO `json:"books"`
}

type CollectionDTO struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	ParentID   *int64  `json:"parentId"`
	Source     string  `json:"source"`
	FolderPath *string `json:"folderPath"`
}

type LibraryDTO struct {
	Categories        []CategoryDTO   `json:"categories"`
	Collections       []CollectionDTO `json:"collections"`
	ScannedAt         time.Time       `json:"scannedAt"`
	LibraryConfigured bool            `json:"libraryConfigured"`
}

type SettingsDTO struct {
	LibraryDir string `json:"libraryDir"`
}

type PutSettingsReq struct {
	LibraryDir string `json:"libraryDir"`
}

type ScanStatusDTO struct {
	Running    bool       `json:"running"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	Added      int        `json:"added"`
	Updated    int        `json:"updated"`
	Removed    int        `json:"removed"`
	Error      *string    `json:"error"`
}

type PutProgressReq struct {
	CurrentPage int `json:"currentPage"`
	TotalPages  int `json:"totalPages"`
}

type PostBookmarkReq struct {
	Page  int     `json:"page"`
	Label *string `json:"label"`
}

type PutStatusReq struct {
	CurrentlyReading bool `json:"currentlyReading"`
}

type PutHiddenReq struct {
	Hidden bool `json:"hidden"`
}

type CreateCollectionReq struct {
	Name     string `json:"name"`
	ParentID *int64 `json:"parentId"`
}

// UpdateCollectionReq uses a `changeParent` flag plus `parentId` rather than
// `**int64` so the wire format stays plain JSON. Set `changeParent: true` and
// either set `parentId` to a number (reparent) or `null` (clear parent).
type UpdateCollectionReq struct {
	Name         *string `json:"name"`
	ChangeParent bool    `json:"changeParent"`
	ParentID     *int64  `json:"parentId"`
}

type AddBookToCollectionReq struct {
	Path string `json:"path"`
}

func progressToDTO(p library.Progress, hasRow bool) ProgressDTO {
	return ProgressDTO{
		CurrentPage:      p.CurrentPage,
		TotalPages:       p.TotalPages,
		Percent:          p.Percent(),
		LastReadAt:       p.LastReadAt,
		CurrentlyReading: p.CurrentlyReading,
		Status:           string(library.ComputeStatus(p, hasRow)),
	}
}

func bookmarkToDTO(b library.Bookmark) BookmarkDTO {
	return BookmarkDTO{
		ID:        b.ID,
		Page:      b.Page,
		Label:     b.Label,
		CreatedAt: b.CreatedAt,
	}
}

func noteToDTO(n library.Note) NoteDTO {
	return NoteDTO{
		ID:        n.ID,
		Page:      n.Page,
		Body:      n.Body,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
		X:         n.X,
		Y:         n.Y,
	}
}

func collectionToDTO(c library.Collection) CollectionDTO {
	return CollectionDTO{
		ID:         c.ID,
		Name:       c.Name,
		ParentID:   c.ParentID,
		Source:     c.Source,
		FolderPath: c.FolderPath,
	}
}

func scanResultToDTO(r scanner.Result, running bool) ScanStatusDTO {
	var fin *time.Time
	if !r.FinishedAt.IsZero() {
		f := r.FinishedAt
		fin = &f
	}
	var errStr *string
	if len(r.Errors) > 0 {
		s := r.Errors[0]
		errStr = &s
	}
	return ScanStatusDTO{
		Running:    running,
		StartedAt:  r.StartedAt,
		FinishedAt: fin,
		Added:      r.Added,
		Updated:    r.Updated,
		Removed:    r.Removed,
		Error:      errStr,
	}
}
