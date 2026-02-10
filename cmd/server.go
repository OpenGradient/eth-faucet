package cmd

import (
	"crypto/ecdsa"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/chainflag/eth-faucet/internal/chain"
	"github.com/chainflag/eth-faucet/internal/server"
)

var (
	appVersion = "v1.2.0"
	chainIDMap = map[string]int{"sepolia": 11155111, "holesky": 17000, "opengradient": 10740, "base_sepolia": 84532}

	httpPortFlag = flag.Int("httpport", 8090, "Listener port to serve HTTP connection")
	proxyCntFlag = flag.Int("proxycount", 1, "Count of reverse proxies in front of the server")
	versionFlag  = flag.Bool("version", false, "Print version number")

	payoutFlag     = flag.Float64("faucet.amount", 0.03, "Number of tokens to transfer per user request")
	intervalFlag   = flag.Int("faucet.minutes", 300, "Number of minutes to wait between funding rounds")
	netnameFlag    = flag.String("faucet.name", "base_sepolia", "Network name to display on the frontend")
	symbolFlag     = flag.String("faucet.symbol", "OPG", "Token symbol to display on the frontend")
	tokenAddrFlag  = flag.String("faucet.tokenaddr", "0x240b09731D96979f50B2C649C9CE10FcF9C7987F", "ERC-20 token contract address to disperse (empty for native ETH)")

	keyJSONFlag  = flag.String("wallet.keyjson", os.Getenv("KEYSTORE"), "Keystore file to fund user requests with")
	keyPassFlag  = flag.String("wallet.keypass", "password.txt", "Passphrase text file to decrypt keystore")
	privKeyFlag  = flag.String("wallet.privkey", os.Getenv("PRIVATE_KEY"), "Private key hex to fund user requests with")
	providerFlag = flag.String("wallet.provider", "https://sepolia.base.org", "Endpoint for Ethereum JSON-RPC connection")

	hcaptchaSiteKeyFlag = flag.String("hcaptcha.sitekey", os.Getenv("HCAPTCHA_SITEKEY"), "hCaptcha sitekey")
	hcaptchaSecretFlag  = flag.String("hcaptcha.secret", os.Getenv("HCAPTCHA_SECRET"), "hCaptcha secret")
)

func init() {
	flag.Parse()
	if *versionFlag {
		fmt.Println(appVersion)
		os.Exit(0)
	}
}

func Execute() {
	privateKey, err := getPrivateKeyFromFlags()
	if err != nil {
		panic(fmt.Errorf("failed to read private key: %w", err))
	}
	var chainID *big.Int
	if value, ok := chainIDMap[strings.ToLower(*netnameFlag)]; ok {
		chainID = big.NewInt(int64(value))
	}

	txBuilder, err := chain.NewTxBuilder(*providerFlag, privateKey, chainID, *tokenAddrFlag)
	if err != nil {
		panic(fmt.Errorf("cannot connect to web3 provider: %w", err))
	}
	config := server.NewConfig(*netnameFlag, *symbolFlag, *httpPortFlag, *intervalFlag, *proxyCntFlag, *payoutFlag, *hcaptchaSiteKeyFlag, *hcaptchaSecretFlag)
	go server.NewServer(txBuilder, config).Run()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func getPrivateKeyFromFlags() (*ecdsa.PrivateKey, error) {
	if *privKeyFlag != "" {
		hexkey := *privKeyFlag
		if chain.Has0xPrefix(hexkey) {
			hexkey = hexkey[2:]
		}
		return crypto.HexToECDSA(hexkey)
	} else if *keyJSONFlag == "" {
		return nil, errors.New("missing private key or keystore")
	}

	keyfile, err := chain.ResolveKeyfilePath(*keyJSONFlag)
	if err != nil {
		return nil, err
	}
	password, err := os.ReadFile(*keyPassFlag)
	if err != nil {
		return nil, err
	}

	return chain.DecryptKeyfile(keyfile, strings.TrimRight(string(password), "\r\n"))
}
