package grab

import (
	"errors"
	"reflect"
	"sync"
)

var (
	ErrSrcMustBePointer           = errors.New("src must be a pointer")
	ErrDestMustBeDoublePointer    = errors.New("dest must be a pointer to a pointer type")
	ErrDestMustBePointer          = errors.New("dest must be a pointer")
	ErrDestInterfaceMustBePointer = errors.New("dest interface must be pass as pointer")
	ErrCircularDependency         = errors.New("circular dependency detected")
)

var empty struct{}

// Grabber interface is a single method interface intended to
// initialize an object. By using container, grabber can request
// initialization of other modules by just calling their grabber func
type Grabber interface {
	Grab(c Container) (interface{}, error)
}

type grabber struct {
	fn func(c Container) (interface{}, error)
}

func (f grabber) Grab(c Container) (interface{}, error) {
	return f.fn(c)
}

// Func is a hellper function to create Grabber type
func Func(fn func(c Container) (interface{}, error)) Grabber {
	return &grabber{fn: fn}
}

// Container is a base interface for this package. It provides the basic
// interface to load an object using it's grabber func
type Container interface {
	Get(dest interface{}, g Grabber) error
}

// System is an implementation for Container interface. It is thread safe
// it also support circular dependency detection.
type System struct {
	grabbers map[Grabber]interface{}
	pendding map[Grabber]struct{}
	mtx      sync.RWMutex
}

func (s *System) Get(dest interface{}, g Grabber) error {
	var err error

	// we need the read lock here to make sure that
	// no one can update the grabbers map
	s.mtx.RLock()
	value, ok := s.grabbers[g]
	s.mtx.RUnlock()

	if ok {
		return assign(dest, value)
	}

	s.mtx.RLock()
	_, ok = s.pendding[g]
	s.mtx.RUnlock()

	if ok {
		return ErrCircularDependency
	}

	s.mtx.Lock()
	s.pendding[g] = empty
	s.mtx.Unlock()

	value, err = g.Grab(s)
	if err != nil {
		s.mtx.Lock()
		delete(s.pendding, g)
		s.mtx.Unlock()
		return err
	}

	s.mtx.Lock()
	s.grabbers[g] = value
	delete(s.pendding, g)
	s.mtx.Unlock()

	return assign(dest, value)
}

// New initialize the System container. System is Thread-Safe
func New() *System {
	return &System{
		grabbers: make(map[Grabber]interface{}, 0),
		pendding: make(map[Grabber]struct{}, 0),
	}
}

// assign is the heart of this package it uses reflect to assign a value to given dest interface
func assign(dest interface{}, src interface{}) error {
	destType := reflect.TypeOf(dest)

	// if destType is nil, it means that dest was an interface type
	if destType == nil {
		return ErrDestInterfaceMustBePointer
	}

	if destType.Kind() != reflect.Ptr {
		return ErrDestMustBePointer
	}

	switch destType.Elem().Kind() {
	case reflect.Interface:
		// do nothing, dest a pointer to an interface
	case reflect.Ptr:
		// do nothing, dest a pointer to another pointer
	default:
		return ErrDestMustBeDoublePointer
	}

	// make sure that srcType is a pointer
	srcType := reflect.TypeOf(src)
	if srcType.Kind() != reflect.Ptr {
		return ErrSrcMustBePointer
	}

	// assign the value to dest
	reflect.ValueOf(dest).Elem().Set(reflect.ValueOf(src))
	return nil
}
