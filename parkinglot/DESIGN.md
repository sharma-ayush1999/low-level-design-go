# Parking Lot — Class Diagram, Design Decisions & Extensibility

## Class Diagram

![Parking Lot Class Diagram](parkinglot-class-diagram.png)

---

## What the Diagram Shows (Full Picture)

The diagram captures a production-grade parking lot design. It is significantly richer than a naive implementation. The key insight is that **two independent axes of variation** — *how to find a spot* and *how to charge for it* — are each extracted behind their own interface. Everything else is wiring.

```
ParkingLotSystem  ──── (has many) ────  ParkingFloor  ──── (has many) ────  ParkingSpot
       │                                      │                                    │
  activeTickets                     findAvailableSpot()                   canFitVehicle()
  Map<String, ParkingTicket>              uses ▼                                  │
       │                              ParkingStrategy ◄──── NearestFirstStrategy  │
       │                              (interface)      ◄──── BestFitStrategy       │
       │                                               ◄──── FarthestFirstStrategy │
       │                                                                           │
  ParkingTicket ──── references ──── ParkingSpot                        VehicleSize (enum)
  (entry/exit timestamps)                                                SMALL / MEDIUM / LARGE
                                                                                   ▲
  FeeStrategy  ◄──── FlatRateFeeStrategy                                Vehicle (base)
  (interface)  ◄──── VehicleBasedFeeStrategy                            ├── Bike
                      uses ParkingTicket                                ├── Car
                      (timestamps + vehicle type)                       └── Truck
```

---

## Every Class and Its Role

### `ParkingLotSystem` — The Single Entry Point (Singleton)

```
ParkingLotSystem
 ├── instance: ParkingLotSystem          ← the one global instance
 ├── floors: List<ParkingFloor>          ← all floors in the lot
 ├── activeTickets: Map<String, Ticket>  ← live tickets keyed by license plate
 ├── parkingStrategy: ParkingStrategy    ← injected: how to find a spot
 ├── feeStrategy: FeeStrategy            ← injected: how to compute the fee
 │
 ├── getInstance()                       ← Singleton access point
 ├── addParkingFloor(floor)
 ├── setFeeStrategy(strategy)            ← swap fee strategy at runtime
 ├── setParkingStrategy(strategy)        ← swap parking strategy at runtime
 ├── parkVehicle(licensePlate): Optional<Double>
 └── unparkVehicle(licensePlate): Optional<Double>
```

**Why Singleton?**
A physical parking lot is one entity. One `ParkingLotSystem` governs all floors, all active tickets, and the selected strategies. If two instances existed they would have diverging views of available spots and active tickets — a consistency disaster.

**Why hold `activeTickets` here (not on the floor)?**
When a car exits, you look it up by license plate — you don't know which floor it's on. A central map lets unpark be O(1): look up the ticket, get the spot, free it. If tickets were scattered per floor, unparking would require searching all floors.

**Why inject `parkingStrategy` and `feeStrategy`?**
These are the two things that change across deployments (a mall vs an airport charges differently; a premium lot may use best-fit while a budget lot uses nearest-first). Injecting them means changing behavior is one setter call — no subclassing, no `if` chains.

---

### `ParkingFloor` — One Level of the Lot

```
ParkingFloor
 ├── floorNumber: int
 ├── spots: Map<VehicleSize, List<ParkingSpot>>   ← spots grouped by size
 ├── findAvailableSpot(vehicle): Optional<ParkingSpot>
 ├── addParkingSpot(spot)
 └── displayAvailability()
```

**Why `Map<VehicleSize, List<ParkingSpot>>` instead of a flat list?**
When looking for a spot for a SMALL vehicle (bike), you want to search only SMALL spots — not scan every LARGE truck spot too. The map makes per-size lookup O(k) where k is spots of that size, not O(n) across all spots.

**Why does `findAvailableSpot` live on `ParkingFloor` rather than `ParkingLotSystem`?**
Single Responsibility. The floor knows its own layout. `ParkingLotSystem` delegates to the floor; it doesn't need to know *how* spots are organized on each floor. This also lets different floors have different physical layouts.

---

### `ParkingSpot` — The Atomic Unit

```
ParkingSpot
 ├── spotId: String           ← unique identifier (e.g. "F1-S007")
 ├── spotSize: VehicleSize    ← SMALL / MEDIUM / LARGE
 ├── isOccupied: boolean
 ├── canFitVehicle(v): boolean   ← size compatibility check
 ├── addParkingVehicle(v): boolean
 ├── setSpotFree(): void
 └── isAvailable(): boolean
```

