go build p-agent.go
go build p-master.go
upx -9 p-agent
upx -9 p-master
rm -rf /var/www/html/p-*
rm -rf /var/www/html/install.sh
cp p-master p-agent sh/install.sh  /var/www/html/