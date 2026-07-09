package client

import (
	"pkg.akt.dev/go/node/client/v1beta4"
)

type Client interface {
	v1beta4.Client
}

type LightClient interface {
	v1beta4.LightClient
}

type QueryClient interface {
	v1beta4.QueryClient
}
