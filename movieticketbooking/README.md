# Movie Ticket Booking System — Low Level Design

## Problem Statement

Design a backend system (similar to BookMyShow) where users can browse movies and shows, select seats, book tickets, confirm payments, and cancel bookings — while ensuring that two users can never book the same seat for the same show.

---

## Requirements

1. The system manages movies, theatres, and shows.
2. Users can select a show and choose specific seats to book.
3. Seats have types (Normal / Premium) with different prices.
4. A booking goes through three states: Pending → Confirmed → Cancelled.
5. The system prevents double-booking: a seat that is booked cannot be booked again.
6. Concurrent access is handled safely — multiple users booking simultaneously must not corrupt seat state.
7. Theatre administrators can add movies, theatres, and shows.

---

## Project Structure

```
movieticketbooking/
├── types.go                        # Enums: SeatType, SeatStatus, BookingStatus
├── movie.go                        # Movie entity
├── theatre.go                      # Theatre entity
├── seat.go                         # Seat entity with mutex for thread safety
├── show.go                         # Show entity (movie + theatre + seats + time)
├── user.go                         # User entity
├── booking.go                      # Booking entity with mutex
├── movie_ticket_booking_system.go  # Core system logic + Singleton + utilities
└── demo.go                         # Entry point / demo
```

---

## Design Patterns Used

| Pattern | Where | Why |
|---|---|---|
| **Singleton** | `MovieTicketBookingSystem` | The booking system is one central service. `sync.Once` guarantees exactly one instance is created even under concurrent initialization. |
| **Factory** | `NewMovie`, `NewTheatre`, `NewShow`, `NewSeat`, `NewUser`, `NewBooking` | Each entity has a constructor that handles initialization. Callers never build structs directly. |
| **Read-Write Mutex (RWMutex)** | `Seat`, `Booking`, `MovieTicketBookingSystem` | Read-heavy operations (checking status) use `RLock` (multiple readers allowed). Write operations (booking, cancelling) use `Lock` (exclusive). This maximizes concurrency for reads without sacrificing safety for writes. |

---

## File-by-File Breakdown

### `types.go` — Enumerations

All domain enums live here, defined as typed `int` constants using `iota`.

```
SeatType:      SeatTypeNormal (0)   | SeatTypePremium (1)
SeatStatus:    SeatStatusAvailable  | SeatStatusBooked
BookingStatus: BookingStatusPending | BookingStatusConfirmed | BookingStatusCancelled
```

**Why typed enums over plain strings?**
- Type safety: you can't accidentally pass a `SeatType` where a `BookingStatus` is expected.
- Comparisons are integer comparisons — fast and unambiguous.
- `iota` auto-increments, so adding a new status is one line with no manual numbering.

**Why separate `SeatStatus` from `BookingStatus`?**
These model different things. `SeatStatus` belongs to the physical seat (is it occupied?). `BookingStatus` belongs to the transaction (is the payment confirmed?). Mixing them would blur the domain model — e.g., a seat can be marked Booked while the Booking is still Pending payment.

---

### `movie.go` — The Movie Entity

```
Movie {
    ID              string
    Title           string
    Description     string
    DurationMinutes int
}
```

`Movie` is a pure data entity — it has no behavior. It represents a film in the catalogue.

**Why store `DurationMinutes` as an int?**
In `demo.go`, the show's `EndTime` is computed as `StartTime + time.Duration(movie.DurationMinutes) * time.Minute`. Storing it as an int makes this arithmetic direct.

#### Functions

| Function | What it does |
|---|---|
| `NewMovie(id, title, description string, durationMinutes int) *Movie` | Constructs a Movie. The `ID` (e.g., `"M1"`) is the key used to reference this movie in other parts of the system. |

---

### `theatre.go` — The Theatre Entity

```
Theatre {
    ID       string
    Name     string
    Location string
    Shows    []*Show   ← list of shows running in this theatre
}
```

`Theatre` is a physical venue. It holds references to the shows playing there.

#### Functions

