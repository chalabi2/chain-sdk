package v2beta1

import (
	sdkmath "cosmossdk.io/math"
)

const (
	GatewayVersion = "v2beta1"
)

func (m QueryLeasesResponse) TotalPriceAmount() sdkmath.LegacyDec {
	total := sdkmath.LegacyNewDec(0)

	for _, lease := range m.Leases {
		total = total.Add(lease.Lease.Price.Amount)
	}

	return total
}
