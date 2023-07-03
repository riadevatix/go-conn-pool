package pool

import (
	"log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// serverAddr  test tcp server address
var serverAddr = "127.0.0.1:8003"

func TestMain(t *testing.T) {
	var pool *Pool[*net.TCPConn]
	var err error
	var n int
	go tcpServer()
	// wait TCP server start
	time.Sleep(time.Millisecond * 10)

	t.Run("create connection pool", func(t *testing.T) {
		pool, err = New[*net.TCPConn](2, 10, func() (*net.TCPConn, error) {
			addr, _ := net.ResolveTCPAddr("tcp4", serverAddr)
			cli, err := net.DialTCP("tcp4", nil, addr)
			if err != nil {
				return nil, err
				// log.Fatalf("create client connection error: %v", err)
			}
			return cli, nil
		})
		assert.NoError(t, err)
		pool.Ping = func(t *net.TCPConn) bool {
			return true
		}
		pool.Close = func(conn *net.TCPConn) {
			conn.Close()
		}
		assert.Equal(t, pool.Len(), 2)
	})

	t.Run("get connection then put", func(t *testing.T) {
		v, err := pool.Get()
		assert.NoError(t, err)
		n, err = v.Write([]byte("PING"))
		assert.NoError(t, err)
		assert.Equal(t, n, 4)
		re := make([]byte, 4)
		n, err = v.Read(re)
		assert.NoError(t, err)
		assert.Equal(t, n, 4)
		assert.Equal(t, string(re), "PONG")
		assert.Equal(t, pool.Len(), 1)
		pool.Put(v)
		assert.Equal(t, pool.Len(), 2)
	})

	t.Run("get connection reuse then put", func(t *testing.T) {
		v, err := pool.Get()
		assert.NoError(t, err)
		for i := 0; i < 10; i++ {
			n, err = v.Write([]byte("PING"))
			assert.NoError(t, err)
			assert.Equal(t, n, 4)
			re := make([]byte, 4)
			n, err = v.Read(re)
			assert.NoError(t, err)
			assert.Equal(t, n, 4)
			assert.Equal(t, string(re), "PONG")
		}
		assert.Equal(t, pool.Len(), 1)
		pool.Put(v)
		assert.Equal(t, pool.Len(), 2)
	})

	t.Run("get many connections", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			v, err := pool.Get()
			assert.NoError(t, err)
			n, err = v.Write([]byte("PING"))
			assert.NoError(t, err)
			assert.Equal(t, n, 4)
			re := make([]byte, 4)
			n, err = v.Read(re)
			assert.NoError(t, err)
			assert.Equal(t, n, 4)
			assert.Equal(t, string(re), "PONG")
			pool.Put(v)
		}
		assert.Equal(t, pool.Len(), 2)
	})

	t.Run("get overlay connections", func(t *testing.T) {
		conns := make([]*net.TCPConn, 20)
		for i := 0; i < 20; i++ {
			conn, err := pool.Get()
			assert.NoError(t, err)
			n, err = conn.Write([]byte("PING"))
			assert.NoError(t, err)
			assert.Equal(t, n, 4)
			re := make([]byte, 4)
			n, err = conn.Read(re)
			assert.NoError(t, err)
			assert.Equal(t, n, 4)
			assert.Equal(t, string(re), "PONG")
			conns[i] = conn
		}
		for _, cli := range conns {
			pool.Put(cli)
		}
		assert.Equal(t, pool.Len(), 10)
	})

	t.Run("get connection and no back", func(t *testing.T) {
		v, err := pool.Get()
		assert.NoError(t, err)
		n, err = v.Write([]byte("PING"))
		assert.NoError(t, err)
		assert.Equal(t, n, 4)
		re := make([]byte, 4)
		n, err = v.Read(re)
		assert.NoError(t, err)
		assert.Equal(t, n, 4)
		assert.Equal(t, string(re), "PONG")
		assert.Equal(t, pool.Len(), 9)
	})

	t.Run("destroy connection pool", func(t *testing.T) {
		pool.Destroy()
		assert.Equal(t, pool.Len(), 0)
	})

	t.Run("get connection after destroy", func(t *testing.T) {
		v, err := pool.Get()
		assert.Error(t, err)
		assert.Nil(t, v)
	})
}

func tcpServer() error {
	ln, err := net.Listen("tcp4", serverAddr)
	if err != nil {
		log.Fatalf("test server start error: %v", err)
	}
	var connNum int
	for {
		conn, err := ln.Accept()
		connNum++
		//log.Printf("\n->accept new connection %v, now has %d connections\n", conn.RemoteAddr(), connNum)
		if err != nil {
			log.Printf("test server accept error: %v", err)
			continue
		}
		go func(conn net.Conn) {
			for {
				re := make([]byte, 4)
				n, err := conn.Read(re)
				if err == nil && n == 4 {
					conn.Write([]byte("PONG"))
				}
			}
		}(conn)
	}
}