**Why `canFitVehicle` on the spot instead of a size equality check?**
In real lots, a MEDIUM spot can fit a SMALL vehicle (a bike in a car spot is fine; a car in a bike spot is not). `canFitVehicle` encapsulates this rule. Callers don't hardcode size comparisons — they ask the spot "can this vehicle fit here?" and the spot decides. Changing the rule (e.g., never allow oversized) is one method change.

---

### `ParkingTicket` — The Transaction Record

```
ParkingTicket
 ├── spot: ParkingSpot        ← which physical spot
 ├── vehicle: Vehicle         ← which vehicle
 ├── entryTimestamp: long     ← Unix timestamp in ms when parked
 └── exitTimestamp: long      ← set on unpark, used by FeeStrategy
```

**Why a ticket object (not just a spot reference)?**
The ticket is the billing record. A spot only knows if it's occupied. The ticket knows *when* the vehicle arrived, *which vehicle* it was, and *where* it was parked. Without the ticket, you cannot compute duration-based fees. Separating this into its own class means the fee calculation logic never touches ParkingSpot internals.

**Why `long` timestamps instead of `LocalDateTime`?**
Long (Unix epoch ms) is simple to subtract: `exitTimestamp - entryTimestamp = durationMs`. No timezone issues. Easy to serialize. `LocalDateTime` arithmetic requires `Duration.between` and timezone awareness — unnecessary complexity for a fee calculation.

---

### `ParkingStrategy` — How to Pick a Spot (Strategy Pattern)

```java
interface ParkingStrategy {
    Optional<ParkingSpot> findSpot(ParkingFloor floor, Vehicle vehicle);
}

class NearestFirstStrategy   implements ParkingStrategy  // spot with lowest spotId / closest to entrance
class BestFitStrategy        implements ParkingStrategy  // smallest spot that still fits the vehicle
class FarthestFirstStrategy  implements ParkingStrategy  // fills from back, keeps front available
```

**Why a Strategy interface?**
Parking lot operators have competing goals:
- **NearestFirst**: best user experience — driver walks least
- **BestFit**: maximizes lot density — avoids wasting a LARGE spot on a SMALL car
- **FarthestFirst**: keeps high-turnover front spots free for short stays

None of these is universally correct. The Strategy pattern lets the operator inject the policy that fits their business without changing any other code. A premium valet lot and a budget garage can share the same `ParkingLotSystem` class.

**Trade-off:** Three separate classes vs one class with a mode enum.

| Approach | Pro | Con |
|---|---|---|
| Strategy interface | Open/Closed — add `SmartPricingStrategy` without touching existing code | Slightly more files |
| Mode enum + switch | Fewer files | Every new strategy requires modifying the switch, violating Open/Closed |

The Strategy wins for extension-heavy scenarios.

---

### `FeeStrategy` — How to Charge (Strategy Pattern)

```java
interface FeeStrategy {
    double calculateFee(ParkingTicket ticket);
}

class FlatRateFeeStrategy {
    RATE, PB, HOUR, BIDIR   ← flat rate constants
    calculateFee(ticket): double
}

class VehicleBasedFeeStrategy {
    HOURLY_RATES: Map<VehicleSize, Double>   ← e.g. SMALL=10/hr, MEDIUM=20/hr, LARGE=40/hr
    calculateFee(ticket): double
}
```

**Why two fee strategies?**
- A **FlatRateFeeStrategy** works for lots that charge the same regardless of vehicle (e.g. ₹50 for any car up to 2 hours).
- A **VehicleBasedFeeStrategy** works for lots where trucks pay more per hour than bikes.

Both receive the same `ParkingTicket` (which has timestamps and vehicle type) — they just use different parts of it.

**Why `Map<VehicleSize, Double>` in `VehicleBasedFeeStrategy` instead of hardcoded constants?**
Rates change. If they're in a Map, you can load them from config at startup. If they're hardcoded constants, changing rates requires a recompile.

---

### `Vehicle` Hierarchy — Polymorphic Vehicles

```
Vehicle (base class)
 ├── size: VehicleSize      ← determines which spots it can use
 └── licensePlate: String   ← key for activeTickets lookup

     ├── Bike     (size = SMALL)
     ├── Car      (size = MEDIUM)
     └── Truck    (size = LARGE)
```

