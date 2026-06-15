# Parking Lot — Low Level Design

## Problem Statement

Design a parking lot system that can manage multiple floors, support different vehicle types, assign spots on entry, and release them on exit — while keeping the door open for real-time availability tracking and concurrent access.

---

## Requirements

1. The parking lot has multiple levels (floors), each with a fixed number of spots.
2. Three vehicle types are supported: **Motorcycle**, **Car**, and **Truck**.
3. Each spot is dedicated to exactly one vehicle type — a Car cannot park in a Motorcycle spot.
4. On entry, the system finds and assigns the first available matching spot.
5. On exit, the system frees the spot so it becomes available again.
6. Availability can be displayed at any time showing every spot's status.

---

## Project Structure

```
parkinglot/
├── vehicle.go          # Vehicle interface + BaseVehicle + VehicleType enum
├── car.go              # Car factory
├── motorcycle.go       # Motorcycle factory
├── truck.go            # Truck factory
├── parking_spot.go     # Single spot logic
├── level.go            # Floor-level logic (holds spots)
├── parking_lot.go      # Top-level manager (Singleton)
└── parking_lot_demo.go # Entry point / demo
```

---

## Class Diagram

![Parking Lot Class Diagram](parkinglot-class-diagram.png)

---

## Design Patterns Used

| Pattern | Where | Why |
|---|---|---|
| **Singleton** | `ParkingLot` | There is exactly one physical parking lot. Prevents duplicate instances from managing the same resource. |
| **Interface / Polymorphism** | `Vehicle` interface | Lets `ParkingSpot` and `Level` work with any vehicle type without knowing its concrete class. |
| **Factory** | `NewCar`, `NewMotorcycle`, `NewTruck` | Hides construction details. Callers ask for a vehicle by type and get back a `Vehicle` — they don't care about `BaseVehicle`. |

---

## File-by-File Breakdown

### `vehicle.go` — The Vehicle Abstraction

```
VehicleType  →  CAR | MOTORCYCLE | TRUCK  (iota enum)
Vehicle      →  interface: GetLicensePlate() + GetType()
BaseVehicle  →  concrete struct implementing Vehicle
```

**Why an interface?**
`ParkingSpot` needs to hold any vehicle and ask "what type are you?". By coding against the `Vehicle` interface instead of a concrete struct, the spot doesn't care whether it holds a Car or Truck. This is the **Dependency Inversion Principle** — high-level logic depends on abstractions, not implementations.

**Why `VehicleType` as an `int` enum (iota)?**
Spot allocation is type-based. Using an enum makes comparisons like `spot.vehicleType == vehicle.GetType()` clean and type-safe. Adding a new type (e.g., `BUS`) later is a one-line change.

#### Functions

| Function | What it does |
|---|---|
| `GetLicensePlate() string` | Returns the vehicle's license plate. Used for identification when un-parking (future extension). |
| `GetType() VehicleType` | Returns the vehicle's type. Used by `ParkingSpot.ParkVehicle` to enforce type matching. |

---

### `car.go` / `motorcycle.go` / `truck.go` — Vehicle Factories

```go
func NewCar(licensePlate string) Vehicle
func NewMotorcycle(licensePlate string) Vehicle
func NewTruck(licensePlate string) Vehicle
```

**Why three separate files?**
Each file is a factory that creates a `BaseVehicle` pre-wired to its type. Callers get a `Vehicle` (interface) back — they never need to import or know about `BaseVehicle`. If Car needed extra fields in the future (e.g., `numDoors`), only `car.go` changes.

---

### `parking_spot.go` — The Smallest Unit

A `ParkingSpot` represents one physical space in the lot. It knows:
- Its spot number
- Which vehicle type it accepts
- Which vehicle is currently parked (nil = empty)

```
ParkingSpot {
    spotNumber    int
    vehicleType   VehicleType   ← what type this spot accepts
    parkedVehicle Vehicle       ← nil when available
}
```

#### Functions

| Function | What it does | Why |
|---|---|---|
| `NewParkingSpot(spotNumber, vehicleType)` | Constructor. Creates an empty spot of the given type. | Encapsulates initialization. Spot starts with `parkedVehicle = nil`, so `IsAvailable()` returns true from the start. |
| `IsAvailable() bool` | Returns `true` if no vehicle is parked (`parkedVehicle == nil`). | The canonical way to check occupancy. Used by `Level.ParkVehicle` to find a free spot. |
| `ParkVehicle(vehicle Vehicle)` | Parks the vehicle **only if** the spot is available AND the vehicle type matches. | Enforces both conditions at the spot level. Even if Level accidentally passes a wrong vehicle, the spot acts as a last guard. |
| `UnParkVehicle()` | Sets `parkedVehicle = nil`, marking the spot as free. | Simple release. No need to know which vehicle was there — the spot just becomes empty again. |
| `GetSpotNumber() int` | Returns the spot's number. | Used in `DisplayAvailability` for human-readable output. |
| `GetVehicleType() VehicleType` | Returns the type this spot accepts. | Used by `Level.ParkVehicle` to match vehicle type to spot type before attempting to park. |
| `GetParkedVehicle() Vehicle` | Returns the currently parked vehicle (or nil). | Used by `Level.UnparkVehicle` to find which spot holds the vehicle being removed. |

---

### `level.go` — A Floor in the Lot

A `Level` is one floor of the parking lot. It holds a collection of `ParkingSpot`s and is responsible for parking/unparking on that floor.

