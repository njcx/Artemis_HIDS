# peppa_hids



使用 cgroups + etcd + kafka + eBPF 开发而成的hids的架构，agent 部分使用go 开发而成， 会把采集的数据写入到kafka里面，

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


