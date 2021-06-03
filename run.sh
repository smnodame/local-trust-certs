#!/bin/bash

openssl req -x509 -nodes -new -sha256 -days 999999 -newkey rsa:2048 -keyout RootCA.key -out RootCA.pem -subj "/C=US/CN=Example-Root-CA"
openssl x509 -outform pem -in RootCA.pem -out RootCA.crt

cat > domains.ext << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names
[alt_names]

DNS.1 = doszy-dev.com
DNS.2 = app.doszy-dev.com
DNS.3 = api.doszy-dev.com
DNS.4 = initial-app.doszy-dev.com
DNS.5 = initial-app2.doszy-dev.com
DNS.6 = app-3rd.doszy-dev.com
DNS.7 = flex-simulator.doszy-dev.com
EOF

openssl req -new -nodes -newkey rsa:2048 -keyout localhost.key -out localhost.csr -subj "/C=US/ST=YourState/L=YourCity/O=Example-Certificates/CN=localhost.local"
openssl x509 -req -sha256 -days 1024 -in localhost.csr -CA RootCA.pem -CAkey RootCA.key -CAcreateserial -extfile domains.ext -out localhost.crt

if [ "$(uname)" == "Darwin" ]; then
    sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ./RootCA.pem
elif [ "$(expr substr $(uname -s) 1 5)" == "Linux" ]; then
    sudo mv RootCA.pem /usr/local/share/ca-certificates/
    sudo update-ca-certificates
fi

sudo rm localhost.csr RootCA.srl

echo "Now, you should be able to serve your local server with localhost.crt and localhost.key"