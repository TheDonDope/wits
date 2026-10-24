package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListensOnThisMachineOnly(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:7341", "localhost:7341", "[::1]:7341", "127.0.0.2:80"} {
		assert.NoError(t, loopbackOnly(addr), addr)
	}
	for _, addr := range []string{"0.0.0.0:7341", ":7341", "192.168.1.20:7341", "[::]:7341", "wits.local:7341"} {
		assert.Error(t, loopbackOnly(addr), "%s would answer the network", addr)
	}
	assert.Error(t, loopbackOnly("no-port"), "an address without a port")
}

func TestRunRefusesAnOpenAddress(t *testing.T) {
	assert.ErrorContains(t, run([]string{"--addr", "0.0.0.0:7341"}), "this machine only")
}
