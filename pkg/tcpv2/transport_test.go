package tcpv2

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListen(t *testing.T) {
	l, err := Listen("127.0.0.1:0") // Use ephemeral port
	require.NoError(t, err)
	require.NotNil(t, l)
	
	addr := l.Addr()
	require.NotNil(t, addr)
	
	err = l.Close()
	require.NoError(t, err)
}

func TestListener_AcceptAfterClose(t *testing.T) {
	l, err := Listen("127.0.0.1:0")
	require.NoError(t, err)

	// Close listener
	err = l.Close()
	require.NoError(t, err)

	// Accept should return error after close
	_, err = l.Accept()
	require.Error(t, err)
}

func TestListener_DoubleClose(t *testing.T) {
	l, err := Listen("127.0.0.1:0")
	require.NoError(t, err)

	err = l.Close()
	require.NoError(t, err)

	// Second close should be idempotent
	err = l.Close()
	require.NoError(t, err)
}
