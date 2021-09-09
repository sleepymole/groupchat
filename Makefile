build: chat-server chat-ctl

chat-server:
	CGO_ENABLED=0 go build -o bin/chat-server github.com/gozssky/groupchat/cmd/chat-server

chat-ctl:
	CGO_ENABLED=0 go build -o bin/chat-ctl github.com/gozssky/groupchat/cmd/chat-ctl

test:
	go test -v github.com/gozssky/groupchat/...

clean:
	rm -rf bin

deploy: build
	@mkdir -p build  \
		&& zip -q -j /tmp/application.zip bin/chat-server \
		&& zip -q -j deploy.zip /tmp/application.zip build/start.sh build/application.ros.json \
		&& rm -f /tmp/application.zip

.PHONY: build deploy
