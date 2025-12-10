package concurrent_hustles

import (
	"sync"
	"testing"
)

func TestNewParkManager(t *testing.T) {
	tests := []struct {
		name          string
		capMoto       int
		capCar        int
		capReg        int
		expectedTotal int
	}{
		{"Standard parking lot", 10, 20, 30, 60},
		{"Zero moto spots", 0, 20, 30, 50},
		{"Zero car spots", 10, 0, 30, 40},
		{"Zero regular spots", 10, 20, 0, 30},
		{"All zeros", 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewParkManager(tt.capMoto, tt.capCar, tt.capReg)
			if pm.TotalSpots() != tt.expectedTotal {
				t.Errorf("TotalSpots() = %d, want %d", pm.TotalSpots(), tt.expectedTotal)
			}
		})
	}
}

func TestRemainingSpots(t *testing.T) {
	pm := NewParkManager(5, 10, 15)

	if pm.RemainingSpots() != 30 {
		t.Errorf("Initial RemainingSpots() = %d, want 30", pm.RemainingSpots())
	}

	moto := Auto{t: TypeMoto, id: 1}
	err := pm.RegisterAuto(moto)
	if err != nil {
		t.Errorf("register failed %v", err)
	}

	if pm.RemainingSpots() != 29 {
		t.Errorf("After 1 moto, RemainingSpots() = %d, want 29", pm.RemainingSpots())
	}
}

func TestIsEmpty(t *testing.T) {
	pm := NewParkManager(5, 10, 15)

	if !pm.IsEmpty() {
		t.Error("New parking lot should be empty")
	}

	moto := Auto{t: TypeMoto, id: 1}
	pm.RegisterAuto(moto)

	if pm.IsEmpty() {
		t.Error("Parking lot with vehicle should not be empty")
	}
}

func TestIsFull(t *testing.T) {
	pm := NewParkManager(1, 1, 1)

	if pm.IsFull() {
		t.Error("New parking lot should not be full")
	}

	pm.RegisterAuto(Auto{t: TypeMoto, id: 1})
	pm.RegisterAuto(Auto{t: TypeCar, id: 2})
	pm.RegisterAuto(Auto{t: TypeMoto, id: 3})

	if !pm.IsFull() {
		t.Error("Parking lot should be full")
	}
}

func TestIsAvailable(t *testing.T) {
	pm := NewParkManager(2, 2, 6)

	tests := []struct {
		name     string
		autoType AutoType
		expected bool
	}{
		{"Moto available initially", TypeMoto, true},
		{"Car available initially", TypeCar, true},
		{"Van available initially", TypeVan, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pm.IsAvailable(tt.autoType); got != tt.expected {
				t.Errorf("IsAvailable(%v) = %v, want %v", tt.autoType, got, tt.expected)
			}
		})
	}
}

