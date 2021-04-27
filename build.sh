go build artemis-agent.go
go build artemis-master.go
upx -9 artemis-agent
upx -9 artemis-master
rm -rf /var/www/html/*
cp artemis-master artemis-agent sh/*  /var/www/html/