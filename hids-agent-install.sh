#!/bin/sh

set -e

# For simplicity this script provides no flexibility

# If cgroup is mounted by fstab, don't run
# Don't get too smart - bail on any uncommented entry with 'cgroup' in it
if grep -v '^#' /etc/fstab | grep -q cgroup; then
	echo "cgroups mounted from fstab, not mounting /sys/fs/cgroup"
	exit 0
fi

# kernel provides cgroups?
if [ ! -e /proc/cgroups ]; then
	exit 0
fi

# Mount /sys/fs/cgroup if not already done
mountpoint -q /sys/fs/cgroup || mount -t tmpfs -o uid=0,gid=0,mode=0755 cgroup /sys/fs/cgroup

# get list of cgroup kernel controllers
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

# Find any named controllers which already exist
# If we do this we have to make sure to run after proc is mounted
#for d in `sed -e '/name=/!d;s/^.*name=\([^:]*\).*/\1/' /proc/self/cgroup`; do
#	mkdir -p "/sys/fs/cgroup/${d}"
#	mountpoint -q /sys/fs/cgroup/"${d}" || (mount -n -t cgroup -o none,name="${d}" name="${d}" "/sys/fs/cgroup/${d}" || rmdir "/sys/fs/cgroup/${d}" || true)
#done

# Always mount name=systemd
dir=/sys/fs/cgroup/systemd
if [ ! -d "${dir}" ]; then
	mkdir "${dir}"
	mount -n -t cgroup -o none,name=systemd name=systemd "${dir}" || rmdir "${dir}" || true
fi


exit 0