build:
	docker build . -t opengradient-faucet

run:
	docker run -d -p 8080:8080 --name faucet --network host -e WEB3_PROVIDER=http://127.0.0.1:8545 -e PRIVATE_KEY=$(PRIVATE_KEY) opengradient-faucet:latest -faucet.name OpenGradient -faucet.amount 0.1
