package pool

import (
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrClosed is the error resulting if the pool is closed via pool.Close().
	ErrClosed = errors.New("pool is closed")
)

// Pool common connection pool
type Pool[T any] struct {
	// New create connection function
	New func() (T, error)
	// Ping check connection is ok
	Ping func(T) bool
	// Close close connection
	Close     func(T)
	connStore chan T
	mu        sync.Mutex
}

// New create a pool with capacity
func New[T any](initCap, maxCap int, newFunc func() (T, error)) (*Pool[T], error) {
	if maxCap == 0 || initCap > maxCap {
		return nil, fmt.Errorf("invalid capacity settings")
	}
	p := new(Pool[T])
	p.connStore = make(chan T, maxCap)
	if newFunc != nil {
		p.New = newFunc
	}
	for i := 0; i < initCap; i++ {
		conn, err := p.create()
		if err != nil {
			return p, err
		}
		p.connStore <- conn
	}
	return p, nil
}

// Len returns current connections in pool
func (p *Pool[T]) Len() int {
	return len(p.connStore)
}

// Get returns a conn form store or create one
func (p *Pool[T]) Get() (conn T, err error) {
	if p.connStore == nil {
		// pool aleardy destroyed, returns error
		return conn, ErrClosed
	}

	for {
		select {
		case v := <-p.connStore:
			if p.Ping != nil && !p.Ping(v) {
				continue
			}
			return v, nil
		default:
			// pool is empty, returns new connection
			return p.create()
		}
	}
}

// Put set back conn into store again
func (p *Pool[T]) Put(conn T) {
	select {
	case p.connStore <- conn:
		return
	default:
		// pool is full, close passed connection
		if p.Close != nil {
			p.Close(conn)
		}
		return
	}
}

// Destroy clear all connections
func (p *Pool[T]) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.connStore == nil {
		// pool aleardy destroyed
		return
	}
	close(p.connStore)
	for v := range p.connStore {
		if p.Close != nil {
			p.Close(v)
		}
	}
	p.connStore = nil
}

func (p *Pool[T]) create() (conn T, err error) {
	if p.New == nil {
		return conn, fmt.Errorf("Pool.New is nil, can not create connection")
	}
	return p.New()
}
