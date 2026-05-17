package library

import "time"

type Book struct {
	Path        string     `db:"path"`
	Category    string     `db:"category"`
	Title       string     `db:"title"`
	SizeBytes   int64      `db:"size_bytes"`
	Fingerprint string     `db:"fingerprint"`
	AddedAt     time.Time  `db:"added_at"`
	RemovedAt   *time.Time `db:"removed_at"`
	Hidden      bool       `db:"hidden"`
}

type Progress struct {
	BookPath         string     `db:"book_path"`
	CurrentPage      int        `db:"current_page"`
	TotalPages       int        `db:"total_pages"`
	LastReadAt       *time.Time `db:"last_read_at"`
	CurrentlyReading bool       `db:"currently_reading"`
}

func (p Progress) Percent() float64 {
	if p.TotalPages <= 0 {
		return 0
	}
	return float64(p.CurrentPage) / float64(p.TotalPages) * 100
}

type Bookmark struct {
	ID        int64     `db:"id"`
	BookPath  string    `db:"book_path"`
	Page      int       `db:"page"`
	Label     *string   `db:"label"`
	CreatedAt time.Time `db:"created_at"`
}

type Note struct {
	ID        int64     `db:"id"`
	BookPath  string    `db:"book_path"`
	Page      int       `db:"page"`
	Body      string    `db:"body"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	X         *float64  `db:"x"`
	Y         *float64  `db:"y"`
}

type Collection struct {
	ID         int64     `db:"id"`
	Name       string    `db:"name"`
	ParentID   *int64    `db:"parent_id"`
	Source     string    `db:"source"`
	FolderPath *string   `db:"folder_path"`
	CreatedAt  time.Time `db:"created_at"`
}

// Status is a unified reading status derived from progress fields.
type Status string

const (
	StatusNeverStarted     Status = "never_started"
	StatusCurrentlyReading Status = "currently_reading"
	StatusInProgress       Status = "in_progress"
	StatusCompleted        Status = "completed"
)

// ComputeStatus derives a Status from a Progress row. hasProgressRow indicates
// whether an actual row exists in the progress table (vs. a zero-value default).
func ComputeStatus(p Progress, hasProgressRow bool) Status {
	if p.TotalPages > 0 && p.CurrentPage >= p.TotalPages {
		return StatusCompleted
	}
	if p.CurrentlyReading {
		return StatusCurrentlyReading
	}
	if !hasProgressRow || p.CurrentPage <= 1 {
		return StatusNeverStarted
	}
	return StatusInProgress
}
