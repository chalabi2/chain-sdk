package v1

// leaseResizeRejectBase is the first value of the provider rejection range.
const leaseResizeRejectBase = LeaseResizeRejectCapacity

// IsProviderReject reports whether the reason is one a provider may give in
// MsgRejectLeaseResize.
func (r LeaseResizeCancelReason) IsProviderReject() bool {
	_, known := LeaseResizeCancelReason_name[int32(r)]
	return known && r >= leaseResizeRejectBase
}
