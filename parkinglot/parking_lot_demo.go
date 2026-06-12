package parkinglot

func Run(){
	parkingLot := GetParkingLotInstance()
	parkingLot.AddLevel(NewLevel(1, 5))
	parkingLot.AddLevel(NewLevel(2, 3))

	car := NewCar("ABC123")
	truck := NewTruck("XYZ456")
	motorcycle := NewMotorcycle("PQR789")

	parkingLot.ParkVehicle(car)
	parkingLot.ParkVehicle(truck)
	parkingLot.ParkVehicle(motorcycle)

	parkingLot.DisplayAvailability()

	parkingLot.UnParkVehicle(motorcycle)

	parkingLot.DisplayAvailability()
}