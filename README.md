# Local Trust Certificates
This repo folks from mkcert repo [link](https://github.com/FiloSottile/mkcert)

## How to build
```
go build ./mkcert/main.go -o generate-certs
```

## How to run
```
> sudo ./generate-certs example.com v1.example.com v2.example.com -compose=./docker-compose.yml -caddy=./Caddyfile -service=https
```