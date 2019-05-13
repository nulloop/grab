package grab

import (
	"reflect"
	"sync"
)

// E a simple type to make all errors const
type E string

func (e E) Error() string {
	return string(e)
}

const (
	ErrSrcMustBePointer           = E("src must be a pointer")
	ErrDestMustBeDoublePointer    = E("dest must be a pointer to a pointer type")
	ErrDestMustBePointer          = E("dest must be a pointer")
	ErrDestInterfaceMustBePointer = E("dest interface must be pass as pointer")
	ErrCircularDependency         = E("circular dependency detected")
	ErrAlreadyMocked              = E("already mocked")
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

// Repository is an implementation for Container interface. It is thread safe
// it also support circular dependency detection.
type Repository struct {
	grabbers map[Grabber]interface{}
	pendding map[Grabber]struct{}
	mtx      sync.RWMutex
}

// Get accepts a pointer to any types (struct or interface), and grabber.
func (r *Repository) Get(dest interface{}, g Grabber) error {
	var err error

	// we need the read lock here to make sure that
	// no one can update the grabbers map
	r.mtx.RLock()
	value, ok := r.grabbers[g]
	r.mtx.RUnlock()

	if ok {
		return assign(dest, value)
	}

	r.mtx.RLock()
	_, ok = r.pendding[g]
	r.mtx.RUnlock()

	if ok {
		return ErrCircularDependency
	}

	r.mtx.Lock()
	r.pendding[g] = empty
	r.mtx.Unlock()

	value, err = g.Grab(r)
	if err != nil {
		r.mtx.Lock()
		delete(r.pendding, g)
		r.mtx.Unlock()
		return err
	}

	r.mtx.Lock()
	r.grabbers[g] = value
	delete(r.pendding, g)
	r.mtx.Unlock()

	return assign(dest, value)
}

// New initialize the Repository container. Repository is Thread-Safe
func New() *Repository {
	return &Repository{
		grabbers: make(map[Grabber]interface{}, 0),
		pendding: make(map[Grabber]struct{}, 0),
	}
}

// RepositoryWithMock is a composite struct which adds a new method call Mock
// It also wrap Get to returns value that has already been mocked
type RepositoryWithMock struct {
	*Repository
	mocked map[Grabber]interface{}
}

// Get this method has been overrid to provide mock system
func (r *RepositoryWithMock) Get(dest interface{}, g Grabber) error {
	r.mtx.Lock()
	if value, ok := r.mocked[g]; ok {
		r.mtx.Unlock()
		return assign(dest, value)
	}
	r.mtx.Unlock()

	// go back to regular routine
	return r.Repository.Get(dest, g)
}

// Mock simply return a new value to provided Grabber
func (r *RepositoryWithMock) Mock(g Grabber, val interface{}) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	if _, ok := r.mocked[g]; ok {
		return ErrAlreadyMocked
	}

	r.mocked[g] = val
	return nil
}

// Mock returns a Repository with Mock capability
func Mock() *RepositoryWithMock {
	return &RepositoryWithMock{
		Repository: New(),
		mocked:     make(map[Grabber]interface{}, 0),
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
