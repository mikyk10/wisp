package finder

import "context"

type PathFinder interface {
	Find(basename ...string) string
}

type PathStreamFinder interface {
	Find(context.Context, chan<- string)
}

type PathCallbackFinder interface {
	Find(func(path string))
}
