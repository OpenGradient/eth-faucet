package chain

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
)

type TxBuilder interface {
	Sender() common.Address
	Transfer(ctx context.Context, to string, value *big.Int) (common.Hash, error)
}

type TxBuild struct {
	client          bind.ContractTransactor
	privateKey      *ecdsa.PrivateKey
	signer          types.Signer
	fromAddress     common.Address
	nonce           uint64
	supportsEIP1559 bool
	tokenAddress    *common.Address
}

func NewTxBuilder(provider string, privateKey *ecdsa.PrivateKey, chainID *big.Int, tokenAddr string) (TxBuilder, error) {
	client, err := ethclient.Dial(provider)
	if err != nil {
		return nil, err
	}

	if chainID == nil {
		chainID, err = client.ChainID(context.Background())
		if err != nil {
			return nil, err
		}
	}

	supportsEIP1559, err := checkEIP1559Support(client)
	if err != nil {
		return nil, err
	}

	txBuilder := &TxBuild{
		client:          client,
		privateKey:      privateKey,
		signer:          types.NewLondonSigner(chainID),
		fromAddress:     crypto.PubkeyToAddress(privateKey.PublicKey),
		supportsEIP1559: supportsEIP1559,
	}

	if tokenAddr != "" {
		addr := common.HexToAddress(tokenAddr)
		txBuilder.tokenAddress = &addr
	}

	txBuilder.refreshNonce(context.Background())

	return txBuilder, nil
}

func (b *TxBuild) Sender() common.Address {
	return b.fromAddress
}

func (b *TxBuild) Transfer(ctx context.Context, to string, value *big.Int) (common.Hash, error) {
	gasLimit := uint64(200_000)
	toAddress := common.HexToAddress(to)
	nonce := b.getAndIncrementNonce()

	// Determine transaction target and data based on whether this is an ERC-20 transfer
	txTo := &toAddress
	txValue := value
	var txData []byte

	if b.tokenAddress != nil {
		txTo = b.tokenAddress
		txValue = big.NewInt(0)
		txData = buildERC20TransferData(toAddress, value)
	}

	var err error
	var unsignedTx *types.Transaction

	if b.supportsEIP1559 {
		unsignedTx, err = b.buildEIP1559Tx(ctx, txTo, txValue, gasLimit, nonce, txData)
	} else {
		unsignedTx, err = b.buildLegacyTx(ctx, txTo, txValue, gasLimit, nonce, txData)
	}

	if err != nil {
		return common.Hash{}, err
	}

	log.WithFields(log.Fields{
		"gasPrice": unsignedTx.GasPrice(),
		"gasLimit": unsignedTx.Gas(),
	}).Info("Faucet txn details")

	signedTx, err := types.SignTx(unsignedTx, b.signer, b.privateKey)
	if err != nil {
		return common.Hash{}, err
	}

	if err = b.client.SendTransaction(ctx, signedTx); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "nonce") {
			b.refreshNonce(context.Background())
		}
		return common.Hash{}, err
	}

	return signedTx.Hash(), nil
}

// buildERC20TransferData constructs calldata for ERC-20 transfer(address,uint256)
func buildERC20TransferData(to common.Address, amount *big.Int) []byte {
	// Function selector: keccak256("transfer(address,uint256)") = 0xa9059cbb
	methodID := []byte{0xa9, 0x05, 0x9c, 0xbb}
	paddedAddress := common.LeftPadBytes(to.Bytes(), 32)
	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)

	var data []byte
	data = append(data, methodID...)
	data = append(data, paddedAddress...)
	data = append(data, paddedAmount...)
	return data
}

func (b *TxBuild) buildEIP1559Tx(ctx context.Context, to *common.Address, value *big.Int, gasLimit uint64, nonce uint64, data []byte) (*types.Transaction, error) {
	header, err := b.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, err
	}

	gasTipCap, err := b.client.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, err
	}

	// gasFeeCap = baseFee * 2 + gasTipCap
	gasFeeCap := new(big.Int).Mul(header.BaseFee, big.NewInt(2))
	gasFeeCap = new(big.Int).Add(gasFeeCap, gasTipCap)

	return types.NewTx(&types.DynamicFeeTx{
		ChainID:   b.signer.ChainID(),
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       gasLimit,
		To:        to,
		Value:     value,
		Data:      data,
	}), nil
}

func (b *TxBuild) buildLegacyTx(ctx context.Context, to *common.Address, value *big.Int, gasLimit uint64, nonce uint64, data []byte) (*types.Transaction, error) {
	gasPrice, err := b.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, err
	}

	// Add 2
	gasPrice.Add(gasPrice, big.NewInt(20))

	return types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      gasLimit,
		To:       to,
		Value:    value,
		Data:     data,
	}), nil
}

func (b *TxBuild) getAndIncrementNonce() uint64 {
	return atomic.AddUint64(&b.nonce, 1) - 1
}

func (b *TxBuild) refreshNonce(ctx context.Context) {
	nonce, err := b.client.PendingNonceAt(ctx, b.Sender())
	if err != nil {
		log.WithFields(log.Fields{
			"address": b.Sender(),
			"error":   err,
		}).Error("failed to refresh account nonce")
		return
	}

	atomic.StoreUint64(&b.nonce, nonce)
}

func checkEIP1559Support(client *ethclient.Client) (bool, error) {
	// hardcode no eip 1559
	return false, nil
}
