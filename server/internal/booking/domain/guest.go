package domain

import "github.com/google/uuid"

type Guest struct {
	id   uuid.UUID
	name Name
}

func NewGuest(id uuid.UUID, name Name) Guest {
	return Guest{id: id, name: name}
}

func (g Guest) Id() uuid.UUID {
	return g.id
}

func (g Guest) Name() Name {
	return g.name
}
