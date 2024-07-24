cd $(dirname "$0")

rm *.pem

# 1. Generate CA's private key and self-signed certificate
openssl req -x509 -newkey rsa:2048 -days 180 -nodes -keyout ca-key.pem -out ca-cert.pem -subj "/C=AM/ST=Yerevan/L=Yerevan/O=Tech School/OU=Education/CN=neone.am,*.neone.am/emailAddress=off@neone.a"

echo "CA's self-signed certificate"
openssl x509 -in ca-cert.pem -noout -text

# 2. Generate web server's private key and certificate signing request (CSR)
openssl req -newkey rsa:2048 -nodes -keyout server-key.pem -out server-req.pem -subj "/C=AM/ST=Yerevan/L=Yerevan/O=Tech School/OU=Education/CN=neone.am,*.neone.am/emailAddress=off@neone.am"

# 3. Use CA's private key to sign web server's CSR and get back the signed certificate
openssl x509 -req -in server-req.pem -days 60 -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -extfile server-ext.cnf

echo "Server's signed certificate"
openssl x509 -in server-cert.pem -noout -text