// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// StakingUnbondingEntry is an auto generated low-level Go binding around an user-defined struct.
type StakingUnbondingEntry struct {
	Amount          *big.Int
	CompletionBlock *big.Int
}

// StakingMetaData contains all meta data concerning the Staking contract.
var StakingMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"COMMISSION_RATE_BASE\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"CONSENSUS_MAX_VALIDATORS\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"MAX_UNBONDING_ENTRIES\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"PROPOSAL_ADDR\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"PUNISH_ADDR\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"STAKING_ADDR\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"VALIDATOR_ADDR\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"addValidatorStake\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"allValidators\",\"inputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"claimRewards\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"claimValidatorRewards\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"currentEpochId\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"decreaseValidatorStake\",\"inputs\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"delegate\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"delegations\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"rewardDebt\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"delegatorCount\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"delegatorExists\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"delegatorValidatorCount\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"distributeRewards\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"epoch\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"exitValidator\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"getDelegationInfo\",\"inputs\":[{\"name\":\"delegator\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"validator\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"pendingRewards\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getTopValidators\",\"inputs\":[{\"name\":\"validators\",\"type\":\"address[]\",\"internalType\":\"address[]\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address[]\",\"internalType\":\"address[]\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getUnbondingEntries\",\"inputs\":[{\"name\":\"delegator\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"validator\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"tuple[]\",\"internalType\":\"structStaking.UnbondingEntry[]\",\"components\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"completionBlock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getUnbondingEntriesCount\",\"inputs\":[{\"name\":\"delegator\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"validator\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getValidatorCount\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getValidatorInfo\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"selfStake\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"totalDelegated\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"commissionRate\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"accumulatedRewards\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"isJailed\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"jailUntilBlock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"totalClaimedRewards\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"lastClaimBlock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"isRegistered\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"totalRewards\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getValidatorStatus\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"isActive\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"isJailed\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"initialize\",\"inputs\":[{\"name\":\"validators_\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"proposal_\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"punish_\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"initializeWithValidators\",\"inputs\":[{\"name\":\"validators_\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"proposal_\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"punish_\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"initialValidators\",\"type\":\"address[]\",\"internalType\":\"address[]\"},{\"name\":\"commissionRate\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"initialized\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"isValidatorJailed\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"jailValidator\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"jailBlocks\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"lastActiveBlock\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"lastCommissionUpdateBlock\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"pendingPayouts\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"proposalContract\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractIProposal\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"punishContract\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractIPunish\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"registerValidator\",\"inputs\":[{\"name\":\"commissionRate\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"reinitializeV2\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"resignValidator\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"revision\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"rewardPerShare\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"slashValidator\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"slashAmount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"reporter\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"rewardAmount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"burnAddress\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"actualSlash\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"actualReward\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"totalStaked\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"unbondingDelegations\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"completionBlock\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"undelegate\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"unjailValidator\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"updateCommissionRate\",\"inputs\":[{\"name\":\"newCommissionRate\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"updateLastActiveBlock\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"validatorDelegatorCount\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"validatorStakes\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"selfStake\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"totalDelegated\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"commissionRate\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"totalRewards\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"accumulatedRewards\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"isJailed\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"jailUntilBlock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"totalClaimedRewards\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"lastClaimBlock\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"isRegistered\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"validatorsAddedInEpoch\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"validatorsContract\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractIValidators\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"validatorsRemovedInEpoch\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"withdrawPendingPayout\",\"inputs\":[{\"name\":\"recipient\",\"type\":\"address\",\"internalType\":\"addresspayable\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"withdrawUnbonded\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"maxEntries\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"CommissionRateUpdated\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"commissionRate\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Delegated\",\"inputs\":[{\"name\":\"delegator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"validator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"PendingPayoutQueued\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"PendingPayoutWithdrawn\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"recipient\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RewardsClaimed\",\"inputs\":[{\"name\":\"delegator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"validator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RewardsDistributed\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"UnbondingCompleted\",\"inputs\":[{\"name\":\"delegator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"validator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Undelegated\",\"inputs\":[{\"name\":\"delegator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"validator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"ValidatorExited\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"ValidatorJailed\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"jailUntilBlock\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"ValidatorRegistered\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"selfStake\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"commissionRate\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"ValidatorSlashed\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"slashAmount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"rewardAmount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"burnAmount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"reporter\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"burnAddress\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"ValidatorStakeDecreased\",\"inputs\":[{\"name\":\"delegator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"validator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"ValidatorStakeIncreased\",\"inputs\":[{\"name\":\"delegator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"validator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"ValidatorUnjailed\",\"inputs\":[{\"name\":\"validator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"ReentrancyGuardReentrantCall\",\"inputs\":[]}]",
	Bin: "0x6080604052348015600e575f5ffd5b5060017f9b779b17422d0df92223018b32b4d1fa46e071723d6817e2486d003becc55f0055615b02806100405f395ff3fe608060405260043610610365575f3560e01c80637cafdd79116101c8578063b4669217116100fd578063c64814dd1161009d578063ef5cfb8c1161006d578063ef5cfb8c14610b16578063f29efef614610b35578063fb290af914610b60578063ff6061b214610ba4575f5ffd5b8063c64814dd14610a7a578063e50a7db814610ab7578063e691e8f014610ae2578063eacdc5ff14610b01575f5ffd5b8063be22c64c116100d8578063be22c64c14610a1e578063bee8380e14610a32578063c0c53b8b14610a47578063c411587414610a66575f5ffd5b8063b4669217146109d7578063b85f5da2146109eb578063bcecf81b146109ff575f5ffd5b806396902c8211610168578063a310624f11610143578063a310624f146108a3578063a41fcdc3146108d9578063aa735578146108ed578063b43b695b146109ac575f5ffd5b806396902c82146108505780639abd63051461086f5780639f759dba1461088e575f5ffd5b80638a11d7c9116101a35780638a11d7c9146107565780638c872d051461081e578063900cf0cf1461083357806395e0266914610848575f5ffd5b80637cafdd791461070d5780637cc963801461072c578063817b1cd214610741575f5ffd5b8063531e05411161029e5780636f4a2cd01161023e57806372c44ba11161021957806372c44ba114610681578063764e7feb146106a05780637664fc92146106b4578063784712f2146106e2575f5ffd5b80636f4a2cd0146106465780637071688a1461064e57806371a1bb7514610662575f5ffd5b80635d4f0cb6116102795780635d4f0cb6146105cf57806360731435146105e4578063666183ee1461061e578063679cdb0614610633575f5ffd5b8063531e0541146105725780635a4d66c01461059d5780635c19a95c146105bc575f5ffd5b80633b058e421161030957806346da2a94116102e457806346da2a94146104f45780634815b3411461051f5780634d99dd16146105345780634eb4b3e314610553575f5ffd5b80633b058e42146104a1578063437ccda8146104c057806344f99900146104d5575f5ffd5b80631c4e449a116103445780631c4e449a146103e75780631c7e75e71461041b578063266f55bb146104475780633aef39001461046a575f5ffd5b8062fa3d50146103695780630864662b1461038a578063158ef93e146103bf575b5f5ffd5b348015610374575f5ffd5b50610388610383366004615430565b610bc3565b005b348015610395575f5ffd5b506103a96103a436600461547f565b610dcc565b6040516103b6919061554a565b60405180910390f35b3480156103ca575f5ffd5b505f546103d79060ff1681565b60405190151581526020016103b6565b3480156103f2575f5ffd5b50610406610401366004615595565b611304565b604080519283526020830191909152016103b6565b348015610426575f5ffd5b5061043a6104353660046155f0565b61167d565b6040516103b69190615627565b348015610452575f5ffd5b5061045c60455481565b6040519081526020016103b6565b348015610475575f5ffd5b50603e54610489906001600160a01b031681565b6040516001600160a01b0390911681526020016103b6565b3480156104ac575f5ffd5b506103886104bb36600461566a565b611710565b3480156104cb575f5ffd5b5061048961f01281565b3480156104e0575f5ffd5b50603f54610489906001600160a01b031681565b3480156104ff575f5ffd5b5061045c61050e36600461566a565b60426020525f908152604090205481565b34801561052a575f5ffd5b5061045c60465481565b34801561053f575f5ffd5b5061038861054e36600461568c565b61189d565b34801561055e575f5ffd5b5061040661056d3660046156b6565b611dca565b34801561057d575f5ffd5b5061045c61058c36600461566a565b603b6020525f908152604090205481565b3480156105a8575f5ffd5b506103886105b736600461568c565b611e0e565b6103886105ca36600461566a565b611f22565b3480156105da575f5ffd5b5061048961f01381565b3480156105ef575f5ffd5b506103d76105fe36600461566a565b6001600160a01b03165f9081526034602052604090206005015460ff1690565b348015610629575f5ffd5b5061045c60405481565b610388610641366004615430565b612264565b6103886127be565b348015610659575f5ffd5b5060395461045c565b34801561066d575f5ffd5b50603d54610489906001600160a01b031681565b34801561068c575f5ffd5b5061038861069b3660046156f4565b612a78565b3480156106ab575f5ffd5b5061045c601581565b3480156106bf575f5ffd5b506103d76106ce36600461566a565b60416020525f908152604090205460ff1681565b3480156106ed575f5ffd5b5061045c6106fc36600461566a565b60486020525f908152604090205481565b348015610718575f5ffd5b5061038861072736600461566a565b613005565b348015610737575f5ffd5b5061045c60435481565b34801561074c575f5ffd5b5061045c603a5481565b348015610761575f5ffd5b506107d261077036600461566a565b6001600160a01b03165f908152603460205260409020805460018201546002830154600484015460058501546006860154600787015460088801546009890154600390990154979996989597949660ff94851696939592949193919091169190565b604080519a8b5260208b0199909952978901969096526060880194909452911515608087015260a086015260c085015260e08401521515610100830152610120820152610140016103b6565b348015610829575f5ffd5b5061048961f01181565b34801561083e575f5ffd5b5061045c60015481565b61038861342c565b34801561085b575f5ffd5b5061038861086a36600461568c565b613505565b34801561087a575f5ffd5b506104066108893660046155f0565b613811565b348015610899575f5ffd5b5061048961f01081565b3480156108ae575f5ffd5b506108c26108bd36600461566a565b613898565b6040805192151583529015156020830152016103b6565b3480156108e4575f5ffd5b5061045c601481565b3480156108f8575f5ffd5b5061095e61090736600461566a565b60346020525f908152604090208054600182015460028301546003840154600485015460058601546006870154600788015460088901546009909901549798969795969495939460ff93841694929391929091168a565b604080519a8b5260208b01999099529789019690965260608801949094526080870192909252151560a086015260c085015260e08401526101008301521515610120820152610140016103b6565b3480156109b7575f5ffd5b5061045c6109c636600461566a565b60366020525f908152604090205481565b3480156109e2575f5ffd5b5061038861392c565b3480156109f6575f5ffd5b50610388613d5c565b348015610a0a575f5ffd5b50610489610a19366004615430565b613fee565b348015610a29575f5ffd5b50610388614016565b348015610a3d575f5ffd5b5061045c61271081565b348015610a52575f5ffd5b50610388610a613660046157a4565b6141cd565b348015610a71575f5ffd5b5061038861440d565b348015610a85575f5ffd5b50610406610a943660046155f0565b603760209081525f92835260408084209091529082529020805460019091015482565b348015610ac2575f5ffd5b5061045c610ad136600461566a565b60356020525f908152604090205481565b348015610aed575f5ffd5b50610388610afc36600461566a565b61446e565b348015610b0c575f5ffd5b5061045c60445481565b348015610b21575f5ffd5b50610388610b3036600461566a565b6144b7565b348015610b40575f5ffd5b5061045c610b4f36600461566a565b60476020525f908152604090205481565b348015610b6b575f5ffd5b5061045c610b7a3660046155f0565b6001600160a01b039182165f90815260386020908152604080832093909416825291909152205490565b348015610baf575f5ffd5b50610388610bbe366004615430565b6145e5565b33610bcd81614884565b610bd5614990565b5f8211610bfd5760405162461bcd60e51b8152600401610bf4906157ec565b60405180910390fd5b603e5f9054906101000a90046001600160a01b03166001600160a01b031663c673316c6040518163ffffffff1660e01b8152600401602060405180830381865afa158015610c4d573d5f5f3e3d5ffd5b505050506040513d601f19601f82011682018060405250810190610c719190615832565b821115610c905760405162461bcd60e51b8152600401610bf490615849565b603e5460408051635b6a61f760e01b815290515f926001600160a01b031691635b6a61f79160048083019260209291908290030181865afa158015610cd7573d5f5f3e3d5ffd5b505050506040513d601f19601f82011682018060405250810190610cfb9190615832565b335f908152603660205260409020549091508015610d6c57610d1d82826158a4565b431015610d6c5760405162461bcd60e51b815260206004820152601e60248201527f436f6d6d697373696f6e2075706461746520746f6f206672657175656e7400006044820152606401610bf4565b335f818152603660209081526040808320439055603482529182902060020187905590518681527f86d576c20e383fc2413ef692209cc48ddad5e52f25db5b32f8f7ec5076461ae9910160405180910390a25050610dc86149be565b5050565b606081515f03610de9575050604080515f81526020810190915290565b5f825167ffffffffffffffff811115610e0457610e04615447565b604051908082528060200260200182016040528015610e2d578160200160208202803683370190505b5090505f835167ffffffffffffffff811115610e4b57610e4b615447565b604051908082528060200260200182016040528015610e74578160200160208202803683370190505b5090505f805b8551811015610f36575f868281518110610e9657610e966158b7565b6020908102919091018101516001600160a01b0381165f9081526034909252604090912080549192509015610f2c5781868581518110610ed857610ed86158b7565b6001600160a01b039092166020928302919091019091015260018101548154610f0191906158a4565b858581518110610f1357610f136158b7565b602090810291909101015283610f28816158cb565b9450505b5050600101610e7a565b50805f03610f55575050604080515f8152602081019091529392505050565b5f610f616002836158f7565b90505b8015610f9257610f80848484610f7b60018661590a565b6149e4565b80610f8a8161591d565b915050610f64565b505f610f9f60018361590a565b90505b80156110af57838181518110610fba57610fba6158b7565b6020026020010151845f81518110610fd457610fd46158b7565b6020026020010151855f81518110610fee57610fee6158b7565b60200260200101868481518110611007576110076158b7565b6001600160a01b039384166020918202929092010152911690528251839082908110611035576110356158b7565b6020026020010151835f8151811061104f5761104f6158b7565b6020026020010151845f81518110611069576110696158b7565b60200260200101858481518110611082576110826158b7565b60209081029190910101919091525261109d8484835f6149e4565b806110a78161591d565b915050610fa2565b505f5b6110bd6002836158f7565b8110156111cf575f816110d160018561590a565b6110db919061590a565b90508481815181106110ef576110ef6158b7565b6020026020010151858381518110611109576111096158b7565b6020026020010151868481518110611123576111236158b7565b6020026020010187848151811061113c5761113c6158b7565b6001600160a01b03938416602091820292909201015291169052835184908290811061116a5761116a6158b7565b6020026020010151848381518110611184576111846158b7565b602002602001015185848151811061119e5761119e6158b7565b602002602001018684815181106111b7576111b76158b7565b602090810291909101019190915252506001016110b2565b50603e5460408051630456292b60e11b815290515f926001600160a01b0316916308ac52569160048083019260209291908290030181865afa158015611217573d5f5f3e3d5ffd5b505050506040513d601f19601f8201168201806040525081019061123b9190615832565b9050601581111561124a575060155b5f818310611258578161125a565b825b90505f8167ffffffffffffffff81111561127657611276615447565b60405190808252806020026020018201604052801561129f578160200160208202803683370190505b5090505f5b828110156112f8578681815181106112be576112be6158b7565b60200260200101518282815181106112d8576112d86158b7565b6001600160a01b03909216602092830291909101909101526001016112a4565b50979650505050505050565b5f5f61130e614bae565b611316614990565b6001600160a01b03871661133c5760405162461bcd60e51b8152600401610bf490615932565b5f861161138b5760405162461bcd60e51b815260206004820152601d60248201527f536c61736820616d6f756e74206d75737420626520706f7369746976650000006044820152606401610bf4565b6001600160a01b0383166113d85760405162461bcd60e51b8152602060048201526014602482015273496e76616c6964206275726e206164647265737360601b6044820152606401610bf4565b6001600160a01b0387165f90815260346020526040812080549091819003611407575f5f93509350505061166b565b603d546040516303d76f6f60e31b81526001600160a01b038b811660048301525f921690631ebb7b7890602401602060405180830381865afa15801561144f573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906114739190615969565b9050801561152357603e546040805163017ddd3560e01b815290515f926001600160a01b03169163017ddd359160048083019260209291908290030181865afa1580156114c2573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906114e69190615832565b90508083116114fe575f5f955095505050505061166b565b5f611509828561590a565b9050808b11611518578a61151a565b805b96505050611535565b8189116115305788611532565b815b94505b845f0361154a575f5f9450945050505061166b565b611554858361590a565b8355603a5461156490869061590a565b603a558015801561157457508254155b156115d057603d54604051636a48a57160e11b81526001600160a01b038c811660048301529091169063d4914ae2906024015f604051808303815f87803b1580156115bd575f5ffd5b505af19250505080156115ce575060015b505b8487116115dd57866115df565b845b93505f6115ec858761590a565b90508415611600576115fe8986614bf6565b505b8015611612576116108782614bf6565b505b60408051878152602081018790529081018290526001600160a01b03808916918b8216918e16907f5bd6f9405e6c0a71ad3df5e2e346286287acc47544e763f77889c264066d154a9060600160405180910390a4505050505b6116736149be565b9550959350505050565b6001600160a01b038083165f9081526038602090815260408083209385168352928152828220805484518184028101840190955280855260609493919290919084015b82821015611703578382905f5260205f2090600202016040518060400160405290815f8201548152602001600182015481525050815260200190600101906116c0565b5050505090505b92915050565b611718614990565b6001600160a01b0381166117625760405162461bcd60e51b8152602060048201526011602482015270125b9d985b1a59081c9958da5c1a595b9d607a1b6044820152606401610bf4565b335f90815260486020526040902054806117b25760405162461bcd60e51b8152602060048201526011602482015270139bc81c195b991a5b99c81c185e5bdd5d607a1b6044820152606401610bf4565b335f90815260486020526040808220829055516001600160a01b0384169083908381818185875af1925050503d805f8114611808576040519150601f19603f3d011682016040523d82523d5f602084013e61180d565b606091505b50509050806118505760405162461bcd60e51b815260206004820152600f60248201526e151c985b9cd9995c8819985a5b1959608a1b6044820152606401610bf4565b6040518281526001600160a01b0384169033907f3c00101edd308ddcdda38bff41fc7dc07c50174f055cda38460ff1c389c903059060200160405180910390a3505061189a6149be565b50565b6118a5614cf2565b6118ad614990565b6001600160a01b0382166118d35760405162461bcd60e51b8152600401610bf490615932565b5f81116118f25760405162461bcd60e51b8152600401610bf490615988565b603e5f9054906101000a90046001600160a01b03166001600160a01b031663b2b2732a6040518163ffffffff1660e01b8152600401602060405180830381865afa158015611942573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906119669190615832565b8110156119b55760405162461bcd60e51b815260206004820181905260248201527f496e73756666696369656e7420756e64656c65676174696f6e20616d6f756e746044820152606401610bf4565b335f9081526038602090815260408083206001600160a01b0386168452909152902054601411611a275760405162461bcd60e51b815260206004820152601a60248201527f546f6f206d616e7920756e626f6e64696e6720656e74726965730000000000006044820152606401610bf4565b336001600160a01b03831603611a7f5760405162461bcd60e51b815260206004820152601f60248201527f43616e6e6f7420756e64656c65676174652066726f6d20796f757273656c66006044820152606401610bf4565b335f9081526037602090815260408083206001600160a01b0386168452909152902054811115611af15760405162461bcd60e51b815260206004820152601760248201527f496e73756666696369656e742064656c65676174696f6e0000000000000000006044820152606401610bf4565b5f611afc3384614d86565b335f9081526037602090815260408083206001600160a01b03881684529091528120805492935084831492859290611b3590849061590a565b90915550506001600160a01b0384165f9081526034602052604081206001018054859290611b6490849061590a565b9250508190555082603a5f828254611b7c919061590a565b90915550506001600160a01b0384165f818152603b60209081526040808320543384526037835281842094845293909152902054670de0b6b3a764000091611bc3916159bf565b611bcd91906158f7565b335f9081526037602090815260408083206001600160a01b03891684529091529020600101558015611ca5576001600160a01b0384165f908152604260205260408120805491611c1c8361591d565b9091555050335f908152604760205260409020548015611ca3575f611c4260018361590a565b335f908152604760205260409020819055905080158015611c715750335f9081526041602052604090205460ff165b15611ca15760408054905f611c858361591d565b9091555050335f908152604160205260409020805460ff191690555b505b505b335f9081526038602090815260408083206001600160a01b0388811685529083529281902081518083018352878152603e548351636cf6d67560e01b8152935192959194858101949190921692636cf6d675926004808401939192918290030181865afa158015611d18573d5f5f3e3d5ffd5b505050506040513d601f19601f82011682018060405250810190611d3c9190615832565b611d4690436158a4565b90528154600181810184555f938452602093849020835160029093020191825592909101519101558115611d7f57611d7f338584614e0a565b6040518381526001600160a01b0385169033907f4d10bd049775c77bd7f255195afba5088028ecb3c7c277d393ccff7934f2f92c906020015b60405180910390a35050610dc86149be565b6038602052825f5260405f20602052815f5260405f208181548110611ded575f80fd5b5f918252602090912060029091020180546001909101549093509150839050565b611e16614e78565b611e1e614990565b6001600160a01b038216611e445760405162461bcd60e51b8152600401610bf490615932565b5f8111611e935760405162461bcd60e51b815260206004820152601c60248201527f4a61696c20626c6f636b73206d75737420626520706f736974697665000000006044820152606401610bf4565b6001600160a01b0382165f908152603460205260409020600501805460ff19166001179055611ec281436158a4565b6001600160a01b0383165f81815260346020526040908190206006018390555190917feb7d7a49847ec491969db21a0e31b234565a9923145a2d1b56a75c9e9582580291611f1291815260200190565b60405180910390a2610dc86149be565b611f2a614cf2565b80611f3481614efb565b611f3c614990565b6001600160a01b038216611f625760405162461bcd60e51b8152600401610bf490615932565b603e5f9054906101000a90046001600160a01b03166001600160a01b031663029859926040518163ffffffff1660e01b8152600401602060405180830381865afa158015611fb2573d5f5f3e3d5ffd5b505050506040513d601f19601f82011682018060405250810190611fd69190615832565b3410156120255760405162461bcd60e51b815260206004820152601e60248201527f496e73756666696369656e742064656c65676174696f6e20616d6f756e7400006044820152606401610bf4565b336001600160a01b0383160361207d5760405162461bcd60e51b815260206004820152601b60248201527f43616e6e6f742064656c656761746520746f20796f757273656c6600000000006044820152606401610bf4565b5f6120883384614d86565b335f9081526037602090815260408083206001600160a01b0388168452909152812080549293508215923492906120c09084906158a4565b90915550506001600160a01b0384165f90815260346020526040812060010180543492906120ef9084906158a4565b9250508190555034603a5f82825461210791906158a4565b90915550506001600160a01b0384165f818152603b60209081526040808320543384526037835281842094845293909152902054670de0b6b3a76400009161214e916159bf565b61215891906158f7565b335f9081526037602090815260408083206001600160a01b03891684529091529020600101558015612216576001600160a01b0384165f9081526042602052604081208054916121a7836158cb565b9091555050335f908152604760205260409020546121c69060016158a4565b335f9081526047602090815260408083209390935560419052205460ff166122165760408054905f6121f7836158cb565b9091555050335f908152604160205260409020805460ff191660011790555b811561222757612227338584614e0a565b6040513481526001600160a01b0385169033907fe5541a6b6103d4fa7e021ed54fad39c66f27a76bd13d374cf6240ae6bd0bb72b90602001611db8565b61226c61502e565b612274614cf2565b61227c614990565b612284615075565b5f81116122a35760405162461bcd60e51b8152600401610bf4906157ec565b603e5f9054906101000a90046001600160a01b03166001600160a01b031663c673316c6040518163ffffffff1660e01b8152600401602060405180830381865afa1580156122f3573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906123179190615832565b8111156123365760405162461bcd60e51b8152600401610bf490615849565b335f9081526034602052604090206009015460ff161561238d5760405162461bcd60e51b8152602060048201526012602482015271105b1c9958591e481c9959da5cdd195c995960721b6044820152606401610bf4565b603e5460405163416259d960e11b81523360048201526001600160a01b03909116906382c4b3b290602401602060405180830381865afa1580156123d3573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906123f79190615969565b6124435760405162461bcd60e51b815260206004820152601860248201527f4d75737420706173732070726f706f73616c20666972737400000000000000006044820152606401610bf4565b603e5460405163020f407560e31b81523360048201526001600160a01b039091169063107a03a890602401602060405180830381865afa158015612489573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906124ad9190615969565b6124f95760405162461bcd60e51b815260206004820181905260248201527f50726f706f73616c20657870697265642c206d75737420726570726f706f73656044820152606401610bf4565b603e5f9054906101000a90046001600160a01b03166001600160a01b031663017ddd356040518163ffffffff1660e01b8152600401602060405180830381865afa158015612549573d5f5f3e3d5ffd5b505050506040513d601f19601f8201168201806040525081019061256d9190615832565b3410156125b65760405162461bcd60e51b8152602060048201526017602482015276496e73756666696369656e742073656c662d7374616b6560481b6044820152606401610bf4565b6040805161014081018252348082525f6020808401828152848601878152606086018481526080870185815260a0880186815260c0890187815260e08a018881526101008b0189815260016101208d0181815233808d526034909b529d8b209c518d5597518c890155955160028c0155935160038b0155915160048a01555160058901805491151560ff19928316179055905160068901559051600788015590516008870155955160099095018054951515959096169490941790945560398054938401815590527fdc16fef70f8d5ddbc01ee3d903d1e69c18a3c7be080eb86a81e0578814ee58d390910180546001600160a01b031916909217909155603a546126c191906158a4565b603a55603d5460405163503cc43160e11b81523360048201526001600160a01b039091169063a0798862906024016020604051808303815f875af115801561270b573d5f5f3e3d5ffd5b505050506040513d601f19601f8201168201806040525081019061272f9190615969565b61277b5760405162461bcd60e51b815260206004820152601b60248201527f56616c696461746f722061637469766174696f6e206661696c656400000000006044820152606401610bf4565b604080513481526020810183905233917f4fedf9289a156b214915bd5c2db58d3ee16acc185e80df66ee404e4573c821e1910160405180910390a261189a6149be565b6127c661518a565b6127ce61502e565b6127d6614990565b435f908152603c6020908152604080832083805290915290205460ff16156128365760405162461bcd60e51b8152602060048201526013602482015272105b1c9958591e48191a5cdd1c9a589d5d1959606a1b6044820152606401610bf4565b431561288457603c5f61284a60014361590a565b81526020019081526020015f205f5f5f811115612869576128696159d6565b60ff16815260208101919091526040015f20805460ff191690555b435f908152603c602090815260408083208380529091528120805460ff1916600117905534908190036128b75750612a6e565b415f8181526035602090815260408083204390556034909152812080549091036128e357505050612a6e565b600181015481545f916128f5916158a4565b9050805f036129075750505050612a6e565b83826003015f82825461291a91906158a4565b909155505060028201545f906127109061293490876159bf565b61293e91906158f7565b905080836004015461295091906158a4565b60048401555f612960828761590a565b90505f83855f01548361297391906159bf565b61297d91906158f7565b905080856004015461298f91906158a4565b60048601555f61299f828461590a565b600187015490915015612a0c5760018601546129c382670de0b6b3a76400006159bf565b6129cd91906158f7565b6001600160a01b0388165f908152603b60205260409020546129ef91906158a4565b6001600160a01b0388165f908152603b6020526040902055612a22565b808660040154612a1c91906158a4565b60048701555b866001600160a01b03167fdf29796aad820e4bb192f3a8d631b76519bcd2cbe77cc85af20e9df53cece08689604051612a5d91815260200190565b60405180910390a250505050505050505b612a766149be565b565b612a806151c6565b6001600160a01b038616612ad65760405162461bcd60e51b815260206004820152601a60248201527f496e76616c69642076616c696461746f727320616464726573730000000000006044820152606401610bf4565b6001600160a01b038516612b275760405162461bcd60e51b8152602060048201526018602482015277496e76616c69642070726f706f73616c206164647265737360401b6044820152606401610bf4565b6001600160a01b038416612b765760405162461bcd60e51b8152602060048201526016602482015275496e76616c69642070756e697368206164647265737360501b6044820152606401610bf4565b6001600160a01b03861661f01014612ba05760405162461bcd60e51b8152600401610bf4906159ea565b6001600160a01b03851661f01214612bca5760405162461bcd60e51b8152600401610bf490615a2d565b6001600160a01b03841661f01114612c245760405162461bcd60e51b815260206004820152601f60248201527f496e76616c69642070756e69736820636f6e74726163742061646472657373006044820152606401610bf4565b81612c6a5760405162461bcd60e51b8152602060048201526016602482015275139bc81d985b1a59185d1bdc9cc81c1c9bdd9a59195960521b6044820152606401610bf4565b5f8111612c895760405162461bcd60e51b8152600401610bf4906157ec565b612710811115612cab5760405162461bcd60e51b8152600401610bf490615849565b670de0b6b3a76400005f612cbf84836159bf565b905080471015612d1c5760405162461bcd60e51b815260206004820152602260248201527f496e73756666696369656e7420696e697469616c207374616b652066756e64696044820152616e6760f01b6064820152608401610bf4565b603d80546001600160a01b03808b166001600160a01b031992831617909255603e80548a84169083168117909155603f8054938a16939092169290921790556040805163900cf0cf60e01b81529051612dc1929163900cf0cf9160048083019260209291908290030181865afa158015612d98573d5f5f3e3d5ffd5b505050506040513d601f19601f82011682018060405250810190612dbc9190615832565b61520e565b5f5b84811015612fe7575f868683818110612dde57612dde6158b7565b9050602002016020810190612df3919061566a565b90506001600160a01b038116612e1b5760405162461bcd60e51b8152600401610bf490615932565b6001600160a01b0381165f9081526034602052604090206009015460ff1615612e865760405162461bcd60e51b815260206004820152601860248201527f56616c696461746f7220616c72656164792065786973747300000000000000006044820152606401610bf4565b60408051610140810182528581525f60208083018281528385018a8152606085018481526080860185815260a0870186815260c0880187815260e089018881526101008a0189815260016101208c018181526001600160a01b038f16808d526034909b529c8b209b518c5597518b890155955160028b0155935160038a0155915160048901555160058801805491151560ff19928316179055905160068801559051600787015590516008860155945160099094018054941515949095169390931790935560398054928301815590527fdc16fef70f8d5ddbc01ee3d903d1e69c18a3c7be080eb86a81e0578814ee58d30180546001600160a01b0319169091179055603a54612f979085906158a4565b603a5560408051858152602081018790526001600160a01b038316917f4fedf9289a156b214915bd5c2db58d3ee16acc185e80df66ee404e4573c821e1910160405180910390a250600101612dc3565b5050600160438190555f805460ff1916909117905550505050505050565b61300d614cf2565b613015614990565b61301d615075565b6001600160a01b0381166130435760405162461bcd60e51b8152600401610bf490615932565b336001600160a01b038216146130a75760405162461bcd60e51b8152602060048201526024808201527f4f6e6c792076616c696461746f722063616e20756e6a61696c207468656d73656044820152636c76657360e01b6064820152608401610bf4565b6001600160a01b0381165f9081526034602052604090206005015460ff166131085760405162461bcd60e51b815260206004820152601460248201527315985b1a59185d1bdc881b9bdd081a985a5b195960621b6044820152606401610bf4565b6001600160a01b0381165f908152603460205260409020600601544310156131725760405162461bcd60e51b815260206004820152601860248201527f4a61696c20706572696f64206e6f7420636f6d706c65746500000000000000006044820152606401610bf4565b603e5f9054906101000a90046001600160a01b03166001600160a01b031663017ddd356040518163ffffffff1660e01b8152600401602060405180830381865afa1580156131c2573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906131e69190615832565b6001600160a01b0382165f90815260346020526040902054101561325d5760405162461bcd60e51b815260206004820152602860248201527f496e73756666696369656e74207374616b652c206d75737420616464207374616044820152671ad948199a5c9cdd60c21b6064820152608401610bf4565b603e5460405163416259d960e11b81526001600160a01b038381166004830152909116906382c4b3b290602401602060405180830381865afa1580156132a5573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906132c99190615969565b6133155760405162461bcd60e51b815260206004820152601a60248201527f4d757374207061737320726570726f706f73616c2066697273740000000000006044820152606401610bf4565b6001600160a01b038181165f8181526034602052604080822060058101805460ff1916905560060191909155603d54905163503cc43160e11b815260048101929092529091169063a0798862906024016020604051808303815f875af1158015613381573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906133a59190615969565b6133f15760405162461bcd60e51b815260206004820152601c60248201527f4661696c656420746f2061637469766174652076616c696461746f72000000006044820152606401610bf4565b6040516001600160a01b038216907f9390b453426557da5ebdc31f19a37753ca04addf656d32f35232211bb2af3f19905f90a261189a6149be565b613434614cf2565b61343c614990565b5f341161345b5760405162461bcd60e51b8152600401610bf490615988565b335f9081526034602052604090206009015460ff1661348c5760405162461bcd60e51b8152600401610bf490615a6e565b335f908152603460205260409020546134a69034906158a4565b335f90815260346020526040902055603a546134c39034906158a4565b603a55604051348152339081907fae6946de73a7ea549b21272efc1797dca3c65c4136f9d878585b0e100d8ad5bd9060200160405180910390a3612a766149be565b61350d614990565b5f811161355c5760405162461bcd60e51b815260206004820152601b60248201527f6d6178456e7472696573206d75737420626520706f73697469766500000000006044820152606401610bf4565b60148111156135a45760405162461bcd60e51b81526020600482015260146024820152736d6178456e747269657320746f6f206c6172676560601b6044820152606401610bf4565b335f9081526038602090815260408083206001600160a01b038616845290915281209080805b8354811080156135d957508482105b156136e157438482815481106135f1576135f16158b7565b905f5260205f20906002020160010154116136cf57838181548110613618576136186158b7565b905f5260205f2090600202015f01548361363291906158a4565b845490935084906136459060019061590a565b81548110613655576136556158b7565b905f5260205f209060020201848281548110613673576136736158b7565b5f9182526020909120825460029092020190815560019182015491015583548490806136a1576136a1615aa5565b5f8281526020812060025f1990930192830201818155600101559055816136c7816158cb565b9250506135ca565b806136d9816158cb565b9150506135ca565b505f82116137315760405162461bcd60e51b815260206004820152601c60248201527f4e6f20756e626f6e64656420746f6b656e7320617661696c61626c65000000006044820152606401610bf4565b335f9081526037602090815260408083206001600160a01b03891684529091528120541580156137815750335f9081526038602090815260408083206001600160a01b038a168452909152902054155b1561378a575060015b80156137ba57335f9081526037602090815260408083206001600160a01b038a1684529091528120818155600101555b6137c43384614bf6565b506040518381526001600160a01b0387169033907f29a354857110d4b0895fcb3571575b9346fa04cc4a08991d49b315894f57ce7d9060200160405180910390a350505050610dc86149be565b6001600160a01b038083165f9081526037602090815260408083209385168352929052908120805482919082901561388c5760018201546001600160a01b0386165f908152603b60205260409020548354670de0b6b3a764000091613875916159bf565b61387f91906158f7565b613889919061590a565b90505b90549590945092505050565b6001600160a01b038181165f818152603460205260408082206005810154603d549251631015428760e21b81526004810195909552929460ff90931693909291909116906340550a1c90602401602060405180830381865afa158015613900573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906139249190615969565b925050915091565b613934614cf2565b61393c614990565b3361394681614884565b335f9081526034602090815260408083206035909252909120548015613a3257603e5f9054906101000a90046001600160a01b03166001600160a01b031663c5ae519d6040518163ffffffff1660e01b8152600401602060405180830381865afa1580156139b6573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906139da9190615832565b6139e490826158a4565b4311613a325760405162461bcd60e51b815260206004820181905260248201527f4578697420626c6f636b656420696e20646f75626c655369676e57696e646f776044820152606401610bf4565b815480613a8c5760405162461bcd60e51b815260206004820152602260248201527f56616c696461746f7220686173206e6f207374616b6520746f20776974686472604482015261617760f01b6064820152608401610bf4565b603d54604051631015428760e21b81523360048201525f916001600160a01b0316906340550a1c90602401602060405180830381865afa158015613ad2573d5f5f3e3d5ffd5b505050506040513d601f19601f82011682018060405250810190613af69190615969565b90508015613b845760405162461bcd60e51b815260206004820152604f60248201527f43616e6e6f7420657869743a2076616c696461746f7220697320696e2061637460448201527f697665207365742c2072657369676e20666972737420616e642077616974207560648201526e0dce8d2d840dccaf0e840cae0dec6d608b1b608482015260a401610bf4565b603e5460405163416259d960e11b81523360048201525f916001600160a01b0316906382c4b3b290602401602060405180830381865afa158015613bca573d5f5f3e3d5ffd5b505050506040513d601f19601f82011682018060405250810190613bee9190615969565b5f8655603a54909150613c0290849061590a565b603a55335f90815260386020908152604080832082529182902082518084018452868152603e548451636cf6d67560e01b8152945192949193848101936001600160a01b0390921692636cf6d67592600480830193928290030181865afa158015613c6f573d5f5f3e3d5ffd5b505050506040513d601f19601f82011682018060405250810190613c939190615832565b613c9d90436158a4565b90528154600181810184555f938452602093849020835160029093020191825592909101519101558015613d2457603d54604051636a48a57160e11b81523360048201526001600160a01b039091169063d4914ae2906024015f604051808303815f87803b158015613d0d575f5ffd5b505af1158015613d1f573d5f5f3e3d5ffd5b505050505b60405133907f05956ba8f9bd4bcb79fc3301e702e6bd74e3df03d048ed441fa2420dd6ffa8be905f90a2505050505050612a766149be565b613d64614cf2565b33613d6e81614884565b613d76614990565b613d7e61529f565b335f9081526034602090815260408083206035909252909120548015613e6a57603e5f9054906101000a90046001600160a01b03166001600160a01b031663c5ae519d6040518163ffffffff1660e01b8152600401602060405180830381865afa158015613dee573d5f5f3e3d5ffd5b505050506040513d601f19601f82011682018060405250810190613e129190615832565b613e1c90826158a4565b4311613e6a5760405162461bcd60e51b815260206004820181905260248201527f4578697420626c6f636b656420696e20646f75626c655369676e57696e646f776044820152606401610bf4565b600582015460ff1615613ecb5760405162461bcd60e51b8152602060048201526024808201527f56616c696461746f7220616c72656164792072657369676e6564206f72206a616044820152631a5b195960e21b6064820152608401610bf4565b60058201805460ff19166001179055603e546040805163f945b62360e01b815290516001600160a01b039092169163f945b623916004808201926020929091908290030181865afa158015613f22573d5f5f3e3d5ffd5b505050506040513d601f19601f82011682018060405250810190613f469190615832565b613f5090436158a4565b6006830181905560405190815233907feb7d7a49847ec491969db21a0e31b234565a9923145a2d1b56a75c9e958258029060200160405180910390a2603d54604051636a48a57160e11b81523360048201526001600160a01b039091169063d4914ae2906024015f604051808303815f87803b158015613fce575f5ffd5b505af1158015613fe0573d5f5f3e3d5ffd5b50505050505061189a6149be565b60398181548110613ffd575f80fd5b5f918252602090912001546001600160a01b0316905081565b61401e614990565b335f8181526034602052604090206009015460ff1661407f5760405162461bcd60e51b815260206004820152601a60248201527f4e6f74206120726567697374657265642076616c696461746f720000000000006044820152606401610bf4565b6001600160a01b0381165f908152603460205260409020600481015480156141c25760088201541561419657603e54604080516394522b6d60e01b815290515f926001600160a01b0316916394522b6d9160048083019260209291908290030181865afa1580156140f2573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906141169190615832565b905080836008015461412891906158a4565b4310156141945760405162461bcd60e51b815260206004820152603460248201527f4d757374207761697420776974686472617750726f666974506572696f6420626044820152736c6f636b73206265747765656e20636c61696d7360601b6064820152608401610bf4565b505b5f600483015560078201546141ac9082906158a4565b60078301554360088301556141c2338483614e0a565b505050612a766149be565b6141d56151c6565b6001600160a01b03831661422b5760405162461bcd60e51b815260206004820152601a60248201527f496e76616c69642076616c696461746f727320616464726573730000000000006044820152606401610bf4565b6001600160a01b03821661427c5760405162461bcd60e51b8152602060048201526018602482015277496e76616c69642070726f706f73616c206164647265737360401b6044820152606401610bf4565b6001600160a01b0381166142cb5760405162461bcd60e51b8152602060048201526016602482015275496e76616c69642070756e697368206164647265737360501b6044820152606401610bf4565b6001600160a01b03831661f010146142f55760405162461bcd60e51b8152600401610bf4906159ea565b6001600160a01b03821661f0121461431f5760405162461bcd60e51b8152600401610bf490615a2d565b6001600160a01b03811661f011146143795760405162461bcd60e51b815260206004820152601f60248201527f496e76616c69642070756e69736820636f6e74726163742061646472657373006044820152606401610bf4565b603d80546001600160a01b038086166001600160a01b031992831617909255603e80548584169083168117909155603f8054938516939092169290921790556040805163900cf0cf60e01b815290516143f5929163900cf0cf9160048083019260209291908290030181865afa158015612d98573d5f5f3e3d5ffd5b5050600160438190555f805460ff1916909117905550565b61441561502e565b61441d61518a565b6002604354106144675760405162461bcd60e51b8152602060048201526015602482015274105b1c9958591e481c995a5b9a5d1a585b1a5e9959605a1b6044820152606401610bf4565b6002604355565b61447661539d565b6001600160a01b03811661449c5760405162461bcd60e51b8152600401610bf490615932565b6001600160a01b03165f908152603560205260409020439055565b6144bf614990565b6001600160a01b0381165f9081526034602052604090206009015460ff166144f95760405162461bcd60e51b8152600401610bf490615a6e565b335f9081526037602090815260408083206001600160a01b038516845290915290205461455e5760405162461bcd60e51b8152602060048201526013602482015272139bc819195b1959d85d1a5bdb88199bdd5b99606a1b6044820152606401610bf4565b5f6145693383614d86565b335f9081526037602090815260408083206001600160a01b0387168452909152902090915081156145db576001600160a01b0383165f908152603b60205260409020548154670de0b6b3a7640000916145c1916159bf565b6145cb91906158f7565b60018201556145db338484614e0a565b505061189a6149be565b6145ed614cf2565b6145f5614990565b336145ff81614884565b5f821161461e5760405162461bcd60e51b8152600401610bf490615988565b335f90815260346020526040902080548311156146775760405162461bcd60e51b8152602060048201526017602482015276496e73756666696369656e742073656c662d7374616b6560481b6044820152606401610bf4565b80545f9061468690859061590a565b9050603e5f9054906101000a90046001600160a01b03166001600160a01b031663017ddd356040518163ffffffff1660e01b8152600401602060405180830381865afa1580156146d8573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906146fc9190615832565b8110156147715760405162461bcd60e51b815260206004820152603960248201527f52656d61696e696e67207374616b652062656c6f77206d696e696d756d2c207760448201527f6974686472617720616c6c207374616b6520696e7374656164000000000000006064820152608401610bf4565b808255603a5461478290859061590a565b603a55335f90815260386020908152604080832082529182902082518084018452878152603e548451636cf6d67560e01b8152945192949193848101936001600160a01b0390921692636cf6d67592600480830193928290030181865afa1580156147ef573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906148139190615832565b61481d90436158a4565b90528154600180820184555f938452602093849020835160029093020191825591830151910155604051858152339182917f64bcb4cf4c65b4bfe23bf707cd7770998b00489a298f3c1e019a8a915348dd43910160405180910390a350505061189a6149be565b6001600160a01b0381165f9081526034602052604090206009015460ff166148be5760405162461bcd60e51b8152600401610bf490615a6e565b603e5f9054906101000a90046001600160a01b03166001600160a01b031663017ddd356040518163ffffffff1660e01b8152600401602060405180830381865afa15801561490e573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906149329190615832565b6001600160a01b0382165f90815260346020526040902054101561189a5760405162461bcd60e51b81526020600482015260156024820152742737ba1030903b30b634b2103b30b634b230ba37b960591b6044820152606401610bf4565b6149986153ee565b60027f9b779b17422d0df92223018b32b4d1fa46e071723d6817e2486d003becc55f0055565b60017f9b779b17422d0df92223018b32b4d1fa46e071723d6817e2486d003becc55f0055565b805f6149f18260026159bf565b6149fc9060016158a4565b90505f614a0a8460026159bf565b614a159060026158a4565b90508482108015614a575750858381518110614a3357614a336158b7565b6020026020010151868381518110614a4d57614a4d6158b7565b6020026020010151115b15614a60578192505b8481108015614aa05750858381518110614a7c57614a7c6158b7565b6020026020010151868281518110614a9657614a966158b7565b6020026020010151115b15614aa9578092505b838314614ba557868381518110614ac257614ac26158b7565b6020026020010151878581518110614adc57614adc6158b7565b6020026020010151888681518110614af657614af66158b7565b60200260200101898681518110614b0f57614b0f6158b7565b6001600160a01b039384166020918202929092010152911690528551869084908110614b3d57614b3d6158b7565b6020026020010151868581518110614b5757614b576158b7565b6020026020010151878681518110614b7157614b716158b7565b60200260200101888681518110614b8a57614b8a6158b7565b602090810291909101019190915252614ba5878787866149e4565b50505050505050565b3361f01114612a765760405162461bcd60e51b815260206004820152601460248201527350756e69736820636f6e7472616374206f6e6c7960601b6044820152606401610bf4565b5f815f03614c065750600161170a565b814710614c71575f836001600160a01b0316836040515f6040518083038185875af1925050503d805f8114614c56576040519150601f19603f3d011682016040523d82523d5f602084013e614c5b565b606091505b505090508015614c6f57600191505061170a565b505b6001600160a01b0383165f90815260486020526040902054614c949083906158a4565b6001600160a01b0384165f81815260486020526040908190209290925590517f55b06c80d6c74575c15af6a6b40b8b909be9ec4976c7641beb80036e9b1986e890614ce29085815260200190565b60405180910390a2505f92915050565b5f60015411614d335760405162461bcd60e51b815260206004820152600d60248201526c115c1bd8da081b9bdd081cd95d609a1b6044820152606401610bf4565b600154614d409043615ab9565b5f03612a765760405162461bcd60e51b815260206004820152601560248201527422b837b1b410313637b1b5903337b93134b23232b760591b6044820152606401610bf4565b6001600160a01b038083165f9081526037602090815260408083209385168352929052908120805415614e015760018101546001600160a01b0384165f908152603b60205260409020548254670de0b6b3a764000091614de5916159bf565b614def91906158f7565b614df9919061590a565b91505061170a565b505f9392505050565b8015614e73575f614e1b8483614bf6565b90508015614e7157826001600160a01b0316846001600160a01b03167f9310ccfcb8de723f578a9e4282ea9f521f05ae40dc08f3068dfad528a65ee3c784604051614e6891815260200190565b60405180910390a35b505b505050565b3361f0111480614e8957503361f010145b612a765760405162461bcd60e51b815260206004820152603960248201527f4f6e6c792070756e697368206f722076616c696461746f727320636f6e74726160448201527f63742063616e2063616c6c20746869732066756e6374696f6e000000000000006064820152608401610bf4565b603e5f9054906101000a90046001600160a01b03166001600160a01b031663017ddd356040518163ffffffff1660e01b8152600401602060405180830381865afa158015614f4b573d5f5f3e3d5ffd5b505050506040513d601f19601f82011682018060405250810190614f6f9190615832565b6001600160a01b0382165f908152603460205260409020541015614fcd5760405162461bcd60e51b81526020600482015260156024820152742737ba1030903b30b634b2103b30b634b230ba37b960591b6044820152606401610bf4565b6001600160a01b0381165f9081526034602052604090206005015460ff161561189a5760405162461bcd60e51b815260206004820152601360248201527215985b1a59185d1bdc881a5cc81a985a5b1959606a1b6044820152606401610bf4565b5f5460ff16612a765760405162461bcd60e51b8152602060048201526013602482015272139bdd081a5b9a5d1a585b1a5e9959081e595d606a1b6044820152606401610bf4565b603e546040805163900cf0cf60e01b815290515f926001600160a01b03169163900cf0cf9160048083019260209291908290030181865afa1580156150bc573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906150e09190615832565b9050805f036150ec5750565b5f6150f782436158f7565b90506044548111156151125760448190555f60458190556046555b6001604554106151725760405162461bcd60e51b815260206004820152602560248201527f546f6f206d616e79206e65772076616c696461746f727320696e2074686973206044820152640cae0dec6d60db1b6064820152608401610bf4565b60458054905f615181836158cb565b91905055505050565b334114612a765760405162461bcd60e51b815260206004820152600a6024820152694d696e6572206f6e6c7960b01b6044820152606401610bf4565b5f5460ff1615612a765760405162461bcd60e51b8152602060048201526013602482015272105b1c9958591e481a5b9a5d1a585b1a5e9959606a1b6044820152606401610bf4565b5f81116152565760405162461bcd60e51b815260206004820152601660248201527545706f6368206d75737420626520706f73697469766560501b6044820152606401610bf4565b6001541561529a5760405162461bcd60e51b8152602060048201526011602482015270115c1bd8da08185b1c9958591e481cd95d607a1b6044820152606401610bf4565b600155565b603e546040805163900cf0cf60e01b815290515f926001600160a01b03169163900cf0cf9160048083019260209291908290030181865afa1580156152e6573d5f5f3e3d5ffd5b505050506040513d601f19601f8201168201806040525081019061530a9190615832565b9050805f036153165750565b5f61532182436158f7565b905060445481111561533c5760448190555f60458190556046555b60016046541061538e5760405162461bcd60e51b815260206004820152601f60248201527f546f6f206d616e792072656d6f76616c7320696e20746869732065706f6368006044820152606401610bf4565b60468054905f615181836158cb565b3361f01014612a765760405162461bcd60e51b815260206004820152601860248201527f56616c696461746f727320636f6e7472616374206f6e6c7900000000000000006044820152606401610bf4565b7f9b779b17422d0df92223018b32b4d1fa46e071723d6817e2486d003becc55f0054600203612a7657604051633ee5aeb560e01b815260040160405180910390fd5b5f60208284031215615440575f5ffd5b5035919050565b634e487b7160e01b5f52604160045260245ffd5b6001600160a01b038116811461189a575f5ffd5b803561547a8161545b565b919050565b5f6020828403121561548f575f5ffd5b813567ffffffffffffffff8111156154a5575f5ffd5b8201601f810184136154b5575f5ffd5b803567ffffffffffffffff8111156154cf576154cf615447565b8060051b604051601f19603f830116810181811067ffffffffffffffff821117156154fc576154fc615447565b604052918252602081840181019290810187841115615519575f5ffd5b6020850194505b8385101561553f576155318561546f565b815260209485019401615520565b509695505050505050565b602080825282518282018190525f918401906040840190835b8181101561558a5783516001600160a01b0316835260209384019390920191600101615563565b509095945050505050565b5f5f5f5f5f60a086880312156155a9575f5ffd5b85356155b48161545b565b94506020860135935060408601356155cb8161545b565b92506060860135915060808601356155e28161545b565b809150509295509295909350565b5f5f60408385031215615601575f5ffd5b823561560c8161545b565b9150602083013561561c8161545b565b809150509250929050565b602080825282518282018190525f918401906040840190835b8181101561558a578351805184526020908101518185015290930192604090920191600101615640565b5f6020828403121561567a575f5ffd5b81356156858161545b565b9392505050565b5f5f6040838503121561569d575f5ffd5b82356156a88161545b565b946020939093013593505050565b5f5f5f606084860312156156c8575f5ffd5b83356156d38161545b565b925060208401356156e38161545b565b929592945050506040919091013590565b5f5f5f5f5f5f60a08789031215615709575f5ffd5b86356157148161545b565b955060208701356157248161545b565b945060408701356157348161545b565b9350606087013567ffffffffffffffff81111561574f575f5ffd5b8701601f8101891361575f575f5ffd5b803567ffffffffffffffff811115615775575f5ffd5b8960208260051b8401011115615789575f5ffd5b96999598509396602090940195946080909401359392505050565b5f5f5f606084860312156157b6575f5ffd5b83356157c18161545b565b925060208401356157d18161545b565b915060408401356157e18161545b565b809150509250925092565b60208082526026908201527f436f6d6d697373696f6e2072617465206d75737420626520677265617465722060408201526507468616e20360d41b606082015260800190565b5f60208284031215615842575f5ffd5b5051919050565b60208082526027908201527f436f6d6d697373696f6e20726174652065786365656473206d6178696d756d20604082015266185b1b1bddd95960ca1b606082015260800190565b634e487b7160e01b5f52601160045260245ffd5b8082018082111561170a5761170a615890565b634e487b7160e01b5f52603260045260245ffd5b5f600182016158dc576158dc615890565b5060010190565b634e487b7160e01b5f52601260045260245ffd5b5f82615905576159056158e3565b500490565b8181038181111561170a5761170a615890565b5f8161592b5761592b615890565b505f190190565b60208082526019908201527f496e76616c69642076616c696461746f72206164647265737300000000000000604082015260600190565b5f60208284031215615979575f5ffd5b81518015158114615685575f5ffd5b60208082526017908201527f416d6f756e74206d75737420626520706f736974697665000000000000000000604082015260600190565b808202811582820484141761170a5761170a615890565b634e487b7160e01b5f52602160045260245ffd5b60208082526023908201527f496e76616c69642076616c696461746f727320636f6e7472616374206164647260408201526265737360e81b606082015260800190565b60208082526021908201527f496e76616c69642070726f706f73616c20636f6e7472616374206164647265736040820152607360f81b606082015260800190565b60208082526018908201527f56616c696461746f72206e6f7420726567697374657265640000000000000000604082015260600190565b634e487b7160e01b5f52603160045260245ffd5b5f82615ac757615ac76158e3565b50069056fea264697066735822122087feceb7a40163e1a2b16b294146a9ba86d9660efd24b4e8e60d6980a0afb60b64736f6c634300081d0033",
}

