package edgerequest

import (
	"context"
	"net/http"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicIPRejectsReservedRanges(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "192.0.2.1", "198.18.0.1", "::1", "::ffff:127.0.0.1", "fc00::1", "2001:db8::1", "2002:7f00:1::"} {
		t.Run(address, func(t *testing.T) { require.False(t, PublicIP(netip.MustParseAddr(address))) })
	}
	require.True(t, PublicIP(netip.MustParseAddr("1.1.1.1")))
	require.True(t, PublicIP(netip.MustParseAddr("2606:4700:4700::1111")))
}

type mixedResolver struct{}

func (mixedResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("127.0.0.1")}, nil
}

func TestExecuteRejectsMixedPublicAndPrivateDNS(t *testing.T) {
	// Given a hostname with both public and private addresses.
	client, auth := wireClient(t, func(http.ResponseWriter, *http.Request) { t.Error("private DNS dispatched") })
	client.config.Resolver = mixedResolver{}
	// When validating the target before any authorization.
	_, err := client.Execute(t.Context(), probe())
	// Then all addresses must be public.
	var failure *Error
	require.ErrorAs(t, err, &failure)
	require.Equal(t, Security, failure.Kind)
	require.Zero(t, auth.calls.Load())
}
