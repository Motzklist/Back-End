FROM ubuntu:latest
LABEL authors="avner"

ENTRYPOINT ["top", "-b"]