// StakingABI is the input ABI used to generate the binding from.
// Deprecated: Use StakingMetaData.ABI instead.
var StakingABI = StakingMetaData.ABI

// StakingBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use StakingMetaData.Bin instead.
var StakingBin = StakingMetaData.Bin

// DeployStaking deploys a new Ethereum contract, binding an instance of Staking to it.
func DeployStaking(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Staking, error) {
	parsed, err := StakingMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(StakingBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Staking{StakingCaller: StakingCaller{contract: contract}, StakingTransactor: StakingTransactor{contract: contract}, StakingFilterer: StakingFilterer{contract: contract}}, nil
}

// Staking is an auto generated Go binding around an Ethereum contract.
type Staking struct {
	StakingCaller     // Read-only binding to the contract
	StakingTransactor // Write-only binding to the contract
	StakingFilterer   // Log filterer for contract events
}

// StakingCaller is an auto generated read-only Go binding around an Ethereum contract.
type StakingCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StakingTransactor is an auto generated write-only Go binding around an Ethereum contract.
type StakingTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StakingFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type StakingFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StakingSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type StakingSession struct {
	Contract     *Staking          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// StakingCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type StakingCallerSession struct {
	Contract *StakingCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// StakingTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type StakingTransactorSession struct {
	Contract     *StakingTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// StakingRaw is an auto generated low-level Go binding around an Ethereum contract.
type StakingRaw struct {
	Contract *Staking // Generic contract binding to access the raw methods on
}

// StakingCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type StakingCallerRaw struct {
	Contract *StakingCaller // Generic read-only contract binding to access the raw methods on
}

// StakingTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type StakingTransactorRaw struct {
	Contract *StakingTransactor // Generic write-only contract binding to access the raw methods on
}

