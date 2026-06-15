package movieticketbooking

import "sync"

type Seat struct {
	ID string
	Row int
	COl int
	Type SeatType
	Price float64
	Status SeatStatus
	mu sync.RWMutex
}

func NewSeat(id string, row, col int, seatType SeatType, price float64, status SeatStatus) *Seat {
	return &Seat{ID: id, Row: row, COl: col, Type: seatType, Price: price, Status: status}
}

func (s *Seat) GetStatus() SeatStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status
}

func (s *Seat) SetStatus(status SeatStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
}

func (s *Seat) GetSeatPrice() float64 {
	return s.Price
}