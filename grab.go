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

// NamedGrabber is a same as Grabber but contains name.
// it mainly used for pre defined grabbers.
type NamedGrabber interface {
	Grabber
	Name() string
}

type namedGrabber struct {
	fn   func(c Container) (interface{}, error)
	name string
}

func (n namedGrabber) Grab(c Container) (interface{}, error) {
	return n.fn(c)
}

func (n namedGrabber) Name() string {
	return n.name
}

// NamedFunc is a hellper function to create Grabber type with name
func NamedFunc(name string, fn func(c Container) (interface{}, error)) Grabber {
	return &namedGrabber{fn: fn, name: name}
}

// Container is a base interface for this package. It provides the basic
// interface to load an object using it's grabber func
type Container interface {
	Get(dest interface{}, g Grabber) error
}

// Repository is an implementation for Container interface. It is thread safe
// it also support circular dependency detection.
type Repository struct {
	predefinedGrabbers map[string]Grabber
	grabbers           map[Grabber]interface{}
	pendding           map[Grabber]struct{}
	mtx                sync.RWMutex
}

// Get accepts a pointer to any types (struct or interface), and grabber.
func (s *Repository) Get(dest interface{}, g Grabber) error {
	var err error

	// tries to see if given grabber already been defined
	// during creation of container
	if grab, ok := g.(NamedGrabber); ok {
		name := grab.Name()
		if name != "" {
			s.mtx.RLock()
			if grab, ok := s.predefinedGrabbers[name]; ok {
				g = grab
			}
			s.mtx.Unlock()
		}
	}

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

// New initialize the Repository container. Repository is Thread-Safe
func New(grabs ...NamedGrabber) *Repository {
	predefinedGrabbers := make(map[string]Grabber, 0)
	for _, grab := range grabs {
		predefinedGrabbers[grab.Name()] = grab
	}

	return &Repository{
		predefinedGrabbers: predefinedGrabbers,
		grabbers:           make(map[Grabber]interface{}, 0),
		pendding:           make(map[Grabber]struct{}, 0),
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
