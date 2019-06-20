package grab_test

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/nulloop/grab/v2"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}

type speak interface {
	Say() string
}

type dummy struct {
	name string
}

func (d *dummy) Say() string {
	return "dummy:" + d.name
}

var grabDummy = grab.Func(func(c grab.Container) (interface{}, error) {
	return &dummy{
		name: randString(5),
	}, nil
})

type foo struct {
	name string
}

func (f *foo) Say() string {
	return "foo:" + f.name
}

var grabFoo = grab.Func(func(c grab.Container) (interface{}, error) {
	return &foo{
		name: "hello world",
	}, nil
})

type bar struct {
	name  string
	speak speak
}

func (b *bar) Say() string {
	return b.name + b.speak.Say()
}

var grabBar = grab.Func(func(c grab.Container) (interface{}, error) {
	var speak speak

	err := c.Get(&speak, grabFoo)
	if err != nil {
		return nil, err
	}

	return &bar{
		name:  "bar:",
		speak: speak,
	}, nil
})

func TestGrabSingleton(t *testing.T) {
	container := grab.New()

	var dummy1 *dummy
	var dummy2 *dummy

	err := container.Get(&dummy1, grabDummy)
	if err != nil {
		t.Fatal(err)
	}
	err = container.Get(&dummy2, grabDummy)
	if err != nil {
		t.Fatal(err)
	}

	if dummy1.Say() != dummy2.Say() {
		t.Fatal("container not grabbing the same instance")
	}
}

func TestMultipleDependency(t *testing.T) {
	container := grab.New()

	var s1 speak

	err := container.Get(&s1, grabBar)
	if err != nil {
		t.Fatal(err)
	}

	expected := "bar:foo:hello world"

	if s1.Say() != expected {
		t.Fatalf("expected '%s' but got '%s'", expected, s1.Say())
	}
}

func TestSrcMustPointer(t *testing.T) {
	container := grab.New()

	var s1 *bar
	err := container.Get(s1, grabBar)
	if err != grab.ErrDestMustBeDoublePointer {
		t.Fatalf("expected double pointer error but got %s", err)
	}

	var s2 speak
	err = container.Get(s2, grabDummy)
	if err != grab.ErrDestInterfaceMustBePointer {
		t.Fatalf("expected interface pointer error but got %s", err)
	}

	var s3 bar
	err = container.Get(s3, grabBar)
	if err != grab.ErrDestMustBePointer {
		t.Fatalf("expected pointer error but got %s", err)
	}

	grabTest := grab.Func(func(c grab.Container) (interface{}, error) {
		return bar{}, nil
	})

	var s4 *bar
	err = container.Get(&s4, grabTest)
	if err != grab.ErrSrcMustBePointer {
		t.Fatalf("expected pointer as src error but got %s", err)
	}
}

func TestInitializationFails(t *testing.T) {
	container := grab.New()

	errOops := errors.New("Oops")

	var s1 speak
	grabTest := grab.Func(func(c grab.Container) (interface{}, error) {
		return nil, errOops
	})

	err := container.Get(&s1, grabTest)
	if err != errOops {
		t.Fatalf("expected oops error but got %s", err)
	}
}

type MockTestData struct {
	Name string
}

func TestMockDependency(t *testing.T) {
	container := grab.New()

	GrabTest := grab.Func(func(c grab.Container) (interface{}, error) {
		return &MockTestData{
			Name: "Bye",
		}, nil
	})

	err := container.Mock(GrabTest, &MockTestData{
		Name: "Hello",
	})
	if err != nil {
		t.Error(err)
	}

	var val *MockTestData

	err = container.Get(&val, GrabTest)
	if err != nil {
		t.Error(err)
	}

	if val.Name != "Hello" {
		t.Errorf("expected %s but got %s", "Hello", val.Name)
	}

	err = container.Mock(GrabTest, &MockTestData{
		Name: "Hello 2",
	})
	if err != grab.ErrAlreadyMocked {
		t.Errorf("should got '%s' error but got %s", grab.ErrAlreadyMocked, err)
	}
}
