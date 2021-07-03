# Local Trust Certificates
This repo folks from mkcert repo [link](https://github.com/FiloSottile/mkcert)

## How to build
```
cd mkcert && go build -o generate-certs
```

## How to run
```
> cd ./mkcert sudo ./generate-certs example.com v1.example.com v2.example.com -compose=../docker-compose.yml -caddy=../Caddyfile -service=https
```