#!/bin/sh

set -e

download_url_master="http://10.10.252.215:8888/artemis-master"
download_url_agent="http://10.10.252.215:8888/artemis-agent"
download_url_master_bak="http://10.10.252.215:8888/artemis-master"
download_url_agent_bak="http://10.10.252.215:8888/artemis-agent"
download_url_lastb="http://10.10.252.215:8888/artemis-lastb"
download_url_lastb_bak="http://10.10.252.215:8888/artemis-lastb"



#curl http://10.10.252.215:8888/install.sh|bash

downloads()
{
    if [ -f "/usr/bin/curl" ]
    then
        http_code=`curl -I -m 10 -o /dev/null -s -w %{http_code} $1`
        if [ "$http_code" -eq "200" ]
        then
            curl --connect-timeout 5 --retry 5 $1 > $2
        elif [ "$http_code" -eq "405" ]
        then
            curl --connect-timeout 5 --retry 5 $1 > $2
        else
            curl --connect-timeout 5 --retry 5 $3 > $2
    fi
    elif [ -f "/usr/bin/wget" ]
    then
        wget --timeout=5 --tries=5 -O $2 $1
        if [ $? -ne 0 ]
        then
                wget --timeout=5 --tries=5 -O $2 $3
    fi

   fi
}



if grep -v '^#' /etc/fstab | grep -q cgroup; then
        echo "cgroups mounted from fstab, not mounting /sys/fs/cgroup"
        exit 0
fi


if [ ! -e /proc/cgroups ]; then
        exit 0
fi


mountpoint -q /sys/fs/cgroup || mount -t tmpfs -o uid=0,gid=0,mode=0755 cgroup /sys/fs/cgroup


for d in `tail -n +2 /proc/cgroups | awk '{
        if ($2 == 0)
                print $1
        else if (a[$2])
                a[$2] = a[$2]","$1
        else
                a[$2]=$1
};END{
        for(i in a) {
                print a[i]
        }
}'`; do
        mkdir -p /sys/fs/cgroup/$d
        mountpoint -q /sys/fs/cgroup/$d || (mount -n -t cgroup -o $d cgroup /sys/fs/cgroup/$d || rmdir /sys/fs/cgroup/$d || true)
done



dir=/sys/fs/cgroup/systemd
if [ ! -d "${dir}" ]; then
        mkdir "${dir}"
        mount -n -t cgroup -o none,name=systemd name=systemd "${dir}" || rmdir "${dir}" || true
fi



agent_dir="/usr/local/artemis"

if [ ! -d $agent_dir  ];then
  mkdir $agent_dir
  downloads $download_url_master /usr/local/artemis/artemis-master $download_url_master_bak
  downloads $download_url_agent /usr/local/artemis/artemis-agent $download_url_agent_bak
  downloads $download_url_lastb /usr/local/artemis/artemis-lastb $download_url_lastb_bak
  chmod +x  /usr/local/artemis/*
  /usr/local/artemis/artemis-master install
  /usr/local/artemis/artemis-master start
else
      if [ -f "/usr/local/artemis/artemis-master" ]
        then
          /usr/local/artemis/artemis-master stop
          /usr/local/artemis/artemis-master remove
          rm -rf  /usr/local/artemis/*
      fi
      downloads $download_url_master /usr/local/artemis/artemis-master $download_url_master_bak
      downloads $download_url_agent /usr/local/artemis/artemis-agent $download_url_agent_bak
      downloads $download_url_lastb /usr/local/artemis/artemis-lastb $download_url_lastb_bak
      chmod +x  /usr/local/artemis/*
      /usr/local/artemis/artemis-master install
      /usr/local/artemis/artemis-master start

fi


echo "Artemis-HIDS Service successfully installed"

exit 0