func TestRegisterAuto_Moto(t *testing.T) {
	tests := []struct {
		name        string
		capMoto     int
		capCar      int
		capReg      int
		registerCnt int
		wantErr     bool
	}{
		{"Register within moto capacity", 3, 0, 0, 3, false},
		{"Register exceeding moto capacity", 2, 0, 0, 3, true},
		{"Register moto in car spot", 1, 2, 0, 2, false},
		{"Register moto in regular spot", 1, 1, 1, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewParkManager(tt.capMoto, tt.capCar, tt.capReg)
			var err error
			for i := 0; i < tt.registerCnt; i++ {
				err = pm.RegisterAuto(Auto{t: TypeMoto, id: i})
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterAuto() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegisterAuto_Car(t *testing.T) {
	tests := []struct {
		name        string
		capMoto     int
		capCar      int
		capReg      int
		registerCnt int
		wantErr     bool
	}{
		{"Register within car capacity", 0, 3, 0, 3, false},
		{"Register exceeding car capacity", 0, 2, 0, 3, true},
		{"Register car in regular spot", 0, 1, 2, 2, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewParkManager(tt.capMoto, tt.capCar, tt.capReg)
			var err error
			for i := 0; i < tt.registerCnt; i++ {
				err = pm.RegisterAuto(Auto{t: TypeCar, id: i})
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterAuto() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegisterAuto_Van(t *testing.T) {
	tests := []struct {
		name    string
		capReg  int
		vanCnt  int
		wantErr bool
	}{
		{"Register single van", 3, 1, false},
		{"Register two vans", 6, 2, false},
		{"Register van without enough space", 2, 1, true},
		{"Register multiple vans exceeding capacity", 6, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewParkManager(0, 0, tt.capReg)
			var err error
			for i := 0; i < tt.vanCnt; i++ {
				err = pm.RegisterAuto(Auto{t: TypeVan, id: i})
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterAuto() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegisterAuto_Closed(t *testing.T) {
	pm := NewParkManager(5, 5, 5)
	pm.Close()

	err := pm.RegisterAuto(Auto{t: TypeMoto, id: 1})
	if err == nil {
		t.Error("RegisterAuto() should return error when parking lot is closed")
	}
}

func TestUnregisterAuto_Moto(t *testing.T) {
	pm := NewParkManager(2, 2, 2)
	moto := Auto{t: TypeMoto, id: 1}

	// Try to unregister from empty lot
	err := pm.UnregisterAuto(moto)
	if err == nil {
		t.Error("UnregisterAuto() should return error when lot is empty")
	}

	// Register and unregister
	pm.RegisterAuto(moto)
	err = pm.UnregisterAuto(moto)
	if err != nil {
		t.Errorf("UnregisterAuto() error = %v, want nil", err)
	}

	// Try to unregister non-existent moto
	err = pm.UnregisterAuto(Auto{t: TypeMoto, id: 999})
	if err == nil {
		t.Error("UnregisterAuto() should return error for non-existent moto")
	}
}

func TestUnregisterAuto_Car(t *testing.T) {
	pm := NewParkManager(2, 2, 2)
	car := Auto{t: TypeCar, id: 1}

	pm.RegisterAuto(car)
	err := pm.UnregisterAuto(car)
	if err != nil {
		t.Errorf("UnregisterAuto() error = %v, want nil", err)
	}

	// Try to unregister non-existent car
	err = pm.UnregisterAuto(Auto{t: TypeCar, id: 999})
	if err == nil {
		t.Error("UnregisterAuto() should return error for non-existent car")
	}
}

func TestUnregisterAuto_Van(t *testing.T) {
	pm := NewParkManager(0, 0, 6)
	van := Auto{t: TypeVan, id: 1}

	pm.RegisterAuto(van)
	err := pm.UnregisterAuto(van)
	if err != nil {
		t.Errorf("UnregisterAuto() error = %v, want nil", err)
	}

	// Try to unregister non-existent van
	err = pm.UnregisterAuto(Auto{t: TypeVan, id: 999})
	if err == nil {
		t.Error("UnregisterAuto() should return error for non-existent van")
	}
}

func TestMixedOperations(t *testing.T) {
	pm := NewParkManager(5, 10, 15)

	// Register various vehicles
	vehicles := []Auto{
		{t: TypeMoto, id: 1},
		{t: TypeMoto, id: 2},
		{t: TypeCar, id: 3},
		{t: TypeCar, id: 4},
		{t: TypeVan, id: 5},
	}

	for _, v := range vehicles {
		if err := pm.RegisterAuto(v); err != nil {
			t.Errorf("RegisterAuto(%v) error = %v", v, err)
		}
	}

	initialRemaining := pm.RemainingSpots()

	// Unregister some vehicles
	if err := pm.UnregisterAuto(vehicles[0]); err != nil {
		t.Errorf("UnregisterAuto error = %v", err)
	}
	if err := pm.UnregisterAuto(vehicles[2]); err != nil {
		t.Errorf("UnregisterAuto error = %v", err)
	}

	if pm.RemainingSpots() <= initialRemaining {
		t.Error("RemainingSpots should increase after unregistering")
	}
}

// Concurrency Tests

func TestConcurrentRegister(t *testing.T) {
	pm := NewParkManager(50, 50, 50)
	var wg sync.WaitGroup
	goroutines := 100

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			autoType := TypeMoto
			if id%3 == 0 {
				autoType = TypeCar
			} else if id%5 == 0 {
				autoType = TypeVan
			}
			pm.RegisterAuto(Auto{t: autoType, id: id})
		}(i)
	}
	wg.Wait()

	if pm.RemainingSpots() < 0 {
		t.Error("RemainingSpots should not be negative after concurrent registration")
	}
}

func TestConcurrentUnregister(t *testing.T) {
	pm := NewParkManager(50, 50, 50)
	vehicles := make([]Auto, 50)

	// Register vehicles first
	for i := 0; i < 50; i++ {
		vehicles[i] = Auto{t: TypeMoto, id: i}
		pm.RegisterAuto(vehicles[i])
	}

	var wg sync.WaitGroup
	wg.Add(50)
	for i := 0; i < 50; i++ {
		go func(idx int) {
			defer wg.Done()
			pm.UnregisterAuto(vehicles[idx])
		}(i)
	}
	wg.Wait()

	if !pm.IsEmpty() {
		t.Error("Parking lot should be empty after unregistering all vehicles")
	}
}

func TestConcurrentRegisterAndUnregister(t *testing.T) {
	pm := NewParkManager(30, 30, 30)
	var wg sync.WaitGroup
	operations := 200

	wg.Add(operations)
	for i := 0; i < operations; i++ {
		go func(id int) {
			defer wg.Done()
			auto := Auto{t: TypeMoto, id: id % 50}
			if id%2 == 0 {
				pm.RegisterAuto(auto)
			} else {
				pm.UnregisterAuto(auto)
			}
		}(i)
	}
	wg.Wait()

	remaining := pm.RemainingSpots()
	if remaining < 0 || remaining > pm.TotalSpots() {
		t.Errorf("RemainingSpots = %d, should be between 0 and %d", remaining, pm.TotalSpots())
	}
}

func TestConcurrentReadOperations(t *testing.T) {
	pm := NewParkManager(20, 20, 20)
	var wg sync.WaitGroup
	goroutines := 100

	// Register some vehicles
	for i := 0; i < 10; i++ {
		pm.RegisterAuto(Auto{t: TypeMoto, id: i})
	}

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			switch id % 5 {
			case 0:
				pm.RemainingSpots()
			case 1:
				pm.TotalSpots()
			case 2:
				pm.IsEmpty()
			case 3:
				pm.IsFull()
			case 4:
				pm.IsAvailable(TypeMoto)
			}
		}(i)
	}
	wg.Wait()
}

func TestConcurrentMixedOperations(t *testing.T) {
	pm := NewParkManager(50, 50, 50)
	var wg sync.WaitGroup
	goroutines := 300

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			auto := Auto{t: TypeMoto, id: id % 100}

			switch id % 7 {
			case 0, 1:
				pm.RegisterAuto(auto)
			case 2:
				pm.UnregisterAuto(auto)
			case 3:
				pm.RemainingSpots()
			case 4:
				pm.IsEmpty()
			case 5:
				pm.IsFull()
			case 6:
				pm.IsAvailable(TypeMoto)
			}
		}(i)
	}
	wg.Wait()

	remaining := pm.RemainingSpots()
	if remaining < 0 || remaining > pm.TotalSpots() {
		t.Errorf("After concurrent operations: RemainingSpots = %d, should be between 0 and %d",
			remaining, pm.TotalSpots())
	}
}

func TestConcurrentClose(t *testing.T) {
	pm := NewParkManager(20, 20, 20)
	var wg sync.WaitGroup
	goroutines := 50

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			if id == 25 {
				pm.Close()
			} else {
				pm.RegisterAuto(Auto{t: TypeMoto, id: id})
			}
		}(i)
	}
	wg.Wait()

	// Try to register after close
	err := pm.RegisterAuto(Auto{t: TypeMoto, id: 999})
	if err == nil {
		t.Error("RegisterAuto should fail after Close()")
	}
}

func TestRaceConditionStressTest(t *testing.T) {
	pm := NewParkManager(100, 100, 100)
	var wg sync.WaitGroup
	iterations := 1000

	wg.Add(iterations * 2)

	// Concurrent writers
	for i := 0; i < iterations; i++ {
		go func(id int) {
			defer wg.Done()
			auto := Auto{t: TypeCar, id: id}
			pm.RegisterAuto(auto)
		}(i)
	}

	// Concurrent readers
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			pm.RemainingSpots()
			pm.IsEmpty()
			pm.IsFull()
		}()
	}

	wg.Wait()
}

func TestEdgeCases(t *testing.T) {
	t.Run("Zero capacity parking lot", func(t *testing.T) {
		pm := NewParkManager(0, 0, 0)
		if !pm.IsFull() {
			t.Error("Zero capacity lot should be full")
		}
		err := pm.RegisterAuto(Auto{t: TypeMoto, id: 1})
		if err == nil {
			t.Error("Should not be able to register in zero capacity lot")
		}
	})

	t.Run("Register same vehicle twice", func(t *testing.T) {
		pm := NewParkManager(5, 5, 5)
		auto := Auto{t: TypeMoto, id: 1}

		pm.RegisterAuto(auto)
		err := pm.RegisterAuto(auto)
		// This should succeed as the code doesn't check for duplicates
		if err != nil {
			t.Logf("Registering same vehicle twice: %v", err)
		}
	})

	t.Run("Large parking lot", func(t *testing.T) {
		pm := NewParkManager(1000, 1000, 1000)
		if pm.TotalSpots() != 3000 {
			t.Errorf("TotalSpots = %d, want 3000", pm.TotalSpots())
		}
	})
}
