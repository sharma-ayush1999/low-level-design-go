package movieticketbooking

import (
	"sync"
	"time"
)

type Show struct {
	ID string
	Movie *Movie
	Theatre *Theatre
	StartTime time.Time
	EndTime time.Time
	Seats map[string]*Seat
	mu sync.RWMutex
}

func NewShow(id string, movie *Movie, theatre *Theatre, startTime time.Time, endTime time.Time, seats map[string]*Seat) *Show {
	return &Show{ID: id, Movie: movie, Theatre: theatre, StartTime: startTime, EndTime: endTime, Seats: seats}
}