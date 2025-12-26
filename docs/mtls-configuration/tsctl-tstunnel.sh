
tsctl start --listen 127.0.0.1:23750 --log-level debug \
    --ts-server vm1.localhost:8443 --ts-insecure \
    --ts-cert ../../linux-bin/mac.crt --ts-key ../../linux-bin/mac.key