package movieticketbooking

import (
	"fmt"
	"time"
)

func Run() {
	bookingSystem := GetBookingSystem()

	//Add movies
	movie1 := NewMovie("M1", "Movie 1", "description1 ", 120)
	movie2 := NewMovie("M2", "Movie 2", "description 2", 180)
	bookingSystem.AddMovie(movie1)
	bookingSystem.AddMovie(movie2)

	//Add theatres
	theatre1 := NewTheatre("T1", "Theatre 1", "Location 1")
	theatre2 := NewTheatre("T2", "Theatre 2", "Location 2")
	bookingSystem.AddTheatre(theatre1)
	bookingSystem.AddTheatre(theatre2)

	//Add shows
	show1 := NewShow(
		"s1",
		movie1, 
		theatre1, 
		time.Now(), time.Now().Add(time.Duration(movie1.DurationMinutes)*time.Minute), 
		CreateSeats(8, 8),
	)

	show2 := NewShow(
		"s2",
		movie2, 
		theatre2, 
		time.Now(), time.Now().Add(time.Duration(movie2.DurationMinutes)*time.Minute), 
		CreateSeats(15, 10),
	)

	bookingSystem.AddShow(show1)
	bookingSystem.AddShow(show2)

	//Create user
	user := NewUser("U1", "Tom Ford", "tom@gmail.com")

	//Select seat
	selectedSeats := []*Seat{
		show1.Seats["1-5"],
		show2.Seats["1-2"],
	}

	//Book tickets
	booking, err := bookingSystem.BookTickets(user, show1, selectedSeats)
	if err != nil {
		fmt.Printf("Booking failed: %v\n", err)
		return
	}
	fmt.Printf("Booking successful. Booking ID: %s\n", booking.ID)

	//Confirm booking
	if err := bookingSystem.ConfirmBooking(booking.ID); err != nil {
		fmt.Printf("Failed to confirm booking: %v\n", err)
		return
	}
	fmt.Println("booking Confirmed")

	//Cancel booking
	if err := bookingSystem.CancelBooking(booking.ID); err != nil {
		fmt.Printf("Failed to cancel booking: %v\n", err)
		return
	}

	fmt.Printf("Booking cancelled. Booking ID: %s\n", booking.ID)

}