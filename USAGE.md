# Filecoin Datacap操作流程

## 1. 打包car文件
#### 编译打包工具
先编译打包工具，需要安装Go 1.20+。
```sh
$ git clone https://github.com/minerdao/lotus-car.git
$ cd lotus-car
$ make
```

#### 打包car文件
安装完毕后，执行以下命令打包car文件：
```sh
./lotus-car generate --input=/mnt/md0/1000/1000.json --parent=/mnt/md0/1000/raw --tmp-dir=/mnt/md0/tmp1 --quantity=320 --out-dir=/mnt/md0/car/dataset_1000_1_320  --out-file=/home/fil/csv/dataset_1000_1_320.csv
```
参数说明：
- **--input**：原始文件对应的索引文件路径，`.json`格式，通过[lotus-car](https://github.com/minerdao/lotus-car.git)仓库中`python3 main.py -i`来生成。  
  ⚠️ 注意生成索引文件时，需先修改`main.py`中的数据集根目录和数据集名称。
- **--parent**：原始文件所在目录，通常存放在`raw`目录下。
- **--tmp-dir**：打包过程中的临时文件路径，需放在SSD上。
- **--quantity**：一次打包的car文件数量(320个 = 10TiB)。
- **--out-dir**：car文件的保存位置。
- **--out-file**：打包完car文件后，输出的csv文件名称及路径，此文件用于Client发布存储订单。

一般情况下，SSD上的文件目录按照以下结构进行组织：
```sh
/mnt/md0/   # 根目录
    |- 1000 # 数据集对应的ID
        |- raw  # 原始文件
        |- car  # car文件
        |- tmp1 # 临时文件（2个进程各创建一个临时目录）
        |- tmp2
    |- 1001 # 数据集对应的ID
        |- raw
        |- car
        |- tmp1
        |- tmp2
    ...
```

一台4块以上nVME U.2组成的Raid0的机器，可启动2个进程同时打包。  
每天大约可打包**15TiB**的数据，每个car文件(18GiB)打包时间约5～6分钟。