| Function | What it does |
|---|---|
| `NewTheatre(id, name, location string) *Theatre` | Constructs a Theatre with an empty Shows list. Shows are added later via `AddShow` on the booking system. |

---

### `seat.go` — The Seat Entity (Thread-Safe)

```
Seat {
    ID     string
    Row    int
    Col    int         ← note: field name is "COl" in source (typo, works fine)
    Type   SeatType    ← Normal or Premium
    Price  float64
    Status SeatStatus  ← Available or Booked
    mu     sync.RWMutex
}
```

**Why does Seat have its own mutex?**
Seats are the most contended resource. Multiple users may try to book the same seat simultaneously. The per-seat mutex ensures that checking and updating a seat's status is atomic. A RWMutex is used because:
- `GetStatus` (read) can run concurrently for multiple readers.
- `SetStatus` (write) needs exclusive access.

#### Functions

| Function | What it does | Why |
|---|---|---|
| `NewSeat(id, row, col, seatType, price, status)` | Constructs a Seat with all fields set. Status is passed in (typically `SeatStatusAvailable`). | Factory ensures all fields are set. The mutex is zero-valued by Go, so it's automatically initialized to unlocked. |
| `GetStatus() SeatStatus` | Acquires a read lock, reads `Status`, releases the lock. | Thread-safe read. Multiple goroutines can call this simultaneously without blocking each other. |
| `SetStatus(status SeatStatus)` | Acquires a write lock, sets `Status`, releases the lock. | Thread-safe write. Exclusive access ensures no two goroutines write at the same time. |
| `GetSeatPrice() float64` | Returns `Price` directly (no lock). | Price never changes after construction — it's immutable — so no locking needed. |

---

### `show.go` — The Show Entity

```
Show {
    ID        string
    Movie     *Movie
    Theatre   *Theatre
    StartTime time.Time
    EndTime   time.Time
    Seats     map[string]*Seat   ← key: seat ID, value: Seat pointer
    mu        sync.RWMutex
}
```

A `Show` is a specific screening of a Movie at a Theatre at a specific time. It owns the seat map for that screening — the same physical seat (row 1, col 5) exists as a separate `Seat` object per show, because availability resets for every show.

**Why `map[string]*Seat` instead of a slice?**
Booking requires looking up a specific seat by ID (e.g., `"1-5"`). A map makes this O(1) instead of O(n). The key is the seat ID string.

#### Functions

| Function | What it does |
|---|---|
| `NewShow(id, movie, theatre, startTime, endTime, seats)` | Constructs a Show with a pre-built seat map. The seat map is created by `CreateSeats` in the booking system utility. |

---

### `user.go` — The User Entity

```
User {
    ID    string
    Name  string
    Email string
}
```

A user of the booking system. Pure data entity.

#### Functions

| Function | What it does |
|---|---|
| `NewUser(id, name, email string) *User` | Constructs a User. ID identifies the user across bookings. |

---

### `booking.go` — The Booking Entity (Thread-Safe)

```
Booking {
    ID         string
    User       *User
    Show       *Show
    Seats      []*Seat
    TotalPrice float64
    Status     BookingStatus
    mu         sync.RWMutex
}
```

A `Booking` represents a transaction: a specific User reserving specific Seats in a specific Show.

**Why does Booking have its own mutex?**
Booking status can be read by display/reporting and written by confirm/cancel operations concurrently. The RWMutex protects against a race between a goroutine reading `Status` and another cancelling the booking.

#### Functions

| Function | What it does | Why |
|---|---|---|
| `NewBooking(id, user, show, seats, totalPrice, status)` | Constructs a Booking. Status starts as `BookingStatusPending`. | Pending means "seats are reserved but payment not confirmed yet". |
| `GetStatus() BookingStatus` | Thread-safe read of `Status` with RLock. | Multiple parts of the system may check booking status simultaneously. |
| `SetStatus(status BookingStatus)` | Thread-safe write of `Status` with Lock. | Called by `ConfirmBooking` and `CancelBooking`. Exclusive lock prevents partial writes. |

