build:
	go build -ldflags="-s -w" -o trading-bot .

test:
	go test ./... -v

vet:
	go vet ./...

deploy:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o trading-bot .
	@echo "Stopping bot..."
	gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo systemctl stop trading-bot' || true
	@echo "Uploading to VM..."
	gcloud compute scp ./trading-bot trading-bot:~ --zone=us-east1-c
	gcloud compute scp ./config.yaml trading-bot:~ --zone=us-east1-c
	gcloud compute scp ./.env trading-bot:~ --zone=us-east1-c
	@echo "Starting bot..."
	gcloud compute ssh trading-bot --zone=us-east1-c --command='chmod +x ~/trading-bot && sudo systemctl start trading-bot'
	@echo "Done!"

logs:
	gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo journalctl -u trading-bot --no-pager -n 50'

logs-follow:
	gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo journalctl -u trading-bot -f'

vm-status:
	gcloud compute ssh trading-bot --zone=us-east1-c --command='sudo systemctl status trading-bot'

api-health:
	gcloud compute ssh trading-bot --zone=us-east1-c --command='curl -s http://localhost:8080/api/health | jq .'

api-stats:
	gcloud compute ssh trading-bot --zone=us-east1-c --command='curl -s http://localhost:8080/api/stats | jq .'

api-latest:
	gcloud compute ssh trading-bot --zone=us-east1-c --command='curl -s http://localhost:8080/api/sessions/latest | jq .'

api-trades:
	gcloud compute ssh trading-bot --zone=us-east1-c --command='curl -s http://localhost:8080/api/trades?limit=10 | jq .'

run:
	go run . run

backtest:
	go run . backtest

paper:
	go run . paper

testnet:
	go run . testnet

clean:
	rm -f trading-bot trading_bot.db
	rm -f logs/*.log logs/*.jsonl
