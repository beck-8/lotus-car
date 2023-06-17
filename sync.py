import os
import subprocess
import time

source_path = '/ipfsdata/car'
dest_path = '/datacap'
prefix = 'baga6ea4seaq'

def move_files(source_file, dest_file):
    subprocess.run(['mv', source_file, dest_file])


def process_files():
    for root, dirs, files in os.walk(source_path):
        for file in files:
            if file.startswith(prefix) and file.endswith('.car'):
                source_file = os.path.join(root, file)
                dest_file = os.path.join(dest_path, os.path.relpath(source_file, source_path))
                print(f'移动文件: {source_file} -> {dest_file}')
                move_files(source_file, dest_file)

# 先处理源路径下已经存在的符合条件的文件
process_files()

# 无限循环，监听是否有新的符合条件的文件产生
while True:
    process_files()
    print('Sleeping 60s...')
    time.sleep(60)  # 每隔60秒检查一次是否有新的文件