// NewStaking creates a new instance of Staking, bound to a specific deployed contract.
func NewStaking(address common.Address, backend bind.ContractBackend) (*Staking, error) {
	contract, err := bindStaking(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Staking{StakingCaller: StakingCaller{contract: contract}, StakingTransactor: StakingTransactor{contract: contract}, StakingFilterer: StakingFilterer{contract: contract}}, nil
}

// NewStakingCaller creates a new read-only instance of Staking, bound to a specific deployed contract.
func NewStakingCaller(address common.Address, caller bind.ContractCaller) (*StakingCaller, error) {
	contract, err := bindStaking(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &StakingCaller{contract: contract}, nil
}

// NewStakingTransactor creates a new write-only instance of Staking, bound to a specific deployed contract.
func NewStakingTransactor(address common.Address, transactor bind.ContractTransactor) (*StakingTransactor, error) {
	contract, err := bindStaking(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &StakingTransactor{contract: contract}, nil
}

// NewStakingFilterer creates a new log filterer instance of Staking, bound to a specific deployed contract.
func NewStakingFilterer(address common.Address, filterer bind.ContractFilterer) (*StakingFilterer, error) {
	contract, err := bindStaking(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &StakingFilterer{contract: contract}, nil
}

// bindStaking binds a generic wrapper to an already deployed contract.
func bindStaking(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := StakingMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Staking *StakingRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Staking.Contract.StakingCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Staking *StakingRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Staking.Contract.StakingTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Staking *StakingRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Staking.Contract.StakingTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Staking *StakingCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Staking.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Staking *StakingTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Staking.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Staking *StakingTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Staking.Contract.contract.Transact(opts, method, params...)
}

// COMMISSIONRATEBASE is a free data retrieval call binding the contract method 0xbee8380e.
//
// Solidity: function COMMISSION_RATE_BASE() view returns(uint256)
func (_Staking *StakingCaller) COMMISSIONRATEBASE(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "COMMISSION_RATE_BASE")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// COMMISSIONRATEBASE is a free data retrieval call binding the contract method 0xbee8380e.
//
// Solidity: function COMMISSION_RATE_BASE() view returns(uint256)
func (_Staking *StakingSession) COMMISSIONRATEBASE() (*big.Int, error) {
	return _Staking.Contract.COMMISSIONRATEBASE(&_Staking.CallOpts)
}

// COMMISSIONRATEBASE is a free data retrieval call binding the contract method 0xbee8380e.
//
// Solidity: function COMMISSION_RATE_BASE() view returns(uint256)
func (_Staking *StakingCallerSession) COMMISSIONRATEBASE() (*big.Int, error) {
	return _Staking.Contract.COMMISSIONRATEBASE(&_Staking.CallOpts)
}

// CONSENSUSMAXVALIDATORS is a free data retrieval call binding the contract method 0x764e7feb.
//
// Solidity: function CONSENSUS_MAX_VALIDATORS() view returns(uint256)
func (_Staking *StakingCaller) CONSENSUSMAXVALIDATORS(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "CONSENSUS_MAX_VALIDATORS")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// CONSENSUSMAXVALIDATORS is a free data retrieval call binding the contract method 0x764e7feb.
//
// Solidity: function CONSENSUS_MAX_VALIDATORS() view returns(uint256)
func (_Staking *StakingSession) CONSENSUSMAXVALIDATORS() (*big.Int, error) {
	return _Staking.Contract.CONSENSUSMAXVALIDATORS(&_Staking.CallOpts)
}

// CONSENSUSMAXVALIDATORS is a free data retrieval call binding the contract method 0x764e7feb.
//
// Solidity: function CONSENSUS_MAX_VALIDATORS() view returns(uint256)
func (_Staking *StakingCallerSession) CONSENSUSMAXVALIDATORS() (*big.Int, error) {
	return _Staking.Contract.CONSENSUSMAXVALIDATORS(&_Staking.CallOpts)
}

// MAXUNBONDINGENTRIES is a free data retrieval call binding the contract method 0xa41fcdc3.
//
// Solidity: function MAX_UNBONDING_ENTRIES() view returns(uint256)
func (_Staking *StakingCaller) MAXUNBONDINGENTRIES(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "MAX_UNBONDING_ENTRIES")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// MAXUNBONDINGENTRIES is a free data retrieval call binding the contract method 0xa41fcdc3.
//
// Solidity: function MAX_UNBONDING_ENTRIES() view returns(uint256)
func (_Staking *StakingSession) MAXUNBONDINGENTRIES() (*big.Int, error) {
	return _Staking.Contract.MAXUNBONDINGENTRIES(&_Staking.CallOpts)
}

// MAXUNBONDINGENTRIES is a free data retrieval call binding the contract method 0xa41fcdc3.
//
// Solidity: function MAX_UNBONDING_ENTRIES() view returns(uint256)
func (_Staking *StakingCallerSession) MAXUNBONDINGENTRIES() (*big.Int, error) {
	return _Staking.Contract.MAXUNBONDINGENTRIES(&_Staking.CallOpts)
}

// PROPOSALADDR is a free data retrieval call binding the contract method 0x437ccda8.
//
// Solidity: function PROPOSAL_ADDR() view returns(address)
func (_Staking *StakingCaller) PROPOSALADDR(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "PROPOSAL_ADDR")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// PROPOSALADDR is a free data retrieval call binding the contract method 0x437ccda8.
//
// Solidity: function PROPOSAL_ADDR() view returns(address)
func (_Staking *StakingSession) PROPOSALADDR() (common.Address, error) {
	return _Staking.Contract.PROPOSALADDR(&_Staking.CallOpts)
}

// PROPOSALADDR is a free data retrieval call binding the contract method 0x437ccda8.
//
// Solidity: function PROPOSAL_ADDR() view returns(address)
func (_Staking *StakingCallerSession) PROPOSALADDR() (common.Address, error) {
	return _Staking.Contract.PROPOSALADDR(&_Staking.CallOpts)
}

// PUNISHADDR is a free data retrieval call binding the contract method 0x8c872d05.
//
// Solidity: function PUNISH_ADDR() view returns(address)
func (_Staking *StakingCaller) PUNISHADDR(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "PUNISH_ADDR")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// PUNISHADDR is a free data retrieval call binding the contract method 0x8c872d05.
//
// Solidity: function PUNISH_ADDR() view returns(address)
func (_Staking *StakingSession) PUNISHADDR() (common.Address, error) {
	return _Staking.Contract.PUNISHADDR(&_Staking.CallOpts)
}

// PUNISHADDR is a free data retrieval call binding the contract method 0x8c872d05.
//
// Solidity: function PUNISH_ADDR() view returns(address)
func (_Staking *StakingCallerSession) PUNISHADDR() (common.Address, error) {
	return _Staking.Contract.PUNISHADDR(&_Staking.CallOpts)
}

// STAKINGADDR is a free data retrieval call binding the contract method 0x5d4f0cb6.
//
// Solidity: function STAKING_ADDR() view returns(address)
func (_Staking *StakingCaller) STAKINGADDR(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "STAKING_ADDR")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// STAKINGADDR is a free data retrieval call binding the contract method 0x5d4f0cb6.
//
// Solidity: function STAKING_ADDR() view returns(address)
func (_Staking *StakingSession) STAKINGADDR() (common.Address, error) {
	return _Staking.Contract.STAKINGADDR(&_Staking.CallOpts)
}

// STAKINGADDR is a free data retrieval call binding the contract method 0x5d4f0cb6.
//
// Solidity: function STAKING_ADDR() view returns(address)
func (_Staking *StakingCallerSession) STAKINGADDR() (common.Address, error) {
	return _Staking.Contract.STAKINGADDR(&_Staking.CallOpts)
}

// VALIDATORADDR is a free data retrieval call binding the contract method 0x9f759dba.
//
// Solidity: function VALIDATOR_ADDR() view returns(address)
func (_Staking *StakingCaller) VALIDATORADDR(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "VALIDATOR_ADDR")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// VALIDATORADDR is a free data retrieval call binding the contract method 0x9f759dba.
//
// Solidity: function VALIDATOR_ADDR() view returns(address)
func (_Staking *StakingSession) VALIDATORADDR() (common.Address, error) {
	return _Staking.Contract.VALIDATORADDR(&_Staking.CallOpts)
}

// VALIDATORADDR is a free data retrieval call binding the contract method 0x9f759dba.
//
// Solidity: function VALIDATOR_ADDR() view returns(address)
func (_Staking *StakingCallerSession) VALIDATORADDR() (common.Address, error) {
	return _Staking.Contract.VALIDATORADDR(&_Staking.CallOpts)
}

// AllValidators is a free data retrieval call binding the contract method 0xbcecf81b.
//
// Solidity: function allValidators(uint256 ) view returns(address)
func (_Staking *StakingCaller) AllValidators(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "allValidators", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// AllValidators is a free data retrieval call binding the contract method 0xbcecf81b.
//
// Solidity: function allValidators(uint256 ) view returns(address)
func (_Staking *StakingSession) AllValidators(arg0 *big.Int) (common.Address, error) {
	return _Staking.Contract.AllValidators(&_Staking.CallOpts, arg0)
}

// AllValidators is a free data retrieval call binding the contract method 0xbcecf81b.
//
// Solidity: function allValidators(uint256 ) view returns(address)
func (_Staking *StakingCallerSession) AllValidators(arg0 *big.Int) (common.Address, error) {
	return _Staking.Contract.AllValidators(&_Staking.CallOpts, arg0)
}

// CurrentEpochId is a free data retrieval call binding the contract method 0xeacdc5ff.
//
// Solidity: function currentEpochId() view returns(uint256)
func (_Staking *StakingCaller) CurrentEpochId(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "currentEpochId")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// CurrentEpochId is a free data retrieval call binding the contract method 0xeacdc5ff.
//
// Solidity: function currentEpochId() view returns(uint256)
func (_Staking *StakingSession) CurrentEpochId() (*big.Int, error) {
	return _Staking.Contract.CurrentEpochId(&_Staking.CallOpts)
}

// CurrentEpochId is a free data retrieval call binding the contract method 0xeacdc5ff.
//
// Solidity: function currentEpochId() view returns(uint256)
func (_Staking *StakingCallerSession) CurrentEpochId() (*big.Int, error) {
	return _Staking.Contract.CurrentEpochId(&_Staking.CallOpts)
}

// Delegations is a free data retrieval call binding the contract method 0xc64814dd.
//
// Solidity: function delegations(address , address ) view returns(uint256 amount, uint256 rewardDebt)
func (_Staking *StakingCaller) Delegations(opts *bind.CallOpts, arg0 common.Address, arg1 common.Address) (struct {
	Amount     *big.Int
	RewardDebt *big.Int
}, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "delegations", arg0, arg1)

	outstruct := new(struct {
		Amount     *big.Int
		RewardDebt *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Amount = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.RewardDebt = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// Delegations is a free data retrieval call binding the contract method 0xc64814dd.
//
// Solidity: function delegations(address , address ) view returns(uint256 amount, uint256 rewardDebt)
func (_Staking *StakingSession) Delegations(arg0 common.Address, arg1 common.Address) (struct {
	Amount     *big.Int
	RewardDebt *big.Int
}, error) {
	return _Staking.Contract.Delegations(&_Staking.CallOpts, arg0, arg1)
}

// Delegations is a free data retrieval call binding the contract method 0xc64814dd.
//
// Solidity: function delegations(address , address ) view returns(uint256 amount, uint256 rewardDebt)
func (_Staking *StakingCallerSession) Delegations(arg0 common.Address, arg1 common.Address) (struct {
	Amount     *big.Int
	RewardDebt *big.Int
}, error) {
	return _Staking.Contract.Delegations(&_Staking.CallOpts, arg0, arg1)
}

// DelegatorCount is a free data retrieval call binding the contract method 0x666183ee.
//
// Solidity: function delegatorCount() view returns(uint256)
func (_Staking *StakingCaller) DelegatorCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "delegatorCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// DelegatorCount is a free data retrieval call binding the contract method 0x666183ee.
//
// Solidity: function delegatorCount() view returns(uint256)
func (_Staking *StakingSession) DelegatorCount() (*big.Int, error) {
	return _Staking.Contract.DelegatorCount(&_Staking.CallOpts)
}

// DelegatorCount is a free data retrieval call binding the contract method 0x666183ee.
//
// Solidity: function delegatorCount() view returns(uint256)
func (_Staking *StakingCallerSession) DelegatorCount() (*big.Int, error) {
	return _Staking.Contract.DelegatorCount(&_Staking.CallOpts)
}

// DelegatorExists is a free data retrieval call binding the contract method 0x7664fc92.
//
// Solidity: function delegatorExists(address ) view returns(bool)
func (_Staking *StakingCaller) DelegatorExists(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "delegatorExists", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// DelegatorExists is a free data retrieval call binding the contract method 0x7664fc92.
//
// Solidity: function delegatorExists(address ) view returns(bool)
func (_Staking *StakingSession) DelegatorExists(arg0 common.Address) (bool, error) {
	return _Staking.Contract.DelegatorExists(&_Staking.CallOpts, arg0)
}

// DelegatorExists is a free data retrieval call binding the contract method 0x7664fc92.
//
// Solidity: function delegatorExists(address ) view returns(bool)
func (_Staking *StakingCallerSession) DelegatorExists(arg0 common.Address) (bool, error) {
	return _Staking.Contract.DelegatorExists(&_Staking.CallOpts, arg0)
}

// DelegatorValidatorCount is a free data retrieval call binding the contract method 0xf29efef6.
//
// Solidity: function delegatorValidatorCount(address ) view returns(uint256)
func (_Staking *StakingCaller) DelegatorValidatorCount(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "delegatorValidatorCount", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// DelegatorValidatorCount is a free data retrieval call binding the contract method 0xf29efef6.
//
// Solidity: function delegatorValidatorCount(address ) view returns(uint256)
func (_Staking *StakingSession) DelegatorValidatorCount(arg0 common.Address) (*big.Int, error) {
	return _Staking.Contract.DelegatorValidatorCount(&_Staking.CallOpts, arg0)
}

// DelegatorValidatorCount is a free data retrieval call binding the contract method 0xf29efef6.
//
// Solidity: function delegatorValidatorCount(address ) view returns(uint256)
func (_Staking *StakingCallerSession) DelegatorValidatorCount(arg0 common.Address) (*big.Int, error) {
	return _Staking.Contract.DelegatorValidatorCount(&_Staking.CallOpts, arg0)
}

// Epoch is a free data retrieval call binding the contract method 0x900cf0cf.
//
// Solidity: function epoch() view returns(uint256)
func (_Staking *StakingCaller) Epoch(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "epoch")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Epoch is a free data retrieval call binding the contract method 0x900cf0cf.
//
// Solidity: function epoch() view returns(uint256)
func (_Staking *StakingSession) Epoch() (*big.Int, error) {
	return _Staking.Contract.Epoch(&_Staking.CallOpts)
}

// Epoch is a free data retrieval call binding the contract method 0x900cf0cf.
//
// Solidity: function epoch() view returns(uint256)
func (_Staking *StakingCallerSession) Epoch() (*big.Int, error) {
	return _Staking.Contract.Epoch(&_Staking.CallOpts)
}

// GetDelegationInfo is a free data retrieval call binding the contract method 0x9abd6305.
//
// Solidity: function getDelegationInfo(address delegator, address validator) view returns(uint256 amount, uint256 pendingRewards)
func (_Staking *StakingCaller) GetDelegationInfo(opts *bind.CallOpts, delegator common.Address, validator common.Address) (struct {
	Amount         *big.Int
	PendingRewards *big.Int
}, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "getDelegationInfo", delegator, validator)

	outstruct := new(struct {
		Amount         *big.Int
		PendingRewards *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Amount = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.PendingRewards = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GetDelegationInfo is a free data retrieval call binding the contract method 0x9abd6305.
//
// Solidity: function getDelegationInfo(address delegator, address validator) view returns(uint256 amount, uint256 pendingRewards)
func (_Staking *StakingSession) GetDelegationInfo(delegator common.Address, validator common.Address) (struct {
	Amount         *big.Int
	PendingRewards *big.Int
}, error) {
	return _Staking.Contract.GetDelegationInfo(&_Staking.CallOpts, delegator, validator)
}

// GetDelegationInfo is a free data retrieval call binding the contract method 0x9abd6305.
//
// Solidity: function getDelegationInfo(address delegator, address validator) view returns(uint256 amount, uint256 pendingRewards)
func (_Staking *StakingCallerSession) GetDelegationInfo(delegator common.Address, validator common.Address) (struct {
	Amount         *big.Int
	PendingRewards *big.Int
}, error) {
	return _Staking.Contract.GetDelegationInfo(&_Staking.CallOpts, delegator, validator)
}

// GetTopValidators is a free data retrieval call binding the contract method 0x0864662b.
//
// Solidity: function getTopValidators(address[] validators) view returns(address[])
func (_Staking *StakingCaller) GetTopValidators(opts *bind.CallOpts, validators []common.Address) ([]common.Address, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "getTopValidators", validators)

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetTopValidators is a free data retrieval call binding the contract method 0x0864662b.
//
// Solidity: function getTopValidators(address[] validators) view returns(address[])
func (_Staking *StakingSession) GetTopValidators(validators []common.Address) ([]common.Address, error) {
	return _Staking.Contract.GetTopValidators(&_Staking.CallOpts, validators)
}

// GetTopValidators is a free data retrieval call binding the contract method 0x0864662b.
//
// Solidity: function getTopValidators(address[] validators) view returns(address[])
func (_Staking *StakingCallerSession) GetTopValidators(validators []common.Address) ([]common.Address, error) {
	return _Staking.Contract.GetTopValidators(&_Staking.CallOpts, validators)
}

// GetUnbondingEntries is a free data retrieval call binding the contract method 0x1c7e75e7.
//
// Solidity: function getUnbondingEntries(address delegator, address validator) view returns((uint256,uint256)[])
func (_Staking *StakingCaller) GetUnbondingEntries(opts *bind.CallOpts, delegator common.Address, validator common.Address) ([]StakingUnbondingEntry, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "getUnbondingEntries", delegator, validator)

	if err != nil {
		return *new([]StakingUnbondingEntry), err
	}

	out0 := *abi.ConvertType(out[0], new([]StakingUnbondingEntry)).(*[]StakingUnbondingEntry)

	return out0, err

}

// GetUnbondingEntries is a free data retrieval call binding the contract method 0x1c7e75e7.
//
// Solidity: function getUnbondingEntries(address delegator, address validator) view returns((uint256,uint256)[])
func (_Staking *StakingSession) GetUnbondingEntries(delegator common.Address, validator common.Address) ([]StakingUnbondingEntry, error) {
	return _Staking.Contract.GetUnbondingEntries(&_Staking.CallOpts, delegator, validator)
}

// GetUnbondingEntries is a free data retrieval call binding the contract method 0x1c7e75e7.
//
// Solidity: function getUnbondingEntries(address delegator, address validator) view returns((uint256,uint256)[])
func (_Staking *StakingCallerSession) GetUnbondingEntries(delegator common.Address, validator common.Address) ([]StakingUnbondingEntry, error) {
	return _Staking.Contract.GetUnbondingEntries(&_Staking.CallOpts, delegator, validator)
}

// GetUnbondingEntriesCount is a free data retrieval call binding the contract method 0xfb290af9.
//
// Solidity: function getUnbondingEntriesCount(address delegator, address validator) view returns(uint256)
func (_Staking *StakingCaller) GetUnbondingEntriesCount(opts *bind.CallOpts, delegator common.Address, validator common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "getUnbondingEntriesCount", delegator, validator)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetUnbondingEntriesCount is a free data retrieval call binding the contract method 0xfb290af9.
//
// Solidity: function getUnbondingEntriesCount(address delegator, address validator) view returns(uint256)
func (_Staking *StakingSession) GetUnbondingEntriesCount(delegator common.Address, validator common.Address) (*big.Int, error) {
	return _Staking.Contract.GetUnbondingEntriesCount(&_Staking.CallOpts, delegator, validator)
}

// GetUnbondingEntriesCount is a free data retrieval call binding the contract method 0xfb290af9.
//
// Solidity: function getUnbondingEntriesCount(address delegator, address validator) view returns(uint256)
func (_Staking *StakingCallerSession) GetUnbondingEntriesCount(delegator common.Address, validator common.Address) (*big.Int, error) {
	return _Staking.Contract.GetUnbondingEntriesCount(&_Staking.CallOpts, delegator, validator)
}

// GetValidatorCount is a free data retrieval call binding the contract method 0x7071688a.
//
// Solidity: function getValidatorCount() view returns(uint256)
func (_Staking *StakingCaller) GetValidatorCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "getValidatorCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetValidatorCount is a free data retrieval call binding the contract method 0x7071688a.
//
// Solidity: function getValidatorCount() view returns(uint256)
func (_Staking *StakingSession) GetValidatorCount() (*big.Int, error) {
	return _Staking.Contract.GetValidatorCount(&_Staking.CallOpts)
}

// GetValidatorCount is a free data retrieval call binding the contract method 0x7071688a.
//
// Solidity: function getValidatorCount() view returns(uint256)
func (_Staking *StakingCallerSession) GetValidatorCount() (*big.Int, error) {
	return _Staking.Contract.GetValidatorCount(&_Staking.CallOpts)
}

// GetValidatorInfo is a free data retrieval call binding the contract method 0x8a11d7c9.
//
// Solidity: function getValidatorInfo(address validator) view returns(uint256 selfStake, uint256 totalDelegated, uint256 commissionRate, uint256 accumulatedRewards, bool isJailed, uint256 jailUntilBlock, uint256 totalClaimedRewards, uint256 lastClaimBlock, bool isRegistered, uint256 totalRewards)
func (_Staking *StakingCaller) GetValidatorInfo(opts *bind.CallOpts, validator common.Address) (struct {
	SelfStake           *big.Int
	TotalDelegated      *big.Int
	CommissionRate      *big.Int
	AccumulatedRewards  *big.Int
	IsJailed            bool
	JailUntilBlock      *big.Int
	TotalClaimedRewards *big.Int
	LastClaimBlock      *big.Int
	IsRegistered        bool
	TotalRewards        *big.Int
}, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "getValidatorInfo", validator)

	outstruct := new(struct {
		SelfStake           *big.Int
		TotalDelegated      *big.Int
		CommissionRate      *big.Int
		AccumulatedRewards  *big.Int
		IsJailed            bool
		JailUntilBlock      *big.Int
		TotalClaimedRewards *big.Int
		LastClaimBlock      *big.Int
		IsRegistered        bool
		TotalRewards        *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.SelfStake = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.TotalDelegated = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.CommissionRate = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.AccumulatedRewards = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)
	outstruct.IsJailed = *abi.ConvertType(out[4], new(bool)).(*bool)
	outstruct.JailUntilBlock = *abi.ConvertType(out[5], new(*big.Int)).(**big.Int)
	outstruct.TotalClaimedRewards = *abi.ConvertType(out[6], new(*big.Int)).(**big.Int)
	outstruct.LastClaimBlock = *abi.ConvertType(out[7], new(*big.Int)).(**big.Int)
	outstruct.IsRegistered = *abi.ConvertType(out[8], new(bool)).(*bool)
	outstruct.TotalRewards = *abi.ConvertType(out[9], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// GetValidatorInfo is a free data retrieval call binding the contract method 0x8a11d7c9.
//
// Solidity: function getValidatorInfo(address validator) view returns(uint256 selfStake, uint256 totalDelegated, uint256 commissionRate, uint256 accumulatedRewards, bool isJailed, uint256 jailUntilBlock, uint256 totalClaimedRewards, uint256 lastClaimBlock, bool isRegistered, uint256 totalRewards)
func (_Staking *StakingSession) GetValidatorInfo(validator common.Address) (struct {
	SelfStake           *big.Int
	TotalDelegated      *big.Int
	CommissionRate      *big.Int
	AccumulatedRewards  *big.Int
	IsJailed            bool
	JailUntilBlock      *big.Int
	TotalClaimedRewards *big.Int
	LastClaimBlock      *big.Int
	IsRegistered        bool
	TotalRewards        *big.Int
}, error) {
	return _Staking.Contract.GetValidatorInfo(&_Staking.CallOpts, validator)
}

// GetValidatorInfo is a free data retrieval call binding the contract method 0x8a11d7c9.
//
// Solidity: function getValidatorInfo(address validator) view returns(uint256 selfStake, uint256 totalDelegated, uint256 commissionRate, uint256 accumulatedRewards, bool isJailed, uint256 jailUntilBlock, uint256 totalClaimedRewards, uint256 lastClaimBlock, bool isRegistered, uint256 totalRewards)
func (_Staking *StakingCallerSession) GetValidatorInfo(validator common.Address) (struct {
	SelfStake           *big.Int
	TotalDelegated      *big.Int
	CommissionRate      *big.Int
	AccumulatedRewards  *big.Int
	IsJailed            bool
	JailUntilBlock      *big.Int
	TotalClaimedRewards *big.Int
	LastClaimBlock      *big.Int
	IsRegistered        bool
	TotalRewards        *big.Int
}, error) {
	return _Staking.Contract.GetValidatorInfo(&_Staking.CallOpts, validator)
}

// GetValidatorStatus is a free data retrieval call binding the contract method 0xa310624f.
//
// Solidity: function getValidatorStatus(address validator) view returns(bool isActive, bool isJailed)
func (_Staking *StakingCaller) GetValidatorStatus(opts *bind.CallOpts, validator common.Address) (struct {
	IsActive bool
	IsJailed bool
}, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "getValidatorStatus", validator)

	outstruct := new(struct {
		IsActive bool
		IsJailed bool
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.IsActive = *abi.ConvertType(out[0], new(bool)).(*bool)
	outstruct.IsJailed = *abi.ConvertType(out[1], new(bool)).(*bool)

	return *outstruct, err

}

// GetValidatorStatus is a free data retrieval call binding the contract method 0xa310624f.
//
// Solidity: function getValidatorStatus(address validator) view returns(bool isActive, bool isJailed)
func (_Staking *StakingSession) GetValidatorStatus(validator common.Address) (struct {
	IsActive bool
	IsJailed bool
}, error) {
	return _Staking.Contract.GetValidatorStatus(&_Staking.CallOpts, validator)
}

// GetValidatorStatus is a free data retrieval call binding the contract method 0xa310624f.
//
// Solidity: function getValidatorStatus(address validator) view returns(bool isActive, bool isJailed)
func (_Staking *StakingCallerSession) GetValidatorStatus(validator common.Address) (struct {
	IsActive bool
	IsJailed bool
}, error) {
	return _Staking.Contract.GetValidatorStatus(&_Staking.CallOpts, validator)
}

// Initialized is a free data retrieval call binding the contract method 0x158ef93e.
//
// Solidity: function initialized() view returns(bool)
func (_Staking *StakingCaller) Initialized(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "initialized")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Initialized is a free data retrieval call binding the contract method 0x158ef93e.
//
// Solidity: function initialized() view returns(bool)
func (_Staking *StakingSession) Initialized() (bool, error) {
	return _Staking.Contract.Initialized(&_Staking.CallOpts)
}

// Initialized is a free data retrieval call binding the contract method 0x158ef93e.
//
// Solidity: function initialized() view returns(bool)
func (_Staking *StakingCallerSession) Initialized() (bool, error) {
	return _Staking.Contract.Initialized(&_Staking.CallOpts)
}

// IsValidatorJailed is a free data retrieval call binding the contract method 0x60731435.
//
// Solidity: function isValidatorJailed(address validator) view returns(bool)
func (_Staking *StakingCaller) IsValidatorJailed(opts *bind.CallOpts, validator common.Address) (bool, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "isValidatorJailed", validator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsValidatorJailed is a free data retrieval call binding the contract method 0x60731435.
//
// Solidity: function isValidatorJailed(address validator) view returns(bool)
func (_Staking *StakingSession) IsValidatorJailed(validator common.Address) (bool, error) {
	return _Staking.Contract.IsValidatorJailed(&_Staking.CallOpts, validator)
}

// IsValidatorJailed is a free data retrieval call binding the contract method 0x60731435.
//
// Solidity: function isValidatorJailed(address validator) view returns(bool)
func (_Staking *StakingCallerSession) IsValidatorJailed(validator common.Address) (bool, error) {
	return _Staking.Contract.IsValidatorJailed(&_Staking.CallOpts, validator)
}

// LastActiveBlock is a free data retrieval call binding the contract method 0xe50a7db8.
//
// Solidity: function lastActiveBlock(address ) view returns(uint256)
func (_Staking *StakingCaller) LastActiveBlock(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "lastActiveBlock", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LastActiveBlock is a free data retrieval call binding the contract method 0xe50a7db8.
//
// Solidity: function lastActiveBlock(address ) view returns(uint256)
func (_Staking *StakingSession) LastActiveBlock(arg0 common.Address) (*big.Int, error) {
	return _Staking.Contract.LastActiveBlock(&_Staking.CallOpts, arg0)
}

// LastActiveBlock is a free data retrieval call binding the contract method 0xe50a7db8.
//
// Solidity: function lastActiveBlock(address ) view returns(uint256)
func (_Staking *StakingCallerSession) LastActiveBlock(arg0 common.Address) (*big.Int, error) {
	return _Staking.Contract.LastActiveBlock(&_Staking.CallOpts, arg0)
}

// LastCommissionUpdateBlock is a free data retrieval call binding the contract method 0xb43b695b.
//
// Solidity: function lastCommissionUpdateBlock(address ) view returns(uint256)
func (_Staking *StakingCaller) LastCommissionUpdateBlock(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "lastCommissionUpdateBlock", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LastCommissionUpdateBlock is a free data retrieval call binding the contract method 0xb43b695b.
//
// Solidity: function lastCommissionUpdateBlock(address ) view returns(uint256)
func (_Staking *StakingSession) LastCommissionUpdateBlock(arg0 common.Address) (*big.Int, error) {
	return _Staking.Contract.LastCommissionUpdateBlock(&_Staking.CallOpts, arg0)
}

// LastCommissionUpdateBlock is a free data retrieval call binding the contract method 0xb43b695b.
//
// Solidity: function lastCommissionUpdateBlock(address ) view returns(uint256)
func (_Staking *StakingCallerSession) LastCommissionUpdateBlock(arg0 common.Address) (*big.Int, error) {
	return _Staking.Contract.LastCommissionUpdateBlock(&_Staking.CallOpts, arg0)
}

// PendingPayouts is a free data retrieval call binding the contract method 0x784712f2.
//
// Solidity: function pendingPayouts(address ) view returns(uint256)
func (_Staking *StakingCaller) PendingPayouts(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "pendingPayouts", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PendingPayouts is a free data retrieval call binding the contract method 0x784712f2.
//
// Solidity: function pendingPayouts(address ) view returns(uint256)
func (_Staking *StakingSession) PendingPayouts(arg0 common.Address) (*big.Int, error) {
	return _Staking.Contract.PendingPayouts(&_Staking.CallOpts, arg0)
}

// PendingPayouts is a free data retrieval call binding the contract method 0x784712f2.
//
// Solidity: function pendingPayouts(address ) view returns(uint256)
func (_Staking *StakingCallerSession) PendingPayouts(arg0 common.Address) (*big.Int, error) {
	return _Staking.Contract.PendingPayouts(&_Staking.CallOpts, arg0)
}

// ProposalContract is a free data retrieval call binding the contract method 0x3aef3900.
//
// Solidity: function proposalContract() view returns(address)
func (_Staking *StakingCaller) ProposalContract(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "proposalContract")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ProposalContract is a free data retrieval call binding the contract method 0x3aef3900.
//
// Solidity: function proposalContract() view returns(address)
func (_Staking *StakingSession) ProposalContract() (common.Address, error) {
	return _Staking.Contract.ProposalContract(&_Staking.CallOpts)
}

// ProposalContract is a free data retrieval call binding the contract method 0x3aef3900.
//
// Solidity: function proposalContract() view returns(address)
func (_Staking *StakingCallerSession) ProposalContract() (common.Address, error) {
	return _Staking.Contract.ProposalContract(&_Staking.CallOpts)
}

// PunishContract is a free data retrieval call binding the contract method 0x44f99900.
//
// Solidity: function punishContract() view returns(address)
func (_Staking *StakingCaller) PunishContract(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "punishContract")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// PunishContract is a free data retrieval call binding the contract method 0x44f99900.
//
// Solidity: function punishContract() view returns(address)
func (_Staking *StakingSession) PunishContract() (common.Address, error) {
	return _Staking.Contract.PunishContract(&_Staking.CallOpts)
}

// PunishContract is a free data retrieval call binding the contract method 0x44f99900.
//
// Solidity: function punishContract() view returns(address)
func (_Staking *StakingCallerSession) PunishContract() (common.Address, error) {
	return _Staking.Contract.PunishContract(&_Staking.CallOpts)
}

// Revision is a free data retrieval call binding the contract method 0x7cc96380.
//
// Solidity: function revision() view returns(uint256)
func (_Staking *StakingCaller) Revision(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "revision")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Revision is a free data retrieval call binding the contract method 0x7cc96380.
//
// Solidity: function revision() view returns(uint256)
func (_Staking *StakingSession) Revision() (*big.Int, error) {
	return _Staking.Contract.Revision(&_Staking.CallOpts)
}

// Revision is a free data retrieval call binding the contract method 0x7cc96380.
//
// Solidity: function revision() view returns(uint256)
func (_Staking *StakingCallerSession) Revision() (*big.Int, error) {
	return _Staking.Contract.Revision(&_Staking.CallOpts)
}

// RewardPerShare is a free data retrieval call binding the contract method 0x531e0541.
//
// Solidity: function rewardPerShare(address ) view returns(uint256)
func (_Staking *StakingCaller) RewardPerShare(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "rewardPerShare", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// RewardPerShare is a free data retrieval call binding the contract method 0x531e0541.
//
// Solidity: function rewardPerShare(address ) view returns(uint256)
func (_Staking *StakingSession) RewardPerShare(arg0 common.Address) (*big.Int, error) {
	return _Staking.Contract.RewardPerShare(&_Staking.CallOpts, arg0)
}

// RewardPerShare is a free data retrieval call binding the contract method 0x531e0541.
//
// Solidity: function rewardPerShare(address ) view returns(uint256)
func (_Staking *StakingCallerSession) RewardPerShare(arg0 common.Address) (*big.Int, error) {
	return _Staking.Contract.RewardPerShare(&_Staking.CallOpts, arg0)
}

// TotalStaked is a free data retrieval call binding the contract method 0x817b1cd2.
//
// Solidity: function totalStaked() view returns(uint256)
func (_Staking *StakingCaller) TotalStaked(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "totalStaked")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalStaked is a free data retrieval call binding the contract method 0x817b1cd2.
//
// Solidity: function totalStaked() view returns(uint256)
func (_Staking *StakingSession) TotalStaked() (*big.Int, error) {
	return _Staking.Contract.TotalStaked(&_Staking.CallOpts)
}

// TotalStaked is a free data retrieval call binding the contract method 0x817b1cd2.
//
// Solidity: function totalStaked() view returns(uint256)
func (_Staking *StakingCallerSession) TotalStaked() (*big.Int, error) {
	return _Staking.Contract.TotalStaked(&_Staking.CallOpts)
}

// UnbondingDelegations is a free data retrieval call binding the contract method 0x4eb4b3e3.
//
// Solidity: function unbondingDelegations(address , address , uint256 ) view returns(uint256 amount, uint256 completionBlock)
func (_Staking *StakingCaller) UnbondingDelegations(opts *bind.CallOpts, arg0 common.Address, arg1 common.Address, arg2 *big.Int) (struct {
	Amount          *big.Int
	CompletionBlock *big.Int
}, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "unbondingDelegations", arg0, arg1, arg2)

	outstruct := new(struct {
		Amount          *big.Int
		CompletionBlock *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Amount = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.CompletionBlock = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// UnbondingDelegations is a free data retrieval call binding the contract method 0x4eb4b3e3.
//
// Solidity: function unbondingDelegations(address , address , uint256 ) view returns(uint256 amount, uint256 completionBlock)
func (_Staking *StakingSession) UnbondingDelegations(arg0 common.Address, arg1 common.Address, arg2 *big.Int) (struct {
	Amount          *big.Int
	CompletionBlock *big.Int
}, error) {
	return _Staking.Contract.UnbondingDelegations(&_Staking.CallOpts, arg0, arg1, arg2)
}

// UnbondingDelegations is a free data retrieval call binding the contract method 0x4eb4b3e3.
//
// Solidity: function unbondingDelegations(address , address , uint256 ) view returns(uint256 amount, uint256 completionBlock)
func (_Staking *StakingCallerSession) UnbondingDelegations(arg0 common.Address, arg1 common.Address, arg2 *big.Int) (struct {
	Amount          *big.Int
	CompletionBlock *big.Int
}, error) {
	return _Staking.Contract.UnbondingDelegations(&_Staking.CallOpts, arg0, arg1, arg2)
}

// ValidatorDelegatorCount is a free data retrieval call binding the contract method 0x46da2a94.
//
// Solidity: function validatorDelegatorCount(address ) view returns(uint256)
func (_Staking *StakingCaller) ValidatorDelegatorCount(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "validatorDelegatorCount", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ValidatorDelegatorCount is a free data retrieval call binding the contract method 0x46da2a94.
//
// Solidity: function validatorDelegatorCount(address ) view returns(uint256)
func (_Staking *StakingSession) ValidatorDelegatorCount(arg0 common.Address) (*big.Int, error) {
	return _Staking.Contract.ValidatorDelegatorCount(&_Staking.CallOpts, arg0)
}

// ValidatorDelegatorCount is a free data retrieval call binding the contract method 0x46da2a94.
//
// Solidity: function validatorDelegatorCount(address ) view returns(uint256)
func (_Staking *StakingCallerSession) ValidatorDelegatorCount(arg0 common.Address) (*big.Int, error) {
	return _Staking.Contract.ValidatorDelegatorCount(&_Staking.CallOpts, arg0)
}

// ValidatorStakes is a free data retrieval call binding the contract method 0xaa735578.
//
// Solidity: function validatorStakes(address ) view returns(uint256 selfStake, uint256 totalDelegated, uint256 commissionRate, uint256 totalRewards, uint256 accumulatedRewards, bool isJailed, uint256 jailUntilBlock, uint256 totalClaimedRewards, uint256 lastClaimBlock, bool isRegistered)
func (_Staking *StakingCaller) ValidatorStakes(opts *bind.CallOpts, arg0 common.Address) (struct {
	SelfStake           *big.Int
	TotalDelegated      *big.Int
	CommissionRate      *big.Int
	TotalRewards        *big.Int
	AccumulatedRewards  *big.Int
	IsJailed            bool
	JailUntilBlock      *big.Int
	TotalClaimedRewards *big.Int
	LastClaimBlock      *big.Int
	IsRegistered        bool
}, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "validatorStakes", arg0)

	outstruct := new(struct {
		SelfStake           *big.Int
		TotalDelegated      *big.Int
		CommissionRate      *big.Int
		TotalRewards        *big.Int
		AccumulatedRewards  *big.Int
		IsJailed            bool
		JailUntilBlock      *big.Int
		TotalClaimedRewards *big.Int
		LastClaimBlock      *big.Int
		IsRegistered        bool
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.SelfStake = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.TotalDelegated = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.CommissionRate = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.TotalRewards = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)
	outstruct.AccumulatedRewards = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)
	outstruct.IsJailed = *abi.ConvertType(out[5], new(bool)).(*bool)
	outstruct.JailUntilBlock = *abi.ConvertType(out[6], new(*big.Int)).(**big.Int)
	outstruct.TotalClaimedRewards = *abi.ConvertType(out[7], new(*big.Int)).(**big.Int)
	outstruct.LastClaimBlock = *abi.ConvertType(out[8], new(*big.Int)).(**big.Int)
	outstruct.IsRegistered = *abi.ConvertType(out[9], new(bool)).(*bool)

	return *outstruct, err

}

// ValidatorStakes is a free data retrieval call binding the contract method 0xaa735578.
//
// Solidity: function validatorStakes(address ) view returns(uint256 selfStake, uint256 totalDelegated, uint256 commissionRate, uint256 totalRewards, uint256 accumulatedRewards, bool isJailed, uint256 jailUntilBlock, uint256 totalClaimedRewards, uint256 lastClaimBlock, bool isRegistered)
func (_Staking *StakingSession) ValidatorStakes(arg0 common.Address) (struct {
	SelfStake           *big.Int
	TotalDelegated      *big.Int
	CommissionRate      *big.Int
	TotalRewards        *big.Int
	AccumulatedRewards  *big.Int
	IsJailed            bool
	JailUntilBlock      *big.Int
	TotalClaimedRewards *big.Int
	LastClaimBlock      *big.Int
	IsRegistered        bool
}, error) {
	return _Staking.Contract.ValidatorStakes(&_Staking.CallOpts, arg0)
}

// ValidatorStakes is a free data retrieval call binding the contract method 0xaa735578.
//
// Solidity: function validatorStakes(address ) view returns(uint256 selfStake, uint256 totalDelegated, uint256 commissionRate, uint256 totalRewards, uint256 accumulatedRewards, bool isJailed, uint256 jailUntilBlock, uint256 totalClaimedRewards, uint256 lastClaimBlock, bool isRegistered)
func (_Staking *StakingCallerSession) ValidatorStakes(arg0 common.Address) (struct {
	SelfStake           *big.Int
	TotalDelegated      *big.Int
	CommissionRate      *big.Int
	TotalRewards        *big.Int
	AccumulatedRewards  *big.Int
	IsJailed            bool
	JailUntilBlock      *big.Int
	TotalClaimedRewards *big.Int
	LastClaimBlock      *big.Int
	IsRegistered        bool
}, error) {
	return _Staking.Contract.ValidatorStakes(&_Staking.CallOpts, arg0)
}

// ValidatorsAddedInEpoch is a free data retrieval call binding the contract method 0x266f55bb.
//
// Solidity: function validatorsAddedInEpoch() view returns(uint256)
func (_Staking *StakingCaller) ValidatorsAddedInEpoch(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "validatorsAddedInEpoch")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ValidatorsAddedInEpoch is a free data retrieval call binding the contract method 0x266f55bb.
//
// Solidity: function validatorsAddedInEpoch() view returns(uint256)
func (_Staking *StakingSession) ValidatorsAddedInEpoch() (*big.Int, error) {
	return _Staking.Contract.ValidatorsAddedInEpoch(&_Staking.CallOpts)
}

// ValidatorsAddedInEpoch is a free data retrieval call binding the contract method 0x266f55bb.
//
// Solidity: function validatorsAddedInEpoch() view returns(uint256)
func (_Staking *StakingCallerSession) ValidatorsAddedInEpoch() (*big.Int, error) {
	return _Staking.Contract.ValidatorsAddedInEpoch(&_Staking.CallOpts)
}

// ValidatorsContract is a free data retrieval call binding the contract method 0x71a1bb75.
//
// Solidity: function validatorsContract() view returns(address)
func (_Staking *StakingCaller) ValidatorsContract(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "validatorsContract")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ValidatorsContract is a free data retrieval call binding the contract method 0x71a1bb75.
//
// Solidity: function validatorsContract() view returns(address)
func (_Staking *StakingSession) ValidatorsContract() (common.Address, error) {
	return _Staking.Contract.ValidatorsContract(&_Staking.CallOpts)
}

// ValidatorsContract is a free data retrieval call binding the contract method 0x71a1bb75.
//
// Solidity: function validatorsContract() view returns(address)
func (_Staking *StakingCallerSession) ValidatorsContract() (common.Address, error) {
	return _Staking.Contract.ValidatorsContract(&_Staking.CallOpts)
}

// ValidatorsRemovedInEpoch is a free data retrieval call binding the contract method 0x4815b341.
//
// Solidity: function validatorsRemovedInEpoch() view returns(uint256)
func (_Staking *StakingCaller) ValidatorsRemovedInEpoch(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Staking.contract.Call(opts, &out, "validatorsRemovedInEpoch")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ValidatorsRemovedInEpoch is a free data retrieval call binding the contract method 0x4815b341.
//
// Solidity: function validatorsRemovedInEpoch() view returns(uint256)
func (_Staking *StakingSession) ValidatorsRemovedInEpoch() (*big.Int, error) {
	return _Staking.Contract.ValidatorsRemovedInEpoch(&_Staking.CallOpts)
}

// ValidatorsRemovedInEpoch is a free data retrieval call binding the contract method 0x4815b341.
//
// Solidity: function validatorsRemovedInEpoch() view returns(uint256)
func (_Staking *StakingCallerSession) ValidatorsRemovedInEpoch() (*big.Int, error) {
	return _Staking.Contract.ValidatorsRemovedInEpoch(&_Staking.CallOpts)
}

// AddValidatorStake is a paid mutator transaction binding the contract method 0x95e02669.
//
// Solidity: function addValidatorStake() payable returns()
func (_Staking *StakingTransactor) AddValidatorStake(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "addValidatorStake")
}

// AddValidatorStake is a paid mutator transaction binding the contract method 0x95e02669.
//
// Solidity: function addValidatorStake() payable returns()
func (_Staking *StakingSession) AddValidatorStake() (*types.Transaction, error) {
	return _Staking.Contract.AddValidatorStake(&_Staking.TransactOpts)
}

// AddValidatorStake is a paid mutator transaction binding the contract method 0x95e02669.
//
// Solidity: function addValidatorStake() payable returns()
func (_Staking *StakingTransactorSession) AddValidatorStake() (*types.Transaction, error) {
	return _Staking.Contract.AddValidatorStake(&_Staking.TransactOpts)
}

// ClaimRewards is a paid mutator transaction binding the contract method 0xef5cfb8c.
//
// Solidity: function claimRewards(address validator) returns()
func (_Staking *StakingTransactor) ClaimRewards(opts *bind.TransactOpts, validator common.Address) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "claimRewards", validator)
}

// ClaimRewards is a paid mutator transaction binding the contract method 0xef5cfb8c.
//
// Solidity: function claimRewards(address validator) returns()
func (_Staking *StakingSession) ClaimRewards(validator common.Address) (*types.Transaction, error) {
	return _Staking.Contract.ClaimRewards(&_Staking.TransactOpts, validator)
}

// ClaimRewards is a paid mutator transaction binding the contract method 0xef5cfb8c.
//
// Solidity: function claimRewards(address validator) returns()
func (_Staking *StakingTransactorSession) ClaimRewards(validator common.Address) (*types.Transaction, error) {
	return _Staking.Contract.ClaimRewards(&_Staking.TransactOpts, validator)
}

// ClaimValidatorRewards is a paid mutator transaction binding the contract method 0xbe22c64c.
//
// Solidity: function claimValidatorRewards() returns()
func (_Staking *StakingTransactor) ClaimValidatorRewards(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "claimValidatorRewards")
}

// ClaimValidatorRewards is a paid mutator transaction binding the contract method 0xbe22c64c.
//
// Solidity: function claimValidatorRewards() returns()
func (_Staking *StakingSession) ClaimValidatorRewards() (*types.Transaction, error) {
	return _Staking.Contract.ClaimValidatorRewards(&_Staking.TransactOpts)
}

// ClaimValidatorRewards is a paid mutator transaction binding the contract method 0xbe22c64c.
//
// Solidity: function claimValidatorRewards() returns()
func (_Staking *StakingTransactorSession) ClaimValidatorRewards() (*types.Transaction, error) {
	return _Staking.Contract.ClaimValidatorRewards(&_Staking.TransactOpts)
}

// DecreaseValidatorStake is a paid mutator transaction binding the contract method 0xff6061b2.
//
// Solidity: function decreaseValidatorStake(uint256 amount) returns()
func (_Staking *StakingTransactor) DecreaseValidatorStake(opts *bind.TransactOpts, amount *big.Int) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "decreaseValidatorStake", amount)
}

// DecreaseValidatorStake is a paid mutator transaction binding the contract method 0xff6061b2.
//
// Solidity: function decreaseValidatorStake(uint256 amount) returns()
func (_Staking *StakingSession) DecreaseValidatorStake(amount *big.Int) (*types.Transaction, error) {
	return _Staking.Contract.DecreaseValidatorStake(&_Staking.TransactOpts, amount)
}

// DecreaseValidatorStake is a paid mutator transaction binding the contract method 0xff6061b2.
//
// Solidity: function decreaseValidatorStake(uint256 amount) returns()
func (_Staking *StakingTransactorSession) DecreaseValidatorStake(amount *big.Int) (*types.Transaction, error) {
	return _Staking.Contract.DecreaseValidatorStake(&_Staking.TransactOpts, amount)
}

// Delegate is a paid mutator transaction binding the contract method 0x5c19a95c.
//
// Solidity: function delegate(address validator) payable returns()
func (_Staking *StakingTransactor) Delegate(opts *bind.TransactOpts, validator common.Address) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "delegate", validator)
}

// Delegate is a paid mutator transaction binding the contract method 0x5c19a95c.
//
// Solidity: function delegate(address validator) payable returns()
func (_Staking *StakingSession) Delegate(validator common.Address) (*types.Transaction, error) {
	return _Staking.Contract.Delegate(&_Staking.TransactOpts, validator)
}

// Delegate is a paid mutator transaction binding the contract method 0x5c19a95c.
//
// Solidity: function delegate(address validator) payable returns()
func (_Staking *StakingTransactorSession) Delegate(validator common.Address) (*types.Transaction, error) {
	return _Staking.Contract.Delegate(&_Staking.TransactOpts, validator)
}

// DistributeRewards is a paid mutator transaction binding the contract method 0x6f4a2cd0.
//
// Solidity: function distributeRewards() payable returns()
func (_Staking *StakingTransactor) DistributeRewards(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "distributeRewards")
}

// DistributeRewards is a paid mutator transaction binding the contract method 0x6f4a2cd0.
//
// Solidity: function distributeRewards() payable returns()
func (_Staking *StakingSession) DistributeRewards() (*types.Transaction, error) {
	return _Staking.Contract.DistributeRewards(&_Staking.TransactOpts)
}

// DistributeRewards is a paid mutator transaction binding the contract method 0x6f4a2cd0.
//
// Solidity: function distributeRewards() payable returns()
func (_Staking *StakingTransactorSession) DistributeRewards() (*types.Transaction, error) {
	return _Staking.Contract.DistributeRewards(&_Staking.TransactOpts)
}

// ExitValidator is a paid mutator transaction binding the contract method 0xb4669217.
//
// Solidity: function exitValidator() returns()
func (_Staking *StakingTransactor) ExitValidator(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "exitValidator")
}

// ExitValidator is a paid mutator transaction binding the contract method 0xb4669217.
//
// Solidity: function exitValidator() returns()
func (_Staking *StakingSession) ExitValidator() (*types.Transaction, error) {
	return _Staking.Contract.ExitValidator(&_Staking.TransactOpts)
}

// ExitValidator is a paid mutator transaction binding the contract method 0xb4669217.
//
// Solidity: function exitValidator() returns()
func (_Staking *StakingTransactorSession) ExitValidator() (*types.Transaction, error) {
	return _Staking.Contract.ExitValidator(&_Staking.TransactOpts)
}

// Initialize is a paid mutator transaction binding the contract method 0xc0c53b8b.
//
// Solidity: function initialize(address validators_, address proposal_, address punish_) returns()
func (_Staking *StakingTransactor) Initialize(opts *bind.TransactOpts, validators_ common.Address, proposal_ common.Address, punish_ common.Address) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "initialize", validators_, proposal_, punish_)
}

// Initialize is a paid mutator transaction binding the contract method 0xc0c53b8b.
//
// Solidity: function initialize(address validators_, address proposal_, address punish_) returns()
func (_Staking *StakingSession) Initialize(validators_ common.Address, proposal_ common.Address, punish_ common.Address) (*types.Transaction, error) {
	return _Staking.Contract.Initialize(&_Staking.TransactOpts, validators_, proposal_, punish_)
}

// Initialize is a paid mutator transaction binding the contract method 0xc0c53b8b.
//
// Solidity: function initialize(address validators_, address proposal_, address punish_) returns()
func (_Staking *StakingTransactorSession) Initialize(validators_ common.Address, proposal_ common.Address, punish_ common.Address) (*types.Transaction, error) {
	return _Staking.Contract.Initialize(&_Staking.TransactOpts, validators_, proposal_, punish_)
}

// InitializeWithValidators is a paid mutator transaction binding the contract method 0x72c44ba1.
//
// Solidity: function initializeWithValidators(address validators_, address proposal_, address punish_, address[] initialValidators, uint256 commissionRate) returns()
func (_Staking *StakingTransactor) InitializeWithValidators(opts *bind.TransactOpts, validators_ common.Address, proposal_ common.Address, punish_ common.Address, initialValidators []common.Address, commissionRate *big.Int) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "initializeWithValidators", validators_, proposal_, punish_, initialValidators, commissionRate)
}

// InitializeWithValidators is a paid mutator transaction binding the contract method 0x72c44ba1.
//
// Solidity: function initializeWithValidators(address validators_, address proposal_, address punish_, address[] initialValidators, uint256 commissionRate) returns()
func (_Staking *StakingSession) InitializeWithValidators(validators_ common.Address, proposal_ common.Address, punish_ common.Address, initialValidators []common.Address, commissionRate *big.Int) (*types.Transaction, error) {
	return _Staking.Contract.InitializeWithValidators(&_Staking.TransactOpts, validators_, proposal_, punish_, initialValidators, commissionRate)
}

// InitializeWithValidators is a paid mutator transaction binding the contract method 0x72c44ba1.
//
// Solidity: function initializeWithValidators(address validators_, address proposal_, address punish_, address[] initialValidators, uint256 commissionRate) returns()
func (_Staking *StakingTransactorSession) InitializeWithValidators(validators_ common.Address, proposal_ common.Address, punish_ common.Address, initialValidators []common.Address, commissionRate *big.Int) (*types.Transaction, error) {
	return _Staking.Contract.InitializeWithValidators(&_Staking.TransactOpts, validators_, proposal_, punish_, initialValidators, commissionRate)
}

// JailValidator is a paid mutator transaction binding the contract method 0x5a4d66c0.
//
// Solidity: function jailValidator(address validator, uint256 jailBlocks) returns()
func (_Staking *StakingTransactor) JailValidator(opts *bind.TransactOpts, validator common.Address, jailBlocks *big.Int) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "jailValidator", validator, jailBlocks)
}

// JailValidator is a paid mutator transaction binding the contract method 0x5a4d66c0.
//
// Solidity: function jailValidator(address validator, uint256 jailBlocks) returns()
func (_Staking *StakingSession) JailValidator(validator common.Address, jailBlocks *big.Int) (*types.Transaction, error) {
	return _Staking.Contract.JailValidator(&_Staking.TransactOpts, validator, jailBlocks)
}

// JailValidator is a paid mutator transaction binding the contract method 0x5a4d66c0.
//
// Solidity: function jailValidator(address validator, uint256 jailBlocks) returns()
func (_Staking *StakingTransactorSession) JailValidator(validator common.Address, jailBlocks *big.Int) (*types.Transaction, error) {
	return _Staking.Contract.JailValidator(&_Staking.TransactOpts, validator, jailBlocks)
}

// RegisterValidator is a paid mutator transaction binding the contract method 0x679cdb06.
//
// Solidity: function registerValidator(uint256 commissionRate) payable returns()
func (_Staking *StakingTransactor) RegisterValidator(opts *bind.TransactOpts, commissionRate *big.Int) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "registerValidator", commissionRate)
}

// RegisterValidator is a paid mutator transaction binding the contract method 0x679cdb06.
//
// Solidity: function registerValidator(uint256 commissionRate) payable returns()
func (_Staking *StakingSession) RegisterValidator(commissionRate *big.Int) (*types.Transaction, error) {
	return _Staking.Contract.RegisterValidator(&_Staking.TransactOpts, commissionRate)
}

// RegisterValidator is a paid mutator transaction binding the contract method 0x679cdb06.
//
// Solidity: function registerValidator(uint256 commissionRate) payable returns()
func (_Staking *StakingTransactorSession) RegisterValidator(commissionRate *big.Int) (*types.Transaction, error) {
	return _Staking.Contract.RegisterValidator(&_Staking.TransactOpts, commissionRate)
}

// ReinitializeV2 is a paid mutator transaction binding the contract method 0xc4115874.
//
// Solidity: function reinitializeV2() returns()
func (_Staking *StakingTransactor) ReinitializeV2(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "reinitializeV2")
}

// ReinitializeV2 is a paid mutator transaction binding the contract method 0xc4115874.
//
// Solidity: function reinitializeV2() returns()
func (_Staking *StakingSession) ReinitializeV2() (*types.Transaction, error) {
	return _Staking.Contract.ReinitializeV2(&_Staking.TransactOpts)
}

// ReinitializeV2 is a paid mutator transaction binding the contract method 0xc4115874.
//
// Solidity: function reinitializeV2() returns()
func (_Staking *StakingTransactorSession) ReinitializeV2() (*types.Transaction, error) {
	return _Staking.Contract.ReinitializeV2(&_Staking.TransactOpts)
}

// ResignValidator is a paid mutator transaction binding the contract method 0xb85f5da2.
//
// Solidity: function resignValidator() returns()
func (_Staking *StakingTransactor) ResignValidator(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "resignValidator")
}

// ResignValidator is a paid mutator transaction binding the contract method 0xb85f5da2.
//
// Solidity: function resignValidator() returns()
func (_Staking *StakingSession) ResignValidator() (*types.Transaction, error) {
	return _Staking.Contract.ResignValidator(&_Staking.TransactOpts)
}

// ResignValidator is a paid mutator transaction binding the contract method 0xb85f5da2.
//
// Solidity: function resignValidator() returns()
func (_Staking *StakingTransactorSession) ResignValidator() (*types.Transaction, error) {
	return _Staking.Contract.ResignValidator(&_Staking.TransactOpts)
}

// SlashValidator is a paid mutator transaction binding the contract method 0x1c4e449a.
//
// Solidity: function slashValidator(address validator, uint256 slashAmount, address reporter, uint256 rewardAmount, address burnAddress) returns(uint256 actualSlash, uint256 actualReward)
func (_Staking *StakingTransactor) SlashValidator(opts *bind.TransactOpts, validator common.Address, slashAmount *big.Int, reporter common.Address, rewardAmount *big.Int, burnAddress common.Address) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "slashValidator", validator, slashAmount, reporter, rewardAmount, burnAddress)
}

