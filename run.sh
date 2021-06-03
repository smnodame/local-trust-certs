#!/bin/bash

openssl req -x509 -nodes -new -sha256 -days 360 -newkey rsa:2048 -keyout RootCA.key -out RootCA.pem -subj "/C=US/CN=Example-Root-CA"
openssl x509 -outform pem -in RootCA.pem -out RootCA.crt

cat > domains.ext << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names
[alt_names]
EOF

i=1;
for url in "$@" 
do
    echo "DNS.${i} = ${url}" >> domains.ext
    i=$((i + 1));
    shift 1;
done

openssl req -new -nodes -newkey rsa:2048 -keyout localhost.key -out localhost.csr -subj "/C=US/ST=YourState/L=YourCity/O=Example-Certificates/CN=localhost.local"
openssl x509 -req -sha256 -days 360 -in localhost.csr -CA RootCA.pem -CAkey RootCA.key -CAcreateserial -extfile domains.ext -out localhost.crt

if [ "$(uname)" == "Darwin" ]; then
    sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ./RootCA.pem
elif [ "$(expr substr $(uname -s) 1 5)" == "Linux" ]; then
    sudo mv RootCA.pem /usr/local/share/ca-certificates/
    sudo update-ca-certificates
fi

sudo rm localhost.csr RootCA.srl

echo "Now you can serve your local server with localhost.crt and localhost.key"