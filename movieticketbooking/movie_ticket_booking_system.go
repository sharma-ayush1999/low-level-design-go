package movieticketbooking

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)



type MovieTicketBookingSystem struct {
	movies []*Movie
	theatres []*Theatre
	shows map[string]*Show
	bookings map[string]*Booking
	bookingCount int64
	mu sync.RWMutex
}

var (
	instance *MovieTicketBookingSystem
	once sync.Once
)

func GetBookingSystem() *MovieTicketBookingSystem {
	once.Do(func () {
		instance = &MovieTicketBookingSystem{
			movies: make([]*Movie, 0),
			theatres: make([]*Theatre, 0),
			shows: make(map[string]*Show),
			bookings: make(map[string]*Booking),
		}
	})
	return instance
}

func (bs *MovieTicketBookingSystem) AddMovie(movie *Movie){
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.movies = append(bs.movies, movie)
}

func (bs *MovieTicketBookingSystem) AddTheatre(theatre *Theatre) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.theatres = append(bs.theatres, theatre)
}

func (bs *MovieTicketBookingSystem) AddShow(show *Show) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.shows[show.ID] = show
}

func (bs *MovieTicketBookingSystem) GetShow(showId string) *Show {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.shows[showId]
}

func (bs *MovieTicketBookingSystem) BookTickets(user *User, show *Show, selectedSeats []*Seat) (*Booking, error) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	//check seat availability
	for _, seat := range selectedSeats {
		showSeat, exists := show.Seats[seat.ID]
		if !exists || showSeat.GetStatus() != SeatStatusAvailable {
			return nil, fmt.Errorf("Seat %s is not available", seat.ID)
		}
	}

	//mark seats as booked
	for _, seat := range selectedSeats {
		show.Seats[seat.ID].SetStatus(SeatStatusBooked)
	}

	//calculate total price
	var totalPrice float64
	for _, seat := range selectedSeats {
		totalPrice += seat.GetSeatPrice()
	}

	//Generate booking Id
	bookingID := bs.GenerateBookingId()

	//create booking
	booking := NewBooking(bookingID, user, show, selectedSeats, totalPrice, BookingStatusPending)
	bs.bookings[bookingID] = booking
	return booking, nil
}

func (bs *MovieTicketBookingSystem) ConfirmBooking(bookingID string) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	
	booking, exists := bs.bookings[bookingID]
	if !exists {
		return fmt.Errorf("booking not found")
	}

	if booking.GetStatus() != BookingStatusPending{
		return fmt.Errorf("booking is not in pending state")
	}

	booking.SetStatus(BookingStatusConfirmed)
	return nil
}

func (bs *MovieTicketBookingSystem) CancelBooking (bookingId string) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	booking, exists := bs.bookings[bookingId]
	if !exists {
		return fmt.Errorf("booking not found")
	}

	if booking.GetStatus() == BookingStatusCancelled {
		return fmt.Errorf("booking is already cancelled")
	}

	booking.SetStatus(BookingStatusCancelled)

	//Release seats
	for _, seat := range booking.Seats {
		booking.Show.Seats[seat.ID].SetStatus(SeatStatusAvailable)
	}

	return nil
}

func (bs *MovieTicketBookingSystem) GenerateBookingId() string {
	atomic.AddInt64(&bs.bookingCount, 1)
	return uuid.New().String()
}

// create utility function for demo
func CreateSeats(rows, cols int) map[string] *Seat {
	seats := make(map[string]*Seat)
	for row := range rows {
		for col := range cols {
			seatID := fmt.Sprintf("%d-%d", row, col)
			seatType := SeatTypeNormal
			price := 100.0

			if row <= 2 {
				seatType = SeatTypePremium
				price = 150.0
			}

			seats[seatID] = NewSeat(seatID, row, col, seatType, price, SeatStatusAvailable)
		}
	}
	return seats
}