**Why extend Vehicle instead of using VehicleType enum?**
Each vehicle subclass could carry different data or behavior in the future. A `Bike` might have `isElectric: boolean` for EV charging spot priority. A `Truck` might have `numAxles` for weight-based fees. An enum can only carry a single discriminator — it cannot hold per-type extra state. The class hierarchy is the right tool here.

**Why `VehicleSize` as a separate enum?**
`VehicleSize` (SMALL/MEDIUM/LARGE) is the physical attribute that determines spot compatibility. `VehicleType` (Bike/Car/Truck) is the category. In real lots, a `Van` (new vehicle type) might be MEDIUM sized — not LARGE. Separating size from type means spot compatibility logic never needs to know about vehicle categories.

---

### `VehicleSize` Enum

```
SMALL   → fits in SMALL, MEDIUM, LARGE spots
MEDIUM  → fits in MEDIUM, LARGE spots
LARGE   → fits in LARGE spots only
```

This is the core compatibility rule encoded in `ParkingSpot.canFitVehicle()`. The enum is reused in both the spot (what it accepts) and the vehicle (what size it is).

---

## Class Relationship Map (Full)

```
ParkingLotDemo
    └── uses ──► ParkingLotSystem (Singleton)
                    ├── has-many ──► ParkingFloor
                    │                   └── has-many ──► ParkingSpot
                    │                                       └── references ──► VehicleSize
                    ├── has-many ──► ParkingTicket
                    │                   ├── references ──► ParkingSpot
                    │                   └── references ──► Vehicle
                    ├── uses ──► ParkingStrategy (interface)
                    │               ├── NearestFirstStrategy
                    │               ├── BestFitStrategy
                    │               └── FarthestFirstStrategy
                    └── uses ──► FeeStrategy (interface)
                                    ├── FlatRateFeeStrategy
                                    └── VehicleBasedFeeStrategy

Vehicle (base)
    ├── Bike   (size = SMALL)
    ├── Car    (size = MEDIUM)
    └── Truck  (size = LARGE)
```

**Relationship types explained:**

| Relationship | Pair | Meaning |
|---|---|---|
| Composition | ParkingLotSystem → ParkingFloor | Floors cannot exist without the lot. Lot owns their lifetime. |
| Composition | ParkingFloor → ParkingSpot | Spots cannot exist without a floor. |
| Association | ParkingTicket → ParkingSpot | Ticket references a spot but does not own it. Spot outlives the ticket. |
| Association | ParkingTicket → Vehicle | Ticket records the vehicle but does not own it. |
| Dependency | ParkingLotSystem → ParkingStrategy | Lot uses a strategy but does not own it. Strategy can be swapped. |
| Dependency | ParkingLotSystem → FeeStrategy | Same — fee strategy is injected, not owned. |
| Inheritance | Bike/Car/Truck → Vehicle | IS-A relationship. Each is a concrete vehicle type. |

---

## Design Decisions and Why

### Decision 1: Two Strategy Interfaces (ParkingStrategy + FeeStrategy)

**Why not one class with an `algorithm` flag?**

The parking selection algorithm and the fee calculation algorithm change for completely independent reasons:
- A new parking lot opens and wants NearestFirst but VehicleBased fees.
- The same lot switches to FlatRate fees during a sale without changing its parking algorithm.

Two interfaces means each axis changes independently. Zero coupling between them.

### Decision 2: ParkingTicket as a First-Class Object

**Why not just mark a spot occupied and record start time on the spot?**

Because a `ParkingSpot` is about *space*. A `ParkingTicket` is about *time and money*. Mixing them into one class violates Single Responsibility. The ticket also naturally represents the "receipt" given to the driver — it's a domain concept that deserves its own class. Without a ticket, computing fees requires interrogating the spot object directly, which bleeds fee logic into spatial logic.

### Decision 3: `Map<VehicleSize, List<ParkingSpot>>` on ParkingFloor

**Why not a flat list?**

Finding an available spot for a given vehicle size is the most frequent operation. A flat list forces scanning all spots of all sizes. The map makes the search scoped: only look at spots that could possibly fit the vehicle. For a floor with 200 spots (50 small, 100 medium, 50 large), searching a Car requires scanning 100 spots instead of 200.

