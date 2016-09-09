Package `schematypes`
====================
[![Build Status](https://travis-ci.org/taskcluster/go-schematypes.svg?branch=master)](https://travis-ci.org/taskcluster/go-schematypes)

Package schematypes provides types for constructing JSON schemas
programmatically. This is useful when having a plugin architecture that
accepts JSON matching a given schema as input. As this will allow nesting
of plugins.

**Example** using an integer, works the same for objects and arrays.
```go

import (
  "encoding/json"
  "fmt"

  "github.com/taskcluster/go-schematypes"
)

func main() {
  // Define a schema
  schema := schematypes.Integer{
    MetaData: schematypes.MetaData{
      Title:       "my-title",
      Description: "my-description",
    },
    Minimum: -240,
    Maximum: 240,
  }

  // Parse JSON
  var data interface{}
  err := json.Unmarshal(data, []byte(`234`))
  if err != nil {
    panic("JSON parsing error")
  }

  // Validate against schema
  err = schema.Validate(data)
  if err != nil {
    panic("JSON didn't validate against schema")
  }

  // Map parse data without casting, this is particularly useful for JSON
  // structures as they can be mapped to structs with the `json:"..."` tag.
  var result int
  err = schema.Map(&result, data)
  if err != nil {
    panic("JSON validation failed, or type given to map doesn't match the schema")
  }

  // Now we can use the value we mapped out
  fmt.Println(result)
}

```

License
=======

This Source Code Form is subject to the terms of the Mozilla Public
License, v. 2.0. If a copy of the MPL was not distributed with this
file, You can obtain one at http://mozilla.org/MPL/2.0/.
