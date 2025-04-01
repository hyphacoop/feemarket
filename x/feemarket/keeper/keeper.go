package keeper

import (
	"fmt"
	"strconv"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/skip-mev/feemarket/x/feemarket/types"
)

// Keeper is the x/feemarket keeper.
type Keeper struct {
	cdc      codec.BinaryCodec
	storeKey storetypes.StoreKey
	ak       types.AccountKeeper
	resolver types.DenomResolver

	paramsPool *types.MessagePool[*types.Params]
	statePool  *types.MessagePool[*types.State]

	// The address that is capable of executing a MsgParams message.
	// Typically, this will be the governance module's address.
	authority string
}

// NewKeeper constructs a new feemarket keeper.
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	authKeeper types.AccountKeeper,
	resolver types.DenomResolver,
	authority string,
) *Keeper {
	if _, err := sdk.AccAddressFromBech32(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address: %s", authority))
	}

	k := &Keeper{
		cdc:       cdc,
		storeKey:  storeKey,
		ak:        authKeeper,
		resolver:  resolver,
		authority: authority,
		paramsPool: types.NewMessagePool(func() *types.Params {
			return &types.Params{
				Alpha:           sdkmath.LegacyOneDec(),
				Beta:            sdkmath.LegacyOneDec(),
				Gamma:           sdkmath.LegacyOneDec(),
				Delta:           sdkmath.LegacyOneDec(),
				MinBaseGasPrice: sdkmath.LegacyOneDec(),
				MinLearningRate: sdkmath.LegacyOneDec(),
				MaxLearningRate: sdkmath.LegacyOneDec(),
			}
		}),
		statePool: types.NewMessagePool(func() *types.State {
			return &types.State{
				BaseGasPrice: sdkmath.LegacyZeroDec(),
				LearningRate: sdkmath.LegacyZeroDec(),
				Window:       make([]uint64, 0, 10),
			}
		}),
	}

	return k
}

// Logger returns a feemarket module-specific logger.
func (k *Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", "x/"+types.ModuleName)
}

// GetAuthority returns the address that is capable of executing a MsgUpdateParams message.
func (k *Keeper) GetAuthority() string {
	return k.authority
}

// GetEnabledHeight returns the height at which the feemarket was enabled.
func (k *Keeper) GetEnabledHeight(ctx sdk.Context) (int64, error) {
	store := ctx.KVStore(k.storeKey)

	key := types.KeyEnabledHeight
	bz := store.Get(key)
	if bz == nil {
		return -1, nil
	}

	return strconv.ParseInt(string(bz), 10, 64)
}

// SetEnabledHeight sets the height at which the feemarket was enabled.
func (k *Keeper) SetEnabledHeight(ctx sdk.Context, height int64) {
	store := ctx.KVStore(k.storeKey)

	bz := []byte(strconv.FormatInt(height, 10))

	store.Set(types.KeyEnabledHeight, bz)
}

// ResolveToDenom converts the given coin to the given denomination.
func (k *Keeper) ResolveToDenom(ctx sdk.Context, coin sdk.DecCoin, denom string) (sdk.DecCoin, error) {
	if k.resolver == nil {
		return sdk.DecCoin{}, types.ErrResolverNotSet
	}

	return k.resolver.ConvertToDenom(ctx, coin, denom)
}

// SetDenomResolver sets the keeper's denom resolver.
func (k *Keeper) SetDenomResolver(resolver types.DenomResolver) {
	k.resolver = resolver
}

// GetState returns the feemarket module's state.
func (k *Keeper) GetState(ctx sdk.Context) (types.State, error) {
	store := ctx.KVStore(k.storeKey)

	key := types.KeyState
	bz := store.Get(key)

	state := types.State{}
	if err := state.Unmarshal(bz); err != nil {
		return types.State{}, err
	}

	return state, nil
}

type pooledKVStore interface {
	storetypes.KVStore
	Release()
}

// GetStateFast returns the feemarket module's state as a pooled message.
// Callers MUST call state.Release() when they are done with the state.
// This method is intended for use in hot paths (e.g. ante/post handlers).
func (k *Keeper) GetStateFast(ctx sdk.Context) (types.PooledMessage[*types.State], error) {
	store := ctx.KVStore(k.storeKey)
	if store, ok := store.(pooledKVStore); ok {
		defer store.Release()
	}

	key := types.KeyState
	bz := store.Get(key)

	state := k.statePool.Get()
	// clear out the window
	state.Value.Window = state.Value.Window[:0]
	if err := state.Value.Unmarshal(bz); err != nil {
		state.Release()
		return types.PooledMessage[*types.State]{}, err
	}

	return state, nil
}

// SetState sets the feemarket module's state.
func (k *Keeper) SetState(ctx sdk.Context, state types.State) error {
	store := ctx.KVStore(k.storeKey)

	bz, err := state.Marshal()
	if err != nil {
		return err
	}

	store.Set(types.KeyState, bz)

	return nil
}

// GetParams returns the feemarket module's parameters.
func (k *Keeper) GetParams(ctx sdk.Context) (types.Params, error) {
	store := ctx.KVStore(k.storeKey)

	key := types.KeyParams
	bz := store.Get(key)

	params := types.Params{}
	if err := params.Unmarshal(bz); err != nil {
		return types.Params{}, err
	}

	return params, nil
}

// GetParamsFast returns the feemarket module's parameters as a pooled message.
// Callers MUST call params.Release() when they are done with the parameters.
// This method is intended for use in hot paths (e.g. ante/post handlers).
func (k *Keeper) GetParamsFast(ctx sdk.Context) (types.PooledMessage[*types.Params], error) {
	store := ctx.KVStore(k.storeKey)
	if store, ok := store.(pooledKVStore); ok {
		defer store.Release()
	}

	key := types.KeyParams
	bz := store.Get(key)

	params := k.paramsPool.Get()
	if err := params.Value.Unmarshal(bz); err != nil {
		params.Release()
		return types.PooledMessage[*types.Params]{}, err
	}

	return params, nil
}

// SetParams sets the feemarket module's parameters.
func (k *Keeper) SetParams(ctx sdk.Context, params types.Params) error {
	store := ctx.KVStore(k.storeKey)

	bz, err := params.Marshal()
	if err != nil {
		return err
	}

	store.Set(types.KeyParams, bz)

	return nil
}
