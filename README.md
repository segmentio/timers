# timers [![CircleCI](https://circleci.com/gh/segmentio/timers.svg?style=shield)](https://circleci.com/gh/segmentio/timers) [![Go Report Card](https://goreportcard.com/badge/github.com/segmentio/timers)](https://goreportcard.com/report/github.com/segmentio/timers) [![GoDoc](https://godoc.org/github.com/segmentio/timers?status.svg)](https://godoc.org/github.com/segmentio/timers)

## Motivations

The Go standard library offers good timer management abstractions through the
[time](https://golang.org/pkg/time/) and [context](https://golang.org/pkg/context/)
packages. However those are built as general purpose solution to fit most
programs out there, but aren't designed for very high performance applications,
and can become sources of inefficiencies for high traffic services.

Take as an example the common pattern of using `context.WithTimeout` to acquire
a context that will control the time limit for an HTTP request. The creation of
such context constructs a new timer within the Go runtime, and allocates a few
hundred bytes of memory on the heap. A large portion of the CPU time and memory
allocation now ends up being spent on creating those timers which in most cases
will never fire since the normal behavior is often for the request to succeed
and not to timeout.

This is where the `timers` package come in play, offering timing management
abstractions which are both compatible with code built on top of the standard
library and designed for efficiency.

## Timelines

Timelines are a key abstraction for efficient timer management. They expose APIs
to create background contexts that expire on a defined deadline, but instead of
creating a new context, it shares contexts that are expire within a same time
window. This means that concurrent operations which are intended to expire at
roughly the same time do not need to create and manage their own context, they
can share one that a timeline has already set for expiration near their own
deadline.

The trade off is on the accuracy of the expirations, when creating a new context
the runtime will try its best to expire it exactly at the time it was set for.
A Timeline on the other hand will use a configurable resolution to group
expiration times together under a single timer.
There are use cases where a program may want to get timers that are as accurate
as possible, but often times (and especially to manage request timeouts) the
program will have no issues dealing with a 10 seconds timeout which triggered
after 11 seconds instead of 10.
