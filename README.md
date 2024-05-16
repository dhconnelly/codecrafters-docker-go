Go solution to the
["Build Your Own Docker" Challenge](https://codecrafters.io/challenges/docker).

Status: [Done](https://app.codecrafters.io/users/dhconnelly)

Usage:

    alias mydocker='docker build -t mydocker . && docker run --cap-add="SYS_ADMIN" mydocker'
    mydocker run alpine cat /etc/alpine-release
