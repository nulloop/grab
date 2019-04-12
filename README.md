# Grab

Yet another dependency injection library,

## Features

- Simple and minimal API foot prints
- detect circular dependency at compile time
- small usage of `reflect` package
- provide a functional way to construct complex objects
- provide a better singleton pattern
- supports both `struct` and `interface` injection
- use only standard library
- supports name module for mocking

## Example

Let's say we have a `Config` struct and `Database` struct. We want to initialize and get `Database` object

```go
type Config struct {
  Database struct {
    URL       string
    Username  string
    Password  string
  }
}

type Database struct {
  DB *Conn // Conn is a defined somewhere else
}
```

in order to use `grab` library to load both of them we need to implement the `grab.Grabber` interface. This
interface has only one method and because of that we can use a helper function, `grab.Func` , to build that interface.

```go
var grabConfig = grab.Func(func(c grab.Container) (interface{}, error) {
  file, err := os.Open("/etc/config.conf")
  if err != nil {
    return nil, err
  }
  defer file.Close()

  config := Config{}

  // populated config from loaded file
  // ...

  return &config, nil
})
```

now we need to create another `grab` function for Database. Now we can use `grab.Container` to request for
config object. We simply pass the `grabConfig` object

```go
var grabDatabase = grab.Func(func(c grab.Container) (interface{}, error) {
  var config *Config

  err := c.Get(&config, grabConfig)
  if err != nil {
    return nil, err
  }

  // now at this time, we have config object
  // we can create a connection
  conn, err := Postgres.OpenConnection(config.Database.URL)
  if err != nil {
    return nil, err
  }

  return &Database{
    conn: conn,
  }, nil
})

```

At this point all the builders for both `Config` and `Database` objects have been defined.
Let's see how we can use them in our main application.

```go
package main

import (
  "github.com/nulloop/grab"
)

func main() {
  // first need to create a main container
  container := grab.New()

  // second we simply pass the grabDatabase to Get function
  var db *Database
  err := container.Get(&db, grabDatabase)
  if err != nil {
    panic(err)
  }

  // at this point of program, db has been initialized properly and we can
  // use it here.
}
```