```
Level {
    floor        int
    parkingSpots []*ParkingSpot
}
```

#### Spot Allocation Strategy in `NewLevel`

```
50% of spots → MOTORCYCLE
40% of spots → CAR
remaining    → TRUCK
```

**Why this split?**
Motorcycles are most common and smallest, so they get the most spots. Trucks are rare and large, so they get the least. This mirrors a realistic parking lot layout. The proportions are hardcoded here for simplicity; in production you'd pass them as config.

#### Functions

| Function | What it does | Why |
|---|---|---|
| `NewLevel(floor, numSpots)` | Creates a level and pre-allocates spots following the 50/40/remaining split. | All spot creation logic lives here. The caller just says "I want a floor with N spots" and gets a properly divided level. |
| `ParkVehicle(vehicle Vehicle) bool` | Iterates all spots and parks in the **first available spot** matching the vehicle type. Returns `true` on success, `false` if no spot found. | **First-fit** strategy — simple and fast. Returns early as soon as a spot is found. If no spot fits, the lot at this level is full for that type. |
| `UnparkVehicle(vehicle Vehicle) bool` | Iterates all spots, finds the one holding this exact vehicle (by pointer equality), and calls `UnParkVehicle()`. Returns `true` if found. | Pointer equality works because we park and unpark the exact same `Vehicle` object in the demo. In production you'd match by license plate. |
| `DisplayAvailability()` | Prints each spot's floor, number, status (Available/Occupied), and vehicle type. | Provides a human-readable snapshot of the floor. Useful for debugging and operator dashboards. |

---

### `parking_lot.go` — The Singleton Manager

`ParkingLot` is the top-level entry point. It holds all levels and delegates parking/unparking to them in order.

```
ParkingLot {
    levels []*Level
}
```

**Why Singleton?**
A parking lot is a single physical entity. There must only ever be one instance managing the levels and spots. The Singleton pattern (`GetParkingLotInstance`) ensures that no matter how many times you call it, you always get the same object.

**Note on thread safety:** The current implementation uses a simple nil-check for the singleton, which is not safe for concurrent initialization. In production, use `sync.Once` (as done in the Movie Booking system) to guarantee safe initialization under concurrency.

#### Functions

| Function | What it does | Why |
|---|---|---|
| `GetParkingLotInstance() *ParkingLot` | Returns the single global instance, creating it on first call. | Singleton entry point. Callers never call `&ParkingLot{}` directly. |
| `AddLevel(level *Level)` | Appends a new floor to the lot. | Lets you configure the lot at startup — add as many levels as needed. |
| `ParkVehicle(vehicle Vehicle) bool` | Tries to park on each level in order (Level 1 first, then Level 2, etc.). Returns `true` if parked somewhere. | **Level-first fit** — fills lower levels before higher ones. Simple and cache-friendly for real-world lots. |
| `UnParkVehicle(vehicle Vehicle) bool` | Searches all levels for the vehicle and unparks it. Returns `true` if found. | Delegates to each level's `UnparkVehicle`. The lot doesn't track which level a vehicle is on, so it searches linearly. |
| `DisplayAvailability()` | Calls `DisplayAvailability()` on every level. | Top-level convenience that prints the full lot status in one call. |

---

### `parking_lot_demo.go` — The Demo / Entry Point

```go
func Run() {
    parkingLot := GetParkingLotInstance()
    parkingLot.AddLevel(NewLevel(1, 5))
    parkingLot.AddLevel(NewLevel(2, 3))
    // ...
}
```

**What it demonstrates:**
1. Creates a 2-level lot (Level 1 with 5 spots, Level 2 with 3 spots).
2. Creates one Car, one Truck, and one Motorcycle with license plates.
3. Parks all three vehicles and prints availability.
4. Unparks the motorcycle and prints availability again — showing the spot is now free.

---

## End-to-End Flow

```
Run()
 └─ GetParkingLotInstance()           → creates ParkingLot (Singleton)
 └─ AddLevel(NewLevel(1, 5))          → Level 1: 2 motorcycle spots, 2 car spots, 1 truck spot
 └─ AddLevel(NewLevel(2, 3))          → Level 2: 1 motorcycle spot, 1 car spot, 1 truck spot
 └─ ParkVehicle(car "ABC123")
      └─ Level 1: finds first CAR spot → parks there → returns true
 └─ ParkVehicle(truck "XYZ456")
      └─ Level 1: finds first TRUCK spot → parks there → returns true
 └─ ParkVehicle(motorcycle "PQR789")
      └─ Level 1: finds first MOTORCYCLE spot → parks there → returns true
 └─ DisplayAvailability()             → shows all spots with their status
 └─ UnParkVehicle(motorcycle)
      └─ Level 1: finds the spot holding motorcycle → frees it → returns true
 └─ DisplayAvailability()             → motorcycle spot now shows "Available"
```

---

## Key Design Decisions & Trade-offs

| Decision | Alternative | Why this approach |
|---|---|---|
| Spot type is fixed at creation | Dynamic spot type | Mirrors real lots. Spots have physical size limits. |
| First-fit allocation | Best-fit / random | Simple, O(n) per park. Good enough at this scale. |
| Pointer equality for unpark | Match by license plate | Simpler for demo. License plate match is more robust for production. |
| No mutex in ParkingLot/Level | sync.Mutex on park/unpark | Demo is single-threaded. Add mutex before deploying to concurrent systems. |
