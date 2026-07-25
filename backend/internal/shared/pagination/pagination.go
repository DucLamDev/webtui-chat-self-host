package pagination

const (
	DefaultLimit = 50
	MaxLimit     = 100
)

type Cursor struct {
	Limit  int
	Before string
	After  string
}

type Meta struct {
	NextCursor string `json:"next_cursor,omitempty"`
	PrevCursor string `json:"prev_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

func NewCursor(limit int, before string, after string) Cursor {
	return Cursor{
		Limit:  NormalizeLimit(limit),
		Before: before,
		After:  after,
	}
}

func NormalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}
