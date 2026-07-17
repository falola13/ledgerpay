package letters

import "time"

type DeadLetters struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	NextRetryAt *time.Time `json:"next_retry_at"`
	LastError   *string    `json:"last_error"`
	CreatedAt   time.Time  `json:"created_at"`
}
