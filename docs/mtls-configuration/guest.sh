
cd ~/
./.mutagen/agents/guest --enable-mtls --allowed-cns 'connector' \
  --server-cert ./.mutagen/agents/server.crt --server-key ./.mutagen/agents/server.key \
   --ca-certs ./.mutagen/agents/ca.pem 