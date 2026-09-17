package artist

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNameRequired = errors.New("artist name cannot be empty")
)

type Artist struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Country     string    `json:"country,omitempty"`
	LabelID     uuid.UUID `json:"label_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (a *Artist) Validate() error {
	if len(a.Name) == 0 {
		return ErrNameRequired
	}
	return nil
}

type Respository interface {
	Create(ctx context.Context, artist *Artist) error
	GetByID(ctx context.Context, id uuid.UUID) (*Artist, error)
}

type Service interface {
	RegisterArtist(ctx context.Context, name, description, country string, labelID uuid.UUID) (*Artist, error)
	GetArtist(ctx context.Context, id uuid.UUID) (*Artist, error)
}
