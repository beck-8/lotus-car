#!/bin/bash

csvpath=$HOME/csv/$1.csv
carpath=/datacap/$1 # 注意修改这里为car文件的存储目录
deals=$(cat $csvpath)
total=$(cat $csvpath | wc -l)
sleep_seconds=150

echo "Import dataset: ${1} from ${carpath} | ${csvpath}"
echo "Total ${total} files"

# 导入订单之前，同步一下fcfs
sudo $HOME/kodo_datacap/fcfs.sh sync
echo "Datacap synced"

for line in $deals
do
    OLD_IFS="$IFS"
    IFS=","
    arr=($line)
    IFS="$OLD_IFS"
    proposalCID="${arr[1]}"
    file="${carpath}"/"${arr[0]}"
    # echo $line
    echo "Importing ${proposalCID} ${file}"

    boostd import-data $proposalCID $file
    echo "Sleeping ${sleep_seconds} seconds.........."
    sleep $sleep_seconds
done