// SlashValidator is a paid mutator transaction binding the contract method 0x1c4e449a.
//
// Solidity: function slashValidator(address validator, uint256 slashAmount, address reporter, uint256 rewardAmount, address burnAddress) returns(uint256 actualSlash, uint256 actualReward)
func (_Staking *StakingSession) SlashValidator(validator common.Address, slashAmount *big.Int, reporter common.Address, rewardAmount *big.Int, burnAddress common.Address) (*types.Transaction, error) {
	return _Staking.Contract.SlashValidator(&_Staking.TransactOpts, validator, slashAmount, reporter, rewardAmount, burnAddress)
}

// SlashValidator is a paid mutator transaction binding the contract method 0x1c4e449a.
//
// Solidity: function slashValidator(address validator, uint256 slashAmount, address reporter, uint256 rewardAmount, address burnAddress) returns(uint256 actualSlash, uint256 actualReward)
func (_Staking *StakingTransactorSession) SlashValidator(validator common.Address, slashAmount *big.Int, reporter common.Address, rewardAmount *big.Int, burnAddress common.Address) (*types.Transaction, error) {
	return _Staking.Contract.SlashValidator(&_Staking.TransactOpts, validator, slashAmount, reporter, rewardAmount, burnAddress)
}

// Undelegate is a paid mutator transaction binding the contract method 0x4d99dd16.
//
// Solidity: function undelegate(address validator, uint256 amount) returns()
func (_Staking *StakingTransactor) Undelegate(opts *bind.TransactOpts, validator common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "undelegate", validator, amount)
}

