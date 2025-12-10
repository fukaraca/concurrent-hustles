package concurrent_hustles

import (
	"errors"
	"sync"
)

type ParkManager interface {
	RemainingSpots() int
	TotalSpots() int
	IsFull() bool
	IsEmpty() bool
	IsAvailable(t AutoType) bool
	HowManySpots(t AutoType) int

	RegisterAuto(a Auto) error
	UnregisterAuto(a Auto) error
	Close()
}

var _ ParkManager = (*Park)(nil)

type Park struct {
	mu         sync.RWMutex
	cTot       int       // total cap
	nm, nc, nv int       // counts of each auto
	sM, sC, sR *ParkSlot // slots as cache
	lR         int       // length of regular spot calculated separately
	closed     bool
}

type ParkSlot struct {
	name     string
	capacity int
	slot     map[Auto]struct{}
}

type Auto struct {
	t     AutoType
	id    int
	dummy int8
}

type AutoType int

const (
	_ AutoType = iota
	TypeMoto
	TypeCar
	TypeVan
)

func NewParkManager(capacityMoto, capacityCar, capacityRegular int) ParkManager {
	return &Park{
		cTot: capacityCar + capacityMoto + capacityRegular,
		sM:   &ParkSlot{name: "Motorcycle", capacity: capacityMoto, slot: make(map[Auto]struct{}, capacityMoto)},
		sC:   &ParkSlot{name: "Car", capacity: capacityCar, slot: make(map[Auto]struct{}, capacityCar)},
		sR:   &ParkSlot{name: "Regular", capacity: capacityRegular, slot: make(map[Auto]struct{}, capacityRegular)},
	}
}

func (p *Park) RemainingSpots() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.remainingSpots()
}

func (p *Park) remainingSpots() int {
	return p.cTot - (len(p.sC.slot) + len(p.sM.slot) + p.lR)
}

func (p *Park) TotalSpots() int {
	return p.cTot
}

func (p *Park) IsEmpty() bool {
	return p.RemainingSpots() == p.cTot
}

func (p *Park) IsFull() bool {
	return p.RemainingSpots() == 0
}

func (p *Park) IsAvailable(t AutoType) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isAvailable(t)
}

func (p *Park) isAvailable(t AutoType) bool {
	switch t {
	case TypeMoto:
		return len(p.sM.slot) < p.sM.capacity
	case TypeCar:
		return len(p.sC.slot) < p.sC.capacity
	case TypeVan:
		return p.lR+2 < p.sR.capacity
	}
	return false
}

func (p *Park) HowManySpots(t AutoType) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.nv * 3
}

func (p *Park) RegisterAuto(a Auto) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return errors.New("parking lot is closed")
	}
	r := p.remainingSpots()
	if (a.t == TypeVan && p.lR+3 > p.sR.capacity) || r == 0 {
		return errors.New("no available spots")
	}

	switch a.t {
	case TypeMoto:
		p.nm++
		if p.isAvailable(TypeMoto) {
			p.sM.slot[a] = struct{}{}
		} else if p.isAvailable(TypeCar) {
			p.sC.slot[a] = struct{}{}
		} else {
			p.sR.slot[a] = struct{}{}
			p.lR++
		}
	case TypeCar:
		p.nc++
		if p.isAvailable(TypeCar) {
			p.sC.slot[a] = struct{}{}
		} else {
			p.sR.slot[a] = struct{}{}
			p.lR++
		}

	case TypeVan:
		p.sR.slot[a] = struct{}{}
		p.lR += 3
	}

	return nil
}

func (p *Park) UnregisterAuto(a Auto) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.remainingSpots() == p.cTot {
		return errors.New("park is empty")
	}
	switch a.t {
	case TypeMoto:
		if _, ok := p.sM.slot[a]; !ok {
			if _, ok := p.sC.slot[a]; !ok {
				if _, ok := p.sR.slot[a]; !ok {
					return errors.New("moto not found")
				} else {
					delete(p.sR.slot, a)
					p.lR--
				}
			} else {
				delete(p.sC.slot, a)

			}
		} else {
			delete(p.sM.slot, a)
		}
		p.nm--
	case TypeCar:
		if _, ok := p.sC.slot[a]; !ok {
			if _, ok := p.sR.slot[a]; !ok {
				return errors.New("car not found")
			} else {
				p.lR--
				delete(p.sR.slot, a)
			}
		} else {
			delete(p.sC.slot, a)
		}
		p.nc--
	case TypeVan:
		if _, ok := p.sR.slot[a]; !ok {
			return errors.New("van not found")
		}
		delete(p.sR.slot, a)
		p.lR -= 3
		p.nv--
	}

	return nil
}

func (p *Park) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
}
