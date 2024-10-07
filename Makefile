build:
	docker build . -t opengradient-faucet

run:
	docker run -d -p 8080:8080 -e WEB3_PROVIDER=http://localhost:8545 -e PRIVATE_KEY=$(PRIVATE_KEY) opengradient-facuet:latest