---

### `movie_ticket_booking_system.go` — The Core System

This is the heart of the design. `MovieTicketBookingSystem` is the Singleton service that manages all data and enforces booking rules.

```
MovieTicketBookingSystem {
    movies        []*Movie
    theatres      []*Theatre
    shows         map[string]*Show     ← key: show ID
    bookings      map[string]*Booking  ← key: booking ID
    bookingCount  int64                ← atomic counter (for future sequential IDs)
    mu            sync.RWMutex         ← system-wide mutex
}
```

**Why `sync.Once` for Singleton initialization?**
Unlike the Parking Lot's nil-check (which is unsafe under concurrency), `sync.Once` guarantees the initialization function runs exactly once even if multiple goroutines call `GetBookingSystem()` simultaneously. This is the correct Go idiom for thread-safe Singletons.

**Two layers of locking — why?**
- The **system-level `mu`** protects the `movies`, `theatres`, `shows`, and `bookings` maps. All writes to these collections are guarded.
- The **per-entity mutexes** on `Seat` and `Booking` protect individual object state. This allows finer-grained locking: two bookings for different shows don't block each other.

#### Functions

| Function | What it does | Why |
|---|---|---|
| `GetBookingSystem() *MovieTicketBookingSystem` | Returns the singleton instance, creating it once using `sync.Once`. | Thread-safe Singleton. The `once.Do` ensures the initialization closure runs exactly once regardless of how many goroutines call this. |
| `AddMovie(movie *Movie)` | Acquires a write lock, appends movie to the list, releases lock. | Write operation on shared state — must be locked. Admin operation called at startup. |
| `AddTheatre(theatre *Theatre)` | Acquires a write lock, appends theatre to the list, releases lock. | Same pattern as `AddMovie`. |
| `AddShow(show *Show)` | Acquires a write lock, adds show to the `shows` map by ID, releases lock. | Shows are keyed by ID for fast lookup by `GetShow`. |
| `GetShow(showId string) *Show` | Acquires a **read** lock, looks up the show by ID, returns it. | Read operation — RLock allows concurrent reads. Returns nil if not found. |
| `BookTickets(user, show, selectedSeats)` | **Critical section.** Acquires a write lock. Checks every selected seat is available in the show. Marks all seats as Booked. Calculates total price. Generates a booking ID. Creates and stores the Booking. Returns the Booking. | The entire operation is one atomic transaction under the write lock. This prevents a TOCTOU (time-of-check to time-of-use) race where two users both see a seat as available and both book it. The seat existence check (`show.Seats[seat.ID]`) also guards against passing seats from a different show. |
| `ConfirmBooking(bookingID string)` | Acquires a write lock. Looks up booking. Validates it's in Pending state. Sets status to Confirmed. | Only Pending bookings can be confirmed — idempotency guard. In production this is where payment processing would occur before confirming. |
| `CancelBooking(bookingId string)` | Acquires a write lock. Looks up booking. Validates it's not already Cancelled. Sets status to Cancelled. **Releases all seats** back to `SeatStatusAvailable`. | Seat release is the key side-effect of cancellation. Without it, cancelled bookings would permanently block seats. The system lock ensures the cancellation and seat release happen atomically. |
| `GenerateBookingId() string` | Atomically increments `bookingCount`, then generates a UUID. | `atomic.AddInt64` makes the counter increment thread-safe without a mutex. UUID ensures globally unique IDs even in distributed setups. The counter is available for sequential booking numbering if needed alongside the UUID. |
| `CreateSeats(rows, cols int) map[string]*Seat` | Utility function. Creates a full grid of seats for a show. Seats in rows 0–2 are Premium (₹150); rows 3+ are Normal (₹100). Returns a map keyed by `"row-col"` string. | Used in `demo.go` when creating shows. The Premium rows are at the front (low row numbers) following real cinema conventions. ID format `"1-5"` is used directly to select specific seats in the demo. |

