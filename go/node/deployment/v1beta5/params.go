package v1beta5

import (
	"fmt"
	"math"
	"time"

	"cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"

	v1 "pkg.akt.dev/go/node/deployment/v1"
	"pkg.akt.dev/go/node/types/unit"
)

var _ paramtypes.ParamSet = (*Params)(nil)

var (
	// DefaultMaxVolumeSize matches the compiled per-unit storage cap the
	// param replaces for volume groups.
	DefaultMaxVolumeSize uint64 = 32 * unit.Ti

	DefaultMaxVolumeRetention = 720 * time.Hour // 30 days

	DefaultMaxVolumeReplicas uint32 = 4
)

const (
	keyMinDeposits        = "MinDeposits"
	keyMaxVolumeSize      = "MaxVolumeSize"
	keyMaxVolumeRetention = "MaxVolumeRetention"
	keyMaxVolumeReplicas  = "MaxVolumeReplicas"
)

func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair([]byte(keyMinDeposits), &p.MinDeposits, validateMinDeposits),
		paramtypes.NewParamSetPair([]byte(keyMaxVolumeSize), &p.MaxVolumeSize, validateMaxVolumeSize),
		paramtypes.NewParamSetPair([]byte(keyMaxVolumeRetention), &p.MaxVolumeRetention, validateMaxVolumeRetention),
		paramtypes.NewParamSetPair([]byte(keyMaxVolumeReplicas), &p.MaxVolumeReplicas, validateMaxVolumeReplicas),
	}
}

func DefaultParams() Params {
	return Params{
		MinDeposits: sdk.Coins{
			sdk.NewCoin("uakt", sdkmath.NewInt(500000)),
			sdk.NewCoin("uact", sdkmath.NewInt(500000)),
		},
		MaxVolumeSize:      DefaultMaxVolumeSize,
		MaxVolumeRetention: DefaultMaxVolumeRetention,
		MaxVolumeReplicas:  DefaultMaxVolumeReplicas,
	}
}

func (p Params) Validate() error {
	if err := validateMinDeposits(p.MinDeposits); err != nil {
		return err
	}
	if err := validateMaxVolumeSize(p.MaxVolumeSize); err != nil {
		return err
	}
	if err := validateMaxVolumeRetention(p.MaxVolumeRetention); err != nil {
		return err
	}
	if err := validateMaxVolumeReplicas(p.MaxVolumeReplicas); err != nil {
		return err
	}
	return nil
}

func (p Params) ValidateDeposit(amt sdk.Coin) error {
	minDeposit, err := p.MinDepositFor(amt.Denom)

	if err != nil {
		return err
	}

	if amt.IsGTE(minDeposit) {
		return nil
	}

	return errors.Wrapf(v1.ErrInvalidDeposit, "Deposit too low - %v < %v", amt.Amount, minDeposit)
}

func (p Params) MinDepositFor(denom string) (sdk.Coin, error) {
	for _, minDeposit := range p.MinDeposits {
		if minDeposit.Denom == denom {
			return sdk.NewCoin(minDeposit.Denom, minDeposit.Amount), nil
		}
	}

	return sdk.NewInt64Coin(denom, math.MaxInt64), fmt.Errorf("%w: Invalid deposit denomination %v", v1.ErrInvalidDeposit, denom)
}

func validateMinDeposits(i interface{}) error {
	vals, ok := i.(sdk.Coins)
	if !ok {
		return fmt.Errorf("%w: Min Deposits - invalid type: %T", v1.ErrInvalidParam, i)
	}

	check := make(map[string]bool)

	for _, minDeposit := range vals {
		if _, exists := check[minDeposit.Denom]; exists {
			return fmt.Errorf("duplicate Min Deposit for denom (%#v)", minDeposit)
		}

		check[minDeposit.Denom] = true

		if minDeposit.Amount.Uint64() >= math.MaxInt32 {
			return fmt.Errorf("%w: Min Deposit (%v) - too large: %v", v1.ErrInvalidParam, minDeposit.Denom, minDeposit.Amount.Uint64())
		}
	}

	if _, exists := check["uact"]; !exists {
		return fmt.Errorf("%w: Min Deposits - uact not given: %#v", v1.ErrInvalidParam, vals)
	}

	return nil
}

func validateMaxVolumeSize(i interface{}) error {
	val, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("%w: Max Volume Size - invalid type: %T", v1.ErrInvalidParam, i)
	}

	if val == 0 {
		return fmt.Errorf("%w: Max Volume Size must be > 0", v1.ErrInvalidParam)
	}

	return nil
}

func validateMaxVolumeRetention(i interface{}) error {
	val, ok := i.(time.Duration)
	if !ok {
		return fmt.Errorf("%w: Max Volume Retention - invalid type: %T", v1.ErrInvalidParam, i)
	}

	if val < 0 {
		return fmt.Errorf("%w: Max Volume Retention must be >= 0", v1.ErrInvalidParam)
	}

	return nil
}

func validateMaxVolumeReplicas(i interface{}) error {
	_, ok := i.(uint32)
	if !ok {
		return fmt.Errorf("%w: Max Volume Replicas - invalid type: %T", v1.ErrInvalidParam, i)
	}

	return nil
}
