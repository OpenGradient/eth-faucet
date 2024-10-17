build:
	docker build . -t opengradient-faucet

run:
	docker run -d -p 8080:8080 --name faucet --network host -e PRIVATE_KEY=$(PRIVATE_KEY) opengradient-faucet:latest
