import argparse
import os
import json
import shutil
import subprocess
import time

def list_all_files(rootdir):
    _files = []
    listroot = os.listdir(rootdir) # 列出文件夹下的所有目录和文件
    for i in range(0,len(listroot)):
        path = os.path.join(rootdir, listroot[i])
        if os.path.isdir(path):
            _files.extend(list_all_files(path))
        if os.path.isfile(path):
            _files.append(path)

    return _files


def rename_all_files(raw_dir, target_dir):
    all_files = list_all_files(raw_dir)
    for file in all_files:
        file_name = os.path.split(file)[-1]
        if file_name != '.DS_Store':
            new_file = target_dir + '/' + file_name
            shutil.move(file, new_file)
            print('Move', file, '========>', new_file)


def index_files_to_json(source_dir, output_dir, file_name):
    all_files = list_all_files(source_dir)
    json_arr = []
    for file in all_files:
        file_size = os.path.getsize(file)
        (file_path, ext_name) = os.path.splitext(file)
        print(file, file_size, ext_name)
        file_info = {
            'Path': file,
            'Size': file_size,
        }
        json_arr.append(file_info)

    output_file = output_dir + '/' + file_name
    with open(output_file, 'a', encoding='utf-8') as f:
        json.dump(json_arr, f)
        print('Index file '+ output_file +' generated')


def move_files(source_file, dest_file):
    subprocess.run(['mv', source_file, dest_file])


def process_files(source_path, dest_path, prefix='baga6ea4seaq'):
    for root, dirs, files in os.walk(source_path):
        for file in files:
            if file.startswith(prefix) and file.endswith('.car'):
                source_file = os.path.join(root, file)
                dest_file = os.path.join(dest_path, os.path.relpath(source_file, source_path))
                print(f'移动文件: {source_file} -> {dest_file}')
                move_files(source_file, dest_file)


def monitor_and_move_files(source_path, dest_path):
    # 先处理源路径下已经存在的符合条件的文件
    process_files(source_path, dest_path)

    # 无限循环，监听是否有新的符合条件的文件产生
    while True:
        process_files(source_path, dest_path)
        print('Sleeping 60s and recheck...')
        time.sleep(60)  # 每隔60秒检查一次是否有新的文件



def main():
    # 创建解析器
    parser = argparse.ArgumentParser(description='This script processes command line arguments.')
    # 添加参数
    parser.add_argument('-i', '--index', action='store_true', help='Index all raw files to json')
    parser.add_argument('-r', '--rename', action='store_true', help='Moving source files with name of uuid')
    parser.add_argument('-s', '--sync', action='store_true', help='Monitor and sync all car files to dest storage')

    # 按照这样的目录结构
    # /mnt/md0
    #   |- 1711
    #       |- 1711.json
    #       |- raw
    #   |- 1712
    #       |- 1711.json
    #       |- raw

    root_dir = '/mnt/md0' # 这里需要修改为SSD下存放数据集的根目录
    dataset_id = 1711 # 这里需要修改为数据集的ID
    car_temp_path = f'{root_dir}/car' # 这里修改为car文件临时存放的路径
    car_dest_path = '/datacap' # 这里修改为car文件最终存放的路径（fcfs/raid）

    parent_dir = f'{root_dir}/{dataset_id}'
    source_dir = parent_dir + '/src'
    target_dir = parent_dir + '/raw'    
    index_file = f'{dataset_id}.json'

    try:
        # 解析参数
        args = parser.parse_args()

        if args.index:
            # 索引文件并存为json
            index_files_to_json(target_dir, parent_dir, index_file)
        elif args.rename:
            # 重命名为uuid
            rename_all_files(source_dir, target_dir)
        elif args.sync:
            # 监听car文件并同步到最终的存储目录下
            monitor_and_move_files(car_temp_path, car_dest_path)
    except argparse.ArgumentError as e:
        print(f'Error: {e}')


if __name__ == '__main__':
    main()
