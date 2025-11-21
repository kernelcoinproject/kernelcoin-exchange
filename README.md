# kernelcoin-exchange
Lets encourage some trading

## Screenshots

## Basic Setup

Get yourself a cloud hosted vm and connect via ssh

As non root user

1. Setup kernelcoind

```
mkdir -p kernelcoin
cd kernelcoin
wget https://github.com/kernelcoinproject/kernelcoin/releases/download/main/kernelcoin-0.21.4-x86_64-linux-gnu.tar.gz
tar xf kernelcoin-0.21.4-x86_64-linux-gnu.tar.gz
```

```
mkdir -p ~/.kernelcoin
cat > ~/.kernelcoin/kernelcoin.conf << EOF
# enable p2p
listen=1
txindex=1
logtimestamps=1
server=1
rpcuser=mike
rpcpassword=x
rpcport=9332
rpcallowip=127.0.0.1
rpcbind=127.0.0.1
EOF
```

```
./kernelcoind
```
```
./kernelcoin-cli createwallet "main"
```

2. Download and run Electrum-LTC 

```
sudo yum install -y fuse
wget https://electrum-ltc.org/download/electrum-ltc-4.2.2.1-x86_64.AppImage
chmod +x electrum-ltc-4.2.2.1-x86_64.AppImage
./electrum-ltc-4.2.2.1-x86_64.AppImage daemon
```
```
./electrum-ltc-4.2.2.1-x86_64.AppImage getinfo
./electrum-ltc-4.2.2.1-x86_64.AppImage create
```
write the generated wallet down
```
./electrum-ltc-4.2.2.1-x86_64.AppImage load_wallet
./electrum-ltc-4.2.2.1-x86_64.AppImage createnewaddress
```

2. Download and run the kernelcoin exchange

```
sudo yum install -y git golang
cd ~
git clone https://github.com/kernelcoinproject/kernelcoin-exchange.git
cd kernelcoin-exchange
go mod tidy
cat > start.sh << EOF
sleep 5
/home/ec2-user/electrum-ltc-4.2.2.1-x86_64.AppImage load_wallet
go run *.go -electrum-binary=/home/ec2-user/electrum-ltc-4.2.2.1-x86_64.AppImage
EOF
chmod +x start.sh
./start.sh
```


3. Setup caddy to host via https with username and password

As root
```
mkdir -p /opt/caddy
cd /opt/caddy
wget https://github.com/caddyserver/caddy/releases/download/v2.10.2/caddy_2.10.2_linux_amd64.tar.gz
tar xf caddy_2.10.2_linux_amd64.tar.gz
```

```
DOMAIN="website.duckdns.org"

cat > /opt/caddy/Caddyfile << EOF
$DOMAIN {

    header {
        X-Content-Type-Options "nosniff"
        X-Frame-Options "SAMEORIGIN"
        X-XSS-Protection "1; mode=block"
        Referrer-Policy "strict-origin-when-cross-origin"
    }

    encode gzip

    log {
        output file /var/log/caddy/wallet.log {
            roll_size 100mb
            roll_keep 5
        }
        format json
    }

    reverse_proxy 127.0.0.1:8080
}
EOF
/opt/caddy/caddy run
```

4. Run it all at boot via tmux

Run as root user (port 443 requires root)
```
yum install -y tmux cronie
cat > /root/startWeb.sh << EOF
tmux kill-session -t caddy 2>/dev/null
tmux new -s caddy -d
tmux send-keys -t caddy "cd /opt/caddy && ./caddy run" C-m
EOF
chmod +x /root/startWeb.sh
```

Run as root user
```
crontab -e
@reboot /root/startWeb.sh
```

Run as non-root user
```

cat > /home/ec2-user/startup.sh << EOF
tmux kill-session -t kce 2>/dev/null
tmux new -s kce -d
tmux neww -t kce -n kernelcoin
tmux neww -t kce -n electrum
tmux neww -t kce -n kce
tmux send-keys -t kce:kernelcoin "cd /home/ec2-user/kernelcoin && ./kernelcoind" C-m
tmux send-keys -t kce:electrum "cd /home/ec2-user/ && ./electrum-ltc-4.2.2.1-x86_64.AppImage daemon" C-m
tmux send-keys -t kce:kce "cd /home/ec2-user/kernelcoin-exchange && ./start.sh" C-m
EOF
chmod +x /home/ec2-user/startup.sh
```

Run as non-root user
```
crontab -e
@reboot /home/ec2-user/startup.sh
```
