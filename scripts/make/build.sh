#!/bin/bash

CLIENT_NAME='ddns-watchdog-client'
CLIENT_CODE_FILE='./cmd/ddns-watchdog-client/main.go'
SERVER_NAME='ddns-watchdog-server'
SERVER_CODE_FILE='./cmd/ddns-watchdog-server/main.go'
OUTPUT_PATH='./build'

# check if the $OUTPUT_PATH folder exists
if [[ ! -d "$OUTPUT_PATH" ]]; then
    mkdir "$OUTPUT_PATH"
fi

# start building
# client
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-w -s" -o $OUTPUT_PATH/$CLIENT_NAME $CLIENT_CODE_FILE
tar -czvf $OUTPUT_PATH/$CLIENT_NAME.linux-amd64.tar.gz -C $OUTPUT_PATH $CLIENT_NAME
rm -f $OUTPUT_PATH/$CLIENT_NAME

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-w -s" -o $OUTPUT_PATH/$CLIENT_NAME.exe $CLIENT_CODE_FILE
tar -czvf $OUTPUT_PATH/$CLIENT_NAME.windows-amd64.tar.gz -C $OUTPUT_PATH $CLIENT_NAME.exe
rm -f $OUTPUT_PATH/$CLIENT_NAME.exe

CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-w -s" -o $OUTPUT_PATH/$CLIENT_NAME $CLIENT_CODE_FILE
tar -czvf $OUTPUT_PATH/$CLIENT_NAME.darwin-amd64.tar.gz -C $OUTPUT_PATH $CLIENT_NAME
rm -f $OUTPUT_PATH/$CLIENT_NAME

# arm
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-w -s" -o $OUTPUT_PATH/$CLIENT_NAME $CLIENT_CODE_FILE
tar -czvf $OUTPUT_PATH/$CLIENT_NAME.linux-arm64.tar.gz -C $OUTPUT_PATH $CLIENT_NAME
rm -f $OUTPUT_PATH/$CLIENT_NAME

CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "-w -s" -o $OUTPUT_PATH/$CLIENT_NAME $CLIENT_CODE_FILE
tar -czvf $OUTPUT_PATH/$CLIENT_NAME.linux-arm_v7.tar.gz -C $OUTPUT_PATH $CLIENT_NAME
rm -f $OUTPUT_PATH/$CLIENT_NAME

# mips
CGO_ENABLED=0 GOOS=linux GOARCH=mips64le go build -ldflags "-w -s" -o $OUTPUT_PATH/$CLIENT_NAME $CLIENT_CODE_FILE
tar -czvf $OUTPUT_PATH/$CLIENT_NAME.linux-mips64le.tar.gz -C $OUTPUT_PATH $CLIENT_NAME
rm -f $OUTPUT_PATH/$CLIENT_NAME

CGO_ENABLED=0 GOOS=linux GOARCH=mips64 go build -ldflags "-w -s" -o $OUTPUT_PATH/$CLIENT_NAME $CLIENT_CODE_FILE
tar -czvf $OUTPUT_PATH/$CLIENT_NAME.linux-mips64.tar.gz -C $OUTPUT_PATH $CLIENT_NAME
rm -f $OUTPUT_PATH/$CLIENT_NAME

CGO_ENABLED=0 GOOS=linux GOARCH=mipsle go build -ldflags "-w -s" -o $OUTPUT_PATH/$CLIENT_NAME $CLIENT_CODE_FILE
tar -czvf $OUTPUT_PATH/$CLIENT_NAME.linux-mipsle.tar.gz -C $OUTPUT_PATH $CLIENT_NAME
rm -f $OUTPUT_PATH/$CLIENT_NAME

CGO_ENABLED=0 GOOS=linux GOARCH=mips go build -ldflags "-w -s" -o $OUTPUT_PATH/$CLIENT_NAME $CLIENT_CODE_FILE
tar -czvf $OUTPUT_PATH/$CLIENT_NAME.linux-mips.tar.gz -C $OUTPUT_PATH $CLIENT_NAME
rm -f $OUTPUT_PATH/$CLIENT_NAME


# server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-w -s" -o $OUTPUT_PATH/$SERVER_NAME $SERVER_CODE_FILE
tar -czvf $OUTPUT_PATH/$SERVER_NAME.linux-amd64.tar.gz -C $OUTPUT_PATH $SERVER_NAME
rm -f $OUTPUT_PATH/$SERVER_NAME

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-w -s" -o $OUTPUT_PATH/$SERVER_NAME.exe $SERVER_CODE_FILE
tar -czvf $OUTPUT_PATH/$SERVER_NAME.windows-amd64.tar.gz -C $OUTPUT_PATH $SERVER_NAME.exe
rm -f $OUTPUT_PATH/$SERVER_NAME.exe

CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-w -s" -o $OUTPUT_PATH/$SERVER_NAME $SERVER_CODE_FILE
tar -czvf $OUTPUT_PATH/$SERVER_NAME.darwin-amd64.tar.gz -C $OUTPUT_PATH $SERVER_NAME
rm -f $OUTPUT_PATH/$SERVER_NAME

# arm
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-w -s" -o $OUTPUT_PATH/$SERVER_NAME $SERVER_CODE_FILE
tar -czvf $OUTPUT_PATH/$SERVER_NAME.linux-arm64.tar.gz -C $OUTPUT_PATH $SERVER_NAME
rm -f $OUTPUT_PATH/$SERVER_NAME

CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "-w -s" -o $OUTPUT_PATH/$SERVER_NAME $SERVER_CODE_FILE
tar -czvf $OUTPUT_PATH/$SERVER_NAME.linux-arm_v7.tar.gz -C $OUTPUT_PATH $SERVER_NAME
rm -f $OUTPUT_PATH/$SERVER_NAME

# mips
CGO_ENABLED=0 GOOS=linux GOARCH=mips64le go build -ldflags "-w -s" -o $OUTPUT_PATH/$SERVER_NAME $SERVER_CODE_FILE
tar -czvf $OUTPUT_PATH/$SERVER_NAME.linux-mips64le.tar.gz -C $OUTPUT_PATH $SERVER_NAME
rm -f $OUTPUT_PATH/$SERVER_NAME

CGO_ENABLED=0 GOOS=linux GOARCH=mips64 go build -ldflags "-w -s" -o $OUTPUT_PATH/$SERVER_NAME $SERVER_CODE_FILE
tar -czvf $OUTPUT_PATH/$SERVER_NAME.linux-mips64.tar.gz -C $OUTPUT_PATH $SERVER_NAME
rm -f $OUTPUT_PATH/$SERVER_NAME

CGO_ENABLED=0 GOOS=linux GOARCH=mipsle go build -ldflags "-w -s" -o $OUTPUT_PATH/$SERVER_NAME $SERVER_CODE_FILE
tar -czvf $OUTPUT_PATH/$SERVER_NAME.linux-mipsle.tar.gz -C $OUTPUT_PATH $SERVER_NAME
rm -f $OUTPUT_PATH/$SERVER_NAME

CGO_ENABLED=0 GOOS=linux GOARCH=mips go build -ldflags "-w -s" -o $OUTPUT_PATH/$SERVER_NAME $SERVER_CODE_FILE
tar -czvf $OUTPUT_PATH/$SERVER_NAME.linux-mips.tar.gz -C $OUTPUT_PATH $SERVER_NAME
rm -f $OUTPUT_PATH/$SERVER_NAME
