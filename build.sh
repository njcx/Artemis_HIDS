go build p-agent.go
go build p-master.go
upx -9 p-agent
upx -9 p-master
rm -rf /var/www/html/*
cp p-master p-agent sh/*  /var/www/html/