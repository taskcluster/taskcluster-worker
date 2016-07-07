Golang Import Sub-tree
======================

The `go-import-subtree` utility is designed to be used with `go generate` to
import all packages in a sub-folder for side-effect imports.

Using [github.com/progrium/go-extpoints](https://github.com/progrium/go-extpoints)
you can have a project with a `plugins/` folder, where plugins register
themselves using go-extpoints as a side-effect of being imported.

When combined with `go-import-subtree` you can import all packages in the
`plugins/` folder automatically. Freeing you from maintaining a file importing
all your plugins, just run 'go generate'.

See the `example/` for a simple example.
