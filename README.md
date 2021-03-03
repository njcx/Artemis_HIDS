# peppa_hids



使用 cgroups + etcd + kafka  开发而成的hids的架构，agent 部分使用go 开发而成， 会把采集的数据写入到kafka里面，

由后端的规则引擎（go开发而成）消费，配置部分以及agent存活使用etcd。