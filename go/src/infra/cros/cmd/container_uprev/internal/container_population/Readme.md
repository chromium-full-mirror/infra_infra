# Container Population

Some containers need to have extra values populated into their directories.
These containers should define a function within this folder with the function definition that follows:
```go
type populator = func(ctx context.Context, dir string) error
```