---

### `demo.go` — The Entry Point

```go
func Run() {
    bookingSystem := GetBookingSystem()
    // 1. Add movies, theatres, shows
    // 2. Create a user
    // 3. Select seats by ID
    // 4. BookTickets → Pending booking
    // 5. ConfirmBooking → Confirmed
    // 6. CancelBooking → Cancelled + seats released
}
```

**What it demonstrates:**
1. Full admin setup: 2 movies, 2 theatres, 2 shows (8×8 and 15×10 grids).
2. User creation and seat selection by string ID (`"1-5"`, `"1-2"`).
3. The complete booking lifecycle: Book → Confirm → Cancel.
4. Error handling: each step checks the error and aborts if something fails.

---

## End-to-End Flow

```
Run()
 └─ GetBookingSystem()              → singleton MovieTicketBookingSystem
 └─ AddMovie(movie1), AddMovie(movie2)
 └─ AddTheatre(theatre1), AddTheatre(theatre2)
 └─ NewShow("s1", movie1, theatre1, now, now+120min, CreateSeats(8,8))
      └─ CreateSeats(8,8)           → 64 seats: rows 0-2 Premium ₹150, rows 3-7 Normal ₹100
 └─ AddShow(show1), AddShow(show2)
 └─ NewUser("U1", "Tom Ford", "tom@gmail.com")
 └─ selectedSeats = [show1.Seats["1-5"], show2.Seats["1-2"]]
 └─ BookTickets(user, show1, selectedSeats)
      └─ lock system mutex
      └─ check show1.Seats["1-5"].GetStatus() == SeatStatusAvailable ✓
      └─ check show1.Seats["1-2"] exists in show1? ← "1-2" IS in show1 (it's 8×8)
      └─ mark both seats as SeatStatusBooked
      └─ totalPrice = 150.0 + 150.0 = 300.0
      └─ GenerateBookingId() → UUID
      └─ NewBooking(id, user, show1, seats, 300.0, BookingStatusPending)
      └─ store in bookings map, return booking
 └─ ConfirmBooking(booking.ID)
      └─ lock → find booking → check Pending → set Confirmed
 └─ CancelBooking(booking.ID)
      └─ lock → find booking → check not Cancelled → set Cancelled
      └─ for each seat: show1.Seats[seat.ID].SetStatus(SeatStatusAvailable)
```

---

## Thread Safety Summary

| Resource | Protected by | Read | Write |
|---|---|---|---|
| `MovieTicketBookingSystem.movies/theatres/shows/bookings` | `bs.mu` (RWMutex) | RLock | Lock |
| `Seat.Status` | `s.mu` (RWMutex per seat) | RLock | Lock |
| `Booking.Status` | `b.mu` (RWMutex per booking) | RLock | Lock |
| `bookingCount` | `atomic.AddInt64` | — | Atomic |

**The critical invariant:** `BookTickets` holds the system write lock for the **entire duration** of seat checking + seat marking. This is what prevents two users from both passing the availability check and both marking the same seat as Booked.

---

## Key Design Decisions & Trade-offs

| Decision | Alternative | Why this approach |
|---|---|---|
| `sync.Once` for Singleton | Nil-check (like ParkingLot) | `sync.Once` is safe under concurrent initialization. Nil-check has a race window. |
| System-wide lock in `BookTickets` | Per-seat lock only | Simpler correctness. A system-wide lock for the whole booking transaction avoids partial booking states. Trade-off: lower throughput when many users book simultaneously. |
| Seats keyed by string ID in map | Slice with index | O(1) seat lookup by ID. The demo selects seats like `show1.Seats["1-5"]` directly. |
| UUID for booking ID | Sequential integer | UUIDs are globally unique and don't require coordination in distributed systems. The `bookingCount` atomic counter is available for sequential numbering if needed. |
| Seats created fresh per Show | Shared seat objects across shows | Each show must have independent availability. Sharing seat objects would mean a booking for the 3pm show would affect the 7pm show. |
