import argparse
import os
import json
import shutil

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


def main():
    # 创建解析器
    parser = argparse.ArgumentParser(description='This script processes command line arguments.')
    # 添加参数
    parser.add_argument('-i', '--index', action='store_true', help='Index all raw files to json')
    parser.add_argument('-r', '--rename', action='store_true', help='Rename source files with name of uuid')

    dataset_id = 1711 # 这里需要修改
    root_dir = '/ipfsdata'
    parent_dir = f'{root_dir}/{dataset_id}'
    source_dir = parent_dir + '/src'
    target_dir = parent_dir + '/raw'
    index_file = f'{dataset_id}.json',

    try:
        # 解析参数
        args = parser.parse_args()
        print(args)

        if args.index:
            # 索引文件并存为json
            index_files_to_json(target_dir, parent_dir, index_file)
        elif args.rename:
            # 重命名为uuid
            rename_all_files(source_dir, target_dir)
    except argparse.ArgumentError as e:
        print(f'Error: {e}')


if __name__ == '__main__':
    main()