### Decision 4: Optional return types on `findAvailableSpot` and `parkVehicle`

**Why Optional instead of null?**

`null` is an invisible contract — callers forget to null-check. `Optional<ParkingSpot>` makes the "no spot found" case **visible in the type signature**. Callers are forced to handle it. This prevents NullPointerExceptions from propagating through parking and fee logic.

---

## Trade-offs

| Decision | Benefit | Cost |
|---|---|---|
| Singleton ParkingLotSystem | One consistent view of all floors and tickets | Hard to test in isolation; tight coupling between test cases if the instance isn't reset |
| Strategy Pattern (2 interfaces) | Swap algorithms at runtime without changing system code | More files; new developers must understand the indirection |
| ParkingTicket with timestamps | Accurate duration-based fee calculation | Every park/unpark operation must maintain the ticket map; extra bookkeeping |
| VehicleSize enum separate from VehicleType | Spot compatibility decoupled from vehicle category | Slightly more domain concepts to learn |
| Map<VehicleSize, spots> on Floor | O(size-filtered) spot search | Map overhead; must maintain consistency when spots change status |
| canFitVehicle on ParkingSpot | Rule for size compatibility is in one place | Spot must know about Vehicle — cross-domain dependency |

---

## Extensibility — What You Can Add Without Changing Existing Code

### Add a new ParkingStrategy (e.g. PriorityLaneStrategy for VIP)

```go
type PriorityLaneStrategy struct{}

func (p *PriorityLaneStrategy) FindSpot(floor *ParkingFloor, vehicle Vehicle) *ParkingSpot {
    // search VIP-tagged spots first, fall back to regular spots
}
```

Inject into `ParkingLotSystem.SetParkingStrategy(new PriorityLaneStrategy())`. Zero changes to existing code.

### Add a new FeeStrategy (e.g. DurationTierFeeStrategy)

```go
type DurationTierFeeStrategy struct{}

func (d *DurationTierFeeStrategy) CalculateFee(ticket *ParkingTicket) float64 {
    // first 1 hr free, then ₹30/hr, then ₹50/hr after 4 hrs
}
```

Inject via `SetFeeStrategy`. Zero changes to ParkingLotSystem, ParkingFloor, or ParkingSpot.

### Add a new Vehicle Type (e.g. ElectricBus)

```go
type ElectricBus struct {
    BaseVehicle
    batteryLevel int
}
func NewElectricBus(plate string) Vehicle {
    return &ElectricBus{BaseVehicle: BaseVehicle{licensePlate: plate, vehicleSize: LARGE}}
}
```

Add `EXTRA_LARGE` to VehicleSize if needed. Update `canFitVehicle` on ParkingSpot. Everything else works.

### Add Reservation/Pre-booking

Introduce a `Reservation` struct (similar to `ParkingTicket`) that holds a future `entryTimestamp`. Add a `reservations` map to `ParkingLotSystem`. Spot's `isAvailable()` checks both current occupation and upcoming reservations. No existing class needs structural changes.

### Add EV Charging Spot Type

Add `isEVCapable: bool` to `ParkingSpot`. Add an `EVFirstStrategy` that prefers EV spots for electric vehicles. The fee strategy can add a `chargingFee` component. Neither ParkingFloor nor ParkingLotSystem changes.

### Add Multi-Tenancy (Multiple Lots)

The Singleton is the main barrier here. To support multiple lots, remove the `instance` field and let callers hold `*ParkingLotSystem` directly (or use a registry pattern: `LotRegistry.GetLot("LAX-Terminal2")`). All other classes remain unchanged.

---

## Current Implementation vs Diagram

The current Go code is a **simplified subset** of the full diagram:

| Feature | Diagram | Current Go |
|---|---|---|
| Parking strategy | 3 strategy classes (pluggable) | Hardcoded first-fit in Level |
| Fee calculation | 2 strategy classes with timestamps | Not implemented |
| ParkingTicket | Full ticket with timestamps | Not present |
| Spot sizing | VehicleSize enum + canFitVehicle | VehicleType enum (type = size) |
| Spot lookup | Map<VehicleSize, List<Spot>> | Linear scan of flat list |
| Vehicle hierarchy | Bike/Car/Truck extend Vehicle | BaseVehicle with type enum |
| Singleton safety | (implied) sync.Once | Nil-check (race-prone) |

The current code is excellent for learning the structure. The diagram represents what you'd build for production.