// Undelegate is a paid mutator transaction binding the contract method 0x4d99dd16.
//
// Solidity: function undelegate(address validator, uint256 amount) returns()
func (_Staking *StakingSession) Undelegate(validator common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Staking.Contract.Undelegate(&_Staking.TransactOpts, validator, amount)
}

// Undelegate is a paid mutator transaction binding the contract method 0x4d99dd16.
//
// Solidity: function undelegate(address validator, uint256 amount) returns()
func (_Staking *StakingTransactorSession) Undelegate(validator common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Staking.Contract.Undelegate(&_Staking.TransactOpts, validator, amount)
}

// UnjailValidator is a paid mutator transaction binding the contract method 0x7cafdd79.
//
// Solidity: function unjailValidator(address validator) returns()
func (_Staking *StakingTransactor) UnjailValidator(opts *bind.TransactOpts, validator common.Address) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "unjailValidator", validator)
}

// UnjailValidator is a paid mutator transaction binding the contract method 0x7cafdd79.
//
// Solidity: function unjailValidator(address validator) returns()
func (_Staking *StakingSession) UnjailValidator(validator common.Address) (*types.Transaction, error) {
	return _Staking.Contract.UnjailValidator(&_Staking.TransactOpts, validator)
}

// UnjailValidator is a paid mutator transaction binding the contract method 0x7cafdd79.
//
// Solidity: function unjailValidator(address validator) returns()
func (_Staking *StakingTransactorSession) UnjailValidator(validator common.Address) (*types.Transaction, error) {
	return _Staking.Contract.UnjailValidator(&_Staking.TransactOpts, validator)
}

