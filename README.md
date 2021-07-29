# Artemis_HIDS



使用 cgroups + etcd + kafka + netlink-connector 开发而成的hids的架构，agent 部分使用go 开发而成， 会把采集的数据写入到kafka里面，

由后端的规则引擎（go开发而成）消费，配置部分以及agent存活使用etcd。


测试系统：

CentOS Linux release 7.2.1511 

内核: 

3.10.0-1127.19.1.el7.x86_64 



如果不需要修该C代码部分，可以直接编译


如果需要修该C代码部分，需要clang 环境

添加yum源 ： c7-clang-x86_64.repo


```go

[c7-devtoolset-8]
name=c7-devtoolset-8
baseurl=https://buildlogs.centos.org/c7-devtoolset-8.x86_64/
gpgcheck=0
enabled=1
[c7-llvm-toolset-9]
name=c7-llvm-toolset-9
baseurl=https://buildlogs.centos.org/c7-llvm-toolset-9.0.x86_64/
gpgcheck=0
enabled=1

``` 


```go
yum install llvm-toolset-9.0  -y

```

环境变量如下：

```go
 
export PATH=$PATH:/opt/rh/llvm-toolset-9.0/root/bin
export PATH=$PATH:/opt/rh/devtoolset-8/root/bin

```


在 /etc/ld.so.conf 添加如下内容，并 ldconfig：

```go
/opt/rh/llvm-toolset-9.0/root/lib64

```




etcd v3 配置

```go

# etcdctl  role add root    
# etcdctl  user add root      
# etcdctl  user grant-role root root   
# etcdctl  auth enable  

# etcdctl  --user=root:passwd  role add HidsConf
# etcdctl  --user=root:passwd  role grant-permission --prefix=true HidsConf readwrite /hids
# etcdctl  --user=root:passwd  user add hids
# etcdctl  --user=root:passwd  user grant-role hids HidsConf

# etcdctl --user=hids:123456   put  /hids/kafka_conf/kafka_host   172.21.129.2:9092    [kafka对应host,逗号分隔]
# etcdctl --user=hids:123456   put  /hids/kafka_conf/kafka_topic  hids-agent           [kafka对应topic]
# etcdctl --user=hids:123456   put  /hids/kafka_conf/aes_key      BGfKOzWNsACBQiOC     [16位aes加密key]

```

kafka 配置


```go

# kafka-topics.sh --create --zookeeper localhost:2181 --replication-factor 1 --partitions 3 --topic hids-agent

```


kafka 查看工具和kafka写es工具见tools,包含aes解密步骤

