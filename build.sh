#/bin/bash
GOOS=linux GOARCH=amd64 go build -o roomCondition
zip -o roomCondition.zip roomCondition