// UpdateCommissionRate is a paid mutator transaction binding the contract method 0x00fa3d50.
//
// Solidity: function updateCommissionRate(uint256 newCommissionRate) returns()
func (_Staking *StakingTransactor) UpdateCommissionRate(opts *bind.TransactOpts, newCommissionRate *big.Int) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "updateCommissionRate", newCommissionRate)
}

// UpdateCommissionRate is a paid mutator transaction binding the contract method 0x00fa3d50.
//
// Solidity: function updateCommissionRate(uint256 newCommissionRate) returns()
func (_Staking *StakingSession) UpdateCommissionRate(newCommissionRate *big.Int) (*types.Transaction, error) {
	return _Staking.Contract.UpdateCommissionRate(&_Staking.TransactOpts, newCommissionRate)
}

// UpdateCommissionRate is a paid mutator transaction binding the contract method 0x00fa3d50.
//
// Solidity: function updateCommissionRate(uint256 newCommissionRate) returns()
func (_Staking *StakingTransactorSession) UpdateCommissionRate(newCommissionRate *big.Int) (*types.Transaction, error) {
	return _Staking.Contract.UpdateCommissionRate(&_Staking.TransactOpts, newCommissionRate)
}

// UpdateLastActiveBlock is a paid mutator transaction binding the contract method 0xe691e8f0.
//
// Solidity: function updateLastActiveBlock(address validator) returns()
func (_Staking *StakingTransactor) UpdateLastActiveBlock(opts *bind.TransactOpts, validator common.Address) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "updateLastActiveBlock", validator)
}

