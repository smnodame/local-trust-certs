# Local Trust Certificates
This repo folks from mkcert repo [link](https://github.com/FiloSottile/mkcert)

## How to build
```
cd mkcert && go build -ldflags "-X main.Version=1.0.0"
```

## How to run
```
> cd mkcert
> ./mkcert -install
> ./mkcert -compose=../docker-compose.yml -caddy=../Caddyfile -service=https example.com v1.example.com v2.example.com 
```