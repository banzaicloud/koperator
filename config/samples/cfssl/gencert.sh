#!/bin/bash

cfssl gencert -initca cfssl-ca.json | cfssljson -bare ca
cfssl gencert -ca ca.pem -ca-key ca-key.pem -config cfssl-ca.json -profile server cfssl-server-csr.json | cfssljson -bare server
cfssl gencert -ca ca.pem -ca-key ca-key.pem -config cfssl-ca.json -profile server cfssl-server-csr.json | cfssljson -bare peer
cfssl gencert -ca ca.pem -ca-key ca-key.pem -config cfssl-ca.json -profile client cfssl-server-csr.json | cfssljson -bare client