// UpdateLastActiveBlock is a paid mutator transaction binding the contract method 0xe691e8f0.
//
// Solidity: function updateLastActiveBlock(address validator) returns()
func (_Staking *StakingSession) UpdateLastActiveBlock(validator common.Address) (*types.Transaction, error) {
	return _Staking.Contract.UpdateLastActiveBlock(&_Staking.TransactOpts, validator)
}

// UpdateLastActiveBlock is a paid mutator transaction binding the contract method 0xe691e8f0.
//
// Solidity: function updateLastActiveBlock(address validator) returns()
func (_Staking *StakingTransactorSession) UpdateLastActiveBlock(validator common.Address) (*types.Transaction, error) {
	return _Staking.Contract.UpdateLastActiveBlock(&_Staking.TransactOpts, validator)
}

// WithdrawPendingPayout is a paid mutator transaction binding the contract method 0x3b058e42.
//
// Solidity: function withdrawPendingPayout(address recipient) returns()
func (_Staking *StakingTransactor) WithdrawPendingPayout(opts *bind.TransactOpts, recipient common.Address) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "withdrawPendingPayout", recipient)
}

// WithdrawPendingPayout is a paid mutator transaction binding the contract method 0x3b058e42.
//
// Solidity: function withdrawPendingPayout(address recipient) returns()
func (_Staking *StakingSession) WithdrawPendingPayout(recipient common.Address) (*types.Transaction, error) {
	return _Staking.Contract.WithdrawPendingPayout(&_Staking.TransactOpts, recipient)
}

// WithdrawPendingPayout is a paid mutator transaction binding the contract method 0x3b058e42.
//
// Solidity: function withdrawPendingPayout(address recipient) returns()
func (_Staking *StakingTransactorSession) WithdrawPendingPayout(recipient common.Address) (*types.Transaction, error) {
	return _Staking.Contract.WithdrawPendingPayout(&_Staking.TransactOpts, recipient)
}

// WithdrawUnbonded is a paid mutator transaction binding the contract method 0x96902c82.
//
// Solidity: function withdrawUnbonded(address validator, uint256 maxEntries) returns()
func (_Staking *StakingTransactor) WithdrawUnbonded(opts *bind.TransactOpts, validator common.Address, maxEntries *big.Int) (*types.Transaction, error) {
	return _Staking.contract.Transact(opts, "withdrawUnbonded", validator, maxEntries)
}

// WithdrawUnbonded is a paid mutator transaction binding the contract method 0x96902c82.
//
// Solidity: function withdrawUnbonded(address validator, uint256 maxEntries) returns()
func (_Staking *StakingSession) WithdrawUnbonded(validator common.Address, maxEntries *big.Int) (*types.Transaction, error) {
	return _Staking.Contract.WithdrawUnbonded(&_Staking.TransactOpts, validator, maxEntries)
}

// WithdrawUnbonded is a paid mutator transaction binding the contract method 0x96902c82.
//
// Solidity: function withdrawUnbonded(address validator, uint256 maxEntries) returns()
func (_Staking *StakingTransactorSession) WithdrawUnbonded(validator common.Address, maxEntries *big.Int) (*types.Transaction, error) {
	return _Staking.Contract.WithdrawUnbonded(&_Staking.TransactOpts, validator, maxEntries)
}

// StakingCommissionRateUpdatedIterator is returned from FilterCommissionRateUpdated and is used to iterate over the raw logs and unpacked data for CommissionRateUpdated events raised by the Staking contract.
type StakingCommissionRateUpdatedIterator struct {
	Event *StakingCommissionRateUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingCommissionRateUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingCommissionRateUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingCommissionRateUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingCommissionRateUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingCommissionRateUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingCommissionRateUpdated represents a CommissionRateUpdated event raised by the Staking contract.
type StakingCommissionRateUpdated struct {
	Validator      common.Address
	CommissionRate *big.Int
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterCommissionRateUpdated is a free log retrieval operation binding the contract event 0x86d576c20e383fc2413ef692209cc48ddad5e52f25db5b32f8f7ec5076461ae9.
//
// Solidity: event CommissionRateUpdated(address indexed validator, uint256 commissionRate)
func (_Staking *StakingFilterer) FilterCommissionRateUpdated(opts *bind.FilterOpts, validator []common.Address) (*StakingCommissionRateUpdatedIterator, error) {

	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "CommissionRateUpdated", validatorRule)
	if err != nil {
		return nil, err
	}
	return &StakingCommissionRateUpdatedIterator{contract: _Staking.contract, event: "CommissionRateUpdated", logs: logs, sub: sub}, nil
}

// WatchCommissionRateUpdated is a free log subscription operation binding the contract event 0x86d576c20e383fc2413ef692209cc48ddad5e52f25db5b32f8f7ec5076461ae9.
//
// Solidity: event CommissionRateUpdated(address indexed validator, uint256 commissionRate)
func (_Staking *StakingFilterer) WatchCommissionRateUpdated(opts *bind.WatchOpts, sink chan<- *StakingCommissionRateUpdated, validator []common.Address) (event.Subscription, error) {

	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "CommissionRateUpdated", validatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingCommissionRateUpdated)
				if err := _Staking.contract.UnpackLog(event, "CommissionRateUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCommissionRateUpdated is a log parse operation binding the contract event 0x86d576c20e383fc2413ef692209cc48ddad5e52f25db5b32f8f7ec5076461ae9.
//
// Solidity: event CommissionRateUpdated(address indexed validator, uint256 commissionRate)
func (_Staking *StakingFilterer) ParseCommissionRateUpdated(log types.Log) (*StakingCommissionRateUpdated, error) {
	event := new(StakingCommissionRateUpdated)
	if err := _Staking.contract.UnpackLog(event, "CommissionRateUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StakingDelegatedIterator is returned from FilterDelegated and is used to iterate over the raw logs and unpacked data for Delegated events raised by the Staking contract.
type StakingDelegatedIterator struct {
	Event *StakingDelegated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingDelegatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingDelegated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingDelegated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingDelegatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingDelegatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingDelegated represents a Delegated event raised by the Staking contract.
type StakingDelegated struct {
	Delegator common.Address
	Validator common.Address
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterDelegated is a free log retrieval operation binding the contract event 0xe5541a6b6103d4fa7e021ed54fad39c66f27a76bd13d374cf6240ae6bd0bb72b.
//
// Solidity: event Delegated(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) FilterDelegated(opts *bind.FilterOpts, delegator []common.Address, validator []common.Address) (*StakingDelegatedIterator, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}
	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "Delegated", delegatorRule, validatorRule)
	if err != nil {
		return nil, err
	}
	return &StakingDelegatedIterator{contract: _Staking.contract, event: "Delegated", logs: logs, sub: sub}, nil
}

// WatchDelegated is a free log subscription operation binding the contract event 0xe5541a6b6103d4fa7e021ed54fad39c66f27a76bd13d374cf6240ae6bd0bb72b.
//
// Solidity: event Delegated(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) WatchDelegated(opts *bind.WatchOpts, sink chan<- *StakingDelegated, delegator []common.Address, validator []common.Address) (event.Subscription, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}
	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "Delegated", delegatorRule, validatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingDelegated)
				if err := _Staking.contract.UnpackLog(event, "Delegated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDelegated is a log parse operation binding the contract event 0xe5541a6b6103d4fa7e021ed54fad39c66f27a76bd13d374cf6240ae6bd0bb72b.
//
// Solidity: event Delegated(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) ParseDelegated(log types.Log) (*StakingDelegated, error) {
	event := new(StakingDelegated)
	if err := _Staking.contract.UnpackLog(event, "Delegated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StakingPendingPayoutQueuedIterator is returned from FilterPendingPayoutQueued and is used to iterate over the raw logs and unpacked data for PendingPayoutQueued events raised by the Staking contract.
type StakingPendingPayoutQueuedIterator struct {
	Event *StakingPendingPayoutQueued // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingPendingPayoutQueuedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingPendingPayoutQueued)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingPendingPayoutQueued)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingPendingPayoutQueuedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingPendingPayoutQueuedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingPendingPayoutQueued represents a PendingPayoutQueued event raised by the Staking contract.
type StakingPendingPayoutQueued struct {
	Account common.Address
	Amount  *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterPendingPayoutQueued is a free log retrieval operation binding the contract event 0x55b06c80d6c74575c15af6a6b40b8b909be9ec4976c7641beb80036e9b1986e8.
//
// Solidity: event PendingPayoutQueued(address indexed account, uint256 amount)
func (_Staking *StakingFilterer) FilterPendingPayoutQueued(opts *bind.FilterOpts, account []common.Address) (*StakingPendingPayoutQueuedIterator, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "PendingPayoutQueued", accountRule)
	if err != nil {
		return nil, err
	}
	return &StakingPendingPayoutQueuedIterator{contract: _Staking.contract, event: "PendingPayoutQueued", logs: logs, sub: sub}, nil
}

// WatchPendingPayoutQueued is a free log subscription operation binding the contract event 0x55b06c80d6c74575c15af6a6b40b8b909be9ec4976c7641beb80036e9b1986e8.
//
// Solidity: event PendingPayoutQueued(address indexed account, uint256 amount)
func (_Staking *StakingFilterer) WatchPendingPayoutQueued(opts *bind.WatchOpts, sink chan<- *StakingPendingPayoutQueued, account []common.Address) (event.Subscription, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "PendingPayoutQueued", accountRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingPendingPayoutQueued)
				if err := _Staking.contract.UnpackLog(event, "PendingPayoutQueued", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePendingPayoutQueued is a log parse operation binding the contract event 0x55b06c80d6c74575c15af6a6b40b8b909be9ec4976c7641beb80036e9b1986e8.
//
// Solidity: event PendingPayoutQueued(address indexed account, uint256 amount)
func (_Staking *StakingFilterer) ParsePendingPayoutQueued(log types.Log) (*StakingPendingPayoutQueued, error) {
	event := new(StakingPendingPayoutQueued)
	if err := _Staking.contract.UnpackLog(event, "PendingPayoutQueued", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StakingPendingPayoutWithdrawnIterator is returned from FilterPendingPayoutWithdrawn and is used to iterate over the raw logs and unpacked data for PendingPayoutWithdrawn events raised by the Staking contract.
type StakingPendingPayoutWithdrawnIterator struct {
	Event *StakingPendingPayoutWithdrawn // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingPendingPayoutWithdrawnIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingPendingPayoutWithdrawn)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingPendingPayoutWithdrawn)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingPendingPayoutWithdrawnIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingPendingPayoutWithdrawnIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingPendingPayoutWithdrawn represents a PendingPayoutWithdrawn event raised by the Staking contract.
type StakingPendingPayoutWithdrawn struct {
	Account   common.Address
	Recipient common.Address
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterPendingPayoutWithdrawn is a free log retrieval operation binding the contract event 0x3c00101edd308ddcdda38bff41fc7dc07c50174f055cda38460ff1c389c90305.
//
// Solidity: event PendingPayoutWithdrawn(address indexed account, address indexed recipient, uint256 amount)
func (_Staking *StakingFilterer) FilterPendingPayoutWithdrawn(opts *bind.FilterOpts, account []common.Address, recipient []common.Address) (*StakingPendingPayoutWithdrawnIterator, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "PendingPayoutWithdrawn", accountRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return &StakingPendingPayoutWithdrawnIterator{contract: _Staking.contract, event: "PendingPayoutWithdrawn", logs: logs, sub: sub}, nil
}

// WatchPendingPayoutWithdrawn is a free log subscription operation binding the contract event 0x3c00101edd308ddcdda38bff41fc7dc07c50174f055cda38460ff1c389c90305.
//
// Solidity: event PendingPayoutWithdrawn(address indexed account, address indexed recipient, uint256 amount)
func (_Staking *StakingFilterer) WatchPendingPayoutWithdrawn(opts *bind.WatchOpts, sink chan<- *StakingPendingPayoutWithdrawn, account []common.Address, recipient []common.Address) (event.Subscription, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "PendingPayoutWithdrawn", accountRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingPendingPayoutWithdrawn)
				if err := _Staking.contract.UnpackLog(event, "PendingPayoutWithdrawn", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePendingPayoutWithdrawn is a log parse operation binding the contract event 0x3c00101edd308ddcdda38bff41fc7dc07c50174f055cda38460ff1c389c90305.
//
// Solidity: event PendingPayoutWithdrawn(address indexed account, address indexed recipient, uint256 amount)
func (_Staking *StakingFilterer) ParsePendingPayoutWithdrawn(log types.Log) (*StakingPendingPayoutWithdrawn, error) {
	event := new(StakingPendingPayoutWithdrawn)
	if err := _Staking.contract.UnpackLog(event, "PendingPayoutWithdrawn", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StakingRewardsClaimedIterator is returned from FilterRewardsClaimed and is used to iterate over the raw logs and unpacked data for RewardsClaimed events raised by the Staking contract.
type StakingRewardsClaimedIterator struct {
	Event *StakingRewardsClaimed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingRewardsClaimedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingRewardsClaimed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingRewardsClaimed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingRewardsClaimedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingRewardsClaimedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingRewardsClaimed represents a RewardsClaimed event raised by the Staking contract.
type StakingRewardsClaimed struct {
	Delegator common.Address
	Validator common.Address
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterRewardsClaimed is a free log retrieval operation binding the contract event 0x9310ccfcb8de723f578a9e4282ea9f521f05ae40dc08f3068dfad528a65ee3c7.
//
// Solidity: event RewardsClaimed(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) FilterRewardsClaimed(opts *bind.FilterOpts, delegator []common.Address, validator []common.Address) (*StakingRewardsClaimedIterator, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}
	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "RewardsClaimed", delegatorRule, validatorRule)
	if err != nil {
		return nil, err
	}
	return &StakingRewardsClaimedIterator{contract: _Staking.contract, event: "RewardsClaimed", logs: logs, sub: sub}, nil
}

// WatchRewardsClaimed is a free log subscription operation binding the contract event 0x9310ccfcb8de723f578a9e4282ea9f521f05ae40dc08f3068dfad528a65ee3c7.
//
// Solidity: event RewardsClaimed(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) WatchRewardsClaimed(opts *bind.WatchOpts, sink chan<- *StakingRewardsClaimed, delegator []common.Address, validator []common.Address) (event.Subscription, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}
	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "RewardsClaimed", delegatorRule, validatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingRewardsClaimed)
				if err := _Staking.contract.UnpackLog(event, "RewardsClaimed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRewardsClaimed is a log parse operation binding the contract event 0x9310ccfcb8de723f578a9e4282ea9f521f05ae40dc08f3068dfad528a65ee3c7.
//
// Solidity: event RewardsClaimed(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) ParseRewardsClaimed(log types.Log) (*StakingRewardsClaimed, error) {
	event := new(StakingRewardsClaimed)
	if err := _Staking.contract.UnpackLog(event, "RewardsClaimed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StakingRewardsDistributedIterator is returned from FilterRewardsDistributed and is used to iterate over the raw logs and unpacked data for RewardsDistributed events raised by the Staking contract.
type StakingRewardsDistributedIterator struct {
	Event *StakingRewardsDistributed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingRewardsDistributedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingRewardsDistributed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingRewardsDistributed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingRewardsDistributedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingRewardsDistributedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingRewardsDistributed represents a RewardsDistributed event raised by the Staking contract.
type StakingRewardsDistributed struct {
	Validator common.Address
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterRewardsDistributed is a free log retrieval operation binding the contract event 0xdf29796aad820e4bb192f3a8d631b76519bcd2cbe77cc85af20e9df53cece086.
//
// Solidity: event RewardsDistributed(address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) FilterRewardsDistributed(opts *bind.FilterOpts, validator []common.Address) (*StakingRewardsDistributedIterator, error) {

	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "RewardsDistributed", validatorRule)
	if err != nil {
		return nil, err
	}
	return &StakingRewardsDistributedIterator{contract: _Staking.contract, event: "RewardsDistributed", logs: logs, sub: sub}, nil
}

// WatchRewardsDistributed is a free log subscription operation binding the contract event 0xdf29796aad820e4bb192f3a8d631b76519bcd2cbe77cc85af20e9df53cece086.
//
// Solidity: event RewardsDistributed(address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) WatchRewardsDistributed(opts *bind.WatchOpts, sink chan<- *StakingRewardsDistributed, validator []common.Address) (event.Subscription, error) {

	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "RewardsDistributed", validatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingRewardsDistributed)
				if err := _Staking.contract.UnpackLog(event, "RewardsDistributed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRewardsDistributed is a log parse operation binding the contract event 0xdf29796aad820e4bb192f3a8d631b76519bcd2cbe77cc85af20e9df53cece086.
//
// Solidity: event RewardsDistributed(address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) ParseRewardsDistributed(log types.Log) (*StakingRewardsDistributed, error) {
	event := new(StakingRewardsDistributed)
	if err := _Staking.contract.UnpackLog(event, "RewardsDistributed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StakingUnbondingCompletedIterator is returned from FilterUnbondingCompleted and is used to iterate over the raw logs and unpacked data for UnbondingCompleted events raised by the Staking contract.
type StakingUnbondingCompletedIterator struct {
	Event *StakingUnbondingCompleted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingUnbondingCompletedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingUnbondingCompleted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingUnbondingCompleted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingUnbondingCompletedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingUnbondingCompletedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingUnbondingCompleted represents a UnbondingCompleted event raised by the Staking contract.
type StakingUnbondingCompleted struct {
	Delegator common.Address
	Validator common.Address
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterUnbondingCompleted is a free log retrieval operation binding the contract event 0x29a354857110d4b0895fcb3571575b9346fa04cc4a08991d49b315894f57ce7d.
//
// Solidity: event UnbondingCompleted(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) FilterUnbondingCompleted(opts *bind.FilterOpts, delegator []common.Address, validator []common.Address) (*StakingUnbondingCompletedIterator, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}
	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "UnbondingCompleted", delegatorRule, validatorRule)
	if err != nil {
		return nil, err
	}
	return &StakingUnbondingCompletedIterator{contract: _Staking.contract, event: "UnbondingCompleted", logs: logs, sub: sub}, nil
}

// WatchUnbondingCompleted is a free log subscription operation binding the contract event 0x29a354857110d4b0895fcb3571575b9346fa04cc4a08991d49b315894f57ce7d.
//
// Solidity: event UnbondingCompleted(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) WatchUnbondingCompleted(opts *bind.WatchOpts, sink chan<- *StakingUnbondingCompleted, delegator []common.Address, validator []common.Address) (event.Subscription, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}
	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "UnbondingCompleted", delegatorRule, validatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingUnbondingCompleted)
				if err := _Staking.contract.UnpackLog(event, "UnbondingCompleted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseUnbondingCompleted is a log parse operation binding the contract event 0x29a354857110d4b0895fcb3571575b9346fa04cc4a08991d49b315894f57ce7d.
//
// Solidity: event UnbondingCompleted(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) ParseUnbondingCompleted(log types.Log) (*StakingUnbondingCompleted, error) {
	event := new(StakingUnbondingCompleted)
	if err := _Staking.contract.UnpackLog(event, "UnbondingCompleted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StakingUndelegatedIterator is returned from FilterUndelegated and is used to iterate over the raw logs and unpacked data for Undelegated events raised by the Staking contract.
type StakingUndelegatedIterator struct {
	Event *StakingUndelegated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingUndelegatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingUndelegated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingUndelegated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingUndelegatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingUndelegatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingUndelegated represents a Undelegated event raised by the Staking contract.
type StakingUndelegated struct {
	Delegator common.Address
	Validator common.Address
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterUndelegated is a free log retrieval operation binding the contract event 0x4d10bd049775c77bd7f255195afba5088028ecb3c7c277d393ccff7934f2f92c.
//
// Solidity: event Undelegated(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) FilterUndelegated(opts *bind.FilterOpts, delegator []common.Address, validator []common.Address) (*StakingUndelegatedIterator, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}
	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "Undelegated", delegatorRule, validatorRule)
	if err != nil {
		return nil, err
	}
	return &StakingUndelegatedIterator{contract: _Staking.contract, event: "Undelegated", logs: logs, sub: sub}, nil
}

// WatchUndelegated is a free log subscription operation binding the contract event 0x4d10bd049775c77bd7f255195afba5088028ecb3c7c277d393ccff7934f2f92c.
//
// Solidity: event Undelegated(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) WatchUndelegated(opts *bind.WatchOpts, sink chan<- *StakingUndelegated, delegator []common.Address, validator []common.Address) (event.Subscription, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}
	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "Undelegated", delegatorRule, validatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingUndelegated)
				if err := _Staking.contract.UnpackLog(event, "Undelegated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseUndelegated is a log parse operation binding the contract event 0x4d10bd049775c77bd7f255195afba5088028ecb3c7c277d393ccff7934f2f92c.
//
// Solidity: event Undelegated(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) ParseUndelegated(log types.Log) (*StakingUndelegated, error) {
	event := new(StakingUndelegated)
	if err := _Staking.contract.UnpackLog(event, "Undelegated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StakingValidatorExitedIterator is returned from FilterValidatorExited and is used to iterate over the raw logs and unpacked data for ValidatorExited events raised by the Staking contract.
type StakingValidatorExitedIterator struct {
	Event *StakingValidatorExited // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingValidatorExitedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingValidatorExited)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingValidatorExited)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingValidatorExitedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingValidatorExitedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingValidatorExited represents a ValidatorExited event raised by the Staking contract.
type StakingValidatorExited struct {
	Validator common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterValidatorExited is a free log retrieval operation binding the contract event 0x05956ba8f9bd4bcb79fc3301e702e6bd74e3df03d048ed441fa2420dd6ffa8be.
//
// Solidity: event ValidatorExited(address indexed validator)
func (_Staking *StakingFilterer) FilterValidatorExited(opts *bind.FilterOpts, validator []common.Address) (*StakingValidatorExitedIterator, error) {

	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "ValidatorExited", validatorRule)
	if err != nil {
		return nil, err
	}
	return &StakingValidatorExitedIterator{contract: _Staking.contract, event: "ValidatorExited", logs: logs, sub: sub}, nil
}

// WatchValidatorExited is a free log subscription operation binding the contract event 0x05956ba8f9bd4bcb79fc3301e702e6bd74e3df03d048ed441fa2420dd6ffa8be.
//
// Solidity: event ValidatorExited(address indexed validator)
func (_Staking *StakingFilterer) WatchValidatorExited(opts *bind.WatchOpts, sink chan<- *StakingValidatorExited, validator []common.Address) (event.Subscription, error) {

	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "ValidatorExited", validatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingValidatorExited)
				if err := _Staking.contract.UnpackLog(event, "ValidatorExited", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseValidatorExited is a log parse operation binding the contract event 0x05956ba8f9bd4bcb79fc3301e702e6bd74e3df03d048ed441fa2420dd6ffa8be.
//
// Solidity: event ValidatorExited(address indexed validator)
func (_Staking *StakingFilterer) ParseValidatorExited(log types.Log) (*StakingValidatorExited, error) {
	event := new(StakingValidatorExited)
	if err := _Staking.contract.UnpackLog(event, "ValidatorExited", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StakingValidatorJailedIterator is returned from FilterValidatorJailed and is used to iterate over the raw logs and unpacked data for ValidatorJailed events raised by the Staking contract.
type StakingValidatorJailedIterator struct {
	Event *StakingValidatorJailed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingValidatorJailedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingValidatorJailed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingValidatorJailed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingValidatorJailedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingValidatorJailedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingValidatorJailed represents a ValidatorJailed event raised by the Staking contract.
type StakingValidatorJailed struct {
	Validator      common.Address
	JailUntilBlock *big.Int
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterValidatorJailed is a free log retrieval operation binding the contract event 0xeb7d7a49847ec491969db21a0e31b234565a9923145a2d1b56a75c9e95825802.
//
// Solidity: event ValidatorJailed(address indexed validator, uint256 jailUntilBlock)
func (_Staking *StakingFilterer) FilterValidatorJailed(opts *bind.FilterOpts, validator []common.Address) (*StakingValidatorJailedIterator, error) {

	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "ValidatorJailed", validatorRule)
	if err != nil {
		return nil, err
	}
	return &StakingValidatorJailedIterator{contract: _Staking.contract, event: "ValidatorJailed", logs: logs, sub: sub}, nil
}

// WatchValidatorJailed is a free log subscription operation binding the contract event 0xeb7d7a49847ec491969db21a0e31b234565a9923145a2d1b56a75c9e95825802.
//
// Solidity: event ValidatorJailed(address indexed validator, uint256 jailUntilBlock)
func (_Staking *StakingFilterer) WatchValidatorJailed(opts *bind.WatchOpts, sink chan<- *StakingValidatorJailed, validator []common.Address) (event.Subscription, error) {

	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "ValidatorJailed", validatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingValidatorJailed)
				if err := _Staking.contract.UnpackLog(event, "ValidatorJailed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseValidatorJailed is a log parse operation binding the contract event 0xeb7d7a49847ec491969db21a0e31b234565a9923145a2d1b56a75c9e95825802.
//
// Solidity: event ValidatorJailed(address indexed validator, uint256 jailUntilBlock)
func (_Staking *StakingFilterer) ParseValidatorJailed(log types.Log) (*StakingValidatorJailed, error) {
	event := new(StakingValidatorJailed)
	if err := _Staking.contract.UnpackLog(event, "ValidatorJailed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StakingValidatorRegisteredIterator is returned from FilterValidatorRegistered and is used to iterate over the raw logs and unpacked data for ValidatorRegistered events raised by the Staking contract.
type StakingValidatorRegisteredIterator struct {
	Event *StakingValidatorRegistered // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingValidatorRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingValidatorRegistered)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingValidatorRegistered)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingValidatorRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingValidatorRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingValidatorRegistered represents a ValidatorRegistered event raised by the Staking contract.
type StakingValidatorRegistered struct {
	Validator      common.Address
	SelfStake      *big.Int
	CommissionRate *big.Int
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterValidatorRegistered is a free log retrieval operation binding the contract event 0x4fedf9289a156b214915bd5c2db58d3ee16acc185e80df66ee404e4573c821e1.
//
// Solidity: event ValidatorRegistered(address indexed validator, uint256 selfStake, uint256 commissionRate)
func (_Staking *StakingFilterer) FilterValidatorRegistered(opts *bind.FilterOpts, validator []common.Address) (*StakingValidatorRegisteredIterator, error) {

	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "ValidatorRegistered", validatorRule)
	if err != nil {
		return nil, err
	}
	return &StakingValidatorRegisteredIterator{contract: _Staking.contract, event: "ValidatorRegistered", logs: logs, sub: sub}, nil
}

// WatchValidatorRegistered is a free log subscription operation binding the contract event 0x4fedf9289a156b214915bd5c2db58d3ee16acc185e80df66ee404e4573c821e1.
//
// Solidity: event ValidatorRegistered(address indexed validator, uint256 selfStake, uint256 commissionRate)
func (_Staking *StakingFilterer) WatchValidatorRegistered(opts *bind.WatchOpts, sink chan<- *StakingValidatorRegistered, validator []common.Address) (event.Subscription, error) {

	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "ValidatorRegistered", validatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingValidatorRegistered)
				if err := _Staking.contract.UnpackLog(event, "ValidatorRegistered", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseValidatorRegistered is a log parse operation binding the contract event 0x4fedf9289a156b214915bd5c2db58d3ee16acc185e80df66ee404e4573c821e1.
//
// Solidity: event ValidatorRegistered(address indexed validator, uint256 selfStake, uint256 commissionRate)
func (_Staking *StakingFilterer) ParseValidatorRegistered(log types.Log) (*StakingValidatorRegistered, error) {
	event := new(StakingValidatorRegistered)
	if err := _Staking.contract.UnpackLog(event, "ValidatorRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StakingValidatorSlashedIterator is returned from FilterValidatorSlashed and is used to iterate over the raw logs and unpacked data for ValidatorSlashed events raised by the Staking contract.
type StakingValidatorSlashedIterator struct {
	Event *StakingValidatorSlashed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingValidatorSlashedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingValidatorSlashed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingValidatorSlashed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingValidatorSlashedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingValidatorSlashedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingValidatorSlashed represents a ValidatorSlashed event raised by the Staking contract.
type StakingValidatorSlashed struct {
	Validator    common.Address
	SlashAmount  *big.Int
	RewardAmount *big.Int
	BurnAmount   *big.Int
	Reporter     common.Address
	BurnAddress  common.Address
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterValidatorSlashed is a free log retrieval operation binding the contract event 0x5bd6f9405e6c0a71ad3df5e2e346286287acc47544e763f77889c264066d154a.
//
// Solidity: event ValidatorSlashed(address indexed validator, uint256 slashAmount, uint256 rewardAmount, uint256 burnAmount, address indexed reporter, address indexed burnAddress)
func (_Staking *StakingFilterer) FilterValidatorSlashed(opts *bind.FilterOpts, validator []common.Address, reporter []common.Address, burnAddress []common.Address) (*StakingValidatorSlashedIterator, error) {

	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	var reporterRule []interface{}
	for _, reporterItem := range reporter {
		reporterRule = append(reporterRule, reporterItem)
	}
	var burnAddressRule []interface{}
	for _, burnAddressItem := range burnAddress {
		burnAddressRule = append(burnAddressRule, burnAddressItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "ValidatorSlashed", validatorRule, reporterRule, burnAddressRule)
	if err != nil {
		return nil, err
	}
	return &StakingValidatorSlashedIterator{contract: _Staking.contract, event: "ValidatorSlashed", logs: logs, sub: sub}, nil
}

// WatchValidatorSlashed is a free log subscription operation binding the contract event 0x5bd6f9405e6c0a71ad3df5e2e346286287acc47544e763f77889c264066d154a.
//
// Solidity: event ValidatorSlashed(address indexed validator, uint256 slashAmount, uint256 rewardAmount, uint256 burnAmount, address indexed reporter, address indexed burnAddress)
func (_Staking *StakingFilterer) WatchValidatorSlashed(opts *bind.WatchOpts, sink chan<- *StakingValidatorSlashed, validator []common.Address, reporter []common.Address, burnAddress []common.Address) (event.Subscription, error) {

	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	var reporterRule []interface{}
	for _, reporterItem := range reporter {
		reporterRule = append(reporterRule, reporterItem)
	}
	var burnAddressRule []interface{}
	for _, burnAddressItem := range burnAddress {
		burnAddressRule = append(burnAddressRule, burnAddressItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "ValidatorSlashed", validatorRule, reporterRule, burnAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingValidatorSlashed)
				if err := _Staking.contract.UnpackLog(event, "ValidatorSlashed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseValidatorSlashed is a log parse operation binding the contract event 0x5bd6f9405e6c0a71ad3df5e2e346286287acc47544e763f77889c264066d154a.
//
// Solidity: event ValidatorSlashed(address indexed validator, uint256 slashAmount, uint256 rewardAmount, uint256 burnAmount, address indexed reporter, address indexed burnAddress)
func (_Staking *StakingFilterer) ParseValidatorSlashed(log types.Log) (*StakingValidatorSlashed, error) {
	event := new(StakingValidatorSlashed)
	if err := _Staking.contract.UnpackLog(event, "ValidatorSlashed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StakingValidatorStakeDecreasedIterator is returned from FilterValidatorStakeDecreased and is used to iterate over the raw logs and unpacked data for ValidatorStakeDecreased events raised by the Staking contract.
type StakingValidatorStakeDecreasedIterator struct {
	Event *StakingValidatorStakeDecreased // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingValidatorStakeDecreasedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingValidatorStakeDecreased)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingValidatorStakeDecreased)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingValidatorStakeDecreasedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingValidatorStakeDecreasedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingValidatorStakeDecreased represents a ValidatorStakeDecreased event raised by the Staking contract.
type StakingValidatorStakeDecreased struct {
	Delegator common.Address
	Validator common.Address
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterValidatorStakeDecreased is a free log retrieval operation binding the contract event 0x64bcb4cf4c65b4bfe23bf707cd7770998b00489a298f3c1e019a8a915348dd43.
//
// Solidity: event ValidatorStakeDecreased(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) FilterValidatorStakeDecreased(opts *bind.FilterOpts, delegator []common.Address, validator []common.Address) (*StakingValidatorStakeDecreasedIterator, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}
	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "ValidatorStakeDecreased", delegatorRule, validatorRule)
	if err != nil {
		return nil, err
	}
	return &StakingValidatorStakeDecreasedIterator{contract: _Staking.contract, event: "ValidatorStakeDecreased", logs: logs, sub: sub}, nil
}

// WatchValidatorStakeDecreased is a free log subscription operation binding the contract event 0x64bcb4cf4c65b4bfe23bf707cd7770998b00489a298f3c1e019a8a915348dd43.
//
// Solidity: event ValidatorStakeDecreased(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) WatchValidatorStakeDecreased(opts *bind.WatchOpts, sink chan<- *StakingValidatorStakeDecreased, delegator []common.Address, validator []common.Address) (event.Subscription, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}
	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "ValidatorStakeDecreased", delegatorRule, validatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingValidatorStakeDecreased)
				if err := _Staking.contract.UnpackLog(event, "ValidatorStakeDecreased", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseValidatorStakeDecreased is a log parse operation binding the contract event 0x64bcb4cf4c65b4bfe23bf707cd7770998b00489a298f3c1e019a8a915348dd43.
//
// Solidity: event ValidatorStakeDecreased(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) ParseValidatorStakeDecreased(log types.Log) (*StakingValidatorStakeDecreased, error) {
	event := new(StakingValidatorStakeDecreased)
	if err := _Staking.contract.UnpackLog(event, "ValidatorStakeDecreased", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StakingValidatorStakeIncreasedIterator is returned from FilterValidatorStakeIncreased and is used to iterate over the raw logs and unpacked data for ValidatorStakeIncreased events raised by the Staking contract.
type StakingValidatorStakeIncreasedIterator struct {
	Event *StakingValidatorStakeIncreased // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingValidatorStakeIncreasedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingValidatorStakeIncreased)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingValidatorStakeIncreased)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingValidatorStakeIncreasedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingValidatorStakeIncreasedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingValidatorStakeIncreased represents a ValidatorStakeIncreased event raised by the Staking contract.
type StakingValidatorStakeIncreased struct {
	Delegator common.Address
	Validator common.Address
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterValidatorStakeIncreased is a free log retrieval operation binding the contract event 0xae6946de73a7ea549b21272efc1797dca3c65c4136f9d878585b0e100d8ad5bd.
//
// Solidity: event ValidatorStakeIncreased(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) FilterValidatorStakeIncreased(opts *bind.FilterOpts, delegator []common.Address, validator []common.Address) (*StakingValidatorStakeIncreasedIterator, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}
	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "ValidatorStakeIncreased", delegatorRule, validatorRule)
	if err != nil {
		return nil, err
	}
	return &StakingValidatorStakeIncreasedIterator{contract: _Staking.contract, event: "ValidatorStakeIncreased", logs: logs, sub: sub}, nil
}

// WatchValidatorStakeIncreased is a free log subscription operation binding the contract event 0xae6946de73a7ea549b21272efc1797dca3c65c4136f9d878585b0e100d8ad5bd.
//
// Solidity: event ValidatorStakeIncreased(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) WatchValidatorStakeIncreased(opts *bind.WatchOpts, sink chan<- *StakingValidatorStakeIncreased, delegator []common.Address, validator []common.Address) (event.Subscription, error) {

	var delegatorRule []interface{}
	for _, delegatorItem := range delegator {
		delegatorRule = append(delegatorRule, delegatorItem)
	}
	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "ValidatorStakeIncreased", delegatorRule, validatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingValidatorStakeIncreased)
				if err := _Staking.contract.UnpackLog(event, "ValidatorStakeIncreased", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseValidatorStakeIncreased is a log parse operation binding the contract event 0xae6946de73a7ea549b21272efc1797dca3c65c4136f9d878585b0e100d8ad5bd.
//
// Solidity: event ValidatorStakeIncreased(address indexed delegator, address indexed validator, uint256 amount)
func (_Staking *StakingFilterer) ParseValidatorStakeIncreased(log types.Log) (*StakingValidatorStakeIncreased, error) {
	event := new(StakingValidatorStakeIncreased)
	if err := _Staking.contract.UnpackLog(event, "ValidatorStakeIncreased", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StakingValidatorUnjailedIterator is returned from FilterValidatorUnjailed and is used to iterate over the raw logs and unpacked data for ValidatorUnjailed events raised by the Staking contract.
type StakingValidatorUnjailedIterator struct {
	Event *StakingValidatorUnjailed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakingValidatorUnjailedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakingValidatorUnjailed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakingValidatorUnjailed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakingValidatorUnjailedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakingValidatorUnjailedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakingValidatorUnjailed represents a ValidatorUnjailed event raised by the Staking contract.
type StakingValidatorUnjailed struct {
	Validator common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterValidatorUnjailed is a free log retrieval operation binding the contract event 0x9390b453426557da5ebdc31f19a37753ca04addf656d32f35232211bb2af3f19.
//
// Solidity: event ValidatorUnjailed(address indexed validator)
func (_Staking *StakingFilterer) FilterValidatorUnjailed(opts *bind.FilterOpts, validator []common.Address) (*StakingValidatorUnjailedIterator, error) {

	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.FilterLogs(opts, "ValidatorUnjailed", validatorRule)
	if err != nil {
		return nil, err
	}
	return &StakingValidatorUnjailedIterator{contract: _Staking.contract, event: "ValidatorUnjailed", logs: logs, sub: sub}, nil
}

// WatchValidatorUnjailed is a free log subscription operation binding the contract event 0x9390b453426557da5ebdc31f19a37753ca04addf656d32f35232211bb2af3f19.
//
// Solidity: event ValidatorUnjailed(address indexed validator)
func (_Staking *StakingFilterer) WatchValidatorUnjailed(opts *bind.WatchOpts, sink chan<- *StakingValidatorUnjailed, validator []common.Address) (event.Subscription, error) {

	var validatorRule []interface{}
	for _, validatorItem := range validator {
		validatorRule = append(validatorRule, validatorItem)
	}

	logs, sub, err := _Staking.contract.WatchLogs(opts, "ValidatorUnjailed", validatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakingValidatorUnjailed)
				if err := _Staking.contract.UnpackLog(event, "ValidatorUnjailed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseValidatorUnjailed is a log parse operation binding the contract event 0x9390b453426557da5ebdc31f19a37753ca04addf656d32f35232211bb2af3f19.
//
// Solidity: event ValidatorUnjailed(address indexed validator)
func (_Staking *StakingFilterer) ParseValidatorUnjailed(log types.Log) (*StakingValidatorUnjailed, error) {
	event := new(StakingValidatorUnjailed)
	if err := _Staking.contract.UnpackLog(event, "ValidatorUnjailed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
